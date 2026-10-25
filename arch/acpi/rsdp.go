package acpi

import (
	"bytes"
	"encoding/binary"
)

const (
	RsdpSize = 36
)

var (
	// RsdpSignature represents the standard ACPI RSDP 8-byte magic string.
	RsdpSignature = [...]byte{'R', 'S', 'D', ' ', 'P', 'T', 'R', ' '}
)

// Rsdp mirrors the ACPI Root System Description Pointer structure (Revision 2.0+).
type Rsdp struct {
	Signature        [8]byte
	Checksum         uint8
	OemID            [6]byte
	Revision         uint8
	RsdtAddr         uint32 // Deprecated in ACPI 2.0+, kept for alignment (0)
	Length           uint32
	XsdtAddr         uint64
	ExtendedChecksum uint8
	Reserved         [3]byte
}

// NewRsdp initializes and completes an RSDP table pointer stub calculating both ACPI checksum phases.
func NewRsdp(oemID [6]byte, xsdtAddr uint64) *Rsdp {

	rsdp := &Rsdp{
		Signature:        RsdpSignature,
		Checksum:         0,
		OemID:            oemID,
		Revision:         2, // ACPI 2.0+ / XSDT enabled extension descriptor
		RsdtAddr:         0,
		Length:           RsdpSize,
		XsdtAddr:         xsdtAddr,
		ExtendedChecksum: 0,
	}

	// Phase 1 Checksum: covers the legacy ACPI 1.0 fields (first 20 bytes of the structure)
	bytes := rsdp.Bytes()
	rsdp.Checksum = GenerateChecksum(bytes[:20])

	// Phase 2 Checksum: covers the entire extended ACPI 2.0+ structure table
	bytes = rsdp.Bytes()
	rsdp.ExtendedChecksum = GenerateChecksum(bytes)

	return rsdp
}

// Bytes serializes the Rsdp structure into a packed binary byte slice (Little Endian).
// This emulates the behavior of Rust's aml_as_bytes! macro wrapper.
func (r *Rsdp) Bytes() []byte {
	buf := new(bytes.Buffer)
	// Serialize fields sequentially to match memory packing alignment expectations
	binary.Write(buf, binary.LittleEndian, r)
	return buf.Bytes()
}
