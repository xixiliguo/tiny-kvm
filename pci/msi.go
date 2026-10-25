// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0 OR BSD-3-Clause

package pci

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
)

// MSI control masks
const (
	MsiCtlEnable         uint16 = 0x1
	MsiCtlMultiMsgEnable uint16 = 0x70
	MsiCtl64Bits         uint16 = 0x80
	MsiCtlPerVector      uint16 = 0x100
)

// MSI message offsets
const (
	MsiMsgCtlOffset    uint64 = 0x2
	MsiMsgAddrLoOffset uint64 = 0x4
)

// MSI message masks
const (
	MsiMsgAddrLoMask uint32 = 0xfffffffc
)

// MsiNumEnabledVectors returns the number of MSI vectors enabled by the guest.
func MsiNumEnabledVectors(msgCtl uint16) int {
	field := (msgCtl >> 4) & 0x7

	if field > 5 {
		return 0
	}

	return 1 << field
}

// --- Errors ---

var (
	ErrEnableInterruptRoute = errors.New("Failed enabling the interrupt route")
	ErrUpdateInterruptRoute = errors.New("Failed updating the interrupt route")
)

const MsiConfigID = "msi_config"

// MsiCap emulates the PCI Message Signaled Interrupts (MSI) Capability structure registers.
type MsiCap struct {
	// Message Control Register
	//   0:     MSI enable.
	//   3-1:   Multiple message capable.
	//   6-4:   Multiple message enable.
	//   7:     64 bits address capable.
	//   8:     Per-vector masking capable.
	//   15-9:  Reserved.
	MsgCtl uint16 `json:"msg_ctl"`

	// Message Address (LSB)
	//   1-0:  Reserved.
	//   31-2: Message address.
	MsgAddrLo uint32 `json:"msg_addr_lo"`

	// Message Upper Address (MSB)
	//   31-0: Message address.
	MsgAddrHi uint32 `json:"msg_addr_hi"`

	// Message Data
	//   15-0: Message data.
	MsgData uint16 `json:"msg_data"`

	// Mask Bits
	//   31-0: Mask bits.
	MaskBits uint32 `json:"mask_bits"`

	// Pending Bits
	//   31-0: Pending bits.
	PendingBits uint32 `json:"pending_bits"`
}

func (c *MsiCap) Addr64Bits() bool {
	return (c.MsgCtl & MsiCtl64Bits) == MsiCtl64Bits
}

func (c *MsiCap) PerVectorMask() bool {
	return (c.MsgCtl & MsiCtlPerVector) == MsiCtlPerVector
}

func (c *MsiCap) Enabled() bool {
	return (c.MsgCtl & MsiCtlEnable) == MsiCtlEnable
}

func (c *MsiCap) NumEnabledVectors() int {
	return MsiNumEnabledVectors(c.MsgCtl)
}

func (c *MsiCap) VectorMasked(vector int) bool {
	if !c.PerVectorMask() {
		return false
	}
	return ((c.MaskBits >> vector) & 0x1) == 0x1
}

func (c *MsiCap) Size() uint64 {
	var size uint64 = 0xa

	if c.Addr64Bits() {
		size += 0x4
	}
	if c.PerVectorMask() {
		size += 0xa
	}

	return size
}

