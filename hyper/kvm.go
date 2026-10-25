package hyper

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	kvmDevName = "/dev/kvm"
)

var (
	_ Hypervisor = &KvmHyper{}
	_ Vm         = &KvmVm{}
	_ Vcpu       = &KvmVcpu{}
)

//go:generate go tool stringer -output kvm_string.go -type=KvmCap,KvmExitReason

type KvmHyper struct {
	kvmFile *os.File
}

func NewKvmHyper() (*KvmHyper, error) {

	kvmFile, err := os.Open(kvmDevName)
	if err != nil {
		return nil, err
	}
	return &KvmHyper{
		kvmFile: kvmFile,
	}, nil
}

func (h *KvmHyper) HypervisorType() HypervisorType {
	return HypervisorTypeKVM
}

func (h *KvmHyper) CreateVM(memSize uint64) (Vm, error) {

	vmFd, err := ioctl(h.kvmFile.Fd(), KVM_CREATE_VM)
	if err != nil {
		return nil, err
	}
	vm := &KvmVm{
		hyper:  h,
		vmFile: os.NewFile(uintptr(vmFd), "VM"),
	}
	return vm, nil
}

func (h *KvmHyper) GetSupportedCpuID() ([]CpuIdEntry, error) {

	// Maximum number of CPUID entries that can be returned by a call to KVM ioctls.
	// See arch/x86/include/asm/kvm_host.h
	const KVM_MAX_CPUID_ENTRIES = 256
	data, head, entrys := genKvmCpuid2(KVM_MAX_CPUID_ENTRIES)

	if err := ioctlBytes(h.kvmFile.Fd(), KVM_GET_SUPPORTED_CPUID, data); err != nil {
		return nil, err
	}
	result := make([]CpuIdEntry, head.Nent)
	for i := range result {
		result[i] = entrys[i].into()
	}
	return result, nil
}

func (h *KvmHyper) GetEmulatedCpuID() ([]CpuIdEntry, error) {

	// Maximum number of CPUID entries that can be returned by a call to KVM ioctls.
	// See arch/x86/include/asm/kvm_host.h
	const KVM_MAX_CPUID_ENTRIES = 256
	data, head, entrys := genKvmCpuid2(KVM_MAX_CPUID_ENTRIES)
	if err := ioctlBytes(h.kvmFile.Fd(), KVM_GET_EMULATED_CPUID, data); err != nil {
		return nil, err
	}
	result := make([]CpuIdEntry, head.Nent)
	for i := range result {
		result[i] = entrys[i].into()
	}
	return result, nil
}

func (h *KvmHyper) KvmGetMsrIndexList() ([]MsrEntry, error) {

	/// Maximum number of MSRs KVM supports (See arch/x86/kvm/x86.c).
	const KVM_MAX_MSR_ENTRIES = 256
	data, head, entrys := genKvmMsrList(KVM_MAX_MSR_ENTRIES)
	if err := ioctlBytes(h.kvmFile.Fd(), KVM_GET_MSR_INDEX_LIST, data); err != nil {
		return nil, err
	}
	result := make([]MsrEntry, head.Nmsrs)
	for i := range result {
		result[i] = entrys[i].into()
	}
	return result, nil
}

func (h *KvmHyper) KvmGetMsrFeatureIndexList() ([]MsrEntry, error) {

	/// Maximum number of MSRs KVM supports (See arch/x86/kvm/x86.c).
	const KVM_MAX_MSR_ENTRIES = 256
	data, head, entrys := genKvmMsrList(KVM_MAX_MSR_ENTRIES)
	if err := ioctlBytes(h.kvmFile.Fd(), KVM_GET_MSR_FEATURE_INDEX_LIST, data); err != nil {
		return nil, err
	}
	result := make([]MsrEntry, head.Nmsrs)
	for i := range result {
		result[i] = entrys[i].into()
	}
	return result, nil
}

func (h *KvmHyper) CheckRequiredExtensions() error {
	requiredExts := []KvmCap{
		KvmCapAdjustClock,
		KvmCapExtCpuids,
		KvmCapGetTscKhz,
		KvmCapImmediateExit,
		KvmCapIoeventfd,
		KvmCapIrqchip,
		KvmCapIrqfd,
		KvmCapIrqRouting,
		KvmCapMpState,
		KvmCapSetIdentityMapAddr,
		KvmCapSetTssAddr,
		KvmCapSplitIrqchip,
		KvmCapTscDeadlineTimer,
		KvmCapUserMemory,
		KvmCapUserNmi,
		KvmCapVcpuEvents,
		KvmCapXcrs,
		KvmCapXsave,
	}
	for _, r := range requiredExts {
		if !h.checkExtension(r) {
			return fmt.Errorf("miss capacity: %s", r)
		}
	}
	return nil
}

