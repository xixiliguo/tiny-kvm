package arch

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"unsafe"

	"github.com/xixiliguo/tiny-kvm/hyper"
)

// BootSetup encapsulates the optional entry point and atomic memory context wrapper.
type BootSetup struct {
	KernelEntryPoint EntryPoint
	GuestMemory      GuestMemoryMmap
}

func ConfigureVcpu(
	vcpu hyper.Vcpu,
	id uint32,
	bootSetup *BootSetup,
	cpuid []hyper.CpuIdEntry,
	kvmHyperv bool,
	cpuVendor hyper.CPUVendor,
) error {
	x2apicId := id

	// Per vCPU CPUID changes; common are handled via GenerateCommonCpuid()
	// CPUID leaves 0xb and 0x1f report the x2APIC ID in EDX. ECX is the
	// sub-leaf selector, so writing the ID there gives every vCPU the same
	// topology when SMP is enabled.
	hyper.SetCpuidReg(&cpuid, 0xb, 0, hyper.CpuidRegEDX, x2apicId)
	hyper.SetCpuidReg(&cpuid, 0x1f, 0, hyper.CpuidRegEDX, x2apicId)

	// Set ApicId in cpuid for each vcpu - found in cpuid ebx when eax = 1
	apicIdPatched := false
	for i := range cpuid {
		entry := &cpuid[i]
		if entry.Function == 1 {
			entry.Ebx &= 0xffffff
			entry.Ebx |= x2apicId << 24
			apicIdPatched = true

			// Disable nested virtualization for Intel (VMX)
			entry.Ecx &^= (1 << hyper.VmxEcxBit)
			break
		}
		if entry.Function == 0x8000_0001 {

			// Disable nested virtualization for AMD (SVM)
			entry.Ecx &^= (1 << hyper.SvmEcxBit)

			break
		}
	}

	if !apicIdPatched {
		panic("assertion failed: apic_id_patched was false")
	}

	for _, c := range cpuid {
		slog.Debug("CPUID entry trace", "entry", c)
	}

	if err := vcpu.SetCpuid2(cpuid); err != nil {
		return fmt.Errorf("set supported cpus failed: %w", err)
	}

	// External platform configuration bindings (e.g., system registers, interrupts)
	if err := vcpu.SetMsrs(vcpu.BootMsrEntries()); err != nil {
		return fmt.Errorf("msrs configuration failed: %w", err)
	}

	if bootSetup != nil {
		if err := SetupRegs(vcpu, bootSetup.KernelEntryPoint); err != nil {
			return fmt.Errorf("regs configuration failed: %w", err)
		}

		fpu := hyper.FpuState{
			Fcw:   0x37f,
			Mxcsr: 0x1f80,
		}

		if err := vcpu.SetFpu(&fpu); err != nil {
			return err
		}
		sregs, err := vcpu.GetSregs()
		if err != nil {
			return err
		}

		if err := ConfigureSegmentsAndSregs(bootSetup.GuestMemory, &sregs, true); err != nil {
			return err
		}

		if err := vcpu.SetSregs(&sregs); err != nil {
			return fmt.Errorf("sregs configuration failed: %w", err)
		}
	}

	if err := hyper.SetLint(vcpu); err != nil {
		return fmt.Errorf("local int configuration failed: %w", err)
	}

	return nil
}

// SetupRegs configures base registers for a given CPU.
func SetupRegs(vcpu hyper.Vcpu, entryPoint EntryPoint) error {
	regs := vcpu.CreateStandardRegs()

	if entryPoint.SetupHeader == nil {
		regs.Rflags = 0x0000000000000002
		regs.Rip = uint64(entryPoint.EntryAddr)
		regs.Rbx = PvhInfoStart
	} else {
		regs.Rflags = 0x0000000000000002
		regs.Rip = uint64(entryPoint.EntryAddr)
		regs.Rsp = BootStackPointer
		regs.Rsi = ZeroPageStart
	}

	return vcpu.SetRegs(&regs)
}

// InitramfsConfig holds details for the initial ramdisk layout.
type InitramfsConfig struct {
	Address uint64
	Size    uint64
}

