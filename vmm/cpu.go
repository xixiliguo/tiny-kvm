package vmm

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/xixiliguo/tiny-kvm/arch"
	"github.com/xixiliguo/tiny-kvm/hyper"
)

// VCPU is a tiny-kvm vCPU and its immutable guest id.
type VCPU struct {
	ID   uint32
	Vcpu hyper.Vcpu
	tid  atomic.Int32
}

// CpuManager owns all boot vCPUs and the common CPUID table.
type CpuManager struct {
	hyper       hyper.Hypervisor
	vm          hyper.Vm
	ops         hyper.VmOps
	config      CpusConfig
	commonCPUID []hyper.CpuIdEntry
	vcpus       []*VCPU
	mu          sync.Mutex
	wg          sync.WaitGroup
	done        chan struct{}
	running     bool
	errMu       sync.Mutex
	err         error
}

var (
	stopSignalOnce    sync.Once
	stopSignalChannel = make(chan os.Signal, 1)
)

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
	cpuid, err := hyper.GenerateCommonCpuid(hv, physBits, &hyper.CpuidConfig{
		PhysBits:  physBits,
		KvmHyperv: config.KvmHyperv,
	})
	if err != nil {
		return nil, fmt.Errorf("cpu: generate CPUID: %w", err)
	}

	manager := &CpuManager{
		hyper:       hv,
		vm:          vm,
		ops:         ops,
		config:      config,
		commonCPUID: cpuid,
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
		cpuid := slices.Clone(c.commonCPUID)
		if err := arch.ConfigureVcpu(
			vcpu.Vcpu,
			vcpu.ID,
			boot,
			cpuid,
			c.config.KvmHyperv,
			c.hyper.GetCPUVendor(),
		); err != nil {
			return fmt.Errorf("cpu: configure vCPU %d: %w", vcpu.ID, err)
		}
	}
	return nil
}

// VCPUs returns the configured vCPU handles.
func (c *CpuManager) VCPUs() []*VCPU {
	return append([]*VCPU(nil), c.vcpus...)
}

// Start launches all boot vCPUs.
func (c *CpuManager) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return errors.New("cpu: vCPUs have already been started")
	}
	stopSignalOnce.Do(func() {
		signal.Notify(stopSignalChannel, syscall.SIGUSR1)
		go func() {
			for range stopSignalChannel {
			}
		}()
	})
	c.running = true
	c.done = make(chan struct{})
	done := c.done
	for _, vcpu := range c.vcpus {
		c.wg.Add(1)
		go c.run(vcpu)
	}
	go func() {
		c.wg.Wait()
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		close(done)
	}()
	return nil
}

func (c *CpuManager) run(vcpu *VCPU) {
	defer c.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	vcpu.tid.Store(int32(syscall.Gettid()))
	defer vcpu.tid.Store(0)
	for {
		exit, err := vcpu.Vcpu.Run()
		if err != nil {
			c.recordError(fmt.Errorf("cpu: vCPU %d run: %w", vcpu.ID, err))
			c.Stop()
			return
		}
		switch exit {
		case hyper.VmExitIgnore, hyper.VmExitIoapicEoi:
			continue
		case hyper.VmExitReset, hyper.VmExitShutdown:
			c.Stop()
			return
		default:
			c.recordError(fmt.Errorf("cpu: vCPU %d stopped with unexpected exit %s", vcpu.ID, exit))
			c.Stop()
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
	c.mu.Lock()
	done := c.done
	c.mu.Unlock()
	if done == nil {
		return errors.New("cpu: vCPUs have not been started")
	}
	<-done
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.err
}

// Stop requests all vCPUs to leave guest execution.
func (c *CpuManager) Stop() {
	for _, vcpu := range c.vcpus {
		vcpu.Vcpu.SetImmediateExit(true)
		if tid := vcpu.tid.Load(); tid > 0 {
			_ = syscall.Tgkill(os.Getpid(), int(tid), syscall.SIGUSR1)
		}
	}
}

// ResetStop clears the immediate-exit state before the vCPUs are run again.
func (c *CpuManager) ResetStop() {
	for _, vcpu := range c.vcpus {
		vcpu.Vcpu.SetImmediateExit(false)
	}
}