func (h *KvmHyper) checkExtension(cap KvmCap) bool {
	n, _ := ioctlInt(h.kvmFile.Fd(), KVM_CHECK_EXTENSION, int(cap))
	return n > 0
}

func (h *KvmHyper) checkExtensionInt(cap KvmCap) int {
	n, _ := ioctlInt(h.kvmFile.Fd(), KVM_CHECK_EXTENSION, int(cap))
	return n
}

func (h *KvmHyper) GetMaxVCPUs() uint32 {
	return uint32(h.checkExtensionInt(KvmCapMaxVcpus))
}

func (h *KvmHyper) GetCPUVendor() CPUVendor {
	return getCPUVendor()
}

func (h *KvmHyper) GetVcpuMmapSize() (int, error) {
	return ioctl(h.kvmFile.Fd(), KVM_GET_VCPU_MMAP_SIZE)
}

type KvmVm struct {
	hyper  *KvmHyper
	vmFile *os.File
}

func (vm *KvmVm) setUserspaceMemoryRegion(region *KvmUserspaceMemoryRegion) error {
	return ioctlPtr(vm.vmFile.Fd(), KVM_SET_USER_MEMORY_REGION, unsafe.Pointer(region))
}

func (vm *KvmVm) CreateUserMemoryRegion(
	slot uint32,
	guestPhysAddr uint64,
	memorySize int,
	userspaceAddr uint64,
) error {
	region := &KvmUserspaceMemoryRegion{
		Slot:            slot,
		Flags:           0,
		Guest_phys_addr: guestPhysAddr,
		Memory_size:     uint64(memorySize),
		Userspace_addr:  userspaceAddr,
	}
	return vm.setUserspaceMemoryRegion(region)

}

func (vm *KvmVm) SetTssAddress(offset uint64) error {
	_, err := ioctlInt(vm.vmFile.Fd(), KVM_SET_TSS_ADDR, int(offset))
	return err
}

func (vm *KvmVm) SetIdentityMapAddress(address uint64) error {
	_, err := ioctlInt(vm.vmFile.Fd(), KVM_SET_IDENTITY_MAP_ADDR, int(address))
	return err
}

func (vm *KvmVm) CreateIrqChip() error {
	_, err := ioctlInt(vm.vmFile.Fd(), KVM_CREATE_IRQCHIP, 0)
	return err
}

// RegisterIrqfd registers an event that will, when signaled, trigger the 'gsi' IRQ.
func (vm *KvmVm) RegisterIrqfd(fd *EventFd, gsi uint32) error {
	irqfd := KvmIrqfd{
		Fd:         uint32(fd.Fd()),
		Gsi:        gsi,
		Flags:      0,
		Resamplefd: 0,
		Pad:        [16]uint8{},
	}
	return ioctlPtr(vm.vmFile.Fd(), KVM_IRQFD, unsafe.Pointer(&irqfd))
}

// UnregisterIrqfd unregisters an event that will, when signaled, trigger the 'gsi' IRQ.
func (vm *KvmVm) UnregisterIrqfd(fd *EventFd, gsi uint32) error {
	irqfd := KvmIrqfd{
		Fd:         uint32(fd.Fd()),
		Gsi:        gsi,
		Flags:      KVM_IRQFD_FLAG_DEASSIGN,
		Resamplefd: 0,
		Pad:        [16]uint8{},
	}
	return ioctlPtr(vm.vmFile.Fd(), KVM_IRQFD, unsafe.Pointer(&irqfd))
}

func (vm *KvmVm) EnableSplitIrq() error {
	cap := KvmEnableCap{
		Cap: uint32(KvmCapSplitIrqchip),
	}
	cap.Args[0] = KVM_IOAPIC_NUM_PINS
	return ioctlPtr(vm.vmFile.Fd(), KVM_ENABLE_CAP, unsafe.Pointer(&cap))
}

func (vm *KvmVm) EnableX2apicApi() error {

	cap := KvmEnableCap{
		Cap: uint32(KvmCapX2apicApi),
	}
	cap.Args[0] = KVM_X2APIC_API_USE_32BIT_IDS | KVM_X2APIC_API_DISABLE_BROADCAST_QUIRK
	return ioctlPtr(vm.vmFile.Fd(), KVM_ENABLE_CAP, unsafe.Pointer(&cap))
}