// ConfigureSystem initializes global firmware structures (EBDA, SMBIOS, MP Table)
// and configures the early system boot parameters for either PVH or standard 32-bit entry.
func ConfigureSystem(
	guestMem GuestMemoryMmap,
	cmdlineAddr uint64,
	cmdlineSize int,
	initramfs *InitramfsConfig, // nil represents None
	numCpus uint32,
	setupHeader *SetupHeader, // nil represents None
	rsdpAddr uint64, // nil represents None
	serialNumber string, // nil represents None
	uuid string, // nil represents None
	oemStrings []string, // Empty/nil slice represents None
) error {
	// Write EBDA address to the exact location where ACPICA expects to discover it.
	// We convert the full address into a 16-bit segment address (shifted right by 4).
	ebdaSegment := uint16(EbdaStart >> 4)
	if _, err := guestMem.WriteObj(ebdaSegment, EbdaPointer); err != nil {
		return fmt.Errorf("EBDA setup failed: %w", err)
	}

	// Set up the SMBIOS tables in guest memory space.
	_, err := SetupSmbios(guestMem, serialNumber, uuid, oemStrings)
	if err != nil {
		return fmt.Errorf("SMBIOS setup failed: %w", err)
	}

	// Validate that the physical RAM bounds are not smaller than the assigned RSDP address.

	if rsdpAddr > guestMem.LastAddr() {
		return fmt.Errorf("RSDP past RAM end error")
	}

	// Select the boot path configuration sequence based on whether a Linux setup_header is present.
	if setupHeader != nil {
		if err := configure32BitEntry(guestMem, cmdlineAddr, cmdlineSize, initramfs, *setupHeader, rsdpAddr); err != nil {
			return err
		}
	} else {
		if err := configurePvh(guestMem, cmdlineAddr, initramfs, rsdpAddr); err != nil {
			return err
		}
	}

	return nil
}

// Configure32BitEntry constructs the traditional 32-bit Linux boot zero page (boot_params)
// and dynamically populates the E820 map entries according to the physical layout of the guest memory.
func configure32BitEntry(
	guestMem GuestMemoryMmap,
	cmdlineAddr uint64,
	cmdlineSize int,
	initramfs *InitramfsConfig,
	setupHdr SetupHeader,
	rsdpAddr uint64,
) error {
	const KernelLoaderOther uint8 = 0xff

	// Initialize a fresh boot_params zero page using the provided setup header template
	params := BootParams{
		Hdr: setupHdr,
	}

	// Apply canonical corrections according to the x86 Linux Boot Protocol
	if params.Hdr.TypeOfLoader == 0 {
		params.Hdr.TypeOfLoader = KernelLoaderOther
	}
	params.Hdr.CmdLinePtr = uint32(cmdlineAddr)
	params.Hdr.CmdlineSize = uint32(cmdlineSize)

	// Configure initial ramdisk (initramfs) bounds if assigned
	if initramfs != nil {
		params.Hdr.RamdiskImage = uint32(initramfs.Address)
		params.Hdr.RamdiskSize = uint32(initramfs.Size)
	}

	// Add the first base E820 entry spanning from address 0 up to the Extended BIOS Data Area (EBDA)
	if err := addE820Entry(&params, 0, EbdaStart, uint32(E820Ram)); err != nil {
		return err
	}

	memEnd := guestMem.LastAddr()

	// Handle standard x86 memory holes (PCI Hole / MMIO Window allocation between 3GB and 4GB)
	if memEnd < Mem32BitReservedStart {
		// Memory capacity sits entirely below 3GB, map continuously above the 1MB high ram boundary
		size := uint64(memEnd-HighRamStart) + 1
		if err := addE820Entry(&params, HighRamStart, size, uint32(E820Ram)); err != nil {
			return err
		}
	} else {
		// Memory crosses the 3GB line, requiring truncation around the reservation gap
		// First segment maps everything available between 1MB and the 3GB reservation edge
		size := uint64(Mem32BitReservedStart - HighRamStart)
		if err := addE820Entry(&params, HighRamStart, size, uint32(E820Ram)); err != nil {
			return err
		}

		// If physical memory extends into the 64-bit address window, append the remaining high ram above 4GB
		if memEnd > Ram64BitStart {
			highSize := uint64(memEnd-Ram64BitStart) + 1
			if err := addE820Entry(&params, Ram64BitStart, highSize, uint32(E820Ram)); err != nil {
				return err
			}
		}
	}

	// Register PCI MMCONFIG (Enhanced Configuration Access Mechanism, ECAM) range as reserved memory
	if err := addE820Entry(&params, PciMmconfigStart, PciMmconfigSize, uint32(E820Reserved)); err != nil {
		return err
	}

	// If an ACPI RSDP descriptor address is active, register it into the system parameter block
	params.AcpiRsdpAddr = rsdpAddr

	// Assert that guest memory ranges can successfully fit the 4096-byte hardware zero page configuration
	if ZeroPageStart+ZeroPageSize > guestMem.LastAddr() {
		return fmt.Errorf("zero page allocation falls past the end of guest RAM")
	}

	if _, err := guestMem.WriteSlice(params.Bytes(), ZeroPageStart); err != nil {
		return fmt.Errorf("zero page write layout failed: %w", err)
	}

	return nil
}

