package vmm

import (
	"bytes"
	"testing"
)

type memoryDevice struct {
	value byte
}

func (d *memoryDevice) Read(_ uint64, _ uint64, data []byte) {
	for i := range data {
		data[i] = d.value
	}
}

func (d *memoryDevice) Write(_ uint64, _ uint64, data []byte) error {
	if len(data) > 0 {
		d.value = data[0]
	}
	return nil
}

func TestBusDispatchAndRangeChecks(t *testing.T) {
	bus := NewBus()
	dev := &memoryDevice{value: 0x11}
	if err := bus.Insert("test", 0x100, 4, dev); err != nil {
		t.Fatal(err)
	}
	if err := bus.Insert("overlap", 0x103, 2, dev); err == nil {
		t.Fatal("expected overlapping insertion to fail")
	}

	if err := bus.Write(0x102, []byte{0x42}); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 2)
	bus.Read(0x101, data)
	if !bytes.Equal(data, []byte{0x42, 0x42}) {
		t.Fatalf("bus read = %v", data)
	}

	data = []byte{0xff}
	bus.Read(0x200, data)
	if data[0] != 0 {
		t.Fatalf("unmapped read = 0x%x, want 0", data[0])
	}
}
