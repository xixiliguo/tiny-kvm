package arch

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Operational errors replicating the original 'thiserror' architecture.
var (
	ErrNotEnoughMemory = errors.New("there was too little guest memory to store the SMBIOS table")
	ErrAddressOverflow = errors.New("the SMBIOS table has too little address space to be stored")
	ErrClear           = errors.New("failure while zeroing out the memory for the SMBIOS table")
	ErrWriteSmbiosEp   = errors.New("failure to write SMBIOS entrypoint structure")
	ErrWriteData       = errors.New("failure to write additional data to memory")
)

// SMBIOS specification constants
var sm3MagicIdent = [5]byte{'_', 'S', 'M', '3', '_'}

const (
	BiosInformation   uint8  = 0
	SystemInformation uint8  = 1
	OemStrings        uint8  = 11
	EndOfTable        uint8  = 127
	PciSupported      uint64 = 1 << 7
	IsVirtualMachine  uint8  = 1 << 4
)

const Smbios30EntrypointSize = 24

// Packed structure definitions matching x86 hardware layouts exactly.
type Smbios30Entrypoint struct {
	Signature [5]byte
	Checksum  uint8
	Length    uint8
	Majorver  uint8
	Minorver  uint8
	Docrev    uint8
	Revision  uint8
	Reserved  uint8
	MaxSize   uint32
	Physptr   uint64
}

const SmbiosBiosInfoSize = 20

type SmbiosBiosInfo struct {
	Type                uint8
	Length              uint8
	Handle              uint16
	Vendor              uint8
	Version             uint8
	StartAddr           uint16
	ReleaseDate         uint8
	RomSize             uint8
	Characteristics     uint64
	CharacteristicsExt1 uint8
	CharacteristicsExt2 uint8
}

const SmbiosSysInfoSize = 27

type SmbiosSysInfo struct {
	Type         uint8
	Length       uint8
	Handle       uint16
	Manufacturer uint8
	ProductName  uint8
	Version      uint8
	SerialNumber uint8
	Uuid         [16]byte
	WakeUpType   uint8
	Sku          uint8
	Family       uint8
}

const SmbiosOemStringsSize = 5

type SmbiosOemStrings struct {
	Type   uint8
	Length uint8
	Handle uint16
	Count  uint8
}

const SmbiosEndOfTableSize = 4

type SmbiosEndOfTable struct {
	Type   uint8
	Length uint8
	Handle uint16
}

// ComputeChecksum calculates the two's complement checksum across a packed struct's bytes.
func computeChecksum(v any) uint8 {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, v)
	var checksum uint8
	for _, b := range buf.Bytes() {
		checksum += b
	}
	return (-checksum)
}

func writeAndIncr(mem GuestMemoryMmap, val any, curptr uint64) (uint64, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, val); err != nil {
		return curptr, ErrWriteData
	}

	payload := buf.Bytes()
	if _, err := mem.WriteSlice(payload, curptr); err != nil {
		return curptr, ErrWriteData
	}

	nextPtr := curptr + uint64(len(payload))
	if nextPtr < curptr { // Overflow verification
		return curptr, ErrNotEnoughMemory
	}
	return nextPtr, nil
}

func writeString(mem GuestMemoryMmap, val string, curptr uint64) (uint64, error) {
	var err error
	for i := 0; i < len(val); i++ {
		curptr, err = writeAndIncr(mem, val[i], curptr)
		if err != nil {
			return curptr, err
		}
	}
	// Append the mandatory null terminator for SMBIOS text structures
	return writeAndIncr(mem, uint8(0), curptr)
}