// addE820Entry appends a new physical memory mapping region onto the end of the e820 table inside boot_params.
func addE820Entry(
	params *BootParams,
	addr uint64,
	size uint64,
	memType uint32,
) error {
	// Guard against array bound mutations to eliminate buffer overflow scenarios
	if params.E820Entries >= uint8(len(params.E820Table)) {
		return fmt.Errorf("e820 configuration failed: no space left in descriptor map")
	}

	idx := params.E820Entries
	params.E820Table[idx].Addr = addr
	params.E820Table[idx].Size = size
	params.E820Table[idx].Type = memType
	params.E820Entries++

	return nil
}

// HvmStartInfo represents the canonical 'hvm_start_info' block required
// by the Xen/PVH Boot Option Specification.
type HvmStartInfo struct {
	Magic        uint32 // Contains the magic value 0x336ec578 ("xEn3" with the 0x80 bit of the "E" set).
	Version      uint32 // Version of this structure.
	Flags        uint32 // SIF_xxx flags.
	NrModules    uint32 // Number of modules passed to the kernel.
	ModlistPaddr uint64 // Physical address of an array of hvm_modlist_entry.
	CmdlinePaddr uint64 // Physical address of the command line.
	RsdpPaddr    uint64 // Physical address of the RSDP ACPI data structure.

	// All following fields only present in version 1 and newer
	MemmapPaddr   uint64 // Physical address of an array of hvm_memmap_table_entry.
	MemmapEntries uint32 // Number of entries in the memmap table. Value is zero if no memory map is provided.
	Reserved      uint32 // Must be zero.
}

// HvmModlistEntry maps exactly to the 'hvm_modlist_entry' structure from the
// Xen/PVH Boot Option Specification, typically used to describe assets like an initramfs.
type HvmModlistEntry struct {
	Paddr        uint64 // Physical address of the module.
	Size         uint64 // Size of the module in bytes.
	CmdlinePaddr uint64 // Physical address of the command line.
	Reserved     uint64 // Reserved for alignment/future use.
}

// HvmMemmapTableEntry maps exactly to the 'hvm_memmap_table_entry' structure,
// which explicitly provisions individual physical memory segments to the guest kernel.
type HvmMemmapTableEntry struct {
	Addr     uint64 // Base address of the memory region.
	Size     uint64 // Size of the memory region in bytes.
	Type     uint32 // Mapping type (e.g., E820Ram, E820Reserved).
	Reserved uint32 // Must be zero for Version 1.
}

// RamRange represents an isolated [Start, End) physical RAM slice window.
type RamRange struct {
	Start uint64
	End   uint64
}

