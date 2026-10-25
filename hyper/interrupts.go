package hyper

import (
	"fmt"
)

// Constants poached from the apicdef.h kernel header.
const (
	ApicLvt0       = 0x350
	ApicLvt1       = 0x360
	ApicModeNmi    = 0x4
	ApicModeExtint = 0x7
)

// SetApicDeliveryMode updates the delivery mode bits (bits 8-10) of an APIC register.
func SetApicDeliveryMode(reg uint32, mode uint32) uint32 {
	// In Go, '&^' is the bit-clear (AND NOT) operator, equivalent to '& !' in Rust.
	return (reg &^ 0x700) | (mode << 8)
}

// SetLint configures LAPICs. LAPIC0 is set for external interrupts, LAPIC1 is set for NMI.
//
// Arguments:
// * vcpu - The VCPU object to configure.
func SetLint(vcpu Vcpu) error {
	vcpu.BootMsrEntries()
	lapic, err := vcpu.GetLapic()
	if err != nil {
		return fmt.Errorf("failed to get lapic: %w", err)
	}

	// Configure LINT0 for external interrupts (ExtINT).
	lvtLint0 := lapic.GetLapicReg(ApicLvt0)
	lapic.SetLapicReg(ApicLvt0, SetApicDeliveryMode(lvtLint0, ApicModeExtint))

	// Configure LINT1 for non-maskable interrupts (NMI).
	lvtLint1 := lapic.GetLapicReg(ApicLvt1)
	lapic.SetLapicReg(ApicLvt1, SetApicDeliveryMode(lvtLint1, ApicModeNmi))

	// Write the modified LAPIC state back to the vCPU.
	if err := vcpu.SetLapic(&lapic); err != nil {
		return fmt.Errorf("failed to set lapic: %w", err)
	}
	return nil
}