// // MakeRoutingEntry constructs an IRQ routing entry.
// MakeRoutingEntry(gsi uint32, config *MsiIrqSourceConfig) IrqRoutingEntry

// // SetGsiRouting sets the GSI routing table entries, overwriting any previously set entries.
// SetGsiRouting(entries []IrqRoutingEntry) error

// translateMsiExtDestID translates the MSI extended destination ID bits according to the logic
// found in the Linux kernel's KVM MSI handling in kvm_msi_to_lapic_irq()/x86_msi_msg_get_destid():
// https://github.com/torvalds/linux/blob/3957a5720157264dcc41415fbec7c51c4000fc2d/arch/x86/kvm/irq.c#L266
// https://github.com/torvalds/linux/blob/3957a5720157264dcc41415fbec7c51c4000fc2d/arch/x86/kernel/apic/apic.c#L2306
//
// This function moves bits [11, 5] from `addressLo` to bits [46, 40] in the combined 64-bit
// address, but only if the Remappable Format (RF) bit (bit 4) in `addressLo` is
// not set and `addressHi` is zero.
//
// The function is roughly equivalent to `uint64_t kvm_swizzle_msi_ext_dest_id(uint64_t address)` in
// qemu/target/i386/kvm/kvm.c:
// https://github.com/qemu/qemu/blob/88f72048d2f5835a1b9eaba690c7861393aef283/target/i386/kvm/kvm.c#L6258
func translateMsiExtDestID(addressLo, addressHi uint32) (uint32, uint32) {
	// Mask for extracting the RF (Remappable Format) bit from addressLo.
	// In the MSI specification, this is bit 4. See
	// VT-d spec section "Interrupt Requests in Remappable Format"
	const remappableFormatBitMask uint32 = 0x10
	remappableFormatBitIsSet := (addressLo & remappableFormatBitMask) != 0

	// Only perform the bit swizzling if the RF bit is unset and the upper
	// 32 bits of the address are all zero. This identifies the legacy format.
	if addressHi == 0 && !remappableFormatBitIsSet {
		// "Move" the bits [11,5] to bits [46,40]. This is a shift of 35 bits, but
		// since address is already split up into lo and hi, it's only a shift of
		// 3 (35 - 32) within hi.
		// "Move" via getting the bits via mask, zeroing out that range, and then
		// ORing them back in at the correct location. The destination was already
		// checked to be all zeroes.
		const extIDMask uint32 = 0xfe0
		const extIDShift uint32 = 3
		extID := addressLo & extIDMask
		addressLo &^= extIDMask // Go use &^ (bit clear) operator instead of &= !
		addressHi |= extID << extIDShift
	}

	return addressLo, addressHi
}

// MakeRoutingEntry constructs an IRQ routing entry.
func (vm *KvmVm) MakeRoutingEntry(gsi uint32, config *MsiIrqSourceConfig) IrqRoutingEntry {

	entry := KvmIrqRoutingEntry{
		GSI:  gsi,
		Type: KVM_IRQ_ROUTING_MSI,
	}

	route := (*KvmIrqRoutingMsi)(unsafe.Pointer(&entry.U[0]))

	addressLo, addressHi := translateMsiExtDestID(config.LowAddr, config.HighAddr)
	route.Lo = addressLo
	route.Hi = addressHi
	route.Data = config.Data

	if vm.hyper.checkExtension(KvmCapMsiDevid) {
		// On AArch64, there is limitation on the range of the 'devid',
		// it cannot be greater than 65536 (the max of u16).
		//
		// BDF cannot be used directly, because 'segment' is in high
		// 16 bits. The layout of the u32 BDF is:
		// |---- 16 bits ----|-- 8 bits --|-- 5 bits --|-- 3 bits --|
		// |      segment    |     bus    |   device   |  function  |
		//
		// Now that we support 1 bus only in a segment, we can build a
		// 'devid' by replacing the 'bus' bits with the low 8 bits of
		// 'segment' data.
		// This way we can resolve the range checking problem and give
		// different `devid` to all the devices. Limitation is that at
		// most 256 segments can be supported.
		//
		const KvmMsiValidDevid = 1

		modifiedDevid := ((config.DevID & 0x00ff_0000) >> 8) | (config.DevID & 0xff)
		entry.Flags = KvmMsiValidDevid
		route.Data = modifiedDevid
	}
	return IrqRoutingEntry(entry)
}

