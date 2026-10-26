package vmm

import (
	"fmt"
	"sync"

	"github.com/xixiliguo/tiny-kvm/device/legacy"
	"github.com/xixiliguo/tiny-kvm/internal/eventfd"
)

const (
	LegacyCmosIndexPort uint64 = 0x70
	LegacyCmosDataPort  uint64 = 0x71
)

// DeviceManager owns the PIO/MMIO buses and the essential legacy devices.
type DeviceManager struct {
	mu         sync.RWMutex
	ioBus      *Bus
	mmioBus    *Bus
	serial     *SerialManager
	cmos       *legacy.Cmos
	resetEvent *eventfd.EventFd
	closed     bool
}

type cmosDevice struct {
	cmos *legacy.Cmos
}

func (c cmosDevice) Read(base, offset uint64, data []byte) {
	c.cmos.Read(base, offset, data)
}

func (c cmosDevice) Write(base, offset uint64, data []byte) error {
	c.cmos.Write(base, offset, data)
	return nil
}

// NewDeviceManager creates the COM1 serial port, CMOS/RTC and PCI placeholder
// buses used by the tiny x86 platform.
func NewDeviceManager(
	memBelow4G uint64,
	memAbove4G uint64,
	interrupts *InterruptManager,
	serialConfig ConsoleConfig,
) (*DeviceManager, error) {
	resetEvent, err := eventfd.NewEventfd()
	if err != nil {
		return nil, fmt.Errorf("device: create reset eventfd: %w", err)
	}
	serial, err := NewSerialManager(NewLegacyInterruptController(interrupts), serialConfig)
	if err != nil {
		return nil, err
	}

	manager := &DeviceManager{
		ioBus:      NewBus(),
		mmioBus:    NewBus(),
		serial:     serial,
		cmos:       legacy.NewCmos(memBelow4G, memAbove4G, resetEvent, nil),
		resetEvent: resetEvent,
	}
	if err := manager.ioBus.Insert("serial0", LegacySerialPort, LegacySerialPortSize, serial); err != nil {
		_ = manager.Close()
		return nil, err
	}
	if err := manager.ioBus.Insert("cmos", LegacyCmosIndexPort, 2, cmosDevice{cmos: manager.cmos}); err != nil {
		_ = manager.Close()
		return nil, err
	}
	if err := manager.ioBus.Insert("debug-port", DebugPort, 1, serial); err != nil {
		_ = manager.Close()
		return nil, err
	}
	return manager, nil
}

// ReadIO dispatches a port I/O read.
func (d *DeviceManager) ReadIO(port uint64, data []byte) {
	d.mu.RLock()
	bus := d.ioBus
	d.mu.RUnlock()
	bus.Read(port, data)
}

// WriteIO dispatches a port I/O write.
func (d *DeviceManager) WriteIO(port uint64, data []byte) error {
	d.mu.RLock()
	bus := d.ioBus
	d.mu.RUnlock()
	return bus.Write(port, data)
}

// ReadMMIO dispatches an MMIO read.
func (d *DeviceManager) ReadMMIO(address uint64, data []byte) {
	d.mu.RLock()
	bus := d.mmioBus
	d.mu.RUnlock()
	bus.Read(address, data)
}

// WriteMMIO dispatches an MMIO write.
func (d *DeviceManager) WriteMMIO(address uint64, data []byte) error {
	d.mu.RLock()
	bus := d.mmioBus
	d.mu.RUnlock()
	return bus.Write(address, data)
}

// IOBus returns the bus for additional platform devices.
func (d *DeviceManager) IOBus() *Bus { return d.ioBus }

// MMIOBus returns the bus for additional platform devices.
func (d *DeviceManager) MMIOBus() *Bus { return d.mmioBus }

// Serial returns the serial manager.
func (d *DeviceManager) Serial() *SerialManager { return d.serial }

// Close releases device resources.
func (d *DeviceManager) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	var first error
	if d.serial != nil {
		if err := d.serial.Close(); err != nil {
			first = err
		}
	}
	return first
}
