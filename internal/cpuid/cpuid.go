package cpuid

func cpuid_low(arg1, arg2 uint32) (eax, ebx, ecx, edx uint32) // implemented in cpuidlow_amd64.s

// Native CPUID register output struct.
// type CpuidLeaf struct {
// 	Eax, Ebx, Ecx, Edx uint32
// }

// Cpuid mimics native assembly execution of CPUID instruction.
// You must provide an assembly file (.s) mapping to this signature or use runtime implementations.
func Cpuid(function uint32, index uint32) (eax, ebx, ecx, edx uint32) {
	// Dummy runtime placeholder; replace with actual raw assembly invocation in practice.
	eax, ebx, ecx, edx = cpuid_low(function, index)
	return
	// return CpuidLeaf{eax, ebx, ecx, edx}
}