// SetGsiRouting sets the GSI routing table entries, overwriting any previously set entries.
func (vm *KvmVm) SetGsiRouting(entries []IrqRoutingEntry) error {
	nums := len(entries)
	data, _, entrys := genkvm_irq_routing(nums)
	for i := range entrys {
		entrys[i] = KvmIrqRoutingEntry(entries[i])
	}
	return ioctlBytes(vm.vmFile.Fd(), KVM_SET_GSI_ROUTING, data)
}

// func (vm *KvmVm) setGsiRoutingWithIrqchip(gsi int, irqchip KvmIrqRoutingIrqchip) error {
// 	headSize := unsafe.Sizeof(kvm_irq_routing{})
// 	entrySize := unsafe.Sizeof(KvmIrqRoutingEntry{})
// 	data := make([]byte, headSize+entrySize)
// 	routes := (*kvm_irq_routing)(unsafe.Pointer(&data[0]))
// 	routes.Nr = 1
// 	entry := (*KvmIrqRoutingEntry)(unsafe.Pointer(&data[headSize]))
// 	entry.Gsi = uint32(gsi)
// 	entry.Type = KVM_IRQ_ROUTING_IRQCHIP

// 	srcBuffer := unsafe.Slice((*byte)(unsafe.Pointer(&irqchip)), unsafe.Sizeof(irqchip))
// 	copy(entry.U[:], srcBuffer)

// 	if err := ioctlBytes(vm.vmFile.Fd(), KVM_SET_GSI_ROUTING, data); err != nil {
// 		return err
// 	}
// 	return nil
// }

// RegisterIoevent registers an event to be signaled whenever a certain address is written to.
// In Go, optional arguments are typically handled via pointers or wrapper configurations.
func (vm *KvmVm) RegisterIoevent(fd *EventFd, addr uint64) error {
	ioEventFd := KvmIoEventFd{
		Datamatch: 0,
		Addr:      addr,
		Len:       0,
		Fd:        int32(fd.Fd()),
		Flags:     0,
		Pad:       [36]uint8{},
	}
	return ioctlPtr(vm.vmFile.Fd(), KVM_IOEVENTFD, unsafe.Pointer(&ioEventFd))
}

// UnregisterIoevent unregisters an event from a certain address it has been previously registered to.
func (vm *KvmVm) UnregisterIoevent(fd *EventFd, addr uint64) error {
	ioEventFd := KvmIoEventFd{
		Datamatch: 0,
		Addr:      addr,
		Len:       0,
		Fd:        int32(fd.Fd()),
		Flags:     KVM_IOEVENTFD_FLAG_DEASSIGN,
		Pad:       [36]uint8{},
	}
	return ioctlPtr(vm.vmFile.Fd(), KVM_IOEVENTFD, unsafe.Pointer(&ioEventFd))
}

func (vm *KvmVm) GetVcpuMmapSize() (int, error) {

	return vm.hyper.GetVcpuMmapSize()
}

func (vm *KvmVm) CreateVcpu(i uint32, ops VmOps) (Vcpu, error) {

	fd, err := ioctl(vm.vmFile.Fd(), KVM_CREATE_VCPU)
	if err != nil {
		return nil, err
	}
	vcpu := &KvmVcpu{}
	name := fmt.Sprintf("vcpu %d", i)
	vcpu.File = os.NewFile(uintptr(fd), name)
	vcpu.Ops = ops

	mmapSize, err := vm.GetVcpuMmapSize()
	if err != nil {
		return vcpu, err
	}

	runMem, err := unix.Mmap(fd, 0, mmapSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return vcpu, err
	}
	vcpu.RunData = (*KvmRun)(unsafe.Pointer(&runMem[0]))

	return vcpu, nil
}

type KvmVcpu struct {
	VM      Vm
	File    *os.File
	RunData *KvmRun
	Ops     VmOps
	stop    atomic.Bool
}

func (v *KvmVcpu) BootMsrEntries() []MsrEntry {

	// MTRR constants
	const K8_MTRRFIXRANGE_DRAM_ENABLE = 0x800 // IA32_MTRR_DEF_TYPE MSR: E (MTRRs enabled) flag, bit 11
	const MTRR_MEM_TYPE_WB = 0x6

	entrys := []MsrEntry{
		{MSR_IA32_SYSENTER_CS, 0},
		{MSR_IA32_SYSENTER_ESP, 0},
		{MSR_IA32_SYSENTER_EIP, 0},
		{MSR_STAR, 0},
		{MSR_CSTAR, 0},
		{MSR_LSTAR, 0},
		{MSR_KERNEL_GS_BASE, 0},

		{MSR_SYSCALL_MASK, 0},
		{MSR_IA32_TSC, 0},
		{MSR_IA32_MISC_ENABLE, MSR_IA32_MISC_ENABLE_FAST_STRING},
		{MSR_MTRRdefType, K8_MTRRFIXRANGE_DRAM_ENABLE | MTRR_MEM_TYPE_WB},
	}
	return entrys
}

