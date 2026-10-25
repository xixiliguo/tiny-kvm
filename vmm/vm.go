package vmm

import (
	"errors"
	"fmt"
	"sync"

	"github.com/xixiliguo/tiny-kvm/arch"
	"github.com/xixiliguo/tiny-kvm/hyper"
)

// VmState is the VM lifecycle state.
type VmState uint8

const (
	VmStateCreated VmState = iota
	VmStateRunning
	VmStateShutdown
	VmStatePaused
	VmStateBreakPoint
)

func (s VmState) String() string {
	switch s {
	case VmStateCreated:
		return "Created"
	case VmStateRunning:
		return "Running"
	case VmStateShutdown:
		return "Shutdown"
	case VmStatePaused:
		return "Paused"
	case VmStateBreakPoint:
		return "BreakPoint"
	default:
		return "Unknown"
	}
}

var (
	ErrInvalidStateTransition = errors.New("vm: invalid state transition")
	ErrVmMissingConfig        = errors.New("vm: missing configuration")
)

// VM is a bootable tiny-kvm virtual machine.
type VM struct {
	mu         sync.RWMutex
	config     VmConfig
	hypervisor hyper.Hypervisor
	hyperVm    hyper.Vm
	memory     *MemoryManager
	interrupts *InterruptManager
	devices    *DeviceManager
	cpus       *CpuManager
	state      VmState
	bootOnce   sync.Once
	bootErr    error
	waitErr    error
}

// NewVM creates and configures a VM. KVM is opened by the caller, while all
// guest resources are created and checked here.
func NewVM(hv hyper.Hypervisor, config VmConfig) (*VM, error) {
	if hv == nil {
		return nil, errors.New("vm: hypervisor is required")
	}
	config = config.Clone()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if err := hv.CheckRequiredExtensions(); err != nil {
		return nil, fmt.Errorf("vm: KVM extensions: %w", err)
	}

	hyperVm, err := hv.CreateVM(config.Memory.Size)
	if err != nil {
		return nil, fmt.Errorf("vm: create KVM VM: %w", err)
	}
	if err := hyperVm.SetIdentityMapAddress(arch.KvmIdentityMapStart); err != nil {
		return nil, fmt.Errorf("vm: set identity map: %w", err)
	}
	if err := hyperVm.SetTssAddress(arch.KvmTssStart); err != nil {
		return nil, fmt.Errorf("vm: set TSS: %w", err)
	}
	if err := hyperVm.CreateIrqChip(); err != nil {
		return nil, fmt.Errorf("vm: create KVM IRQ chip: %w", err)
	}
	if err := hyperVm.EnableX2apicApi(); err != nil {
		return nil, fmt.Errorf("vm: enable x2APIC: %w", err)
	}

	memory, err := NewMemoryManager(hyperVm, config.Memory)
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = memory.Close() }

	interrupts := NewInterruptManager(hyperVm)
	memBelow4G := min(config.Memory.Size, arch.Mem32BitReservedStart)
	memAbove4G := config.Memory.Size - memBelow4G
	devices, err := NewDeviceManager(memBelow4G, memAbove4G, interrupts, config.Serial)
	if err != nil {
		cleanup()
		return nil, err
	}
	cleanup = func() {
		_ = devices.Close()
		_ = interrupts.Close()
		_ = memory.Close()
	}

	payload, err := loadPayload(memory.GuestMemory(), config.Payload)
	if err != nil {
		cleanup()
		return nil, err
	}
	rsdpAddr, err := createACPITables(memory.GuestMemory(), config.Cpus.BootVcpus)
	if err != nil {
		cleanup()
		return nil, err
	}

	var serialNumber, uuid string
	var oemStrings []string
	if config.Platform != nil {
		serialNumber = config.Platform.SerialNumber
		uuid = config.Platform.UUID
		oemStrings = config.Platform.OEMStrings
	}
	if err := arch.ConfigureSystem(
		memory.GuestMemory(),
		arch.CmdlineStart,
		arch.CmdlineMaxSize,
		payload.initramfs,
		config.Cpus.BootVcpus,
		payload.entry.SetupHeader,
		rsdpAddr,
		serialNumber,
		uuid,
		oemStrings,
	); err != nil {
		cleanup()
		return nil, fmt.Errorf("vm: configure system: %w", err)
	}

	ops := &vmOps{memory: memory.GuestMemory(), devices: devices}
	cpus, err := NewCpuManager(hv, hyperVm, ops, config.Cpus)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := cpus.Configure(&arch.BootSetup{
		KernelEntryPoint: payload.entry,
		GuestMemory:      memory.GuestMemory(),
	}); err != nil {
		cleanup()
		return nil, err
	}

	return &VM{
		config:     config,
		hypervisor: hv,
		hyperVm:    hyperVm,
		memory:     memory,
		interrupts: interrupts,
		devices:    devices,
		cpus:       cpus,
		state:      VmStateCreated,
	}, nil
}

