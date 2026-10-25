package arch

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Define dummy or exact structures for sub-components to ensure compilation.
type ScreenInfo [64]byte
type ApmBiosInfo [20]byte
type IstInfo [16]byte
type SysDescTable [16]byte
type OlpcOfwHeader [16]byte
type EdidInfo [128]byte
type EfiInfo [32]byte

type BootE820Entry struct {
	Addr uint64
	Size uint64
	Type uint32
}

type EddInfo [82]byte

// BootParams represents the Linux x86 boot parameters (Zero Page).
// All field names are exported using standard Go CamelCase.
type BootParams struct {
	ScreenInfo          ScreenInfo
	ApmBiosInfo         ApmBiosInfo
	Pad2                [4]byte
	TbootAddr           uint64
	IstInfo             IstInfo
	AcpiRsdpAddr        uint64
	Pad3                [8]byte
	Hd0Info             [16]byte
	Hd1Info             [16]byte
	SysDescTable        SysDescTable
	OlpcOfwHeader       OlpcOfwHeader
	ExtRamdiskImage     uint32
	ExtRamdiskSize      uint32
	ExtCmdLinePtr       uint32
	Pad4                [112]byte
	CcBlobAddress       uint32
	EdidInfo            [128]byte // Replaced edid_info with explicit sized array if defined externally
	EfiInfo             EfiInfo
	AltMemK             uint32
	Scratch             uint32
	E820Entries         uint8
	EddbufEntries       uint8
	EddMbrSigBufEntries uint8
	KbdStatus           uint8
	SecureBoot          uint8
	Pad5                [2]byte
	Sentinel            uint8
	Pad6                [1]byte
	Hdr                 SetupHeader // Reuses the SetupHeader struct we translated previously
	Pad7                [36]byte
	EddMbrSigBuffer     [16]uint32
	E820Table           [128]BootE820Entry
	Pad8                [48]byte
	Eddbuf              [6]EddInfo
	Pad9                [276]byte
}

// ZeroPageSize is exactly 4096 bytes as dictated by the Linux x86/x86_64 boot protocol.
const ZeroPageSize = 4096

// MarshalBinary serializes the BootParams struct into a tightly packed []byte
// that strictly conforms to the Linux kernel C-packed memory layout.
func (bp *BootParams) Bytes() []byte {
	buf := &bytes.Buffer{}

	binary.Write(buf, binary.LittleEndian, bp)
	result := buf.Bytes()
	if len(result) != ZeroPageSize {
		msg := fmt.Sprintf("size of Marshaling BootParams is not %d", ZeroPageSize)
		panic(msg)
	}

	return result
}
