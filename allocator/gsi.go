// Copyright © 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0 OR BSD-3-Clause

package allocator

import (
	"sort"
)

// AllocateGsi allocates a general Global System Interrupt (GSI).
func (g *GsiAllocator) AllocateGsi() (uint32, error) {
	gsi := g.nextGsi
	if g.nextGsi == 0xffffffff {
		return 0, ErrOverflow
	}
	g.nextGsi++
	return gsi, nil
}

// GsiApic defines basic parameters for an I/O APIC routing domain.
type GsiApic struct {
	Base uint32
	Irqs uint32
}

// NewGsiApic initializes a specific GsiApic block.
func NewGsiApic(base uint32, irqs uint32) GsiApic {
	return GsiApic{Base: base, Irqs: irqs}
}

type apicEntry struct {
	base uint32
	irqs uint32
}

// GsiAllocator acts as a state coordinator for mapping architecture interrupts.
type GsiAllocator struct {
	apics   []apicEntry // Kept sorted by base to simulate BTreeMap behavior
	nextIrq uint32
	nextGsi uint32
}

// NewGsiAllocator instantiates an allocator given the specific I/O APIC layout topology.
func NewGsiAllocator(apics []GsiApic) *GsiAllocator {
	allocator := &GsiAllocator{
		apics:   make([]apicEntry, 0, len(apics)),
		nextIrq: 0xffffffff,
		nextGsi: 0,
	}

	for _, apic := range apics {
		if apic.Base < allocator.nextIrq {
			allocator.nextIrq = apic.Base
		}

		if apic.Base+apic.Irqs > allocator.nextGsi {
			allocator.nextGsi = apic.Base + apic.Irqs
		}

		// Insert and maintain sort order by apic.Base
		allocator.insertApic(apic.Base, apic.Irqs)
	}

	return allocator
}

// AllocateIrq allocates an IRQ pin on x86_64 platforms.
func (g *GsiAllocator) AllocateIrq() (uint32, error) {
	var irq uint32 = 0

	for _, entry := range g.apics {
		// HACKHACK - This only works with 1 single IOAPIC...
		if g.nextIrq >= entry.base && g.nextIrq < entry.base+entry.irqs {
			irq = g.nextIrq
			g.nextIrq++
			break
		}
	}

	if irq == 0 {
		return 0, ErrOverflow
	}

	return irq, nil
}

func (g *GsiAllocator) insertApic(base uint32, irqs uint32) {
	idx := sort.Search(len(g.apics), func(i int) bool {
		return g.apics[i].base >= base
	})

	g.apics = append(g.apics, apicEntry{})
	copy(g.apics[idx+1:], g.apics[idx:])
	g.apics[idx] = apicEntry{base: base, irqs: irqs}
}
