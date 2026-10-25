package vmm

import (
	"fmt"
	"sort"
	"sync"

	"github.com/xixiliguo/tiny-kvm/device"
)

// Bus is a non-overlapping address bus for PIO or MMIO devices.
type Bus struct {
	mu      sync.RWMutex
	entries []*busEntry
}

type busEntry struct {
	id     string
	base   uint64
	size   uint64
	device device.BusDevice
}

// NewBus returns an empty device bus.
func NewBus() *Bus { return &Bus{} }

// Insert registers a device in an address range.
func (b *Bus) Insert(id string, base, size uint64, dev device.BusDevice) error {
	if dev == nil {
		return errorsForBus("nil device")
	}
	if size == 0 {
		return errorsForBus("zero-sized range")
	}
	if base+size < base {
		return errorsForBus("address range overflow")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for _, existing := range b.entries {
		if base < existing.base+existing.size && existing.base < base+size {
			return fmt.Errorf("bus: range [0x%x, 0x%x) overlaps %q at [0x%x, 0x%x)",
				base, base+size, existing.id, existing.base, existing.base+existing.size)
		}
	}
	b.entries = append(b.entries, &busEntry{id: id, base: base, size: size, device: dev})
	sort.Slice(b.entries, func(i, j int) bool { return b.entries[i].base < b.entries[j].base })
	return nil
}

// Read dispatches a read. Unmapped reads are zero-filled, which mirrors a
// floating bus and prevents speculative accesses from stalling the guest.
func (b *Bus) Read(address uint64, data []byte) {
	entry, offset := b.lookup(address, uint64(len(data)))
	if entry == nil {
		clear(data)
		return
	}
	entry.device.Read(entry.base, offset, data)
}

// Write dispatches a write.
func (b *Bus) Write(address uint64, data []byte) error {
	entry, offset := b.lookup(address, uint64(len(data)))
	if entry == nil {
		return nil
	}
	return entry.device.Write(entry.base, offset, data)
}

func (b *Bus) lookup(address, length uint64) (*busEntry, uint64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	index := sort.Search(len(b.entries), func(i int) bool {
		return b.entries[i].base+b.entries[i].size > address
	})
	if index >= len(b.entries) {
		return nil, 0
	}
	entry := b.entries[index]
	if address < entry.base || address+length > entry.base+entry.size || address+length < address {
		return nil, 0
	}
	return entry, address - entry.base
}

func errorsForBus(message string) error {
	return fmt.Errorf("bus: %s", message)
}
