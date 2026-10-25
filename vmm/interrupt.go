package vmm

import (
	"fmt"
	"sync"

	"github.com/xixiliguo/tiny-kvm/allocator"
	"github.com/xixiliguo/tiny-kvm/device"
	"github.com/xixiliguo/tiny-kvm/hyper"
)

// InterruptManager owns KVM irqfds and GSI routing for legacy and MSI
// interrupt sources.
type InterruptManager struct {
	mu     sync.Mutex
	vm     hyper.Vm
	gsi    *allocator.GsiAllocator
	legacy map[uint32]*LegacyInterruptGroup
	msi    map[uint32]*MsiInterruptGroup
	closed bool
}

// NewInterruptManager creates an interrupt manager with the standard x86 IOAPIC
// GSI range.
func NewInterruptManager(vm hyper.Vm) *InterruptManager {
	return &InterruptManager{
		vm:     vm,
		gsi:    allocator.NewGsiAllocator([]allocator.GsiApic{allocator.NewGsiApic(0, device.NumIoapicPins)}),
		legacy: make(map[uint32]*LegacyInterruptGroup),
		msi:    make(map[uint32]*MsiInterruptGroup),
	}
}

// AllocateGsi returns a free GSI.
func (m *InterruptManager) AllocateGsi() (uint32, error) {
	return m.gsi.AllocateGsi()
}

// AllocateIrq returns a free legacy IOAPIC pin.
func (m *InterruptManager) AllocateIrq() (uint32, error) {
	return m.gsi.AllocateIrq()
}

// LegacyGroup returns the irqfd group for a GSI, creating it on first use.
func (m *InterruptManager) LegacyGroup(gsi uint32) (*LegacyInterruptGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, fmt.Errorf("interrupt: manager is closed")
	}
	if group := m.legacy[gsi]; group != nil {
		return group, nil
	}
	event, err := hyper.NewEventFd(fmt.Sprintf("gsi-%d", gsi))
	if err != nil {
		return nil, fmt.Errorf("interrupt: create eventfd for GSI %d: %w", gsi, err)
	}
	if err := m.vm.RegisterIrqfd(event, gsi); err != nil {
		_ = event.Close()
		return nil, fmt.Errorf("interrupt: register irqfd for GSI %d: %w", gsi, err)
	}
	group := &LegacyInterruptGroup{gsi: gsi, event: event}
	m.legacy[gsi] = group
	return group, nil
}

// MsiGroup returns an MSI source group for a GSI.
func (m *InterruptManager) MsiGroup(gsi uint32) (*MsiInterruptGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, fmt.Errorf("interrupt: manager is closed")
	}
	if group := m.msi[gsi]; group != nil {
		return group, nil
	}
	event, err := hyper.NewEventFd(fmt.Sprintf("msi-%d", gsi))
	if err != nil {
		return nil, fmt.Errorf("interrupt: create MSI eventfd for GSI %d: %w", gsi, err)
	}
	if err := m.vm.RegisterIrqfd(event, gsi); err != nil {
		_ = event.Close()
		return nil, fmt.Errorf("interrupt: register MSI irqfd for GSI %d: %w", gsi, err)
	}
	group := &MsiInterruptGroup{
		manager: m,
		gsi:     gsi,
		event:   event,
	}
	m.msi[gsi] = group
	return group, nil
}

// Close releases all irqfds.
func (m *InterruptManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	var first error
	for gsi, group := range m.legacy {
		if err := m.vm.UnregisterIrqfd(group.event, gsi); err != nil && first == nil {
			first = err
		}
		if err := group.event.Close(); err != nil && first == nil {
			first = err
		}
	}
	for gsi, group := range m.msi {
		if err := m.vm.UnregisterIrqfd(group.event, gsi); err != nil && first == nil {
			first = err
		}
		if err := group.event.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// LegacyInterruptController is the subset used by the 8250 serial device.
type LegacyInterruptController struct {
	manager *InterruptManager
}

// NewLegacyInterruptController creates a controller backed by KVM irqfds.
func NewLegacyInterruptController(manager *InterruptManager) *LegacyInterruptController {
	return &LegacyInterruptController{manager: manager}
}

// EndOfInterrupt is a no-op because the in-kernel IOAPIC tracks EOI.
func (c *LegacyInterruptController) EndOfInterrupt(uint8) {}

// ServiceIrq raises one legacy IOAPIC pin.
func (c *LegacyInterruptController) ServiceIrq(irq int) error {
	if irq < 0 || irq >= device.NumIoapicPins {
		return fmt.Errorf("interrupt: legacy IRQ %d is out of range", irq)
	}
	group, err := c.manager.LegacyGroup(uint32(irq))
	if err != nil {
		return err
	}
	return group.Trigger(uint32(irq))
}

// LegacyInterruptGroup wraps one KVM irqfd.
type LegacyInterruptGroup struct {
	gsi   uint32
	event *hyper.EventFd
}

// Trigger raises the interrupt by incrementing its eventfd.
func (g *LegacyInterruptGroup) Trigger(uint32) error {
	return g.event.Write(1)
}

// GSI returns the group's Global System Interrupt number.
func (g *LegacyInterruptGroup) GSI() uint32 { return g.gsi }

// MsiInterruptGroup implements device.InterruptSourceGroup.
type MsiInterruptGroup struct {
	manager *InterruptManager
	gsi     uint32
	event   *hyper.EventFd
	config  *hyper.MsiIrqSourceConfig
	masked  bool
}

// Enable unmasks the source.
func (g *MsiInterruptGroup) Enable() error {
	g.masked = false
	return g.manager.refreshMsiRoutes()
}

// Disable masks the source.
func (g *MsiInterruptGroup) Disable() error {
	g.masked = true
	return g.manager.refreshMsiRoutes()
}

// Trigger injects one MSI event.
func (g *MsiInterruptGroup) Trigger(uint32) error {
	if g.masked {
		return nil
	}
	return g.event.Write(1)
}

// Update changes the MSI address/data and routing entry.
func (g *MsiInterruptGroup) Update(_ uint32, config device.MsiIrqSourceConfig, masked bool, setGsi bool) error {
	g.config = &hyper.MsiIrqSourceConfig{
		HighAddr: config.HighAddr,
		LowAddr:  config.LowAddr,
		Data:     config.Data,
		DevID:    config.DevID,
	}
	g.masked = masked
	if setGsi {
		return g.manager.refreshMsiRoutes()
	}
	return nil
}

func (m *InterruptManager) refreshMsiRoutes() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	routes := make([]hyper.IrqRoutingEntry, 0, len(m.msi))
	for gsi, group := range m.msi {
		if group.config == nil {
			continue
		}
		routes = append(routes, m.vm.MakeRoutingEntry(gsi, group.config))
	}
	return m.vm.SetGsiRouting(routes)
}

var _ device.InterruptController = (*LegacyInterruptController)(nil)
var _ device.InterruptSourceGroup = (*MsiInterruptGroup)(nil)
