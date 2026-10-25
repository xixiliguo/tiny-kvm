package vmm

import (
	"encoding/binary"
	"testing"

	"github.com/xixiliguo/tiny-kvm/arch"
)

func TestCreateACPUTables(t *testing.T) {
	memory, err := NewGuestMemory(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	rsdpAddr, err := createACPITables(memory, 2)
	if err != nil {
		t.Fatal(err)
	}
	if rsdpAddr != arch.RsdpPointer {
		t.Fatalf("RSDP address = 0x%x", rsdpAddr)
	}

	rsdp := make([]byte, 36)
	if _, err := memory.ReadSlice(rsdp, rsdpAddr); err != nil {
		t.Fatal(err)
	}
	if string(rsdp[:8]) != "RSD PTR " {
		t.Fatalf("bad RSDP signature %q", rsdp[:8])
	}
	var legacySum, extendedSum byte
	for _, value := range rsdp[:20] {
		legacySum += value
	}
	for _, value := range rsdp {
		extendedSum += value
	}
	if legacySum != 0 || extendedSum != 0 {
		t.Fatalf("bad RSDP checksums: legacy=0x%x extended=0x%x", legacySum, extendedSum)
	}

	xsdtAddr := binary.LittleEndian.Uint64(rsdp[24:32])
	if string(mustRead(t, memory, xsdtAddr, 4)) != "XSDT" {
		t.Fatal("RSDP does not point to an XSDT")
	}
}

func mustRead(t *testing.T, memory *GuestMemory, address uint64, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if _, err := memory.ReadSlice(data, address); err != nil {
		t.Fatal(err)
	}
	return data
}
