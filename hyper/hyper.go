package hyper

import (
	"encoding/binary"

	"github.com/xixiliguo/tiny-kvm/internal/cpuid"
)

//go:generate go tool stringer -output hyper_string.go -type=HypervisorType,CPUVendor,VmExit

type HypervisorType uint32

const (
	HypervisorTypeKVM HypervisorType = iota
	HypervisorTypeMSHV
)

// CPUVendor represents the processor manufacturer.
type CPUVendor int

const (
	CPUVendorIntel CPUVendor = iota
	CPUVendorAMD
	CPUVendorUnknow
)

// GetCPUVendor determines the CPU vendor by querying leaf 0x0 of CPUID.
func getCPUVendor() CPUVendor {
	// Query CPUID leaf 0x0
	_, ebx, ecx, edx := cpuid.Cpuid(0x0, 0x0)

	if ebx == 0x756e6547 && ecx == 0x6c65746e && edx == 0x49656e69 {
		// Vendor string "GenuineIntel"
		return CPUVendorIntel
	} else if ebx == 0x68747541 && ecx == 0x444d4163 && edx == 0x69746e65 {
		// Vendor string "AuthenticAMD"
		return CPUVendorAMD
	}

	// Not known yet, the corresponding manufacturer manual should contain the
	// necessary info. See also https://wiki.osdev.org/CPUID#CPU_Vendor_ID_String
	return CPUVendorUnknow
}

// Hypervisor defines a hypervisor-agnostic interface.
// It is safely concurrent by nature if underlying concrete implementations match Sync/Send requirements.
type Hypervisor interface {
	// HypervisorType returns the type of the hypervisor.
	HypervisorType() HypervisorType

	// CreateVM creates a virtual machine using the underlying hypervisor.
	// Returns a thread-safe implementation of the VM interface.
	CreateVM(memSize uint64) (Vm, error)

	GetSupportedCpuID() ([]CpuIdEntry, error)

	// CheckRequiredExtensions checks target extensions if any are required.
	CheckRequiredExtensions() error

	// GetMaxVCPUs returns the maximum number of vCPUs supported.
	GetMaxVCPUs() uint32

	GetCPUVendor() CPUVendor
}

// MsiIrqSourceConfig holds configuration data for MSI/MSI-X interrupts.
//
// On x86 platforms, these interrupts are vectors delivered directly to the LAPIC.
type MsiIrqSourceConfig struct {
	// HighAddr represents the high address to deliver message signaled interrupt.
	HighAddr uint32

	// LowAddr represents the low address to deliver message signaled interrupt.
	LowAddr uint32

	// Data represents the data to write to deliver message signaled interrupt.
	Data uint32

	// DevID represents the unique ID of the device to deliver message signaled interrupt.
	DevID uint32
}

const (
	IrqRoutingTypeMsi = 0x2
)

// IrqRoutingEntry represents a KVM kernel interrupt routing table entry.
type IrqRoutingEntry struct {
	GSI   uint32 // Global System Interrupt number
	Type  uint32 // Routing entry type (e.g., KVM_IRQ_ROUTING_IRQCHIP, KVM_IRQ_ROUTING_MSI)
	Flags uint32 // Routing entry flags
	Pad   uint32 // Padding for 64-bit alignment

	// U represents the routing entry union data.
	// In Go, unions are typically represented as a byte array matching
	// the size of the largest union member to ensure identical memory layout.
	U [32]byte
}

// IRQRoutingMSI represents the MSI (Message Signaled Interrupts)
// routing entry configuration for KVM.
type IrqRoutingMSI struct {
	AddressLo uint32 // Low 32 bits of the MSI address
	AddressHi uint32 // High 32 bits of the MSI address
	Data      uint32 // MSI data payload
	// PadOrDevID represents the anonymous union block.
	// Since both 'pad' and 'devid' are uint32, they share the exact same 4-byte slot.
	DevID uint32
}