func (v *KvmVcpu) GetSregs() (SpecialRegisters, error) {
	sreg := KvmSregs{}
	err := ioctlPtr(v.File.Fd(), KVM_GET_SREGS, unsafe.Pointer(&sreg))
	return SpecialRegisters(sreg), err
}

func (v *KvmVcpu) SetSregs(sreg *SpecialRegisters) error {
	return ioctlPtr(v.File.Fd(), KVM_SET_SREGS, unsafe.Pointer(sreg))
}

func (v *KvmVcpu) CreateStandardRegs() StandardRegisters {
	return StandardRegisters{}
}

func (v *KvmVcpu) GetRegs() (StandardRegisters, error) {
	reg := KvmRegs{}
	err := ioctlPtr(v.File.Fd(), KVM_GET_REGS, unsafe.Pointer(&reg))
	return StandardRegisters(reg), err
}

func (v *KvmVcpu) SetRegs(reg *StandardRegisters) error {
	return ioctlPtr(v.File.Fd(), KVM_SET_REGS, unsafe.Pointer(reg))
}

func (v *KvmVcpu) GetMsrs() ([]MsrEntry, error) {

	const nums = 1024
	data, head, entrys := genKvmMsrs(nums)
	if err := ioctlBytes(v.File.Fd(), KVM_GET_MSRS, data); err != nil {
		return nil, err
	}
	result := make([]MsrEntry, head.Nmsrs)
	for i := range result {
		result[i] = entrys[i].into()
	}
	return result, nil
}

func (v *KvmVcpu) SetMsrs(msrs []MsrEntry) error {

	nums := len(msrs)
	data, _, entrys := genKvmMsrs(nums)
	for i := range entrys {
		entrys[i].from(msrs[i])
	}

	return ioctlBytes(v.File.Fd(), KVM_SET_MSRS, data)
}

func (v *KvmVcpu) GetCpuid2() ([]CpuIdEntry, error) {
	const KVM_MAX_CPUID_ENTRIES = 256
	data, head, entrys := genKvmCpuid2(KVM_MAX_CPUID_ENTRIES)

	if err := ioctlBytes(v.File.Fd(), KVM_GET_CPUID2, data); err != nil {
		return nil, err
	}
	result := make([]CpuIdEntry, head.Nent)
	for i := range result {
		result[i] = entrys[i].into()
	}
	return result, nil
}

func (v *KvmVcpu) SetCpuid2(cpuids []CpuIdEntry) error {

	nums := len(cpuids)
	data, _, entrys := genKvmCpuid2(nums)
	for i := range entrys {
		entrys[i].from(cpuids[i])
	}

	return ioctlBytes(v.File.Fd(), KVM_SET_CPUID2, data)

}

func (v *KvmVcpu) GetLapic() (LapicState, error) {
	reg := KvmLapicState{}
	err := ioctlPtr(v.File.Fd(), KVM_GET_LAPIC, unsafe.Pointer(&reg))
	result := LapicState{}
	for i := range result.Regs {
		result.Regs[i] = byte(reg.Regs[i])
	}
	return result, err
}

func (v *KvmVcpu) SetLapic(lapic *LapicState) error {
	kvmLapic := KvmLapicState{}
	for i := range kvmLapic.Regs {
		kvmLapic.Regs[i] = int8(lapic.Regs[i])
	}
	return ioctlPtr(v.File.Fd(), KVM_SET_LAPIC, unsafe.Pointer(&kvmLapic))
}

func (v *KvmVcpu) EnableCap(cap KvmEnableCap) error {
	return ioctlPtr(v.File.Fd(), KVM_ENABLE_CAP, unsafe.Pointer(&cap))
}

