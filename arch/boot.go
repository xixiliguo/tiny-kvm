package arch

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// GuestAddress represents a physical address in the guest memory space.
type GuestAddress uint64 // SetupHeader represents the Linux boot protocol setup header struct.

const SetupHeaderSize = 123

// Fully mapped from standard Linux kernel's arch/x86/include/uapi/asm/bootparam.h
type SetupHeader struct {
	SetupSects          uint8
	RootFlags           uint16
	Syssize             uint32
	RamSize             uint16
	VidMode             uint16
	RootDev             uint16
	BootFlag            uint16
	Jump                uint16
	Header              uint32
	Version             uint16
	RealmodeSwtch       uint32
	StartSysSeg         uint16
	KernelVersion       uint16
	TypeOfLoader        uint8
	Loadflags           uint8
	SetupMoveSize       uint16
	Code32Start         uint32
	RamdiskImage        uint32
	RamdiskSize         uint32
	BootsectKludge      uint32
	HeapEndPtr          uint16
	ExtLoaderVer        uint8
	ExtLoaderType       uint8
	CmdLinePtr          uint32
	InitrdAddrMax       uint32
	KernelAlignment     uint32
	RelocatableKernel   uint8
	MinAlignment        uint8
	Xloadflags          uint16
	CmdlineSize         uint32
	HardwareSubarch     uint32
	HardwareSubarchData uint64
	PayloadOffset       uint32
	PayloadLength       uint32
	SetupData           uint64
	PrefAddress         uint64
	InitSize            uint32
	HandoverOffset      uint32
	KernelInfoOffset    uint32
}

// Configure standard MarshalBinary to use the standard library reflection engine.
func (s *SetupHeader) Bytes() []byte {
	// Pre-allocate a buffer with the exact packed size to avoid dynamic resizing overhead.
	// 123 bytes is the exact mathematical sum of all fields in the Linux setup_header.
	buf := &bytes.Buffer{}

	// binary.Write will reflectively iterate over every single field,
	// stripping Go's native struct paddings automatically.
	binary.Write(buf, binary.LittleEndian, s)
	result := buf.Bytes()
	if len(result) != SetupHeaderSize {
		msg := fmt.Sprintf("size of Marshaling SetupHeader is not %d", SetupHeaderSize)
		panic(msg)
	}
	return buf.Bytes()
}

// EntryPoint specifies the entry point address where the guest must start
// executing code, as well as which of the supported boot protocols
// is to be used to configure the guest initial state.
type EntryPoint struct {
	// EntryAddr is the address in guest memory where the guest must start execution.
	EntryAddr GuestAddress

	// SetupHeader is used for bzImage to fill the zero page.
	// We use a pointer (*SetupHeader) to cleanly replicate Rust's Option<setup_header>.
	// A value of 'nil' denotes 'None'.
	SetupHeader *SetupHeader
}
