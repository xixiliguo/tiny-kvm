package allocator

import (
	"errors"
	"sort"
)

// --- Errors ---

var (
	ErrOverflow             = errors.New("overflow")
	ErrOverlap              = errors.New("overlap")
	ErrUnalignedAddress     = errors.New("unalignedAddress")
	ErrNoEnoughAddressSpace = errors.New("no enough address space")
	ErrInvalidAddressOrSize = errors.New("invalid address or size")
	ErrInvalidAlignment     = errors.New("invalid alignment")
)

// rangeEntry represents an allocated memory range.
type rangeEntry struct {
	address uint64
	size    uint64
}

// AddressAllocator manages allocating guest address ranges.
// Use AddressAllocator whenever an address range needs to be allocated to different users.
type AddressAllocator struct {
	base   uint64
	end    uint64
	ranges []rangeEntry // Kept sorted by rangeEntry.address to simulate Rust's BTreeMap
}

// NewAddressAllocator creates a new AddressAllocator for managing a range of addresses.
// Returns nil if base + size overflows a u64.
func NewAddressAllocator(base uint64, size uint64) *AddressAllocator {
	if size == 0 {
		return nil
	}

	// Calculate end address: base + size - 1
	end := base + size - 1
	if end < base {
		return nil // Overflow
	}

	allocator := &AddressAllocator{
		base:   base,
		end:    end,
		ranges: make([]rangeEntry, 0),
	}

	// Insert the last address as a zero size range.
	// This is our end of address space marker.
	markerAddr := base + size
	if markerAddr < base {
		return nil // Overflow
	}

	allocator.ranges = append(allocator.ranges, rangeEntry{address: markerAddr, size: 0})
	return allocator
}

func (a *AddressAllocator) alignAddress(address uint64, alignment uint64) uint64 {
	var alignAdjust uint64
	if address%alignment == 0 {
		alignAdjust = 0
	} else {
		alignAdjust = alignment - address%alignment
	}

	return address + alignAdjust
}

func (a *AddressAllocator) availableRange(reqAddress uint64, reqSize uint64, alignment uint64) (uint64, error) {
	alignedAddress := a.alignAddress(reqAddress, alignment)

	// The requested address should be aligned.
	if alignedAddress != reqAddress {
		return 0, ErrUnalignedAddress
	}

	// The aligned address should be within the address space range.
	if alignedAddress >= a.end || alignedAddress < a.base {
		return 0, ErrOverflow
	}

	prevEndAddress := a.base
	for _, entry := range a.ranges {
		if alignedAddress <= entry.address {
			// Do we overlap with the previous range?
			if prevEndAddress > alignedAddress {
				return 0, ErrOverlap
			}

			// Do we have enough space?
			if entry.address-alignedAddress < reqSize {
				return 0, ErrOverlap
			}

			return alignedAddress, nil
		}

		prevEndAddress = entry.address + entry.size
	}

	// We have not found a range that starts after the requested address,
	// despite having a marker at the end of our range.
	return 0, ErrOverflow
}

func (a *AddressAllocator) firstAvailableRange(reqSize uint64, alignment uint64) (uint64, error) {
	// Iterate through the ranges in reverse order (Rust's ranges.iter().rev())
	for i := len(a.ranges) - 1; i >= 0; i-- {
		entry := a.ranges[i]
		prevRangeIdx := i - 1

		var prevEndAddress uint64
		if prevRangeIdx < 0 {
			prevEndAddress = a.base
		} else {
			// Find the logical "previous" entry in reverse order, which is i - 1 in array space
			prevEntry := a.ranges[prevRangeIdx]
			prevEndAddress = prevEntry.address + prevEntry.size
		}

		// If we have enough space between this range and the previous one,
		// we return the start of this range minus the requested size.
		alignedPrevEnd := a.alignAddress(prevEndAddress, alignment)
		if entry.address >= alignedPrevEnd {
			sizeDelta := entry.address - alignedPrevEnd
			if sizeDelta >= reqSize {
				adjust := alignment - 1
				if alignment == 0 {
					adjust = 0
				}

				// saturating_sub protection for reqSize + adjust
				totalSub := reqSize + adjust
				if totalSub < reqSize { // Check overflow
					continue
				}

				subTarget := entry.address - totalSub
				return a.alignAddress(subTarget, alignment), nil
			}
		}
	}

	return 0, ErrNoEnoughAddressSpace
}

// Allocate allocations a range of addresses from the managed region. Returns (allocatedAddress, true)
// when successful, or (0, false) if an area of size can't be allocated or if alignment isn't a power of two.
func (a *AddressAllocator) Allocate(address uint64, size uint64, alignSize uint64) (uint64, error) {
	if size == 0 {
		return 0, ErrInvalidAddressOrSize
	}

	alignment := uint64(4)
	if alignSize != 0 {
		alignment = alignSize
	}

	// Check power of two and non-zero
	if alignment == 0 || (uint64(alignment)&(uint64(alignment)-1)) != 0 {
		return 0, ErrInvalidAlignment
	}

	var newAddr uint64
	if address != 0 {
		addr, err := a.availableRange(address, size, alignment)
		if err != nil {
			return 0, err
		}
		newAddr = addr
	} else {
		addr, err := a.firstAvailableRange(size, alignment)
		if err != nil {
			return 0, err
		}
		newAddr = addr
	}

	// Insert the new allocated range into sorted list
	a.insertRange(newAddr, size)

	return newAddr, nil
}

// Free an already allocated address range.
// We can only free a range if it matches exactly an already allocated range.
func (a *AddressAllocator) Free(address uint64, size uint64) {
	idx := sort.Search(len(a.ranges), func(i int) bool {
		return a.ranges[i].address >= address
	})

	if idx < len(a.ranges) && a.ranges[idx].address == address {
		if a.ranges[idx].size == size {
			// Remove the entry from slice
			a.ranges = append(a.ranges[:idx], a.ranges[idx+1:]...)
		}
	}
}

// Base returns the start address of the allocator.
func (a *AddressAllocator) Base() uint64 {
	return a.base
}

// End returns the last address of the allocator.
func (a *AddressAllocator) End() uint64 {
	return a.end
}

// Helper to insert a range entry keeping the internal slice sorted.
func (a *AddressAllocator) insertRange(address uint64, size uint64) {
	idx := sort.Search(len(a.ranges), func(i int) bool {
		return a.ranges[i].address >= address
	})

	// Inject at sorted index
	a.ranges = append(a.ranges, rangeEntry{})
	copy(a.ranges[idx+1:], a.ranges[idx:])
	a.ranges[idx] = rangeEntry{address: address, size: size}
}
