package hyper

import (
	"encoding/binary"
	"unsafe"
)

// GetKlapicReg reads a u32 register value at the given offset using Little Endian.
func (s *KvmLapicState) GetlapicReg(regOffset int) uint32 {
	// Slice the buffer from the requested offset
	subSlice := s.Regs[regOffset:]

	// Use unsafe.Slice to safely interpret the []int8 sub-slice as a []byte sub-slice
	// without memory allocation or data copying.
	byteSlice := unsafe.Slice((*byte)(unsafe.Pointer(&subSlice[0])), len(subSlice))

	// Directly read the uint32 value from the byte slice
	return binary.LittleEndian.Uint32(byteSlice)
}

// SetKlapicReg writes a u32 register value at the given offset using Little Endian.
func (s *KvmLapicState) SetlapicReg(regOffset int, value uint32) {
	// Slice the buffer from the requested offset
	subSlice := s.Regs[regOffset:]

	// Cast the []int8 sub-slice to a mutable []byte slice via unsafe.Slice
	byteSlice := unsafe.Slice((*byte)(unsafe.Pointer(&subSlice[0])), len(subSlice))

	// Directly write the uint32 value into the underlying buffer
	binary.LittleEndian.PutUint32(byteSlice, value)
}
