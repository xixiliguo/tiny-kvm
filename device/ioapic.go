package device

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

// NumIoapicPins configures the total supported hardware redirection line pins.
const NumIoapicPins = 24

const IoapicVersionId uint32 = 0x0017_0011

// Direct IOAPIC Register Offsets
const (
	IoapicRegId            uint8 = 0x00
	IoapicRegVersion       uint8 = 0x01
	IoapicRegArbitrationId uint8 = 0x02
)

// Bus MMIO Window Selection Ranges
const (
	IoregselOff  uint8 = 0x0
	IowinOff     uint8 = 0x10
	IowinScale   uint8 = 0x2
	RegMaxOffset uint8 = IowinOff + (NumIoapicPins * 2) - 1
)

// TriggerMode architectural flag assignments.
type TriggerMode uint8

const (
	TriggerModeEdge  TriggerMode = 0
	TriggerModeLevel TriggerMode = 1
)

// DeliveryMode defines APIC architectural targets.
type DeliveryMode uint8

const (
	DeliveryModeFixed      DeliveryMode = 0b000
	DeliveryModeLowest     DeliveryMode = 0b001
	DeliveryModeSmi        DeliveryMode = 0b010 // System Management Interrupt
	DeliveryModeRemoteRead DeliveryMode = 0b011 // No longer supported by Intel hardware
	DeliveryModeNmi        DeliveryMode = 0b100 // Non-Maskable Interrupt
	DeliveryModeInit       DeliveryMode = 0b101
	DeliveryModeStartup    DeliveryMode = 0b110
	DeliveryModeExternal   DeliveryMode = 0b111
)

// I/O REDIRECTION TABLE REGISTER
//
// There are 24 I/O Redirection Table entry registers. Each register is a
// dedicated entry for each interrupt input signal. Each register is 64 bits
// split between two 32 bits registers as follow:
//
// 63-56: Destination Field - R/W
// 55-17: Reserved
// 16:    Interrupt Mask - R/W
// 15:    Trigger Mode - R/W
// 14:    Remote IRR - RO
// 13:    Interrupt Input Pin Polarity - R/W
// 12:    Delivery Status - RO
// 11:    Destination Mode - R/W
// 10-8:  Delivery Mode - R/W
// 7-0:   Interrupt Vector - R/W
type RedirectionTableEntry uint64

func vector(entry RedirectionTableEntry) uint8 {
	return uint8(entry & 0xff)
}
func deliveryMode(entry RedirectionTableEntry) uint8 {
	return uint8((entry >> 8) & 0x7)
}
func destinationMode(entry RedirectionTableEntry) uint8 {
	return uint8((entry >> 11) & 0x1)
}
func remoteIrr(entry RedirectionTableEntry) uint8 {
	return uint8((entry >> 14) & 0x1)
}
func triggerMode(entry RedirectionTableEntry) uint8 {
	return uint8((entry >> 15) & 0x1)
}
func interruptMask(entry RedirectionTableEntry) uint8 {
	return uint8((entry >> 16) & 0x1)
}
func destinationField(entry RedirectionTableEntry) uint8 {
	// When the destination mode is physical, the destination field should only
	// be defined through bits 56-59, as defined in the IOAPIC specification.
	// But from the APIC specification, the APIC ID is always defined on 8 bits
	// no matter which destination mode is selected. That's why we always
	// retrieve the destination field based on bits 56-63.
	return uint8((entry >> 56) & 0xff)
}

func setDeliveryStatus(entry *RedirectionTableEntry, val uint8) {
	*entry &= 0xffff_ffff_ffff_efff // Clear bit 12
	*entry |= RedirectionTableEntry(val&0x1) << 12
}

func setRemoteIrr(entry *RedirectionTableEntry, val uint8) {
	*entry &= 0xffff_ffff_ffff_bfff // Clear bit 14
	*entry |= RedirectionTableEntry(val&0x1) << 14
}

// decodeIrqFromSelector calculates target pinning information from input index parameters.
func decodeIrqFromSelector(selector uint8) (int, bool) {
	return int((selector - IowinOff) / IowinScale), selector&1 != 0
}

