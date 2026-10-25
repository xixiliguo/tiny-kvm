package vmm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"unsafe"

	"github.com/xixiliguo/tiny-kvm/arch"
	"github.com/xixiliguo/tiny-kvm/hyper"
	"golang.org/x/sys/unix"
)

// GuestRegion is one contiguous guest physical RAM mapping.
type GuestRegion struct {
	base uint64
	data []byte
}

// Base returns the guest physical start address.
func (r *GuestRegion) Base() uint64 { return r.base }

// Size returns the region length in bytes.
func (r *GuestRegion) Size() uint64 { return uint64(len(r.data)) }

// Bytes exposes the host mapping for KVM registration and tests.
func (r *GuestRegion) Bytes() []byte { return r.data }

// GuestMemory is a sparse collection of guest physical RAM regions.
//
// It implements arch.GuestMemoryMmap. Accessors are safe for concurrent use
// for the lifetime of the mapping.
type GuestMemory struct {
	mu      sync.RWMutex
	regions []*GuestRegion
}

// NewGuestMemory allocates guest RAM without registering it with a
// hypervisor. It is primarily useful for configuration and boot-layout tests.
func NewGuestMemory(size uint64) (*GuestMemory, error) {
	if size == 0 {
		return nil, errors.New("memory: size must be greater than zero")
	}
	if size%uint64(os.Getpagesize()) != 0 {
		return nil, errors.New("memory: size must be page aligned")
	}

	memory := &GuestMemory{}
	lowSize := min(size, arch.Mem32BitReservedStart)
	if lowSize > 0 {
		data, err := unix.Mmap(-1, 0, int(lowSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_ANONYMOUS)
		if err != nil {
			return nil, fmt.Errorf("memory: mmap low RAM: %w", err)
		}
		memory.regions = append(memory.regions, &GuestRegion{base: 0, data: data})
	}
	if highSize := size - lowSize; highSize > 0 {
		if highSize > uint64(math.MaxInt) {
			memory.Close()
			return nil, errors.New("memory: region is too large")
		}
		data, err := unix.Mmap(-1, 0, int(highSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_ANONYMOUS)
		if err != nil {
			memory.Close()
			return nil, fmt.Errorf("memory: mmap high RAM: %w", err)
		}
		memory.regions = append(memory.regions, &GuestRegion{base: arch.Ram64BitStart, data: data})
	}
	return memory, nil
}

// Regions returns the guest physical RAM layout.
func (m *GuestMemory) Regions() []*GuestRegion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*GuestRegion(nil), m.regions...)
}

// LastAddr implements arch.GuestMemoryMmap.
func (m *GuestMemory) LastAddr() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var last uint64
	for _, region := range m.regions {
		end := region.base + uint64(len(region.data)) - 1
		if end > last {
			last = end
		}
	}
	return last
}

// WriteSlice writes bytes at a guest physical address.
func (m *GuestMemory) WriteSlice(data []byte, address uint64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	remaining := data
	current := address
	written := 0
	for len(remaining) > 0 {
		region := m.findRegionLocked(current, uint64(len(remaining)))
		if region == nil || current < region.base {
			return written, fmt.Errorf("memory: unmapped guest write at 0x%x (%d bytes)", current, len(remaining))
		}
		offset := current - region.base
		count := min(uint64(len(remaining)), uint64(len(region.data))-offset)
		copy(region.data[offset:offset+count], remaining[:count])
		current += count
		written += int(count)
		remaining = remaining[count:]
	}
	return written, nil
}

// ReadSlice reads bytes from a guest physical address.
func (m *GuestMemory) ReadSlice(data []byte, address uint64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	remaining := data
	current := address
	read := 0
	for len(remaining) > 0 {
		region := m.findRegionLocked(current, uint64(len(remaining)))
		if region == nil || current < region.base {
			return read, fmt.Errorf("memory: unmapped guest read at 0x%x (%d bytes)", current, len(remaining))
		}
		offset := current - region.base
		count := min(uint64(len(remaining)), uint64(len(region.data))-offset)
		copy(remaining[:count], region.data[offset:offset+count])
		current += count
		read += int(count)
		remaining = remaining[count:]
	}
	return read, nil
}

// WriteObj serializes an object and writes it to guest memory.
func (m *GuestMemory) WriteObj(obj any, address uint64) (int, error) {
	var buffer bytes.Buffer
	if err := binary.Write(&buffer, binary.LittleEndian, obj); err != nil {
		return 0, fmt.Errorf("memory: serialize object: %w", err)
	}
	return m.WriteSlice(buffer.Bytes(), address)
}

// GetRamRange implements arch.GuestMemoryMmap.
func (m *GuestMemory) GetRamRange() ([]arch.RamRange, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ranges := make([]arch.RamRange, 0, len(m.regions))
	for _, region := range m.regions {
		ranges = append(ranges, arch.RamRange{
			Start: region.base,
			End:   region.base + uint64(len(region.data)),
		})
	}
	return ranges, nil
}

// FindRamRange implements arch.GuestMemoryMmap.
func (m *GuestMemory) FindRamRange(address uint64) (arch.RamRange, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, region := range m.regions {
		end := region.base + uint64(len(region.data))
		if address >= region.base && address < end {
			return arch.RamRange{Start: region.base, End: end}, true
		}
	}
	return arch.RamRange{}, false
}

func (m *GuestMemory) findRegionLocked(address, length uint64) *GuestRegion {
	for _, region := range m.regions {
		end := region.base + uint64(len(region.data))
		if address >= region.base && address < end {
			// A multi-region crossing is handled by the caller one slice at a
			// time, so only the containing region is required here.
			_ = length
			return region
		}
	}
	return nil
}

// Close releases all anonymous mappings.
func (m *GuestMemory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var first error
	for _, region := range m.regions {
		if region.data == nil {
			continue
		}
		if err := unix.Munmap(region.data); err != nil && first == nil {
			first = err
		}
		region.data = nil
	}
	m.regions = nil
	return first
}

// MemoryManager owns guest RAM and its KVM user memory slots.
type MemoryManager struct {
	vm       hyper.Vm
	guest    *GuestMemory
	nextSlot uint32
	mu       sync.Mutex
	closed   bool
}

// NewMemoryManager allocates and registers guest RAM with KVM.
func NewMemoryManager(vm hyper.Vm, config MemoryConfig) (*MemoryManager, error) {
	guest, err := NewGuestMemory(config.Size)
	if err != nil {
		return nil, err
	}
	manager := &MemoryManager{vm: vm, guest: guest}
	for _, region := range guest.Regions() {
		address := uint64(uintptr(unsafe.Pointer(&region.data[0])))
		if err := vm.CreateUserMemoryRegion(manager.nextSlot, region.base, len(region.data), address); err != nil {
			_ = guest.Close()
			return nil, fmt.Errorf("memory: register KVM slot %d: %w", manager.nextSlot, err)
		}
		manager.nextSlot++
	}
	return manager, nil
}

// GuestMemory returns the shared guest memory object.
func (m *MemoryManager) GuestMemory() *GuestMemory { return m.guest }

// Size returns the configured RAM size.
func (m *MemoryManager) Size() uint64 {
	var total uint64
	for _, region := range m.guest.Regions() {
		total += region.Size()
	}
	return total
}

// Close releases guest mappings.
func (m *MemoryManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	return m.guest.Close()
}

var _ arch.GuestMemoryMmap = (*GuestMemory)(nil)
