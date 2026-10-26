package vmm

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/xixiliguo/tiny-kvm/arch"
	"github.com/xixiliguo/tiny-kvm/hyper"
)

type fakeHypervisor struct {
	cpuids []hyper.CpuIdEntry
	vm     *fakeVM
}

func (h *fakeHypervisor) HypervisorType() hyper.HypervisorType { return hyper.HypervisorTypeKVM }
func (h *fakeHypervisor) GetSupportedCpuID() ([]hyper.CpuIdEntry, error) {
	return append([]hyper.CpuIdEntry(nil), h.cpuids...), nil
}
func (h *fakeHypervisor) CheckRequiredExtensions() error { return nil }
func (h *fakeHypervisor) GetMaxVCPUs() uint32            { return 64 }
func (h *fakeHypervisor) GetCPUVendor() hyper.CPUVendor  { return hyper.CPUVendorIntel }
func (h *fakeHypervisor) CreateVM(uint64) (hyper.Vm, error) {
	h.vm = &fakeVM{}
	return h.vm, nil
}

type fakeSlot struct {
	base uint64
	size int
	addr uint64
}

type fakeVM struct {
	slots []fakeSlot
	vcpus []*fakeVcpu
}

func (v *fakeVM) SetIdentityMapAddress(uint64) error { return nil }
func (v *fakeVM) SetTssAddress(uint64) error         { return nil }
func (v *fakeVM) CreateIrqChip() error               { return nil }
func (v *fakeVM) RegisterIrqfd(*hyper.EventFd, uint32) error {
	return nil
}
func (v *fakeVM) UnregisterIrqfd(*hyper.EventFd, uint32) error { return nil }
func (v *fakeVM) RegisterIoevent(*hyper.EventFd, uint64) error { return nil }
func (v *fakeVM) UnregisterIoevent(*hyper.EventFd, uint64) error {
	return nil
}
func (v *fakeVM) MakeRoutingEntry(gsi uint32, _ *hyper.MsiIrqSourceConfig) hyper.IrqRoutingEntry {
	return hyper.IrqRoutingEntry{GSI: gsi}
}
func (v *fakeVM) SetGsiRouting([]hyper.IrqRoutingEntry) error { return nil }
func (v *fakeVM) CreateUserMemoryRegion(slot uint32, base uint64, size int, addr uint64) error {
	v.slots = append(v.slots, fakeSlot{base: base, size: size, addr: addr})
	return nil
}
func (v *fakeVM) EnableSplitIrq() error { return nil }
func (v *fakeVM) EnableX2apicApi() error {
	return nil
}
func (v *fakeVM) CreateVcpu(id uint32, ops hyper.VmOps) (hyper.Vcpu, error) {
	vcpu := &fakeVcpu{id: id, ops: ops, lapic: hyper.LapicState{}}
	v.vcpus = append(v.vcpus, vcpu)
	return vcpu, nil
}

type fakeVcpu struct {
	id    uint32
	ops   hyper.VmOps
	cpuid []hyper.CpuIdEntry
	sregs hyper.SpecialRegisters
	lapic hyper.LapicState
	regs  hyper.StandardRegisters
	exit  bool
}

