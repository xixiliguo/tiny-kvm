package vmm

import (
	"errors"
	"fmt"

	"github.com/xixiliguo/tiny-kvm/arch"
	"github.com/xixiliguo/tiny-kvm/arch/acpi"
)

const (
	acpiTablesStart = uint64(0xb0000)
	acpiTablesEnd   = arch.SmbiosStart
)

// createACPITables writes a minimal ACPI 2.0 table set and returns the RSDP
// address supplied to the Linux boot protocol.
func createACPITables(memory *GuestMemory, bootVcpus uint32) (uint64, error) {
	locals := make([]acpi.LocalX2Apic, bootVcpus)
	for i := range locals {
		locals[i] = acpi.LocalX2Apic{
			Type:        acpi.MadtTypeLocalX2Apic,
			Length:      16,
			ApicID:      uint32(i),
			Flags:       1,
			ProcessorID: uint32(i),
		}
	}

	dsdtAddr := acpiTablesStart
	dsdt := acpi.NewDsdtTable()
	facpAddr := dsdtAddr + uint64(dsdt.Len())
	facp := acpi.NewFacpTable(dsdtAddr, nil)
	madtAddr := facpAddr + uint64(facp.Len())
	madt := acpi.NewMadtTable(locals)
	xsdtAddr := madtAddr + uint64(madt.Len())
	xsdt := acpi.NewXsdtTable([]uint64{facpAddr, madtAddr})

	tables := []struct {
		name string
		addr uint64
		data []byte
	}{
		{"DSDT", dsdtAddr, dsdt.ToAmlBytes()},
		{"FACP", facpAddr, facp.ToAmlBytes()},
		{"MADT", madtAddr, madt.ToAmlBytes()},
		{"XSDT", xsdtAddr, xsdt.ToAmlBytes()},
	}
	for _, table := range tables {
		if table.addr < acpiTablesStart || table.addr+uint64(len(table.data)) > acpiTablesEnd {
			return 0, fmt.Errorf("acpi: %s does not fit in the EBDA table area", table.name)
		}
		if _, err := memory.WriteSlice(table.data, table.addr); err != nil {
			return 0, fmt.Errorf("acpi: write %s: %w", table.name, err)
		}
	}

	rsdp := acpi.NewRsdp(acpi.OemID, xsdtAddr)
	if _, err := memory.WriteSlice(rsdp.Bytes(), arch.RsdpPointer); err != nil {
		return 0, fmt.Errorf("acpi: write RSDP: %w", err)
	}
	if arch.RsdpPointer+uint64(len(rsdp.Bytes())) > acpiTablesStart {
		return 0, errors.New("acpi: RSDP overlaps low boot structures")
	}
	return arch.RsdpPointer, nil
}