// Emulation errors.
var (
	ErrInvalidTriggerMode  = errors.New("ioapic: invalid trigger mode value detected")
	ErrInvalidDeliveryMode = errors.New("ioapic: invalid delivery mode value detected")
)

// InterruptSourceGroup defines integration targets for downstream MSI injection pipelines.
type InterruptSourceGroup interface {
	Enable() error
	Disable() error
	Trigger(index uint32) error
	Update(index uint32, config MsiIrqSourceConfig, masked bool, setGsi bool) error
}

// MsiIrqSourceConfig encapsulates standard architecture delivery vectors.
type MsiIrqSourceConfig struct {
	HighAddr uint32
	LowAddr  uint32
	Data     uint32
	DevID    uint32
}

// Ioapic emulates an Intel 82093AA I/O Advanced Programmable Interrupt Controller.
type Ioapic struct {
	mu                   sync.Mutex // Guard concurrent MMIO loops
	id                   string
	idReg                uint32
	regSel               uint32
	regEntries           [NumIoapicPins]RedirectionTableEntry
	usedEntries          [NumIoapicPins]bool
	apicAddress          uint64
	interruptSourceGroup InterruptSourceGroup
}

// NewIoapic instantiates a workable IOAPIC core component.
func NewIoapic(id string, apicAddress uint64, isg InterruptSourceGroup) (*Ioapic, error) {
	ioapic := &Ioapic{
		id:                   id,
		idReg:                0,
		regSel:               0,
		interruptSourceGroup: isg,
		apicAddress:          apicAddress,
	}

	// Default initialize entries as completely masked (bit 16 raised: 0x10000)
	for i := 0; i < NumIoapicPins; i++ {
		ioapic.regEntries[i] = 0x10000
	}

	return ioapic, nil
}

// Read handles MMIO reads trapped via standard hypervisor traps.
func (io *Ioapic) Read(base uint64, offset uint64, data []byte) {
	if len(data) != 4 {
		return
	}

	io.mu.Lock()
	defer io.mu.Unlock()

	var value uint32
	switch uint8(offset) {
	case IoregselOff:
		value = io.regSel
	case IowinOff:
		value = io.ioapicRead()
	default:
		return
	}

	binary.LittleEndian.PutUint32(data, value)
}

// Write captures downstream MMIO updates.
func (io *Ioapic) Write(base uint64, offset uint64, data []byte) {
	if len(data) != 4 {
		return
	}

	io.mu.Lock()
	defer io.mu.Unlock()

	value := binary.LittleEndian.Uint32(data)

	switch uint8(offset) {
	case IoregselOff:
		io.regSel = value
	case IowinOff:
		io.ioapicWrite(value)
	}
}

func (io *Ioapic) ioapicWrite(val uint32) {
	switch uint8(io.regSel) {
	case IoapicRegVersion:
		// Windows or specific initialization codebases occasionally assert 0 writes here.
		if val != 0 {
			fmt.Printf("IOAPIC: invalid write to version register (0x%x): 0x%x", io.regSel, val)
		}
	case IoapicRegId:
		io.idReg = (val >> 24) & 0xf
	default:
		selector := uint8(io.regSel)
		if selector >= IowinOff && selector <= RegMaxOffset {
			index, isHighBits := decodeIrqFromSelector(selector)
			if index >= NumIoapicPins {
				return
			}

			if isHighBits {
				io.regEntries[index] &= 0xffff_ffff
				io.regEntries[index] |= RedirectionTableEntry(val) << 32
			} else {
				// Guard against overriding read-only attributes:
				// - Delivery Status (bit 12)
				// - Remote IRR (bit 14)
				io.regEntries[index] &= 0xffff_ffff_0000_5000
				io.regEntries[index] |= RedirectionTableEntry(val & 0xffff_afff)
			}

			if err := io.updateEntry(index, true); err != nil {
				fmt.Printf("failed updating IOAPIC entry: %s", err)
			}
			io.usedEntries[index] = true
		} else {
			fmt.Printf("IOAPIC: invalid write to register offset 0x%x", io.regSel)
		}
	}
}

