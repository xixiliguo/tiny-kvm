package hyper

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"slices"

	"github.com/xixiliguo/tiny-kvm/internal/cpuid"
)

// CPUID_FLAG_VALID_INDEX matches the constant used in the hypervisor interface.
const CpuidFlagValidIndex uint32 = 1 // Adjust actual bit-mask value as needed

// CPUID feature bits
const (
	TscDeadlineTimerEcxBit uint8 = 24 // TSC deadline timer ecx bit.
	HypervisorEcxBit       uint8 = 31 // Hypervisor ecx bit.
	VmxEcxBit              uint8 = 5  // VMX for Intel
	SvmEcxBit              uint8 = 2  // SVM for AMD
	MtrrEdxBit             uint8 = 12 // MTRR edx bit.
	InvariantTscEdxBit     uint8 = 8  // Invariant TSC bit on 0x8000_0007 EDX
	AmxBf16                uint8 = 22 // AMX tile computation on bfloat16 numbers
	AmxTile                uint8 = 24 // AMX tile load/store instructions
	AmxInt8                uint8 = 25 // AMX tile computation on 8-bit integers

	AmxFp16    uint8 = 21 // AMX tile computation on fp16 numbers
	AmxComplex uint8 = 8  // AMX tile computation on complex numbers

	AmxTilecfgBit          uint8 = 17 // AMX tile cfg state component bit
	AmxTiledataBit         uint8 = 18 // AMX tile data state component bit
	KvmFeatureMsiExtDestId uint8 = 15
)

type CpuidReg int

const (
	CpuidRegEAX CpuidReg = iota
	CpuidRegEBX
	CpuidRegECX
	CpuidRegEDX
)

type CpuidPatch struct {
	Function uint32
	Index    uint32
	FlagsBit *uint8
	EaxBit   *uint8
	EbxBit   *uint8
	EcxBit   *uint8
	EdxBit   *uint8
}

// GetCpuidReg searches for a CPUID entry and returns the requested register value.
// It returns a boolean status flag alongside the uint32 value to idiomatically mirror Rust's Option type.
func GetCpuidReg(cpuid []CpuIdEntry, function uint32, index uint32, reg CpuidReg) (uint32, bool) {
	for _, entry := range cpuid {
		if entry.Function == function && index == entry.Index {
			switch reg {
			case CpuidRegEAX:
				return entry.Eax, true
			case CpuidRegEBX:
				return entry.Ebx, true
			case CpuidRegECX:
				return entry.Ecx, true
			case CpuidRegEDX:
				return entry.Edx, true
			}
		}
	}
	return 0, false
}

// SetCpuidReg modifies an existing register entry. If not found, it appends a new entry.
// Passing a slice pointer (*[]CpuIdEntry) allows the underlying storage to expand.
func SetCpuidReg(cpuid *[]CpuIdEntry, function uint32, index uint32, reg CpuidReg, value uint32) {
	entryFound := false

	// Try updating in place first
	for i := range *cpuid {
		entry := &(*cpuid)[i]
		if entry.Function == function && index == entry.Index {
			entryFound = true
			switch reg {
			case CpuidRegEAX:
				entry.Eax = value
			case CpuidRegEBX:
				entry.Ebx = value
			case CpuidRegECX:
				entry.Ecx = value
			case CpuidRegEDX:
				entry.Edx = value
			}
		}
	}

	if entryFound {
		return
	}

	// Entry not found, append a new one
	entry := CpuIdEntry{
		Function: function,
		Index:    index,
		Flags:    CpuidFlagValidIndex,
	}

	switch reg {
	case CpuidRegEAX:
		entry.Eax = value
	case CpuidRegEBX:
		entry.Ebx = value
	case CpuidRegECX:
		entry.Ecx = value
	case CpuidRegEDX:
		entry.Edx = value
	}

	*cpuid = append(*cpuid, entry)

}

