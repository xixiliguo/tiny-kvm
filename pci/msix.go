// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0 OR BSD-3-Clause

package pci

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
)

const (
	MaxMsixVectorsPerDevice uint16 = 2048
	MsixTableEntriesModulo  uint64 = 16
	MsixPbaEntriesModulo    uint64 = 8
	BitsPerPbaEntry         int    = 64
	FunctionMaskBit         uint8  = 14
	MsixEnableBit           uint8  = 15
	FunctionMaskMask        uint16 = 1 << FunctionMaskBit
	MsixEnableMask          uint16 = 1 << MsixEnableBit
	MsixTableEntrySize      int    = 16
	MsixConfigID            string = "msix_config"
)

// --- Errors ---

var (
	ErrMsixEnableInterruptRoute = errors.New("Failed enabling the interrupt route")
	ErrMsixUpdateInterruptRoute = errors.New("Failed updating the interrupt route")
)

// MsixTableEntry represents a single MSI-X Table Entry layout.
type MsixTableEntry struct {
	MsgAddrLo uint32 `json:"msg_addr_lo"`
	MsgAddrHi uint32 `json:"msg_addr_hi"`
	MsgData   uint32 `json:"msg_data"`
	VectorCtl uint32 `json:"vector_ctl"`
}

// Masked checks if this specific vector is masked.
func (e *MsixTableEntry) Masked() bool {
	return (e.VectorCtl & 0x1) == 0x1
}

// NewDefaultMsixTableEntry returns an entry populated with standard PCI initialization defaults (masked).
func NewDefaultMsixTableEntry() MsixTableEntry {
	return MsixTableEntry{
		MsgAddrLo: 0,
		MsgAddrHi: 0,
		MsgData:   0,
		VectorCtl: 0x1, // Vector is masked by default on reset
	}
}

// MsixConfigState encapsulates the serializable state of an MSI-X configuration.
type MsixConfigState struct {
	TableEntries []MsixTableEntry `json:"table_entries"`
	PbaEntries   []uint64         `json:"pba_entries"`
	Masked       bool             `json:"masked"`
	Enabled      bool             `json:"enabled"`
}

// MsixConfig emulates the MSI-X capability memory infrastructure.
type MsixConfig struct {
	TableEntries         []MsixTableEntry
	PbaEntries           []uint64
	DevID                uint32
	interruptSourceGroup InterruptSourceGroup
	masked               bool
	enabled              bool
}

// NewMsixConfig initializes a new MsixConfig manager, optionally restoring state from a migration checkpoint.
func NewMsixConfig(msixVectors uint16, interruptSourceGroup InterruptSourceGroup, devid uint32, state *MsixConfigState) (*MsixConfig, error) {
	if msixVectors > MaxMsixVectorsPerDevice {
		panic(fmt.Sprintf("requested vectors %d exceeds max permissible boundary %d", msixVectors, MaxMsixVectorsPerDevice))
	}

	var tableEntries []MsixTableEntry
	var pbaEntries []uint64
	var masked bool
	var enabled bool

	if state != nil {
		if state.Enabled && !state.Masked {
			for idx, tableEntry := range state.TableEntries {
				if tableEntry.Masked() {
					continue
				}

				config := MsiIrqSourceConfig{
					HighAddr: tableEntry.MsgAddrHi,
					LowAddr:  tableEntry.MsgAddrLo,
					Data:     tableEntry.MsgData,
					DevID:    devid,
				}

				if err := interruptSourceGroup.Update(
					InterruptIndex(idx),
					InterruptSourceConfig{MsiIrq: &config},
					state.Masked,
					true,
				); err != nil {
					return nil, fmt.Errorf("%w: %v", ErrMsixUpdateInterruptRoute, err)
				}

				if err := interruptSourceGroup.Enable(); err != nil {
					return nil, fmt.Errorf("%w: %v", ErrMsixEnableInterruptRoute, err)
				}
			}
		}

		tableEntries = state.TableEntries
		pbaEntries = state.PbaEntries
		masked = state.Masked
		enabled = state.Enabled
	} else {
		tableEntries = make([]MsixTableEntry, msixVectors)
		for i := range tableEntries {
			tableEntries[i] = NewDefaultMsixTableEntry()
		}

		numPbaEntries := (int(msixVectors) / BitsPerPbaEntry) + 1
		pbaEntries = make([]uint64, numPbaEntries)

		masked = true
		enabled = false
	}

	return &MsixConfig{
		TableEntries:         tableEntries,
		PbaEntries:           pbaEntries,
		DevID:                devid,
		interruptSourceGroup: interruptSourceGroup,
		masked:               masked,
		enabled:              enabled,
	}, nil
}

