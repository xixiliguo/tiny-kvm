package acpi

import (
	"testing"
)

func TestRsdp(t *testing.T) {
	rsdp := NewRsdp([...]byte{'a', 'b', 'c', 'd', 'e', 'f'}, 0xf)
	t.Logf("rsdp %+v", rsdp)

	result := GenerateChecksum(rsdp.Bytes())
	t.Logf("result %+v", result)

}