func (io *Ioapic) ioapicRead() uint32 {
	switch uint8(io.regSel) {
	case IoapicRegVersion:
		return IoapicVersionId
	case IoapicRegId, IoapicRegArbitrationId:
		return (io.idReg & 0xf) << 24
	default:
		selector := uint8(io.regSel)
		if selector >= IowinOff && selector <= RegMaxOffset {
			index, isHighBits := decodeIrqFromSelector(selector)
			if index >= NumIoapicPins {
				fmt.Printf("IOAPIC index out of range: %d", index)
				return 0
			}

			if isHighBits {
				return uint32(io.regEntries[index] >> 32)
			}
			return uint32(io.regEntries[index] & 0xffff_ffff)
		} else {
			fmt.Printf("IOAPIC: invalid read from register offset 0x%x", io.regSel)
		}
		return 0
	}
}

func (io *Ioapic) updateEntry(irq int, setGsi bool) error {
	entry := io.regEntries[irq]

	destMode := destinationMode(entry)
	destID := destinationField(entry)
	var redirectionHint uint8 = 1

	// Construct x86 architectural MSI target address format
	lowAddr := uint32(io.apicAddress) |
		(uint32(destID) << 12) |
		(uint32(redirectionHint) << 3) |
		(uint32(destMode) << 2)

	tMode := triggerMode(entry)
	if tMode != uint8(TriggerModeEdge) && tMode != uint8(TriggerModeLevel) {
		return ErrInvalidTriggerMode
	}

	dMode := deliveryMode(entry)
	switch DeliveryMode(dMode) {
	case DeliveryModeFixed, DeliveryModeLowest, DeliveryModeSmi,
		DeliveryModeRemoteRead, DeliveryModeNmi, DeliveryModeInit,
		DeliveryModeStartup, DeliveryModeExternal:
	default:
		return ErrInvalidDeliveryMode
	}

	// Generate corresponding MSI internal signal tracking data payload
	data := (uint32(tMode) << 15) |
		(uint32(remoteIrr(entry)) << 14) |
		(uint32(dMode) << 8) |
		uint32(vector(entry))

	config := MsiIrqSourceConfig{
		HighAddr: 0x0,
		LowAddr:  lowAddr,
		Data:     data,
		DevID:    0,
	}

	return io.interruptSourceGroup.Update(
		uint32(irq),
		config,
		interruptMask(entry) == 1,
		setGsi,
	)
}

type InterruptController interface {
	// EndOfInterrupt signals the completion of an interrupt service routine to the APIC components.
	EndOfInterrupt(vec uint8)
	// ServiceIrq triggers/injects a specific hardware interrupt lane.
	ServiceIrq(irq int) error
}

// EndOfInterrupt hooks EOI lifecycle processing actions inside the Local APIC pipelines.
func (io *Ioapic) EndOfInterrupt(vec uint8) {
	io.mu.Lock()
	defer io.mu.Unlock()

	for i := range NumIoapicPins {
		entry := &io.regEntries[i]
		if vector(*entry) == vec && triggerMode(*entry) == uint8(TriggerModeLevel) {
			setRemoteIrr(entry, 0)
		}
	}
}

// ServiceIrq triggers active hardware signaling assertions onto internal host infrastructure layers.
func (io *Ioapic) ServiceIrq(irq int) error {
	io.mu.Lock()
	defer io.mu.Unlock()

	if irq >= NumIoapicPins {
		return fmt.Errorf("ioapic: irq pin %d out of range", irq)
	}

	entry := &io.regEntries[irq]
	if err := io.interruptSourceGroup.Trigger(uint32(irq)); err != nil {
		return err
	}

	// For level-sensitive assignments, configure Remote Interrupt Request Register flags.
	if triggerMode(*entry) == uint8(TriggerModeLevel) {
		setRemoteIrr(entry, 1)
	}
	setDeliveryStatus(entry, 0)

	return nil
}
