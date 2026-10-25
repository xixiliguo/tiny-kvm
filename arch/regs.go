package arch

import (
	"fmt"
	"unsafe"

	"github.com/xixiliguo/tiny-kvm/hyper"
)

// Constants mimicking the guest memory layout and control registers.
const (
	BootGdtMax = 4
	Cr0Pe      = 0x1 // Protected Mode Enable bit
)

// type Segment = kvm.KvmSegment
// type DescriptorTable = kvm.KvmDtable

// ConfigureSegmentsAndSregs configures the GDT, IDT, and segment selectors
// required to initialize the vCPU for the PVH boot protocol.
func ConfigureSegmentsAndSregs(
	mem GuestMemoryMmap,
	sregs *hyper.SpecialRegisters,
	enableX2ApicMode bool,
) error {
	// Configure GDT entries as explicitly defined by the PVH boot protocol specification
	gdtTable := [BootGdtMax]uint64{
		GdtEntry(0, 0, 0),               // NULL Descriptor
		GdtEntry(0xc09b, 0, 0xffffffff), // CODE Segment Descriptor
		GdtEntry(0xc093, 0, 0xffffffff), // DATA Segment Descriptor
		GdtEntry(0x008b, 0, 0x67),       // TSS (Task State Segment) Descriptor
	}

	codeSeg := SegmentFromGdt(gdtTable[1], 1)
	dataSeg := SegmentFromGdt(gdtTable[2], 2)
	tssSeg := SegmentFromGdt(gdtTable[3], 3)

	// Serialize and write the GDT table to guest memory space
	if err := writeGdtTable(gdtTable[:], mem); err != nil {
		return fmt.Errorf("failed to write GDT table to guest memory: %w", err)
	}
	sregs.Gdt.Base = BootGDTStart
	sregs.Gdt.Limit = uint16(unsafe.Sizeof(gdtTable)) - 1

	// Zero out and prepare the IDT value in guest memory space
	if err := writeIdtValue(0, mem); err != nil {
		return fmt.Errorf("failed to write IDT value to guest memory: %w", err)
	}
	sregs.Idt.Base = BootIDTStart
	sregs.Idt.Limit = uint16(unsafe.Sizeof(uint64(0))) - 1

	// Bind initialized architectural segment states to sregs selectors
	sregs.Cs = codeSeg
	sregs.Ds = dataSeg
	sregs.Es = dataSeg
	sregs.Fs = dataSeg
	sregs.Gs = dataSeg
	sregs.Ss = dataSeg
	sregs.Tr = tssSeg

	// Set Control Registers (CR0 Protected Mode enabled, CR4 cleared)
	sregs.Cr0 = Cr0Pe
	sregs.Cr4 = 0

	// Enable x2APIC architectural feature bit if explicitly requested
	if enableX2ApicMode {
		const x2ApicEnableBit uint64 = 1 << 10
		sregs.Apic_base |= x2ApicEnableBit
	}

	return nil
}

// Internal serialization logic writing table slices into MMAP abstraction bindings.
func writeGdtTable(table []uint64, mem GuestMemoryMmap) error {
	// Calculate size: elements * 8 bytes per uint64
	byteLen := len(table) * 8

	// Create a zero-allocation byte slice overlay over the uint64 slice data.
	// This maps exactly to the behavior of casting a reference slice via compiler internals.
	var byteSlice []byte
	if len(table) > 0 {
		byteSlice = unsafe.Slice((*byte)(unsafe.Pointer(&table[0])), byteLen)
	}

	_, err := mem.WriteSlice(byteSlice, BootGDTStart)
	return err
}

func writeIdtValue(val uint64, mem GuestMemoryMmap) error {
	byteSlice := unsafe.Slice((*byte)(unsafe.Pointer(&val)), 8)
	_, err := mem.WriteSlice(byteSlice, BootIDTStart)
	return err
}
