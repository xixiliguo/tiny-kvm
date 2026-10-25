package hyper

const (
	KvmCpuIdFlagSignifcantIndex = 1
)

func (e *KvmCpuidEntry2) into() CpuIdEntry {

	flags := uint32(0)
	if e.Flags&KvmCpuIdFlagSignifcantIndex != 0 {
		flags = KvmCpuIdFlagSignifcantIndex
	} else {
		flags = 0
	}
	return CpuIdEntry{
		Function: e.Function,
		Index:    e.Index,
		Flags:    flags,
		Eax:      e.Eax,
		Ebx:      e.Ebx,
		Ecx:      e.Ecx,
		Edx:      e.Edx,
	}
}

func (e *KvmCpuidEntry2) from(c CpuIdEntry) {
	flags := uint32(0)
	if c.Flags&CpuIdFlagValidIndex != 0 {
		flags = KvmCpuIdFlagSignifcantIndex
	} else {
		flags = 0
	}
	e.Function = c.Function
	e.Index = c.Index
	e.Flags = flags
	e.Eax = c.Eax
	e.Ebx = c.Ebx
	e.Ecx = c.Ecx
	e.Edx = c.Edx
}

func (e *KvmMsrEntry) into() MsrEntry {
	return MsrEntry{
		Index: e.Index,
		Data:  e.Data,
	}
}

func (e *KvmMsrEntry) from(m MsrEntry) {
	e.Index = m.Index
	e.Data = m.Data
}

type KvmRegs StandardRegisters
type KvmSegment SegmentRegister
type KvmDtable DescriptorTable
type KvmSregs SpecialRegisters

type KvmIrqRoutingEntry IrqRoutingEntry
