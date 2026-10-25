package arch

// Copyright © 2020, Oracle and/or its affiliates.
// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Portions Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE-BSD-3-Clause file.

// Memory layout documentation and constants
// ~~~~~~ ~~~~~~ ~~~~~~~~~~~~~ ~~~ ~~~~~~~~~
// Constants are in order and grouped by range. Take care to update all references
// when making changes and keep them in order.

// ** Low RAM (start: 0, length: 640KiB) **
const LowRamStart uint64 = 0x0

// == Fixed addresses within the "Low RAM" range ==

// Location of EBDA address
const EbdaPointer uint64 = 0x40e

// Initial GDT/IDT needed to boot kernel
const BootGDTStart uint64 = 0x500
const BootIDTStart uint64 = 0x520

// Address for the hvm_start_info struct used in PVH boot
const PvhInfoStart uint64 = 0x6000

// Starting address of array of modules of hvm_modlist_entry type.
// Used to enable initrd support using the PVH boot ABI.
const ModlistStart uint64 = 0x6040

// Address of memory map table used in PVH boot. Can overlap
// with the zero page address since they are mutually exclusive.
const MemmapStart uint64 = 0x7000

// The 'zero page', a.k.a linux kernel bootparams.
const ZeroPageStart uint64 = 0x7000

// Initial stack for the boot CPU.
const BootStackStart uint64 = 0x8000
const BootStackPointer uint64 = 0x8ff0

// Initial pagetables.
const Pml5Start uint64 = 0x9000
const Pml4Start uint64 = 0xa000
const PdpteStart uint64 = 0xb000
const PdeStart uint64 = 0xc000

// Kernel command line start address.
const CmdlineStart uint64 = 0x20000

// Kernel command line start address maximum size.
const CmdlineMaxSize int = 0x10000

// MPTABLE, describing VCPUs.
const MpTableStart uint64 = 0x9fc00

// == End of "Low RAM" range ==

// ** EBDA reserved area (start: 640KiB, length: 384KiB) **
const EbdaStart uint64 = 0xa0000

// == Fixed constants within the "EBDA" range ==

// ACPI RSDP table
const RsdpPointer uint64 = EbdaStart

const SmbiosStart uint64 = 0xf0000 // First possible location per the spec.

// == End of "EBDA" range ==

// ** High RAM (start: 1MiB, length: 3071MiB) **
const HighRamStart uint64 = 0x100000

// == No fixed addresses in the "High RAM" range ==

// ** 32-bit reserved area (start: 3GiB, length: 896MiB) **
const Mem32BitReservedStart uint64 = 0xc000_0000

// Sub range: 32-bit PCI devices (start: 3GiB, length: 640Mib)
const Mem32BitDevicesStart uint64 = Mem32BitReservedStart
const Mem32BitDevicesSize uint64 = 640 << 20

// PCI MMCONFIG space (start: after the device space, length: 256MiB)
const PciMmconfigStart uint64 = Mem32BitDevicesStart + Mem32BitDevicesSize
const PciMmconfigSize uint64 = 256 << 20

// Total size calculation for the 32-bit reserved window
const Mem32BitReservedSize uint64 = PciMmconfigSize + Mem32BitDevicesSize

// One bus with potentially 256 devices (32 slots x 8 functions).
const PciMmioConfigSizePerSegment uint64 = 4096 * 256

// TSS is 3 pages after the PCI MMCONFIG space
const KvmTssStart uint64 = PciMmconfigStart + PciMmconfigSize
const KvmTssSize uint64 = (3 * 4) << 10

// Identity map is a one page region after the TSS
const KvmIdentityMapStart uint64 = KvmTssStart + KvmTssSize
const KvmIdentityMapSize uint64 = 4 << 10

// TPM Address Range (Specific to CRB Interface)
const TpmStart uint64 = 0xfed4_0000
const TpmSize uint64 = 0x1000

// IOAPIC
const IoapicStart uint64 = 0xfec0_0000
const IoapicSize uint64 = 0x20

// APIC
const ApicStart uint64 = 0xfee0_0000

// == End of "32-bit reserved" range ==

// ** 64-bit RAM start (start: 4GiB, length: varies) **
const Ram64BitStart uint64 = 0x1_0000_0000