// Boot starts all boot vCPUs without waiting for guest termination.
func (v *VM) Boot() error {
	v.bootOnce.Do(func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		if v.state != VmStateCreated && v.state != VmStatePaused {
			v.bootErr = fmt.Errorf("%w: %s -> Running", ErrInvalidStateTransition, v.state)
			return
		}
		if err := v.cpus.Start(); err != nil {
			v.bootErr = err
			return
		}
		v.state = VmStateRunning
	})
	return v.bootErr
}

// Run boots the VM and blocks until all vCPUs stop.
func (v *VM) Run() error {
	if err := v.Boot(); err != nil {
		return err
	}
	v.waitErr = v.cpus.Wait()
	v.mu.Lock()
	v.state = VmStateShutdown
	v.mu.Unlock()
	return v.waitErr
}

// Wait blocks for a previously booted VM.
func (v *VM) Wait() error {
	v.mu.RLock()
	running := v.state == VmStateRunning
	v.mu.RUnlock()
	if !running {
		return fmt.Errorf("%w: VM is not running", ErrInvalidStateTransition)
	}
	v.waitErr = v.cpus.Wait()
	v.mu.Lock()
	v.state = VmStateShutdown
	v.mu.Unlock()
	return v.waitErr
}

// Pause requests guest execution to stop.
func (v *VM) Pause() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state != VmStateRunning {
		return fmt.Errorf("%w: %s -> Paused", ErrInvalidStateTransition, v.state)
	}
	for _, vcpu := range v.cpus.VCPUs() {
		vcpu.Vcpu.SetImmediateExit(true)
	}
	v.state = VmStatePaused
	return nil
}

// Shutdown requests VM termination. It is safe to call repeatedly.
func (v *VM) Shutdown() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state == VmStateShutdown {
		return nil
	}
	v.cpus.Stop()
	if v.state == VmStateCreated || v.state == VmStatePaused {
		v.state = VmStateShutdown
	}
	return nil
}

// State returns the current lifecycle state.
func (v *VM) State() VmState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state
}

// Config returns a copy of the active VM configuration.
func (v *VM) Config() VmConfig {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.config.Clone()
}

// Memory returns the guest memory manager.
func (v *VM) Memory() *MemoryManager { return v.memory }

// DeviceManager returns the active device manager.
func (v *VM) DeviceManager() *DeviceManager { return v.devices }

// QueueConsoleInput sends host bytes to the guest serial port.
func (v *VM) QueueConsoleInput(data []byte) error {
	return v.devices.Serial().QueueInputBytes(data)
}

// Close releases all userspace resources owned by the VM.
func (v *VM) Close() error {
	_ = v.Shutdown()
	var first error
	if err := v.devices.Close(); err != nil {
		first = err
	}
	if err := v.interrupts.Close(); err != nil && first == nil {
		first = err
	}
	if err := v.memory.Close(); err != nil && first == nil {
		first = err
	}
	return first
}

type vmOps struct {
	memory  *GuestMemory
	devices *DeviceManager
}

func (o *vmOps) GuestMemWrite(gpa uint64, data []byte) (int, error) {
	return o.memory.WriteSlice(data, gpa)
}

func (o *vmOps) GuestMemRead(gpa uint64, data []byte) (int, error) {
	return o.memory.ReadSlice(data, gpa)
}

func (o *vmOps) MmioRead(gpa uint64, data []byte) error {
	o.devices.ReadMMIO(gpa, data)
	return nil
}

func (o *vmOps) MmioWrite(gpa uint64, data []byte) error {
	return o.devices.WriteMMIO(gpa, data)
}

func (o *vmOps) PioRead(port uint64, data []byte) error {
	o.devices.ReadIO(port, data)
	return nil
}

func (o *vmOps) PioWrite(port uint64, data []byte) error {
	return o.devices.WriteIO(port, data)
}

var _ hyper.VmOps = (*vmOps)(nil)
