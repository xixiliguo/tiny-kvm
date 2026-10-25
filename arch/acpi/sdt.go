package acpi

import (
	"bytes"
	"encoding/binary"
)

// GenericAddress represents the ACPI Generic Address Structure (GAS).
type GenericAddress struct {
	AddressSpaceID    uint8
	RegisterBitWidth  uint8
	RegisterBitOffset uint8
	AccessSize        uint8
	Address           uint64
}

// NewIOPortAddress creates a GenericAddress for an I/O port.
func NewIOPortAddress(address uint16) *GenericAddress {
	return &GenericAddress{
		AddressSpaceID:    1, // System I/O
		RegisterBitWidth:  8,
		RegisterBitOffset: 0,
		AccessSize:        1,
		Address:           uint64(address),
	}
}

// NewMMIOAddress creates a GenericAddress for a Memory Mapped I/O address.
func NewMMIOAddress(address uint64) *GenericAddress {

	return &GenericAddress{
		AddressSpaceID:    0, // System Memory
		RegisterBitWidth:  8,
		RegisterBitOffset: 0,
		AccessSize:        1,
		Address:           address,
	}
}

// Sdt models an ACPI System Description Table.
type Sdt struct {
	data []byte
}

// NewSdt builds and allocates a clean standard ACPI SDT header frame.
func NewSdt(
	signature [4]byte,
	length uint32,
	revision uint8,
	oemID [6]byte,
	oemTable [8]byte,
	oemRevision uint32,
) *Sdt {
	if length < 36 {
		panic("ACPI SDT length must be at least 36 bytes")
	}

	data := make([]byte, length)
	copy(data[0:4], signature[:])
	binary.LittleEndian.PutUint32(data[4:8], length)
	data[8] = revision
	data[9] = 0 // Checksum field initial stub

	copy(data[10:16], oemID[:])
	copy(data[16:24], oemTable[:])
	binary.LittleEndian.PutUint32(data[24:28], oemRevision)
	copy(data[28:32], CreatorID[:])
	copy(data[32:36], CreatorRevision[:])

	sdt := &Sdt{data: data}
	sdt.UpdateChecksum()
	return sdt
}

// UpdateChecksum recalculates the ACPI wrapping checksum matrix.
func (s *Sdt) UpdateChecksum() {
	s.data[9] = 0
	s.data[9] = GenerateChecksum(s.data)
}

// Append appends a generic element binary stream directly to the table container end.
// func (s *Sdt) Append(b uint8) {
// 	buf := new(bytes.Buffer)
// 	_ = binary.Write(buf, binary.LittleEndian, value)
// 	valBytes := buf.Bytes()

// 	origLength := len(s.data)
// 	newLength := origLength + len(valBytes)

// 	// Update table length header dynamically at offset [4..8]
// 	s.writeU32(4, uint32(newLength))
// 	s.data = append(s.data, valBytes...)
// }

// AppendSlice appends an arbitrary raw bytes array sequence to the table container end.
func (s *Sdt) AppendSlice(data []byte) {
	origLength := len(s.data)
	newLength := origLength + len(data)

	s.Write(4, uint32(newLength))
	s.data = append(s.data, data...)
	s.UpdateChecksum()
}

func (s *Sdt) AppendAML(aml Aml) {
	s.AppendSlice(aml.ToAmlBytes())
}

// WriteBytes overrides buffer segment at precise positional target bounds.
func (s *Sdt) WriteBytes(offset int, data []byte) {
	if offset+len(data) > len(s.data) {
		panic("out of bounds writing segment directly to SDT buffer slice")
	}
	copy(s.data[offset:offset+len(data)], data)
	s.UpdateChecksum()
}

// Write serializes arbitrary data structure inside exact index mapping boundaries.
func (s *Sdt) Write(offset int, value interface{}) {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, value)
	s.WriteBytes(offset, buf.Bytes())
}

// Len reports current table buffer capacity trace footprint.
func (s *Sdt) Len() int {
	return len(s.data)
}

// IsEmpty evaluates internal byte buffer emptiness.
func (s *Sdt) IsEmpty() bool {
	return len(s.data) == 0
}

func (s *Sdt) ToAmlBytes() []byte {
	return s.data
}

// GenerateChecksum computes wrapping single-byte sum sequence matrix matching zero.
func GenerateChecksum(data []byte) uint8 {
	var sum uint8
	for _, b := range data {
		sum += b
	}
	return -sum
}
