package vmm

import (
	"bytes"
	"testing"

	"github.com/xixiliguo/tiny-kvm/arch"
)

func TestGuestMemoryLayout(t *testing.T) {
	memory, err := NewGuestMemory(5 << 30)
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	regions := memory.Regions()
	if len(regions) != 2 {
		t.Fatalf("got %d regions, want 2", len(regions))
	}
	if regions[0].Base() != 0 || regions[0].Size() != 3<<30 {
		t.Fatalf("unexpected low region: base=0x%x size=0x%x", regions[0].Base(), regions[0].Size())
	}
	if regions[1].Base() != arch.Ram64BitStart || regions[1].Size() != 2<<30 {
		t.Fatalf("unexpected high region: base=0x%x size=0x%x", regions[1].Base(), regions[1].Size())
	}
}

func TestGuestMemoryCrossRegionAccess(t *testing.T) {
	memory := &GuestMemory{regions: []*GuestRegion{
		{base: 0x1000, data: make([]byte, 4)},
		{base: 0x1004, data: make([]byte, 4)},
	}}

	expected := []byte{0xaa, 0xbb}
	if _, err := memory.WriteSlice(expected, 0x1003); err != nil {
		t.Fatal(err)
	}
	actual := make([]byte, 2)
	if _, err := memory.ReadSlice(actual, 0x1003); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("read %v, want %v", actual, expected)
	}
}

func TestGuestMemoryRejectsUnmappedRange(t *testing.T) {
	memory, err := NewGuestMemory(2 << 20)
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	if _, err := memory.WriteSlice([]byte{1}, arch.Mem32BitReservedStart); err == nil {
		t.Fatal("expected unmapped write to fail")
	}
}
