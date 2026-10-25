package vmm

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/xixiliguo/tiny-kvm/arch"
)

const (
	setupHeaderOffset = 0x1f1
	setupHeaderMagic  = 0x53726448 // "HdrS"
	xenPhys32Entry    = 18
)

type loadedPayload struct {
	entry     arch.EntryPoint
	initramfs *arch.InitramfsConfig
}

func loadPayload(memory *GuestMemory, config PayloadConfig) (*loadedPayload, error) {
	kernel, err := os.ReadFile(config.Kernel)
	if err != nil {
		return nil, fmt.Errorf("payload: read kernel %q: %w", config.Kernel, err)
	}
	if len(kernel) == 0 {
		return nil, errors.New("payload: kernel file is empty")
	}

	var entry arch.EntryPoint
	var loadErr error
	if len(kernel) >= 4 && bytes.Equal(kernel[:4], []byte(elf.ELFMAG)) {
		entry, loadErr = loadELF(memory, kernel)
	} else {
		entry, loadErr = loadBzImage(memory, kernel)
	}
	if loadErr != nil {
		return nil, loadErr
	}

	cmdline := config.Cmdline
	if cmdline == "" {
		cmdline = DefaultCmdline
	}
	if len(cmdline)+1 > arch.CmdlineMaxSize {
		return nil, fmt.Errorf("payload: command line is larger than %d bytes", arch.CmdlineMaxSize-1)
	}
	if _, err := memory.WriteSlice(append([]byte(cmdline), 0), arch.CmdlineStart); err != nil {
		return nil, fmt.Errorf("payload: write command line: %w", err)
	}

	var initramfs *arch.InitramfsConfig
	if config.Initramfs != "" {
		data, err := os.ReadFile(config.Initramfs)
		if err != nil {
			return nil, fmt.Errorf("payload: read initramfs %q: %w", config.Initramfs, err)
		}
		if len(data) > 0 {
			address, err := arch.InitramfsLoadAddr(memory, len(data))
			if err != nil {
				return nil, fmt.Errorf("payload: find initramfs address: %w", err)
			}
			if _, err := memory.WriteSlice(data, address); err != nil {
				return nil, fmt.Errorf("payload: write initramfs: %w", err)
			}
			initramfs = &arch.InitramfsConfig{Address: address, Size: uint64(len(data))}
		}
	}

	return &loadedPayload{entry: entry, initramfs: initramfs}, nil
}

func loadBzImage(memory *GuestMemory, kernel []byte) (arch.EntryPoint, error) {
	if len(kernel) < setupHeaderOffset+arch.SetupHeaderSize {
		return arch.EntryPoint{}, errors.New("payload: kernel is too small to contain a bzImage setup header")
	}
	headerBytes := kernel[setupHeaderOffset : setupHeaderOffset+arch.SetupHeaderSize]
	var header arch.SetupHeader
	if err := binary.Read(bytes.NewReader(headerBytes), binary.LittleEndian, &header); err != nil {
		return arch.EntryPoint{}, fmt.Errorf("payload: decode setup header: %w", err)
	}
	if header.Header != setupHeaderMagic {
		return arch.EntryPoint{}, fmt.Errorf("payload: unsupported kernel image (marker 0x%08x)", header.Header)
	}
	if header.Version < 0x0200 {
		return arch.EntryPoint{}, fmt.Errorf("payload: boot protocol 0x%x is too old", header.Version)
	}

	setupSectors := uint32(header.SetupSects)
	if setupSectors == 0 {
		setupSectors = 4
	}
	protectedOffset := (setupSectors + 1) * 512
	if uint64(protectedOffset) >= uint64(len(kernel)) {
		return arch.EntryPoint{}, errors.New("payload: bzImage protected-mode payload is missing")
	}

	entry := uint64(header.Code32Start)
	if entry == 0 {
		entry = arch.HighRamStart
	}
	if _, err := memory.WriteSlice(kernel[protectedOffset:], entry); err != nil {
		return arch.EntryPoint{}, fmt.Errorf("payload: load bzImage at 0x%x: %w", entry, err)
	}
	header.Code32Start = uint32(entry)
	return arch.EntryPoint{
		EntryAddr:   arch.GuestAddress(entry),
		SetupHeader: &header,
	}, nil
}

func loadELF(memory *GuestMemory, kernel []byte) (arch.EntryPoint, error) {
	file, err := elf.NewFile(bytes.NewReader(kernel))
	if err != nil {
		return arch.EntryPoint{}, fmt.Errorf("payload: parse ELF kernel: %w", err)
	}
	defer file.Close()
	if file.Class != elf.ELFCLASS64 || file.Machine != elf.EM_X86_64 {
		return arch.EntryPoint{}, errors.New("payload: kernel ELF must be x86_64")
	}

	loaded := false
	for _, program := range file.Progs {
		if program.Type != elf.PT_LOAD || program.Filesz == 0 {
			continue
		}
		if program.Off+program.Filesz > uint64(len(kernel)) {
			return arch.EntryPoint{}, errors.New("payload: ELF segment extends past end of file")
		}
		target := program.Paddr
		if target == 0 {
			target = program.Vaddr
		}
		if target == 0 {
			continue
		}
		if _, err := memory.WriteSlice(kernel[program.Off:program.Off+program.Filesz], target); err != nil {
			return arch.EntryPoint{}, fmt.Errorf("payload: load ELF segment at 0x%x: %w", target, err)
		}
		loaded = true
	}
	if !loaded {
		return arch.EntryPoint{}, errors.New("payload: ELF kernel has no loadable segments")
	}

	entry, err := findPVHEntry(file, kernel)
	if err != nil {
		return arch.EntryPoint{}, err
	}
	return arch.EntryPoint{EntryAddr: arch.GuestAddress(entry)}, nil
}

func findPVHEntry(file *elf.File, kernel []byte) (uint64, error) {
	for _, program := range file.Progs {
		if program.Type != elf.PT_NOTE {
			continue
		}
		if program.Off+program.Filesz > uint64(len(kernel)) {
			return 0, errors.New("payload: ELF note segment extends past end of file")
		}
		notes := kernel[program.Off : program.Off+program.Filesz]
		for len(notes) >= 12 {
			nameSize := binary.LittleEndian.Uint32(notes[0:4])
			descSize := binary.LittleEndian.Uint32(notes[4:8])
			noteType := binary.LittleEndian.Uint32(notes[8:12])
			notes = notes[12:]
			nameEnd := align4(uint64(nameSize))
			descEnd := align4(uint64(descSize))
			if nameEnd+descEnd > uint64(len(notes)) {
				break
			}
			name := bytes.TrimRight(notes[:nameSize], "\x00")
			desc := notes[nameEnd : nameEnd+descEnd]
			if string(name) == "Xen" && noteType == xenPhys32Entry && len(desc) >= 4 {
				return uint64(binary.LittleEndian.Uint32(desc[:4])), nil
			}
			notes = notes[nameEnd+descEnd:]
		}
	}
	return 0, errors.New("payload: ELF kernel does not expose a Xen PVH entry point")
}

func align4(value uint64) uint64 {
	return (value + 3) &^ 3
}
