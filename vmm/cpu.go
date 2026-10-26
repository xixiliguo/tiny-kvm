package vmm

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"

	"github.com/xixiliguo/tiny-kvm/arch"
	"github.com/xixiliguo/tiny-kvm/hyper"
)

// VCPU is a tiny-kvm vCPU and its immutable guest id.
type VCPU struct {
	ID   uint32
	Vcpu hyper.Vcpu
}

// CpuManager owns all boot vCPUs and the common CPUID table.
type CpuManager struct {
	hyper       hyper.Hypervisor
	vm          hyper.Vm
	ops         hyper.VmOps
	config      CpusConfig
	commonCPUID []hyper.CpuIdEntry
	vcpus       []*VCPU
	wg          sync.WaitGroup
	runOnce     sync.Once
	stopOnce    sync.Once
	done        chan struct{}
	errMu       sync.Mutex
	err         error
}

// NewCpuManager creates boot vCPUs and prepares the guest CPUID table.
func NewCpuManager(hv hyper.Hypervisor, vm hyper.Vm, ops hyper.VmOps, config CpusConfig) (*CpuManager, error) {
	if hv == nil || vm == nil {
		return nil, errors.New("cpu: hypervisor and VM are required")
	}
	maxVCpus := hv.GetMaxVCPUs()
	if maxVCpus != 0 && config.BootVcpus > maxVCpus {
		return nil, fmt.Errorf("cpu: requested %d vCPUs, host supports %d", config.BootVcpus, maxVCpus)
	}
	physBits := min(GetHostCpuPhysBits(), config.MaxPhysBits)
	cpuid, err := generateGuestCPUID(hv, physBits, config.KvmHyperv)
	if err != nil {
		return nil, fmt.Errorf("cpu: generate CPUID: %w", err)
	}

	manager := &CpuManager{
		hyper:       hv,
		vm:          vm,
		ops:         ops,
		config:      config,
		commonCPUID: cpuid,
		done:        make(chan struct{}),
	}
	for i := uint32(0); i < config.BootVcpus; i++ {
		vcpu, err := vm.CreateVcpu(i, ops)
		if err != nil {
			return nil, fmt.Errorf("cpu: create vCPU %d: %w", i, err)
		}
		manager.vcpus = append(manager.vcpus, &VCPU{ID: i, Vcpu: vcpu})
	}
	return manager, nil
}

// Configure prepares every vCPU for the selected kernel entry point.
func (c *CpuManager) Configure(boot *arch.BootSetup) error {
	for _, vcpu := range c.vcpus {
		if err := configureVCPU(
			vcpu.Vcpu,
			vcpu.ID,
			boot,
			slices.Clone(c.commonCPUID),
			c.hyper.GetCPUVendor(),
		); err != nil {
			return fmt.Errorf("cpu: configure vCPU %d: %w", vcpu.ID, err)
		}
	}
	return nil
}

func configureVCPU(
	vcpu hyper.Vcpu,
	id uint32,
	boot *arch.BootSetup,
	cpuid []hyper.CpuIdEntry,
	vendor hyper.CPUVendor,
) error {
	cpuid, err := patchVCPUID(cpuid, id, vendor)
	if err != nil {
		return err
	}
	if err := vcpu.SetCpuid2(cpuid); err != nil {
		return fmt.Errorf("set CPUID: %w", err)
	}
	if err := vcpu.SetMsrs(vcpu.BootMsrEntries()); err != nil {
		return fmt.Errorf("set boot MSRs: %w", err)
	}
	if boot != nil {
		if err := arch.SetupRegs(vcpu, boot.KernelEntryPoint); err != nil {
			return fmt.Errorf("set boot registers: %w", err)
		}
		if err := vcpu.SetFpu(&hyper.FpuState{Fcw: 0x37f, Mxcsr: 0x1f80}); err != nil {
			return fmt.Errorf("set FPU state: %w", err)
		}
		sregs, err := vcpu.GetSregs()
		if err != nil {
			return fmt.Errorf("get special registers: %w", err)
		}
		if err := arch.ConfigureSegmentsAndSregs(boot.GuestMemory, &sregs, true); err != nil {
			return fmt.Errorf("configure segments: %w", err)
		}
		if err := vcpu.SetSregs(&sregs); err != nil {
			return fmt.Errorf("set special registers: %w", err)
		}
	}
	if err := hyper.SetLint(vcpu); err != nil {
		return fmt.Errorf("configure LAPIC: %w", err)
	}
	return nil
}

// VCPUs returns the configured vCPU handles.
func (c *CpuManager) VCPUs() []*VCPU {
	return append([]*VCPU(nil), c.vcpus...)
}

// Start launches all boot vCPUs.
func (c *CpuManager) Start() error {
	started := false
	c.runOnce.Do(func() {
		started = true
		for _, vcpu := range c.vcpus {
			c.wg.Add(1)
			go c.run(vcpu)
		}
		go func() {
			c.wg.Wait()
			close(c.done)
		}()
	})
	if !started {
		return errors.New("cpu: vCPUs have already been started")
	}
	return nil
}

func (c *CpuManager) run(vcpu *VCPU) {
	defer c.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for {
		exit, err := vcpu.Vcpu.Run()
		if err != nil {
			c.recordError(fmt.Errorf("cpu: vCPU %d run: %w", vcpu.ID, err))
			return
		}
		switch exit {
		case hyper.VmExitIgnore, hyper.VmExitIoapicEoi:
			continue
		case hyper.VmExitReset, hyper.VmExitShutdown:
			return
		default:
			c.recordError(fmt.Errorf("cpu: vCPU %d stopped with unexpected exit %s", vcpu.ID, exit))
			return
		}
	}
}

func (c *CpuManager) recordError(err error) {
	c.errMu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.errMu.Unlock()
}

// Wait blocks until every vCPU exits.
func (c *CpuManager) Wait() error {
	<-c.done
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.err
}

// Stop requests all vCPUs to leave guest execution.
func (c *CpuManager) Stop() {
	c.stopOnce.Do(func() {
		for _, vcpu := range c.vcpus {
			vcpu.Vcpu.SetImmediateExit(true)
		}
	})
}