func (m *MsixConfig) State() MsixConfigState {
	// Deep copy slices to isolate snapshots safely
	tableCopy := make([]MsixTableEntry, len(m.TableEntries))
	copy(tableCopy, m.TableEntries)

	pbaCopy := make([]uint64, len(m.PbaEntries))
	copy(pbaCopy, m.PbaEntries)

	return MsixConfigState{
		TableEntries: tableCopy,
		PbaEntries:   pbaCopy,
		Masked:       m.masked,
		Enabled:      m.enabled,
	}
}

func (m *MsixConfig) Masked() bool {
	return m.masked
}

func (m *MsixConfig) Enabled() bool {
	return m.enabled
}

// SetMsgCtl modifies the global MSI-X message control layout flags.
func (m *MsixConfig) SetMsgCtl(reg uint16) {
	oldMasked := m.masked
	oldEnabled := m.enabled

	m.masked = ((reg >> FunctionMaskBit) & 1) == 1
	m.enabled = ((reg >> MsixEnableBit) & 1) == 1

	// Update interrupt routing topologies if the power or function mask switches state
	if oldMasked != m.masked || oldEnabled != m.enabled {
		if m.enabled && !m.masked {
			log.Printf("[DEBUG] MSI-X enabled for device 0x%x", m.DevID)
			for idx, tableEntry := range m.TableEntries {
				config := MsiIrqSourceConfig{
					HighAddr: tableEntry.MsgAddrHi,
					LowAddr:  tableEntry.MsgAddrLo,
					Data:     tableEntry.MsgData,
					DevID:    m.DevID,
				}

				if err := m.interruptSourceGroup.Update(
					InterruptIndex(idx),
					InterruptSourceConfig{MsiIrq: &config},
					tableEntry.Masked(),
					true,
				); err != nil {
					log.Printf("Failed updating vector: %v", err)
				}
			}
		} else if oldEnabled || !oldMasked {
			log.Printf("[DEBUG] MSI-X disabled for device 0x%x", m.DevID)
			if err := m.interruptSourceGroup.Disable(); err != nil {
				log.Printf("Failed disabling irq_fd: %v", err)
			}
		}
	}

	// Unmasking evaluation: If the hardware unmasks the general structure, scan PBA to flush delayed irqs
	if oldMasked && !m.masked {
		for index, entry := range m.TableEntries {
			if !entry.Masked() && m.GetPbaBit(uint16(index)) == 1 {
				m.InjectMsixAndClearPba(index)
			}
		}
	}
}

