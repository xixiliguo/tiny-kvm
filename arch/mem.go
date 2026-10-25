package arch

import (
	"math"
)

// GuestMemoryMmap models the abstracted guest memory wrapper interface.
type GuestMemoryMmap interface {
	LastAddr() uint64
	WriteSlice(slice []byte, address uint64) (int, error)
	WriteObj(obj any, address uint64) (int, error)
	GetRamRange() ([]RamRange, error)
	FindRamRange(index uint64) (RamRange, bool)
}

type MemType int

const (
	E820Ram      MemType = 1
	E820Reserved MemType = 2
)

type MemoryRegion struct {
	GuestAddress uint64
	Size         uint64
	Type         MemType
}

func ArchMemoryRegions() []MemoryRegion {
	regions := []MemoryRegion{
		// 0 GiB ~ 3GiB: memory before the gap
		{
			0,
			Mem32BitReservedStart,
			E820Ram,
		},
		// 4 GiB ~ inf: memory after the gap
		{Ram64BitStart, math.MaxUint64, E820Ram},
		// 3 GiB ~ 3712 MiB: 32-bit device memory hole
		{
			Mem32BitReservedStart,
			Mem32BitDevicesSize,
			E820Reserved,
		},
		// 3712 MiB ~ 3968 MiB: 32-bit reserved memory hole
		{
			Mem32BitReservedStart + Mem32BitDevicesSize,
			Mem32BitReservedSize - Mem32BitDevicesSize,
			E820Reserved,
		},
	}
	return regions
}