// PatchCpuid iterates through entries and flips bits defined inside the provided patches slice.
func PatchCpuid(cpuid []CpuIdEntry, patches []CpuidPatch) {
	for i := range cpuid {
		entry := &cpuid[i]
		for _, patch := range patches {
			if entry.Function == patch.Function && entry.Index == patch.Index {
				if patch.FlagsBit != nil {
					entry.Flags |= 1 << *patch.FlagsBit
				}
				if patch.EaxBit != nil {
					entry.Eax |= 1 << *patch.EaxBit
				}
				if patch.EbxBit != nil {
					entry.Ebx |= 1 << *patch.EbxBit
				}
				if patch.EcxBit != nil {
					entry.Ecx |= 1 << *patch.EcxBit
				}
				if patch.EdxBit != nil {
					entry.Edx |= 1 << *patch.EdxBit
				}
			}
		}
	}
}

// IsFeatureEnabled runs bitwise evaluation against specified register indexes to check for explicit capabilities.
func IsFeatureEnabled(cpuid []CpuIdEntry, function uint32, index uint32, reg CpuidReg, featureBit int) bool {
	mask := uint32(1 << featureBit)

	for _, entry := range cpuid {
		if entry.Function == function && entry.Index == index {
			var regVal uint32
			switch reg {
			case CpuidRegEAX:
				regVal = entry.Eax
			case CpuidRegEBX:
				regVal = entry.Ebx
			case CpuidRegECX:
				regVal = entry.Ecx
			case CpuidRegEDX:
				regVal = entry.Edx
			}

			return (regVal & mask) == mask
		}
	}

	return false
}

type CpuidConfig struct {
	PhysBits  uint8
	Amx       bool
	Tdx       bool
	KvmHyperv bool
}

type TdxCapabilities struct {
	XfamFixed0 uint64
	XfamFixed1 uint64
}

