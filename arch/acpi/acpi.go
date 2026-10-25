package acpi

import "encoding/binary"

// Creator definitions stub (replace with your package constants if necessary)
var (
	DsdtSignature = [4]byte{'D', 'S', 'D', 'T'}
	DsdtLen       = uint32(36)
	DsdtRivision  = uint8(6)

	FacpSignature = [4]byte{'F', 'A', 'C', 'P'}
	FacpLen       = uint32(276)
	FacpRivision  = uint8(6)

	MadtSignature = [4]byte{'A', 'P', 'I', 'C'}
	MadtLen       = uint32(44)
	MadtRivision  = uint8(5)

	XsdtSignature = [4]byte{'X', 'S', 'D', 'T'}
	XsdtLen       = uint32(36)
	XsdtRivision  = uint8(1)

	OemID           = [6]byte{'T', 'I', 'N', 'Y', 'V', 'M'}
	OemDsdtTable    = [8]byte{'T', 'V', 'D', 'S', 'D', 'T'}
	OemFacpTable    = [8]byte{'T', 'V', 'F', 'A', 'C', 'P'}
	OemMadtTable    = [8]byte{'T', 'V', 'M', 'A', 'D', 'T'}
	OemXsdtTable    = [8]byte{'T', 'V', 'X', 'S', 'D', 'T'}
	OemRevision     = uint32(1)
	CreatorID       = [4]byte{'T', 'V', 'A', 'T'}
	CreatorRevision = [4]byte{0, 0, 0, 1}
)

func NewDsdtTable() *Sdt {
	return NewSdt(DsdtSignature, DsdtLen, DsdtRivision, OemID, OemDsdtTable, OemRevision)
}

func NewFacpTable(dsdtOffset uint64, resetReg *GenericAddress) *Sdt {
	sdt := NewSdt(FacpSignature, FacpLen, FacpRivision, OemID, OemFacpTable, OemRevision)

	if resetReg != nil {
		// RESET_REG
		sdt.Write(116, resetReg)
		// RESET_VALUE
		sdt.Write(128, uint8(1))
	}

	// Architecture common fields
	// HW_REDUCED_ACPI, RESET_REG_SUP, TMR_VAL_EXT
	fadt_flags := uint32((1 << 20) | (1 << 10) | (1 << 8))
	sdt.Write(112, fadt_flags)

	// FADT minor version
	sdt.Write(131, uint8(3))

	// X_DSDT

	const FACP_DSDT_OFFSET = 140
	sdt.Write(FACP_DSDT_OFFSET, dsdtOffset)

	// Hypervisor Vendor Identity
	sdt.WriteBytes(268, []byte("TINYVMM "))

	return sdt

}

func NewXsdtTable(addrs []uint64) *Sdt {
	sdt := NewSdt(XsdtSignature, XsdtLen, XsdtRivision, OemID, OemXsdtTable, OemRevision)
	for _, addr := range addrs {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], addr)
		sdt.AppendSlice(b[:])
	}
	return sdt
}