type IrqRoutingIrqchip struct {
	Irqchip uint32 // Low 32 bits of the MSI address
	Pin     uint32 // High 32 bits of the MSI address
}

// Vm represents a hypervisor-agnostic interface for a Virtual Machine.
// In Go, thread safety (Send + Sync) is part of documentation and implementation behavior rather than type constraints.
type Vm interface {
	// SetIdentityMapAddress sets the address of the one-page region in the VM's address space.
	// (Note: Specific to x86_64 architecture)
	SetIdentityMapAddress(address uint64) error

	// SetTssAddress sets the address of the three-page region in the VM's address space.
	// (Note: Specific to x86_64 architecture)
	SetTssAddress(offset uint64) error

	// CreateIrqChip creates an in-kernel interrupt controller.
	// (Note: Not applicable to riscv64 architecture)
	CreateIrqChip() error

	// RegisterIrqfd registers an event that will, when signaled, trigger the 'gsi' IRQ.
	RegisterIrqfd(fd *EventFd, gsi uint32) error

	// UnregisterIrqfd unregisters an event that will, when signaled, trigger the 'gsi' IRQ.
	UnregisterIrqfd(fd *EventFd, gsi uint32) error

	// CreateVcpu creates a new vCPU file descriptor and maps the corresponding memory.
	CreateVcpu(id uint32, vmOps VmOps) (Vcpu, error)

	// RegisterIoevent registers an event to be signaled whenever a certain address is written to.
	// In Go, optional arguments are typically handled via pointers or wrapper configurations.
	RegisterIoevent(fd *EventFd, addr uint64) error

	// UnregisterIoevent unregisters an event from a certain address it has been previously registered to.
	UnregisterIoevent(fd *EventFd, addr uint64) error

	// MakeRoutingEntry constructs an IRQ routing entry.
	MakeRoutingEntry(gsi uint32, config *MsiIrqSourceConfig) IrqRoutingEntry

	// SetGsiRouting sets the GSI routing table entries, overwriting any previously set entries.
	SetGsiRouting(entries []IrqRoutingEntry) error

	// CreateUserMemoryRegion creates a guest physical memory slot.
	// Safety: The memory region defined by userspaceAddr and memorySize must be valid
	// and remain valid until RemoveUserMemoryRegion is called.
	CreateUserMemoryRegion(
		slot uint32,
		guestPhysAddr uint64,
		memorySize int,
		userspaceAddr uint64,
	) error

	// RemoveUserMemoryRegion removes a guest physical memory slot.
	// Safety: The memory region defined by userspaceAddr and memorySize must be valid.
	// RemoveUserMemoryRegion(
	// 	slot uint32,
	// 	guestPhysAddr uint64,
	// 	memorySize int,
	// 	userspaceAddr uint64,
	// ) error

	// EnableSplitIrq enables split IRQ capability.
	// (Note: Specific to x86_64 architecture)
	EnableSplitIrq() error

	// EnableX2apicApi enables the x2APIC API.
	// (Note: Specific to KVM on x86_64 architecture)
	EnableX2apicApi() error
}

// VmOps defines the memory and I/O operations for a Virtual Machine.
// In Go, thread safety (Send + Sync) is achieved by design in the
// underlying implementation rather than through explicit type constraints.
type VmOps interface {
	// GuestMemWrite writes data from buf into the guest physical address (gpa).
	// Returns the number of bytes written or an error.
	GuestMemWrite(gpa uint64, buf []byte) (int, error)

	// GuestMemRead reads data from the guest physical address (gpa) into buf.
	// Returns the number of bytes read or an error.
	GuestMemRead(gpa uint64, buf []byte) (int, error)

	// MmioRead reads data from a memory-mapped I/O (MMIO) address into data.
	MmioRead(gpa uint64, data []byte) error

	// MmioWrite writes data to a memory-mapped I/O (MMIO) address.
	MmioWrite(gpa uint64, data []byte) error

	// PioRead reads data from a Port I/O address into data.
	// (Note: Specific to x86_64 architecture)
	PioRead(port uint64, data []byte) error

	// PioWrite writes data to a Port I/O address.
	// (Note: Specific to x86_64 architecture)
	PioWrite(port uint64, data []byte) error
}