// ConfigurePvh orchestrates the layout generation sequence required to boot a
// guest environment using the PVH boot protocol specification.
func configurePvh(
	guestMem GuestMemoryMmap,
	cmdlineAddr uint64,
	initramfs *InitramfsConfig,
	rsdpAddr uint64,
) error {
	// PVH Boot Protocol Constants
	const XenHvmStartMagicValue uint32 = 0x336ec578
	// Initialize the structural container matching the core layout protocol
	startInfo := HvmStartInfo{
		Magic:        XenHvmStartMagicValue,
		Version:      1, // Version 1 is strictly required by the PVH target specifications
		CmdlinePaddr: cmdlineAddr,
		MemmapPaddr:  MemmapStart,
	}

	startInfo.RsdpPaddr = rsdpAddr

	if initramfs != nil {
		// The initramfs image has already been pushed into guest physical memory.
		// Construct the descriptor block detailing its physical memory dimensions.
		ramdiskMod := HvmModlistEntry{
			Paddr: initramfs.Address,
			Size:  initramfs.Size,
		}

		startInfo.NrModules += 1
		startInfo.ModlistPaddr = ModlistStart

		// High-performance serialize of modlist descriptors into guest memory
		modBuf := make([]byte, 32) // sizeof(struct hvm_modlist_entry) = 32
		binary.LittleEndian.PutUint64(modBuf[0:8], ramdiskMod.Paddr)
		binary.LittleEndian.PutUint64(modBuf[8:16], ramdiskMod.Size)
		binary.LittleEndian.PutUint64(modBuf[16:24], ramdiskMod.CmdlinePaddr)
		binary.LittleEndian.PutUint64(modBuf[24:32], ramdiskMod.Reserved)

		if _, err := guestMem.WriteSlice(modBuf, ModlistStart); err != nil {
			return fmt.Errorf("failed to setup PVH modlist entry: %w", err)
		}
	}

	// Slice tracking the generated memory block descriptions to record onto guest spaces
	var memmap []HvmMemmapTableEntry

	// Pin the initial memory block spanning from zero up to the EBDA boundary

	memmap = append(memmap, HvmMemmapTableEntry{
		Addr: 0,
		Size: EbdaStart,
		Type: uint32(E820Ram),
	})

	// Fetch valid physical RAM ranges from the overarching system controller
	ramRanges, err := guestMem.GetRamRange()
	if err != nil {
		return err
	}

	// Process and adapt raw system mappings into PVH layout targets
	for _, ramRange := range ramRanges {
		// Log matching entries via a standard VMM tracking engine layer wrapper if required.
		// e.g., log.Printf("PVH: map entry: start: 0x%08x, end: 0x%08x", ramRange.Start, ramRange.End)
		memmap = append(memmap, HvmMemmapTableEntry{
			Addr: ramRange.Start,
			Size: ramRange.End - ramRange.Start,
			Type: uint32(E820Ram),
		})
	}

	memmap = append(memmap, HvmMemmapTableEntry{
		Addr: PciMmconfigStart,
		Size: PciMmconfigSize,
		Type: uint32(E820Reserved),
	})

	startInfo.MemmapEntries = uint32(len(memmap))

	// Verify that the memory map table allocation does not overshoot physical RAM limits
	memmapEntrySize := uint64(unsafe.Sizeof(HvmMemmapTableEntry{})) // 24 bytes
	totalMemmapSize := memmapEntrySize * uint64(startInfo.MemmapEntries)

	if MemmapStart+totalMemmapSize > guestMem.LastAddr() {
		return fmt.Errorf("memmap table allocation exceeds bounds of guest RAM")
	}

	// Linearly pack and commit each generated memory map entry to the guest address range
	memmapStartAddr := MemmapStart

	for _, memmapEntry := range memmap {
		if _, err := guestMem.WriteObj(memmapEntry, memmapStartAddr); err != nil {
			return fmt.Errorf("failed to write memmap entry to guest memory: %w", err)
		}
		memmapStartAddr += memmapEntrySize
	}

	// Enforce memory sizing verification thresholds on the baseline start_info layout block
	startInfoSize := uint64(unsafe.Sizeof(HvmStartInfo{})) // 56 bytes
	if PvhInfoStart+startInfoSize > guestMem.LastAddr() {
		return fmt.Errorf("start_info struct falls past the end of guest RAM")
	}

	if _, err := guestMem.WriteObj(startInfo, PvhInfoStart); err != nil {
		return fmt.Errorf("failed to commit start_info context structure: %w", err)
	}

	return nil
}

// InitramfsLoadAddr computes the top-down allocation address for the initramfs image
// inside the first guest physical memory region, ensuring correct hardware page alignment.
func InitramfsLoadAddr(guestMem GuestMemoryMmap, initramfsSize int) (uint64, error) {
	// Find the first memory region starting at guest physical address 0
	firstRegion, found := guestMem.FindRamRange(0)
	if !found {
		return 0, errors.New("Failed to compute initramfs address")
	}

	lowMemSize := int(firstRegion.End - firstRegion.Start)

	// Guard against scenarios where the initramfs payload size exceeds the entire low memory pool region
	if lowMemSize < initramfsSize {
		return 0, errors.New("Failed to compute initramfs address")
	}

	// Fetch system page size (typically 4096 bytes / 4KiB on standard x86_64)
	pageSize := os.Getpagesize()

	// Calculate top-down physical placement position:
	// 1. Subtract initramfs size from total region size to place it at the absolute top of low memory.
	// 2. Perform bitwise AND with the mask matching inverse page size steps to drop down to the nearest page boundary.
	alignedAddr := uint64((lowMemSize - initramfsSize) &^ (pageSize - 1))

	return alignedAddr, nil
}