// ReadTable emulates MMIO reading interface from the MMIO BAR mapping space dedicated to MSI-X Table.
func (m *MsixConfig) ReadTable(offset uint64, data []byte) {
	if len(data) != 4 && len(data) != 8 {
		panic("invalid table read size: expected 4 or 8 bytes")
	}

	index := int(offset / MsixTableEntriesModulo)
	moduloOffset := offset % MsixTableEntriesModulo

	if index >= len(m.TableEntries) {
		log.Printf("[DEBUG] Invalid MSI-X table entry index %d", index)
		for i := range data {
			data[i] = 0xff
		}
		return
	}

	switch len(data) {
	case 4:
		var value uint32
		switch moduloOffset {
		case 0x0:
			value = m.TableEntries[index].MsgAddrLo
		case 0x4:
			value = m.TableEntries[index].MsgAddrHi
		case 0x8:
			value = m.TableEntries[index].MsgData
		case 0xc:
			value = m.TableEntries[index].VectorCtl
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
		log.Printf("[DEBUG] MSI_R TABLE offset 0x%x data 0x%x", offset, value)
		binary.LittleEndian.PutUint32(data, value)

	case 8:
		var value uint64
		switch moduloOffset {
		case 0x0:
			value = (uint64(m.TableEntries[index].MsgAddrHi) << 32) | uint64(m.TableEntries[index].MsgAddrLo)
		case 0x8:
			value = (uint64(m.TableEntries[index].VectorCtl) << 32) | uint64(m.TableEntries[index].MsgData)
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
		log.Printf("[DEBUG] MSI_R TABLE offset 0x%x data 0x%x", offset, value)
		binary.LittleEndian.PutUint64(data, value)
	}
}

// WriteTable handles MMIO configuration storage targeting specific MSI-X table addresses.
func (m *MsixConfig) WriteTable(offset uint64, data []byte) {
	if len(data) != 4 && len(data) != 8 {
		panic("invalid table write size: expected 4 or 8 bytes")
	}

	index := int(offset / MsixTableEntriesModulo)
	moduloOffset := offset % MsixTableEntriesModulo

	if index >= len(m.TableEntries) {
		log.Printf("[DEBUG] Invalid MSI-X table entry index %d", index)
		return
	}

	oldEntry := m.TableEntries[index]

	switch len(data) {
	case 4:
		value := binary.LittleEndian.Uint32(data)
		switch moduloOffset {
		case 0x0:
			m.TableEntries[index].MsgAddrLo = value
		case 0x4:
			m.TableEntries[index].MsgAddrHi = value
		case 0x8:
			m.TableEntries[index].MsgData = value
		case 0xc:
			m.TableEntries[index].VectorCtl = value
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
		log.Printf("[DEBUG] MSI_W TABLE offset 0x%x data 0x%x", offset, value)

	case 8:
		value := binary.LittleEndian.Uint64(data)
		switch moduloOffset {
		case 0x0:
			m.TableEntries[index].MsgAddrLo = uint32(value & 0xffffffff)
			m.TableEntries[index].MsgAddrHi = uint32(value >> 32)
		case 0x8:
			m.TableEntries[index].MsgData = uint32(value & 0xffffffff)
			m.TableEntries[index].VectorCtl = uint32(value >> 32)
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
		log.Printf("[DEBUG] MSI_W TABLE offset 0x%x data 0x%x", offset, value)
	}

	tableEntry := m.TableEntries[index]
	if oldEntry == tableEntry {
		return
	}

	// Route mapping updates: only push modifications downstream if currently activated and unmasked.
	if m.enabled && !m.masked && !tableEntry.Masked() {
		config := MsiIrqSourceConfig{
			HighAddr: tableEntry.MsgAddrHi,
			LowAddr:  tableEntry.MsgAddrLo,
			Data:     tableEntry.MsgData,
			DevID:    m.DevID,
		}

		if err := m.interruptSourceGroup.Update(
			InterruptIndex(index),
			InterruptSourceConfig{MsiIrq: &config},
			tableEntry.Masked(),
			true,
		); err != nil {
			log.Printf("Failed updating vector: %v", err)
		}
	}

	// Check if a discrete unmask operation transitioned while a bit inside the PBA matrix was waiting.
	if !m.Masked() && m.Enabled() && oldEntry.Masked() && !tableEntry.Masked() && m.GetPbaBit(uint16(index)) == 1 {
		m.InjectMsixAndClearPba(index)
	}
}

// ReadPba intercepts MMIO reads referencing the standalone Pending Bit Array region.
func (m *MsixConfig) ReadPba(offset uint64, data []byte) {
	if len(data) != 4 && len(data) != 8 {
		panic("invalid pba read size: expected 4 or 8 bytes")
	}

	index := int(offset / MsixPbaEntriesModulo)
	moduloOffset := offset % MsixPbaEntriesModulo

	if index >= len(m.PbaEntries) {
		log.Printf("[DEBUG] Invalid MSI-X PBA entry index %d", index)
		for i := range data {
			data[i] = 0xff
		}
		return
	}

	switch len(data) {
	case 4:
		var value uint32
		switch moduloOffset {
		case 0x0:
			value = uint32(m.PbaEntries[index] & 0xffffffff)
		case 0x4:
			value = uint32(m.PbaEntries[index] >> 32)
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
		log.Printf("[DEBUG] MSI_R PBA offset 0x%x data 0x%x", offset, value)
		binary.LittleEndian.PutUint32(data, value)

	case 8:
		var value uint64
		switch moduloOffset {
		case 0x0:
			value = m.PbaEntries[index]
		default:
			log.Printf("invalid offset: 0x%x", offset)
		}
		log.Printf("[DEBUG] MSI_R PBA offset 0x%x data 0x%x", offset, value)
		binary.LittleEndian.PutUint64(data, value)
	}
}

// WritePba logs an error since the Pending Bit Array is defined as Read-Only hardware memory.
func (m *MsixConfig) WritePba(offset uint64, data []byte) {
	log.Printf("Pending Bit Array is read only")
}

// SetPbaBit updates a single bit within the bitmask block slice representing hypervisor backed state.
func (m *MsixConfig) SetPbaBit(vector uint16, reset bool) {
	if vector >= MaxMsixVectorsPerDevice {
		panic(fmt.Sprintf("vector offset %d out of bounds", vector))
	}

	index := int(vector) / BitsPerPbaEntry
	shift := int(vector) % BitsPerPbaEntry
	mask := uint64(1 << shift)

	if reset {
		m.PbaEntries[index] &^= mask
	} else {
		m.PbaEntries[index] |= mask
	}
}

// GetPbaBit returns the specific bit status inside the bit-packed array blocks.
func (m *MsixConfig) GetPbaBit(vector uint16) uint8 {
	if vector >= MaxMsixVectorsPerDevice {
		panic(fmt.Sprintf("vector offset %d out of bounds", vector))
	}

	index := int(vector) / BitsPerPbaEntry
	shift := int(vector) % BitsPerPbaEntry

	return uint8((m.PbaEntries[index] >> shift) & 0x1)
}

func (m *MsixConfig) InjectMsixAndClearPba(vector int) {
	if err := m.interruptSourceGroup.Trigger(InterruptIndex(vector)); err != nil {
		log.Printf("failed to inject MSI-X: %v", err)
	} else {
		log.Printf("[DEBUG] MSI-X injected on vector control flip")
	}

	m.SetPbaBit(uint16(vector), true)
}

// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0 OR BSD-3-Clause

// MsixCap emulates the PCI MSI-X Capability structure registers.
type MsixCap struct {
	// Message Control Register
	//   10-0:  MSI-X Table size
	//   13-11: Reserved
	//   14:    Mask. Mask all MSI-X when set.
	//   15:    Enable. Enable all MSI-X when set.
	MsgCtl uint16 `json:"msg_ctl"`

	// Table. Contains the offset and the BAR indicator (BIR)
	//   2-0:   Table BAR indicator (BIR). Can be 0 to 5.
	//   31-3:  Table offset in the BAR pointed by the BIR.
	Table uint32 `json:"table"`

	// Pending Bit Array. Contains the offset and the BAR indicator (BIR)
	//   2-0:   PBA BAR indicator (BIR). Can be 0 to 5.
	//   31-3:  PBA offset in the BAR pointed by the BIR.
	Pba uint32 `json:"pba"`
}

// Bytes serializes the MsixCap structure into a packed binary byte slice (Little Endian).
// This replaces Rust's self.as_slice() powered by unsafe ByteValued.
func (c *MsixCap) Bytes() []byte {
	buf := new(bytes.Buffer)
	// Under packed C representation, fields are serialized sequentially in Little Endian
	_ = binary.Write(buf, binary.LittleEndian, c.MsgCtl)
	_ = binary.Write(buf, binary.LittleEndian, c.Table)
	_ = binary.Write(buf, binary.LittleEndian, c.Pba)
	return buf.Bytes()
}

// ID returns the PciCapabilityId for MSI-X.
func (c *MsixCap) ID() PciCapabilityID {
	return PciCapabilityIDMsiX
}

// NewMsixCap creates and initializes an MsixCap register manager.
func NewMsixCap(tablePciBar uint8, tableSize uint16, tableOff uint32, pbaPciBar uint8, pbaOff uint32) *MsixCap {
	if tableSize >= MaxMsixVectorsPerDevice {
		panic(fmt.Sprintf("tableSize %d exceeds max permissible vectors", tableSize))
	}

	// Set the table size and enable MSI-X (bit 15: 0x8000)
	// In PCI spec, the table size field in MsgCtl is encoded as N - 1.
	msgCtl := uint16(0x8000) + tableSize - 1

	return &MsixCap{
		MsgCtl: msgCtl,
		Table:  (tableOff & 0xfffffff8) | uint32(tablePciBar&0x7),
		Pba:    (pbaOff & 0xfffffff8) | uint32(pbaPciBar&0x7),
	}
}

func (c *MsixCap) SetMsgCtl(data uint16) {
	c.MsgCtl = (c.MsgCtl &^ (FunctionMaskMask | MsixEnableMask)) |
		(data & (FunctionMaskMask | MsixEnableMask))
}

func (c *MsixCap) Masked() bool {
	return ((c.MsgCtl >> FunctionMaskBit) & 0x1) == 0x1
}

func (c *MsixCap) Enabled() bool {
	return ((c.MsgCtl >> MsixEnableBit) & 0x1) == 0x1
}

func (c *MsixCap) TableOffset() uint32 {
	return c.Table & 0xfffffff8
}

func (c *MsixCap) PbaOffset() uint32 {
	return c.Pba & 0xfffffff8
}

func (c *MsixCap) TableSetOffset(addr uint32) {
	c.Table &= 0x7
	c.Table += addr
}

func (c *MsixCap) PbaSetOffset(addr uint32) {
	c.Pba &= 0x7
	c.Pba += addr
}

func (c *MsixCap) TableBir() uint32 {
	return c.Table & 0x7
}

func (c *MsixCap) PbaBir() uint32 {
	return c.Pba & 0x7
}

func (c *MsixCap) TableSize() uint16 {
	return (c.MsgCtl & 0x7ff) + 1
}

func (c *MsixCap) TableRange() (offset uint64, size uint64) {
	// The table takes 16 bytes per entry.
	size = uint64(c.TableSize()) * 16
	offset = uint64(c.TableOffset())
	return
}

func (c *MsixCap) PbaRange() (offset uint64, size uint64) {
	// The PBA takes 1 bit per entry modulo 8 bytes (64 bits per PBA block entry).
	size = ((uint64(c.TableSize()) / 64) + 1) * 8
	offset = uint64(c.PbaOffset())
	return
}