func (v *fakeVcpu) CreateStandardRegs() hyper.StandardRegisters { return hyper.StandardRegisters{} }
func (v *fakeVcpu) GetRegs() (hyper.StandardRegisters, error)   { return v.regs, nil }
func (v *fakeVcpu) SetRegs(regs *hyper.StandardRegisters) error {
	v.regs = *regs
	return nil
}
func (v *fakeVcpu) GetSregs() (hyper.SpecialRegisters, error) { return v.sregs, nil }
func (v *fakeVcpu) SetSregs(sregs *hyper.SpecialRegisters) error {
	v.sregs = *sregs
	return nil
}
func (v *fakeVcpu) GetFpu() (hyper.FpuState, error) { return hyper.FpuState{}, nil }
func (v *fakeVcpu) SetFpu(*hyper.FpuState) error    { return nil }
func (v *fakeVcpu) SetCpuid2(cpuid []hyper.CpuIdEntry) error {
	v.cpuid = append([]hyper.CpuIdEntry(nil), cpuid...)
	return nil
}
func (v *fakeVcpu) GetCpuid2() ([]hyper.CpuIdEntry, error) {
	return append([]hyper.CpuIdEntry(nil), v.cpuid...), nil
}
func (v *fakeVcpu) GetLapic() (hyper.LapicState, error) { return v.lapic, nil }
func (v *fakeVcpu) SetLapic(lapic *hyper.LapicState) error {
	v.lapic = *lapic
	return nil
}
func (v *fakeVcpu) GetMsrs() ([]hyper.MsrEntry, error) { return nil, nil }
func (v *fakeVcpu) SetMsrs([]hyper.MsrEntry) error     { return nil }
func (v *fakeVcpu) BootMsrEntries() []hyper.MsrEntry   { return nil }
func (v *fakeVcpu) Run() (hyper.VmExit, error) {
	if v.exit {
		return hyper.VmExitShutdown, nil
	}
	v.exit = true
	return hyper.VmExitShutdown, nil
}
func (v *fakeVcpu) SetImmediateExit(exit bool) { v.exit = exit }
func (v *fakeVcpu) Nmi() error                 { return nil }

func TestNewVMConfiguresCompleteBootPath(t *testing.T) {
	dir := t.TempDir()
	kernelPath := filepath.Join(dir, "bzImage")
	kernel := syntheticBzImage()
	if err := os.WriteFile(kernelPath, kernel, 0o600); err != nil {
		t.Fatal(err)
	}

	hv := &fakeHypervisor{cpuids: []hyper.CpuIdEntry{
		{Function: 1, Ecx: 1 << hyper.HypervisorEcxBit},
		{Function: 0x80000008, Eax: 48},
	}}
	config := DefaultVmConfig()
	config.Cpus.BootVcpus = 2
	config.Cpus.MaxVcpus = 2
	config.Memory.Size = 64 << 20
	config.Payload.Kernel = kernelPath
	config.Serial.Mode = ConsoleOutputNull

	vm, err := NewVM(hv, config)
	if err != nil {
		t.Fatal(err)
	}
	defer vm.Close()

	if vm.State() != VmStateCreated {
		t.Fatalf("state = %s", vm.State())
	}
	if len(hv.vm.slots) != 1 || hv.vm.slots[0].base != 0 || hv.vm.slots[0].size != 64<<20 {
		t.Fatalf("KVM memory slots = %+v", hv.vm.slots)
	}
	if len(hv.vm.vcpus) != 2 {
		t.Fatalf("created %d vCPUs", len(hv.vm.vcpus))
	}
	for i, vcpu := range hv.vm.vcpus {
		if vcpu.regs.Rip != arch.HighRamStart {
			t.Fatalf("vCPU %d RIP = 0x%x", i, vcpu.regs.Rip)
		}
		apicID, ok := hyper.GetCpuidReg(vcpu.cpuid, 0xb, 0, hyper.CpuidRegEDX)
		if !ok || apicID != uint32(i) {
			t.Fatalf("vCPU %d x2APIC id = %d, present=%v, leaves=%+v", i, apicID, ok, vcpu.cpuid)
		}
	}
	if err := vm.Boot(); err != nil {
		t.Fatal(err)
	}
	if err := vm.Wait(); err != nil {
		t.Fatal(err)
	}
	if vm.State() != VmStateShutdown {
		t.Fatalf("state after run = %s", vm.State())
	}
}

func syntheticBzImage() []byte {
	header := arch.SetupHeader{
		SetupSects:   4,
		Header:       setupHeaderMagic,
		Version:      0x020f,
		Code32Start:  uint32(arch.HighRamStart),
		Loadflags:    1,
		MinAlignment: 0x20,
	}
	image := make([]byte, 5*512+64)
	copy(image[setupHeaderOffset:], header.Bytes())
	binary.LittleEndian.PutUint32(image[5*512:], 0xf4)
	return image
}
