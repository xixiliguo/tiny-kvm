package acpi

import (
	"testing"
)

func TestSdt(t *testing.T) {
	sdt := NewSdt([...]byte{'a', 'b', 'c', 'd'}, 37, 1, [...]byte{'a', 'b', 'c', 'd', 'e', 'f'},
		[...]byte{'a', 'b', 'c', 'd', 'e', 'x', 'y', 'z'}, 0xf)

	t.Logf("sdt %+v", sdt)

	result := GenerateChecksum(sdt.data)
	t.Logf("result %+v", result)

}