// SetupSmbios serializes and writes structured SMBIOS 3.0 descriptor components into Guest Memory maps.
func SetupSmbios(
	mem GuestMemoryMmap,
	serialNumber string,
	uuidStr string,
	oemStrings []string,
) (uint64, error) {

	physptr := SmbiosStart + Smbios30EntrypointSize
	if physptr < SmbiosStart {
		return 0, ErrNotEnoughMemory
	}

	curptr := physptr
	var handle uint16 = 0

	// 1. BIOS Information Block (Type 0)
	{
		handle++
		biosInfo := SmbiosBiosInfo{
			Type:                BiosInformation,
			Length:              SmbiosBiosInfoSize,
			Handle:              handle,
			Vendor:              1,
			Version:             2,
			Characteristics:     PciSupported,
			CharacteristicsExt2: IsVirtualMachine,
		}
		var err error
		if curptr, err = writeAndIncr(mem, biosInfo, curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeString(mem, "tiny-vmm", curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeString(mem, "0", curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeAndIncr(mem, uint8(0), curptr); err != nil {
			return 0, err
		}
	}

	// 2. System Information Block (Type 1)
	{
		handle++
		var uuidBytes [16]byte = uuid.New()
		if uuidStr != "" {
			parsedUuid, err := uuid.Parse(uuidStr)
			if err != nil {
				return 0, fmt.Errorf("failure to parse uuid %s: %w", uuidStr, err)
			}
			// System UUIDs are conventionally stored in little-endian within SMBIOS tables
			copy(uuidBytes[:], parsedUuid[:])
		}

		var serialNumIdx uint8 = 0
		if serialNumber != "" {
			serialNumIdx = 3
		}

		sysInfo := SmbiosSysInfo{
			Type:         SystemInformation,
			Length:       SmbiosSysInfoSize,
			Handle:       handle,
			Manufacturer: 1,
			ProductName:  2,
			SerialNumber: serialNumIdx,
			Uuid:         uuidBytes,
		}

		var err error
		if curptr, err = writeAndIncr(mem, sysInfo, curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeString(mem, "xixiliguo", curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeString(mem, "tiny-vmm", curptr); err != nil {
			return 0, err
		}
		if serialNumber != "" {
			if curptr, err = writeString(mem, serialNumber, curptr); err != nil {
				return 0, err
			}
		}
		if curptr, err = writeAndIncr(mem, uint8(0), curptr); err != nil {
			return 0, err
		}
	}

	// 3. Optional OEM Strings Block (Type 11)
	if len(oemStrings) > 0 {
		handle++
		oemBlock := SmbiosOemStrings{
			Type:   OemStrings,
			Length: SmbiosOemStringsSize,
			Handle: handle,
			Count:  uint8(len(oemStrings)),
		}

		var err error
		if curptr, err = writeAndIncr(mem, oemBlock, curptr); err != nil {
			return 0, err
		}
		for _, s := range oemStrings {
			if curptr, err = writeString(mem, s, curptr); err != nil {
				return 0, err
			}
		}
		if curptr, err = writeAndIncr(mem, uint8(0), curptr); err != nil {
			return 0, err
		}
	}

	// 4. End Of Table Block (Type 127)
	{
		handle++
		endBlock := SmbiosEndOfTable{
			Type:   EndOfTable,
			Length: SmbiosEndOfTableSize,
			Handle: handle,
		}
		var err error
		if curptr, err = writeAndIncr(mem, endBlock, curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeAndIncr(mem, uint8(0), curptr); err != nil {
			return 0, err
		}
		if curptr, err = writeAndIncr(mem, uint8(0), curptr); err != nil {
			return 0, err
		}
	}

	// 5. Finalize SMBIOS 3.0 Anchor Entrypoint Struct
	tableMaxSize := uint32(uint64(curptr) - physptr)
	ep := Smbios30Entrypoint{
		Signature: sm3MagicIdent,
		Length:    Smbios30EntrypointSize,
		Majorver:  0x03,
		Minorver:  0x02,
		Docrev:    0x00,
		Revision:  0x01,
		MaxSize:   tableMaxSize,
		Physptr:   physptr,
	}
	ep.Checksum = computeChecksum(ep)

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, ep)
	if _, err := mem.WriteSlice(buf.Bytes(), SmbiosStart); err != nil {
		return 0, ErrWriteSmbiosEp
	}

	return uint64(tableMaxSize) + Smbios30EntrypointSize, nil
}
