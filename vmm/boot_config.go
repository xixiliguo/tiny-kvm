package vmm

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/xixiliguo/tiny-kvm/arch"
)

const (
	acpiRSDPOffset      = 0x70
	e820CountOffset     = 0x1e8
	e820TableOffset     = 0x2d0
	e820EntrySize       = 20
	maxE820Entries      = 128
	kernelLoaderOther   = 0xff
	pvhMemmapEntrySize  = 24
	pvhStartInfoSize    = 56
	pvhModlistEntrySize = 32
)

type bootPlatform struct {
	serialNumber string
	uuid         string
	oemStrings   []string
}

func configureGuestSystem(
	memory *GuestMemory,
	payload *loadedPayload,
	rsdpAddr uint64,
	platform bootPlatform,
) error {
	ebdaSegment := uint16(arch.EbdaStart >> 4)
	var segmentBytes [2]byte
	binary.LittleEndian.PutUint16(segmentBytes[:], ebdaSegment)
	if _, err := memory.WriteSlice(segmentBytes[:], arch.EbdaPointer); err != nil {
		return fmt.Errorf("boot: write EBDA pointer: %w", err)
	}
	if _, err := arch.SetupSmbios(memory, platform.serialNumber, platform.uuid, platform.oemStrings); err != nil {
		return fmt.Errorf("boot: configure SMBIOS: %w", err)
	}
	if payload.entry.SetupHeader != nil {
		return writeLinuxBootParams(memory, payload, rsdpAddr)
	}
	return writePVHBootInfo(memory, payload, rsdpAddr)
}

func writeLinuxBootParams(memory *GuestMemory, payload *loadedPayload, rsdpAddr uint64) error {
	zeroPage := make([]byte, arch.ZeroPageSize)
	header := *payload.entry.SetupHeader
	if header.TypeOfLoader == 0 {
		header.TypeOfLoader = kernelLoaderOther
	}
	header.CmdLinePtr = uint32(arch.CmdlineStart)
	header.CmdlineSize = uint32(arch.CmdlineMaxSize)
	if payload.initramfs != nil {
		header.RamdiskImage = uint32(payload.initramfs.Address)
		header.RamdiskSize = uint32(payload.initramfs.Size)
	}
	binary.LittleEndian.PutUint64(zeroPage[acpiRSDPOffset:acpiRSDPOffset+8], rsdpAddr)
	copy(zeroPage[setupHeaderOffset:], header.Bytes())

	count := uint8(0)
	addEntry := func(address, size uint64, memoryType uint32) error {
		if count >= maxE820Entries {
			return errors.New("boot: e820 table is full")
		}
		offset := e820TableOffset + int(count)*e820EntrySize
		binary.LittleEndian.PutUint64(zeroPage[offset:offset+8], address)
		binary.LittleEndian.PutUint64(zeroPage[offset+8:offset+16], size)
		binary.LittleEndian.PutUint32(zeroPage[offset+16:offset+20], memoryType)
		count++
		return nil
	}
	if err := addEntry(0, arch.EbdaStart, uint32(arch.E820Ram)); err != nil {
		return err
	}

	memoryEnd := memory.LastAddr()
	if memoryEnd < arch.Mem32BitReservedStart {
		if err := addEntry(arch.HighRamStart, memoryEnd-arch.HighRamStart+1, uint32(arch.E820Ram)); err != nil {
			return err
		}
	} else {
		if err := addEntry(arch.HighRamStart, arch.Mem32BitReservedStart-arch.HighRamStart, uint32(arch.E820Ram)); err != nil {
			return err
		}
		if memoryEnd > arch.Ram64BitStart {
			if err := addEntry(arch.Ram64BitStart, memoryEnd-arch.Ram64BitStart+1, uint32(arch.E820Ram)); err != nil {
				return err
			}
		}
	}
	if err := addEntry(arch.PciMmconfigStart, arch.PciMmconfigSize, uint32(arch.E820Reserved)); err != nil {
		return err
	}
	zeroPage[e820CountOffset] = count
	if _, err := memory.WriteSlice(zeroPage, arch.ZeroPageStart); err != nil {
		return fmt.Errorf("boot: write Linux zero page: %w", err)
	}
	return nil
}

func writePVHBootInfo(memory *GuestMemory, payload *loadedPayload, rsdpAddr uint64) error {
	memmap := make([]byte, 0, 8*pvhMemmapEntrySize)
	addMemmap := func(address, size uint64, memoryType uint32) {
		entry := make([]byte, pvhMemmapEntrySize)
		binary.LittleEndian.PutUint64(entry[0:8], address)
		binary.LittleEndian.PutUint64(entry[8:16], size)
		binary.LittleEndian.PutUint32(entry[16:20], memoryType)
		memmap = append(memmap, entry...)
	}
	addMemmap(0, arch.EbdaStart, uint32(arch.E820Ram))
	ranges, err := memory.GetRamRange()
	if err != nil {
		return err
	}
	for _, region := range ranges {
		start := region.Start
		if start < arch.HighRamStart {
			start = arch.HighRamStart
		}
		if region.End > start {
			addMemmap(start, region.End-start, uint32(arch.E820Ram))
		}
	}
	addMemmap(arch.PciMmconfigStart, arch.PciMmconfigSize, uint32(arch.E820Reserved))

	startInfo := make([]byte, pvhStartInfoSize)
	binary.LittleEndian.PutUint32(startInfo[0:4], 0x336e_c578)
	binary.LittleEndian.PutUint32(startInfo[4:8], 1)
	binary.LittleEndian.PutUint64(startInfo[24:32], arch.CmdlineStart)
	binary.LittleEndian.PutUint64(startInfo[32:40], rsdpAddr)
	binary.LittleEndian.PutUint64(startInfo[40:48], arch.MemmapStart)
	binary.LittleEndian.PutUint32(startInfo[48:52], uint32(len(memmap)/pvhMemmapEntrySize))

	if payload.initramfs != nil {
		binary.LittleEndian.PutUint32(startInfo[12:16], 1)
		binary.LittleEndian.PutUint64(startInfo[16:24], arch.ModlistStart)
		modlist := make([]byte, pvhModlistEntrySize)
		binary.LittleEndian.PutUint64(modlist[0:8], payload.initramfs.Address)
		binary.LittleEndian.PutUint64(modlist[8:16], payload.initramfs.Size)
		if _, err := memory.WriteSlice(modlist, arch.ModlistStart); err != nil {
			return fmt.Errorf("boot: write PVH module list: %w", err)
		}
	}
	if _, err := memory.WriteSlice(memmap, arch.MemmapStart); err != nil {
		return fmt.Errorf("boot: write PVH memory map: %w", err)
	}
	if _, err := memory.WriteSlice(startInfo, arch.PvhInfoStart); err != nil {
		return fmt.Errorf("boot: write PVH start info: %w", err)
	}
	return nil
}