func (v *KvmVcpu) GetFpu() (FpuState, error) {
	fpu := KvmFpu{}
	result := FpuState{}
	if err := ioctlPtr(v.File.Fd(), KVM_GET_FPU, unsafe.Pointer(&fpu)); err != nil {
		return result, err
	}
	result = FpuState{
		Fpr:        fpu.Fpr,
		Fcw:        fpu.Fcw,
		Fsw:        fpu.Fsw,
		Ftwx:       fpu.Ftwx,
		LastOpcode: fpu.Opcode,
		LastIp:     fpu.Ip,
		LastDp:     fpu.Dp,
		Xmm:        fpu.Xmm,
		Mxcsr:      fpu.Mxcsr,
	}
	return result, nil
}

// // SetFpu sets the floating point unit (FPU) SIMD state of a vCPU.
// SetFpu(fpu *FpuState) error
func (v *KvmVcpu) SetFpu(fpu *FpuState) error {
	kvmFpu := KvmFpu{
		Fpr:    fpu.Fpr,
		Fcw:    fpu.Fcw,
		Fsw:    fpu.Fsw,
		Ftwx:   fpu.Ftwx,
		Opcode: fpu.LastOpcode,
		Ip:     fpu.LastIp,
		Dp:     fpu.LastDp,
		Xmm:    fpu.Xmm,
		Mxcsr:  fpu.Mxcsr,
	}
	return ioctlPtr(v.File.Fd(), KVM_SET_FPU, unsafe.Pointer(&kvmFpu))
}

func (v *KvmVcpu) Nmi() error {

	n, _ := ioctl(v.File.Fd(), KVM_NMI)
	if n != 0 {
		return errors.New("nmi error")
	}
	return nil
}

func (v *KvmVcpu) SetImmediateExit(exit bool) {
	v.stop.Store(exit)
	if exit {
		v.RunData.Immediate_exit = 1
	} else {
		v.RunData.Immediate_exit = 0
	}

}

func (v *KvmVcpu) Run() (VmExit, error) {
	if v.stop.Load() {
		return VmExitShutdown, nil
	}

	_, err := ioctlNoRetry(v.File.Fd(), KVM_RUN)
	if err != nil {
		if err == unix.EINTR {
			if v.stop.Load() {
				return VmExitShutdown, nil
			}
			return VmExitIgnore, nil
		}
		if err == unix.EFAULT && v.RunData.Exit_reason == uint32(KvmExitMemoryFault) {
			fmt.Printf("memory fault")
		}
		return VmExitUnknown, err
	}

	switch KvmExitReason(v.RunData.Exit_reason) {
	case KvmExitIO:
		pio := (*io)(unsafe.Pointer(&v.RunData.Data[0]))
		port := uint64(pio.Port)
		pioPtr := unsafe.Add(unsafe.Pointer(v.RunData), pio.Offset)

		data := unsafe.Slice((*byte)(pioPtr), pio.Size*uint8(pio.Count))
		const (
			KvmExitIoIn  = 0
			KvmExitIoOut = 1
		)

		var err error
		switch pio.Direction {
		case KvmExitIoIn:
			err = v.Ops.PioRead(port, data)
		case KvmExitIoOut:
			err = v.Ops.PioWrite(port, data)
		}
		if err != nil {
			return VmExitUnknown, err
		}
		return VmExitIgnore, nil

	case KvmExitMmio:
		mmio := (*mmio)(unsafe.Pointer(&v.RunData.Data[0]))
		addr := mmio.Phys_addr
		data := mmio.Data[:mmio.Len]

		var err error
		if mmio.Is_write != 0 {
			err = v.Ops.MmioWrite(addr, data)
		} else {
			err = v.Ops.MmioRead(addr, data)
		}

		if err != nil {
			return VmExitUnknown, err
		}
		return VmExitIgnore, nil
	case KvmExitIoapciEoi:
		return VmExitIoapicEoi, nil
	case KvmExitHlt:
		return VmExitReset, nil
	case KvmExitShutDown:
		return VmExitReset, nil

	case KvmExitUnkonw:
		return VmExitUnknown, nil
	case KvmExitException:
		return VmExitUnknown, nil
	case KvmExitHypercall:
		return VmExitUnknown, nil
	case KvmExitHyperv:
		return VmExitHyperv, nil
	case KvmExitDebug:
		return VmExitDebug, nil
	case KvmExitX86Rdmsr:
		return VmExitUnknown, nil
	case KvmExitX86Wrmsr:
		return VmExitUnknown, nil
	case KvmExitIrqWinOpen:
		return VmExitUnknown, nil
	case KvmExitFailEntry:
		return VmExitUnknown, nil
	case KvmExitNmi:
		return VmExitUnknown, nil
	default:
		return VmExitUnknown, nil
	}

}
