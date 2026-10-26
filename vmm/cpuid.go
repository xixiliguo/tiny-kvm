package vmm

import (
	"fmt"
	"slices"

	"github.com/xixiliguo/tiny-kvm/hyper"
	"github.com/xixiliguo/tiny-kvm/internal/cpuid"
)

const (
	hypervisorEcxBit       = 31
	tscDeadlineTimerEcxBit = 24
	mtrrEdxBit             = 12
	vmxEcxBit              = 5
	svmEcxBit              = 2
	kvmMsiExtDestID        = 15
)

// generateGuestCPUID builds the common CPUID table locally. tiny-kvm keeps
// this logic in the VMM package so the hypervisor package remains a thin KVM
// binding.
func generateGuestCPUID(hv hyper.Hypervisor, physBits uint8, kvmHyperv bool) ([]hyper.CpuIdEntry, error) {
	entries, err := hv.GetSupportedCpuID()
	if err != nil {
		return nil, fmt.Errorf("cpuid: get host-supported leaves: %w", err)
	}
	for i := range entries {
		entry := &entries[i]
		switch entry.Function {
		case 1:
			entry.Ecx |= 1 << hypervisorEcxBit
			entry.Ecx |= 1 << tscDeadlineTimerEcxBit
			entry.Edx |= 1 << mtrrEdxBit
		case 7:
			if entry.Index == 0 {
				entry.Edx &^= (1 << 22) | (1 << 24) | (1 << 25)
			}
			if entry.Index == 1 {
				entry.Eax &^= 1 << 21
				entry.Edx &^= 1 << 8
			}
		case 0x1d, 0x1e:
			entry.Eax, entry.Ebx, entry.Ecx, entry.Edx = 0, 0, 0, 0
		case 0x8000_0005, 0x8000_0006:
			if entry.Eax|entry.Ebx|entry.Ecx|entry.Edx == 0 {
				copyHostCPUID(entry)
			}
		case 0x8000_0008:
			entry.Eax = (entry.Eax & 0xffff_ff00) | uint32(physBits&0xff)
		case 0x4000_0001:
			entry.Eax |= 1 << kvmMsiExtDestID
		}
	}

	for leaf := uint32(0x8000_0002); leaf <= 0x8000_0004; leaf++ {
		entries = slices.DeleteFunc(entries, func(entry hyper.CpuIdEntry) bool {
			return entry.Function == leaf
		})
		eax, ebx, ecx, edx := cpuid.Cpuid(leaf, 0)
		entries = append(entries, hyper.CpuIdEntry{
			Function: leaf,
			Eax:      eax,
			Ebx:      ebx,
			Ecx:      ecx,
			Edx:      edx,
		})
	}

	if kvmHyperv {
		entries = slices.DeleteFunc(entries, func(entry hyper.CpuIdEntry) bool {
			return entry.Function == 0x4000_0000 || entry.Function == 0x4000_0001
		})
		entries = append(entries,
			hyper.CpuIdEntry{Function: 0x4000_0000, Eax: 0x4000_000a, Ebx: 0x756e694c, Ecx: 0x564b2078, Edx: 0x7648204d},
			hyper.CpuIdEntry{Function: 0x4000_0001, Eax: 0x31237648},
			hyper.CpuIdEntry{Function: 0x4000_0002, Eax: 0x3839, Ebx: 0xa0000},
			hyper.CpuIdEntry{Function: 0x4000_0003, Eax: (1 << 1) | (1 << 2) | (1 << 3) | (1 << 9), Edx: 1 << 3},
			hyper.CpuIdEntry{Function: 0x4000_0004, Eax: 1 << 5},
		)
		for leaf := uint32(0x4000_0005); leaf <= 0x4000_000a; leaf++ {
			entries = append(entries, hyper.CpuIdEntry{Function: leaf})
		}
	}
	return entries, nil
}

func copyHostCPUID(entry *hyper.CpuIdEntry) {
	eax, _, _, _ := cpuid.Cpuid(0x8000_0000, 0)
	if eax < entry.Function {
		return
	}
	eax, ebx, ecx, edx := cpuid.Cpuid(entry.Function, 0)
	entry.Eax, entry.Ebx, entry.Ecx, entry.Edx = eax, ebx, ecx, edx
}

func patchVCPUID(cpuids []hyper.CpuIdEntry, id uint32, vendor hyper.CPUVendor) ([]hyper.CpuIdEntry, error) {
	hyper.SetCpuidReg(&cpuids, 0xb, 0, hyper.CpuidRegEDX, id)
	hyper.SetCpuidReg(&cpuids, 0x1f, 0, hyper.CpuidRegEDX, id)

	foundLeaf1 := false
	for i := range cpuids {
		entry := &cpuids[i]
		switch entry.Function {
		case 1:
			foundLeaf1 = true
			entry.Ebx = (entry.Ebx & 0x00ff_ffff) | (id << 24)
			if vendor == hyper.CPUVendorIntel {
				entry.Ecx &^= 1 << vmxEcxBit
			}
		case 0x8000_0001:
			if vendor == hyper.CPUVendorAMD {
				entry.Ecx &^= 1 << svmEcxBit
			}
		}
	}
	if !foundLeaf1 {
		return nil, fmt.Errorf("cpuid: leaf 1 is missing")
	}
	return cpuids, nil
}