func (c *MsiCap) Update(offset uint64, data []byte) {
	// Calculate register layout dependencies dynamically based on address size and masking capabilities.
	var msgDataOffset uint64
	var addrHiOffset *uint64
	var maskBitsOffset *uint64

	if c.Addr64Bits() {
		msgDataOffset = 0xc
		hiOff := uint64(0x8)
		addrHiOffset = &hiOff
		if c.PerVectorMask() {
			maskOff := uint64(0x10)
			maskBitsOffset = &maskOff
		}
	} else {
		msgDataOffset = 0x8
		if c.PerVectorMask() {
			maskOff := uint64(0xc)
			maskBitsOffset = &maskOff
		}
	}

	// Safely commit partial or double word writes without overriding read-only properties
	switch len(data) {
	case 2:
		value := binary.LittleEndian.Uint16(data)
		switch offset {
		case MsiMsgCtlOffset:
			c.MsgCtl = (c.MsgCtl &^ (MsiCtlEnable | MsiCtlMultiMsgEnable)) |
				(value & (MsiCtlEnable | MsiCtlMultiMsgEnable))
		case msgDataOffset:
			c.MsgData = value
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
	case 4:
		value := binary.LittleEndian.Uint32(data)
		switch offset {
		case 0x0:
			c.MsgCtl = (c.MsgCtl &^ (MsiCtlEnable | MsiCtlMultiMsgEnable)) |
				(uint16(value>>16) & (MsiCtlEnable | MsiCtlMultiMsgEnable))
		case MsiMsgAddrLoOffset:
			c.MsgAddrLo = value & MsiMsgAddrLoMask
		case msgDataOffset:
			c.MsgData = uint16(value)
		default:
			switch {
			case addrHiOffset != nil && offset == *addrHiOffset:
				c.MsgAddrHi = value
			case maskBitsOffset != nil && offset == *maskBitsOffset:
				c.MaskBits = value
			default:
				log.Printf("invalid offset: 0x%x", offset)
			}
		}
	default:
		log.Printf("invalid data length: %d", len(data))
	}
}

// MsiConfigState encapsulates the serializable snapshot representation of the MSI device state.
type MsiConfigState struct {
	Cap MsiCap `json:"cap"`
}

// MsiConfig manages MSI configuration registers alongside backing platform host interrupt bindings.
type MsiConfig struct {
	Cap                  MsiCap
	interruptSourceGroup InterruptSourceGroup
}

// NewMsiConfig creates and instantiates an MsiConfig instance, restoring or wiring interrupt topologies if loaded from state.
func NewMsiConfig(msgCtl uint16, interruptSourceGroup InterruptSourceGroup, state *MsiConfigState) (*MsiConfig, error) {
	var cap MsiCap

	if state != nil {
		if state.Cap.Enabled() {
			for idx := 0; idx < state.Cap.NumEnabledVectors(); idx++ {
				config := MsiIrqSourceConfig{
					HighAddr: state.Cap.MsgAddrHi,
					LowAddr:  state.Cap.MsgAddrLo,
					Data:     uint32(state.Cap.MsgData),
					DevID:    0,
				}

				if err := interruptSourceGroup.Update(
					InterruptIndex(idx),
					InterruptSourceConfig{MsiIrq: &config},
					state.Cap.VectorMasked(idx),
					false,
				); err != nil {
					return nil, fmt.Errorf("%w: %v", ErrUpdateInterruptRoute, err)
				}
			}

			if err := interruptSourceGroup.SetGsi(); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrEnableInterruptRoute, err)
			}

			if err := interruptSourceGroup.Enable(); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrEnableInterruptRoute, err)
			}
		}
		cap = state.Cap
	} else {
		cap = MsiCap{MsgCtl: msgCtl}
	}

	return &MsiConfig{
		Cap:                  cap,
		interruptSourceGroup: interruptSourceGroup,
	}, nil
}

func (m *MsiConfig) State() MsiConfigState {
	return MsiConfigState{Cap: m.Cap}
}

func (m *MsiConfig) Enabled() bool {
	return m.Cap.Enabled()
}

func (m *MsiConfig) Size() uint64 {
	return m.Cap.Size()
}

func (m *MsiConfig) NumEnabledVectors() int {
	return m.Cap.NumEnabledVectors()
}

func (m *MsiConfig) Update(offset uint64, data []byte) {
	oldEnabled := m.Cap.Enabled()

	m.Cap.Update(offset, data)

	if m.Cap.Enabled() {
		for idx := 0; idx < m.NumEnabledVectors(); idx++ {
			config := MsiIrqSourceConfig{
				HighAddr: m.Cap.MsgAddrHi,
				LowAddr:  m.Cap.MsgAddrLo,
				Data:     uint32(m.Cap.MsgData),
				DevID:    0,
			}

			if err := m.interruptSourceGroup.Update(
				InterruptIndex(idx),
				InterruptSourceConfig{MsiIrq: &config},
				m.Cap.VectorMasked(idx),
				true,
			); err != nil {
				log.Printf("Failed updating vector: %v", err)
			}
		}

		if !oldEnabled {
			if err := m.interruptSourceGroup.Enable(); err != nil {
				log.Printf("Failed enabling irq_fd: %v", err)
			}
		}
	} else if oldEnabled {
		if err := m.interruptSourceGroup.Disable(); err != nil {
			log.Printf("Failed disabling irq_fd: %v", err)
		}
	}
}

// --- Pausable & Snapshottable Interfaces ---

func (m *MsiConfig) Pause() error  { return nil }
func (m *MsiConfig) Resume() error { return nil }

func (m *MsiConfig) ID() string {
	return MsiConfigID
}

func (m *MsiConfig) Snapshot() (Snapshot, error) {
	return NewSnapshotFromState(m.State())
}

// --- Platform/Virtualization Engine Placeholders ---

type InterruptIndex int
type MsiIrqSourceConfig struct {
	HighAddr uint32
	LowAddr  uint32
	Data     uint32
	DevID    uint32
}
type InterruptSourceConfig struct {
	MsiIrq *MsiIrqSourceConfig
}

type InterruptSourceGroup interface {
	Update(index InterruptIndex, config InterruptSourceConfig, masked bool, live bool) error
	SetGsi() error
	Enable() error
	Disable() error
	Trigger(index InterruptIndex) error
}

type Snapshot any

func NewSnapshotFromState(state any) (Snapshot, error) { return state, nil }
