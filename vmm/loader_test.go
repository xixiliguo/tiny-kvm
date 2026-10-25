package vmm

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/xixiliguo/tiny-kvm/arch"
)

func TestLoadBundledPayload(t *testing.T) {
	kernelPath := "../bzImage"
	initramfsPath := "../initramfs.img"
	if _, err := os.Stat(kernelPath); err != nil {
		t.Skip("bundled bzImage is not available")
	}
	if _, err := os.Stat(initramfsPath); err != nil {
		t.Skip("bundled initramfs is not available")
	}

	memory, err := NewGuestMemory(256 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	payload, err := loadPayload(memory, PayloadConfig{
		Kernel:    kernelPath,
		Initramfs: initramfsPath,
		Cmdline:   "console=ttyS0 quiet",
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload.entry.SetupHeader == nil {
		t.Fatal("bundled bzImage did not expose a setup header")
	}
	if payload.entry.EntryAddr == 0 {
		t.Fatal("bundled bzImage entry point is zero")
	}
	if payload.initramfs == nil || payload.initramfs.Size == 0 {
		t.Fatal("bundled initramfs was not loaded")
	}
	cmdline := mustRead(t, memory, arch.CmdlineStart, len("console=ttyS0 quiet\x00"))
	if string(cmdline) != "console=ttyS0 quiet\x00" {
		t.Fatalf("cmdline = %q", cmdline)
	}
	initrd := make([]byte, payload.initramfs.Size)
	if _, err := memory.ReadSlice(initrd, payload.initramfs.Address); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(initrd[:3], []byte{0x1f, 0x8b, 0x08}) {
		t.Fatalf("initramfs magic = %x", initrd[:3])
	}

	rsdp, err := createACPITables(memory, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := arch.ConfigureSystem(
		memory,
		arch.CmdlineStart,
		arch.CmdlineMaxSize,
		payload.initramfs,
		1,
		payload.entry.SetupHeader,
		rsdp,
		"",
		"",
		nil,
	); err != nil {
		t.Fatal(err)
	}
	var bootParams arch.BootParams
	zeroPage := mustRead(t, memory, arch.ZeroPageStart, arch.ZeroPageSize)
	if err := binary.Read(bytes.NewReader(zeroPage), binary.LittleEndian, &bootParams); err != nil {
		t.Fatal(err)
	}
	if bootParams.Hdr.CmdLinePtr != uint32(arch.CmdlineStart) {
		t.Fatalf("boot_params cmdline pointer = 0x%x", bootParams.Hdr.CmdLinePtr)
	}
	if bootParams.Hdr.RamdiskSize != uint32(payload.initramfs.Size) {
		t.Fatalf("boot_params initramfs size = %d", bootParams.Hdr.RamdiskSize)
	}
	if bootParams.AcpiRsdpAddr != rsdp {
		t.Fatalf("boot_params RSDP = 0x%x, want 0x%x", bootParams.AcpiRsdpAddr, rsdp)
	}
}

func TestLoadBzImage(t *testing.T) {
	memory, err := NewGuestMemory(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	header := arch.SetupHeader{
		SetupSects:        4,
		Header:            setupHeaderMagic,
		Version:           0x020f,
		Code32Start:       uint32(arch.HighRamStart),
		Loadflags:         1,
		RelocatableKernel: 1,
		MinAlignment:      0x20,
	}
	kernel := make([]byte, 5*512+64)
	headerBytes := header.Bytes()
	copy(kernel[setupHeaderOffset:], headerBytes)
	payload := kernel[5*512:]
	for i := range payload {
		payload[i] = byte(i + 1)
	}

	entry, err := loadBzImage(memory, kernel)
	if err != nil {
		t.Fatal(err)
	}
	if entry.SetupHeader == nil || entry.EntryAddr != arch.GuestAddress(arch.HighRamStart) {
		t.Fatalf("unexpected entry point: %+v", entry)
	}
	actual := make([]byte, len(payload))
	if _, err := memory.ReadSlice(actual, arch.HighRamStart); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, payload) {
		t.Fatal("protected payload was not loaded at 1 MiB")
	}
}

func TestLoadBzImageRejectsMalformedHeader(t *testing.T) {
	memory, err := NewGuestMemory(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	data := make([]byte, 4096)
	if _, err := loadBzImage(memory, data); err == nil {
		t.Fatal("expected malformed image to fail")
	}

	header := arch.SetupHeader{Header: setupHeaderMagic, Version: 0x020f, Code32Start: uint32(arch.HighRamStart)}
	copy(data[setupHeaderOffset:], header.Bytes())
	binary.LittleEndian.PutUint32(data[setupHeaderOffset+17:setupHeaderOffset+21], 0)
	if _, err := loadBzImage(memory, data); err == nil {
		t.Fatal("expected bad magic to fail")
	}
}
