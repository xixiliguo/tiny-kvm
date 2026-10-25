// Copyright © 2019 Intel Corporation
// Copyright © 2023 Rivos, Inc.
//
// SPDX-License-Identifier: Apache-2.0

package acpi

import (
	"bytes"
	"encoding/binary"

	"github.com/xixiliguo/tiny-kvm/arch"
)

const (
	// MadtTypeIoApic represents an I/O APIC structure (Type 1).
	MadtTypeIoApic = 1
	// MadtTypeInterruptOverride represents an Interrupt Source Override structure (Type 2).
	MadtTypeInterruptOverride = 2
	// MadtTypeLocalX2Apic represents a Processor Local X2APIC structure (Type 9).
	MadtTypeLocalX2Apic = 9
)

const (
	// MadtCpuEnableFlag indicates the processor is enabled and capable of generic components receipt (Bit 0).
	MadtCpuEnableFlag = 0
	// MadtCpuOnlineCapableFlag indicates the processor is disabled but can be hot-plugged online (Bit 1).
	MadtCpuOnlineCapableFlag = 1
)

func NewMadtTable(locals []LocalX2Apic) *Sdt {
	sdt := NewSdt(MadtSignature, MadtLen, MadtRivision, OemID, OemMadtTable, OemRevision)
	sdt.Write(36, uint32(arch.ApicStart))
	for _, l := range locals {
		sdt.AppendAML(&l)
	}

	io := &IoApic{
		Type:        MadtTypeIoApic,
		Length:      12,
		IoApicID:    0,
		Reserved:    0,
		ApicAddress: uint32(arch.IoapicStart),
		GsiBase:     0,
	}

	sdt.AppendAML(io)
	return sdt
}

// LocalX2Apic mirrors the MADT Type 9 Processor Local X2APIC structure.
// This structure is used in systems where processor IDs or APIC IDs exceed 255.
type LocalX2Apic struct {
	Type        uint8
	Length      uint8
	Reserved    uint16
	ApicID      uint32
	Flags       uint32
	ProcessorID uint32
}

// Bytes serializes the LocalX2Apic structure into a packed binary byte slice (Little Endian).
func (x *LocalX2Apic) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, x)
	return buf.Bytes()
}

// ToAmlBytes streams the serialized struct footprint into an AML buffer sink.
func (x *LocalX2Apic) ToAmlBytes() []byte {
	return x.Bytes()
}

// IoApic mirrors the MADT Type 1 I/O APIC structure.
// It describes a discrete I/O APIC interrupt controller routing capability in the system.
type IoApic struct {
	Type        uint8
	Length      uint8
	IoApicID    uint8
	Reserved    uint8
	ApicAddress uint32
	GsiBase     uint32
}

// Bytes serializes IoApic into a packed binary byte slice (Little Endian).
func (i *IoApic) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, i)
	return buf.Bytes()
}

// ToAmlBytes streams the serialized struct footprint into an AML buffer sink.
func (i *IoApic) ToAmlBytes() []byte {
	return i.Bytes()
}

// InterruptSourceOverride mirrors the MADT Type 2 Interrupt Source Override structure.
// It explains how legacy ISA interrupts (e.g., PIT Timer on IRQ 0) map to alternative GSIs (usually GSI 2).
type InterruptSourceOverride struct {
	Type   uint8
	Length uint8
	Bus    uint8  // Constant 0 representing the ISA bus
	Source uint8  // Legacy ISA IRQ number
	Gsi    uint32 // The Global System Interrupt number it maps to
	Flags  uint16 // MPS MPS Interrupt Flags (Polarity and Trigger Mode)
}

// Bytes serializes the InterruptSourceOverride structure into a packed binary byte slice (Little Endian).
func (iso *InterruptSourceOverride) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, iso)
	return buf.Bytes()
}

// ToAmlBytes streams the serialized struct footprint into an AML buffer sink.
func (iso *InterruptSourceOverride) ToAmlBytes() []byte {
	return iso.Bytes()
}