type StandardRegisters struct {
	Rax    uint64
	Rbx    uint64
	Rcx    uint64
	Rdx    uint64
	Rsi    uint64
	Rdi    uint64
	Rsp    uint64
	Rbp    uint64
	R8     uint64
	R9     uint64
	R10    uint64
	R11    uint64
	R12    uint64
	R13    uint64
	R14    uint64
	R15    uint64
	Rip    uint64
	Rflags uint64
}

type SegmentRegister struct {
	Base     uint64
	Limit    uint32
	Selector uint16
	Type     uint8
	Present  uint8
	Dpl      uint8
	Db       uint8
	S        uint8
	L        uint8
	G        uint8
	Avl      uint8
	Unusable uint8
}
type DescriptorTable struct {
	Base  uint64
	Limit uint16
}

type SpecialRegisters struct {
	Cs               SegmentRegister
	Ds               SegmentRegister
	Es               SegmentRegister
	Fs               SegmentRegister
	Gs               SegmentRegister
	Ss               SegmentRegister
	Tr               SegmentRegister
	Ldt              SegmentRegister
	Gdt              DescriptorTable
	Idt              DescriptorTable
	Cr0              uint64
	Cr2              uint64
	Cr3              uint64
	Cr4              uint64
	Cr8              uint64
	Efer             uint64
	Apic_base        uint64
	Interrupt_bitmap [4]uint64
}

// FpuState mirrors the x86_64 floating-point unit (FPU) and SIMD extensions state.
// It maps directly to the low-level hypervisor vCPU FPU save area (e.g., kvm_fpu).
type FpuState struct {
	// Fpr represents the 8 legacy x87 FPU data registers (ST0-ST7), each 128-bit (16 bytes).
	// Although x87 registers are technically 80-bit (10 bytes), the hypervisor pads them to 16 bytes.
	Fpr [8][16]byte

	// Fcw is the x87 FPU Control Word (16-bit).
	Fcw uint16

	// Fsw is the x87 FPU Status Word (16-bit).
	Fsw uint16

	// Ftwx is the abbreviated x87 FPU Tag Word (8-bit).
	Ftwx uint8

	// LastOpcode is the last non-control x87 FPU instruction opcode executed (16-bit).
	LastOpcode uint16

	// LastIp is the instruction pointer (IP) of the last non-control x87 FPU instruction.
	LastIp uint64

	// LastDp is the data pointer (DP) of the last non-control x87 FPU instruction operand.
	LastDp uint64

	// Xmm represents the 16 SSE XMM registers (XMM0-XMM15), each 128-bit (16 bytes).
	Xmm [16][16]byte

	// Mxcsr is the SSE Control and Status Register (32-bit), managing MXCSR masking and flags.
	Mxcsr uint32
}

const (
	CpuIdFlagValidIndex = 1
)

// CpuIdEntry mirrors a single x86 CPUID leaf/subleaf registration record.
// It maps directly to low-level hypervisor architectural interfaces (e.g., kvm_cpuid_entry2).
type CpuIdEntry struct {
	// Function specifies the primary CPUID leaf command number (passed via EAX register).
	Function uint32

	// Index specifies the sub-leaf selection index (passed via ECX register, used when flags require it).
	Index uint32

	// Flags defines hardware/VMM bitmask properties governing this entry (e.g., KVM_CPUID_FLAG_SIGNIFCANT_INDEX).
	Flags uint32

	// Eax holds the output value returned within the EAX register for this specific CPUID feature query.
	Eax uint32

	// Ebx holds the output value returned within the EBX register for this specific CPUID feature query.
	Ebx uint32

	// Ecx holds the output value returned within the ECX register for this specific CPUID feature query.
	Ecx uint32

	// Edx holds the output value returned within the EDX register for this specific CPUID feature query.
	Edx uint32
}

