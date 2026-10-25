// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0 OR BSD-3-Clause

//go:build amd64

package allocator

import "os"

// SystemAllocator manages allocating system resources such as address space and interrupt numbers.
type SystemAllocator struct {
	ioAddressSpace           *AddressAllocator
	platformMmioAddressSpace *AddressAllocator
	gsiAllocator             *GsiAllocator
}

// NewSystemAllocator creates a new SystemAllocator for managing addresses and irq numbers on x86_64.
func NewSystemAllocator(
	ioBase uint64,
	ioSize uint64,
	platformMmioBase uint64,
	platformMmioSize uint64,
	apics []GsiApic,
) *SystemAllocator {
	ioSpace := NewAddressAllocator(ioBase, ioSize)
	if ioSpace == nil {
		return nil
	}

	mmioSpace := NewAddressAllocator(platformMmioBase, platformMmioSize)
	if mmioSpace == nil {
		return nil
	}

	return &SystemAllocator{
		ioAddressSpace:           ioSpace,
		platformMmioAddressSpace: mmioSpace,
		gsiAllocator:             NewGsiAllocator(apics),
	}
}

// AllocateIoAddresses reserves a section of size bytes of IO address space.
func (s *SystemAllocator) AllocateIoAddresses(address uint64, size uint64, alignSize uint64) (uint64, error) {
	alignment := uint64(1)
	if alignSize != 0 {
		alignment = alignSize
	}
	return s.ioAddressSpace.Allocate(address, size, alignment)
}

// FreeIoAddresses frees an IO address range.
// We can only free a range if it matches exactly an already allocated range.
func (s *SystemAllocator) FreeIoAddresses(address uint64, size uint64) {
	s.ioAddressSpace.Free(address, size)
}

// AllocateIrq reserves the next available system irq number.
func (s *SystemAllocator) AllocateIrq() (uint32, error) {
	return s.gsiAllocator.AllocateIrq()
}

// AllocateGsi reserves the next available GSI.
func (s *SystemAllocator) AllocateGsi() (uint32, error) {
	return s.gsiAllocator.AllocateGsi()
}

// AllocatePlatformMmioAddresses reserves a section of size bytes of platform MMIO address space.
func (s *SystemAllocator) AllocatePlatformMmioAddresses(address uint64, size uint64, alignSize uint64) (uint64, error) {
	alignment := uint64(os.Getpagesize()) // Fallback to system page size if alignSize is nil (Rust'get_page_size`)
	if alignSize != 0 {
		alignment = alignSize
	}
	return s.platformMmioAddressSpace.Allocate(address, size, alignment)
}

// FreePlatformMmioAddresses frees a platform MMIO address range.
// We can only free a range if it matches exactly an already allocated range.
func (s *SystemAllocator) FreePlatformMmioAddresses(address uint64, size uint64) {
	s.platformMmioAddressSpace.Free(address, size)
}
