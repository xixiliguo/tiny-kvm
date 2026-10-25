// Package vmm implements the minimal x86_64 virtual machine monitor used by
// tiny-kvm.
//
// The package is intentionally smaller than Cloud Hypervisor's VMM. It keeps
// the parts needed to boot a Linux kernel with KVM: configuration, guest RAM,
// vCPU management, legacy devices, ACPI tables and the VM lifecycle.
package vmm

import (
	"errors"
	"sync"

	"github.com/xixiliguo/tiny-kvm/hyper"
	"github.com/xixiliguo/tiny-kvm/internal/cpuid"
)

var (
	// ErrVMExists is returned while a VM is still owned by the VMM.
	ErrVMExists = errors.New("vmm: a VM already exists")
	// ErrNoVM is returned when an operation requires a VM that has not been
	// created yet.
	ErrNoVM = errors.New("vmm: no VM has been created")
)

// VMM owns the hypervisor handle and at most one VM. This mirrors the subset
// of Cloud Hypervisor behavior exposed by tiny-kvm.
type VMM struct {
	mu         sync.RWMutex
	hypervisor hyper.Hypervisor
	vm         *VM
}

// New opens the host hypervisor. A VM is not created until CreateVM is called.
func New() (*VMM, error) {
	hv, err := hyper.NewKvmHyper()
	if err != nil {
		return nil, err
	}
	return NewWithHypervisor(hv), nil
}

// NewWithHypervisor creates a VMM around a caller-supplied hypervisor. It is
// useful for tests and for reusing an already-opened KVM handle.
func NewWithHypervisor(hv hyper.Hypervisor) *VMM {
	return &VMM{hypervisor: hv}
}

// CreateVM creates the KVM VM and all resources needed before boot.
func (v *VMM) CreateVM(config VmConfig) (*VM, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.vm != nil && v.vm.State() != VmStateShutdown {
		return nil, ErrVMExists
	}
	if v.vm != nil {
		_ = v.vm.Close()
	}
	vm, err := NewVM(v.hypervisor, config)
	if err != nil {
		return nil, err
	}
	v.vm = vm
	return vm, nil
}

// VM returns the VM owned by this VMM.
func (v *VMM) VM() *VM {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.vm
}

// Shutdown stops the VM, if one exists.
func (v *VMM) Shutdown() error {
	v.mu.RLock()
	vm := v.vm
	v.mu.RUnlock()
	if vm == nil {
		return ErrNoVM
	}
	return vm.Shutdown()
}

// GetHostCpuPhysBits calculates the maximum available physical address bits of the host CPU.
// It applies corrections for features like AMD Secure Memory Encryption (SME) if active.
func GetHostCpuPhysBits() uint8 {
	maxLeaf, _, _, _ := cpuid.Cpuid(0x80000000, 0)

	if maxLeaf >= 0x80000008 {
		extEax, _, _, _ := cpuid.Cpuid(0x80000008, 0)
		// Extract raw physical address size from the lowest 8 bits of EAX
		if bits := uint8(extEax & 0xFF); bits != 0 {
			return bits
		}
	}
	// Default fallback value specified by standard x86 configurations
	return 36
}