// LapicState represents the local APIC state.
type LapicState struct {
	// Regs represents the raw local APIC registers.
	Regs [1024]byte
}

func (l *LapicState) GetLapicReg(reg int) uint32 {

	return binary.LittleEndian.Uint32(l.Regs[reg:])
}
func (l *LapicState) SetLapicReg(reg int, value uint32) {
	binary.LittleEndian.PutUint32(l.Regs[reg:], value)
}

type MsrEntry struct {
	Index uint32
	Data  uint64
}

// VmExitType represents the reason for a VM exit.
type VmExit int

const (
	// VmExitIgnore indicates the exit should be ignored.
	VmExitIgnore VmExit = iota
	// VmExitReset indicates a VM reset request.
	VmExitReset
	// VmExitShutdown indicates a VM shutdown request.
	VmExitShutdown
	// VmExitHyperv indicates a Hyper-V specific exit.
	VmExitHyperv
	// VmExitTdx indicates an Intel TDX exit.
	// Note: In Go, conditional compilation features are usually handled via build tags.
	VmExitTdx
	// VmExitDebug indicates a guest debugging exit event.
	VmExitDebug
	// VmExitIoapicEoi indicates an IOAPIC End of Interrupt exit.
	// Note: This is specific to the x86_64 architecture.
	VmExitIoapicEoi
	VmExitUnknown
)

// Vcpu represents a generic, thread-safe virtual CPU instance.
// It contains core cross-architecture virtualization operations.
type Vcpu interface {
	// CreateStandardRegs returns StandardRegisters with default values set.
	CreateStandardRegs() StandardRegisters

	// GetRegs returns the vCPU general purpose registers.
	GetRegs() (StandardRegisters, error)

	// SetRegs sets the vCPU general purpose registers.
	SetRegs(regs *StandardRegisters) error

	// GetSregs returns the vCPU special registers (Segment registers, CR0-CR4, GDTR, LDTR, etc.).
	GetSregs() (SpecialRegisters, error)

	// SetSregs sets the vCPU special registers.
	SetSregs(sregs *SpecialRegisters) error

	// GetFpu returns the floating point unit (FPU) SIMD state from the vCPU.
	GetFpu() (FpuState, error)

	// SetFpu sets the floating point unit (FPU) SIMD state of a vCPU.
	SetFpu(fpu *FpuState) error

	// SetCpuid2 configures the KVM/hypervisor CPUID leaf table entries for the guest.
	SetCpuid2(cpuid []CpuIdEntry) error

	// GetCpuid2 retrieves the configured CPUID table entries from the kernel.
	GetCpuid2() ([]CpuIdEntry, error)

	// GetLapic returns the state of the Local Advanced Programmable Interrupt Controller (LAPIC).
	GetLapic() (LapicState, error)

	// SetLapic sets the state of the Local Advanced Programmable Interrupt Controller (LAPIC).
	SetLapic(lapic *LapicState) error

	// GetMsrs reads specified Model-Specific Registers (MSRs) from the vCPU.
	GetMsrs() ([]MsrEntry, error)

	// SetMsrs writes specified Model-Specific Registers (MSRs) into the vCPU.
	SetMsrs(msrs []MsrEntry) error

	// BootMsrEntries return the list of initial MSR entries for a VCPU
	BootMsrEntries() []MsrEntry

	// Run triggers the execution of the current virtual CPU, blocking until a VM-Exit condition occurs.
	Run() (VmExit, error)

	// SetImmediateExit instructs the vCPU loop to exit immediately without entering or re-entering guest mode.
	SetImmediateExit(exit bool)

	// Nmi injects a Non-Maskable Interrupt directly into the execution context.
	Nmi() error
}