func GenerateCommonCpuid(hyper Hypervisor, PhysBits uint8, config *CpuidConfig) ([]CpuIdEntry, error) {
	// Check if running under a nested virtualization environment
	_, _, ecx, _ := cpuid.Cpuid(1, 0)
	if ecx&(1<<HypervisorEcxBit) == (1 << HypervisorEcxBit) {
		_, ebx, ecx, edx := cpuid.Cpuid(0x4000_0000, 0)

		identifier := make([]byte, 12)
		binary.LittleEndian.PutUint32(identifier[0:4], ebx)
		binary.LittleEndian.PutUint32(identifier[4:8], ecx)
		binary.LittleEndian.PutUint32(identifier[8:12], edx)

		slog.Info("Running under nested virtualisation", "hypervisor_string", string(identifier))
	}

	slog.Info("Generating guest CPUID with physical address size", "phys_bits", config.PhysBits)

	cpuidPatches := []CpuidPatch{
		// Patch hypervisor bit
		{
			Function: 1,
			Index:    0,
			EcxBit:   new(HypervisorEcxBit),
		},
		// Enable MTRR feature
		{
			Function: 1,
			Index:    0,
			EdxBit:   new(MtrrEdxBit),
		},
	}

	cpuidPatches = append(cpuidPatches, CpuidPatch{
		Function: 1,
		Index:    0,
		EcxBit:   new(TscDeadlineTimerEcxBit),
	})

	// Obtain initial supported features list from hypervisor interface
	cpuids, err := hyper.GetSupportedCpuID()
	if err != nil {
		return nil, fmt.Errorf("failed to get supported cpuid: %w", err)
	}

	PatchCpuid(cpuids, cpuidPatches)

	// Update specific existing CPUID features dynamically inside slice
	for i := range cpuids {
		entry := &cpuids[i]
		switch entry.Function {
		case 0x7:
			// disable amx
			if entry.Index == 0 {
				entry.Edx &^= (1 << AmxBf16) | (1 << AmxTile) | (1 << AmxInt8)
			}
			if entry.Index == 1 {
				entry.Eax &^= (1 << AmxFp16)
				entry.Edx &^= (1 << AmxComplex)
			}

		case 0x1d, 0x1e:
			// Clear Tile/TMUL Information completely if AMX is disabled

			entry.Eax = 0
			entry.Ebx = 0
			entry.Ecx = 0
			entry.Edx = 0

		case 0x8000_0005:
			// Copy host L1 cache details if unpopulated
			if entry.Eax == 0 && entry.Ebx == 0 && entry.Ecx == 0 && entry.Edx == 0 {
				if eax, _, _, _ := cpuid.Cpuid(0x8000_0000, 0); eax >= 0x8000_0005 {
					eax, ebx, ecx, edx := cpuid.Cpuid(0x8000_0005, 0)
					entry.Eax = eax
					entry.Ebx = ebx
					entry.Ecx = ecx
					entry.Edx = edx
				}
			}
		case 0x8000_0006:
			// Copy host L2 cache details if unpopulated
			if entry.Eax == 0 && entry.Ebx == 0 && entry.Ecx == 0 && entry.Edx == 0 {
				if eax, _, _, _ := cpuid.Cpuid(0x8000_0000, 0); eax >= 0x8000_0006 {
					eax, ebx, ecx, edx := cpuid.Cpuid(0x8000_0006, 0)
					entry.Eax = eax
					entry.Ebx = ebx
					entry.Ecx = ecx
					entry.Edx = edx
				}
			}
		case 0x8000_0008:
			// Set CPU physical address sizes
			entry.Eax = (entry.Eax & 0xffff_ff00) | (uint32(PhysBits) & 0xff)
		case 0x4000_0001:
			entry.Eax |= 1 << KvmFeatureMsiExtDestId

		}
	}

	// Copy brand/identification string leaves (0x80000002 to 0x80000004)
	for i := uint32(0x8000_0002); i <= 0x8000_0004; i++ {

		cpuids = slices.DeleteFunc(cpuids, func(c CpuIdEntry) bool { return c.Function == i })

		eax, ebx, ecx, edx := cpuid.Cpuid(i, 0)
		cpuids = append(cpuids, CpuIdEntry{
			Function: i,
			Eax:      eax,
			Ebx:      ebx,
			Ecx:      ecx,
			Edx:      edx,
		})
	}

	if config.KvmHyperv {
		// Hyper-V compliant leaf set. Keeping the KVM leaves would expose two
		// conflicting paravirtual interfaces to the guest.
		cpuids = slices.DeleteFunc(cpuids, func(c CpuIdEntry) bool { return c.Function == 0x4000_0000 })
		cpuids = slices.DeleteFunc(cpuids, func(c CpuIdEntry) bool { return c.Function == 0x4000_0001 })

		cpuids = append(cpuids, CpuIdEntry{
			Function: 0x40000000,
			Eax:      0x4000000a,
			Ebx:      0x756e694c, // "Linu"
			Ecx:      0x564b2078, // "x KV"
			Edx:      0x7648204d, // "M Hv"
		})
		cpuids = append(cpuids, CpuIdEntry{
			Function: 0x40000001,
			Eax:      0x31237648, // "Hv#1"
		})
		cpuids = append(cpuids, CpuIdEntry{
			Function: 0x40000002,
			Eax:      0x3839,
			Ebx:      0xa0000,
		})
		cpuids = append(cpuids, CpuIdEntry{
			Function: 0x4000_0003,
			Eax:      (1 << 1) | (1 << 2) | (1 << 3) | (1 << 9),
			Edx:      1 << 3,
		})
		cpuids = append(cpuids, CpuIdEntry{
			Function: 0x4000_0004,
			Eax:      1 << 5,
		})

		for i := uint32(0x4000_0005); i <= 0x4000_000a; i++ {
			cpuids = append(cpuids, CpuIdEntry{Function: i})
		}
	}

	return cpuids, nil
}
