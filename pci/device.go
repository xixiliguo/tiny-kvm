// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE-BSD-3-Clause file.
//
// SPDX-License-Identifier: Apache-2.0 AND BSD-3-Clause

package pci

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --- Errors for device management ---

// Error defines the base interface for PCI device errors.
type Error interface {
	error
	isPciDeviceError()
}

// CapabilitiesSetupError represents: Setup of the device capabilities failed.
type CapabilitiesSetupError struct {
	Source error
}

func (e *CapabilitiesSetupError) Error() string {
	return fmt.Sprintf("Setup of the device capabilities failed: %v", e.Source)
}
func (e *CapabilitiesSetupError) Unwrap() error     { return e.Source }
func (e *CapabilitiesSetupError) isPciDeviceError() {}

// IoAllocationFailedError represents: Allocating space for an IO BAR failed.
type IoAllocationFailedError struct {
	Size uint64
}

func (e *IoAllocationFailedError) Error() string {
	return fmt.Sprintf("Allocating space for an IO BAR failed: size 0x%x", e.Size)
}
func (e *IoAllocationFailedError) isPciDeviceError() {}

// IoRegistrationFailedError represents: Registering an IO BAR failed.
type IoRegistrationFailedError struct {
	Base   uint64
	Source error
}

func (e *IoRegistrationFailedError) Error() string {
	return fmt.Sprintf("Registering an IO BAR failed at 0x%x: %v", e.Base, e.Source)
}
func (e *IoRegistrationFailedError) Unwrap() error     { return e.Source }
func (e *IoRegistrationFailedError) isPciDeviceError() {}

// MissingResourceError represents: Expected resource not found.
type MissingResourceError struct{}

func (e *MissingResourceError) Error() string {
	return "Expected resource not found"
}
func (e *MissingResourceError) isPciDeviceError() {}

// InvalidResourceError represents: Invalid resource.
type InvalidResourceError struct {
	Res Resource
}

func (e *InvalidResourceError) Error() string {
	return fmt.Sprintf("Invalid resource: %+v", e.Res)
}
func (e *InvalidResourceError) isPciDeviceError() {}

// --- Core Structs ---

// BarReprogrammingParams defines parameters when a BAR is intercepted and requires reconfiguration.
type BarReprogrammingParams struct {
	OldBase    uint64
	NewBase    uint64
	Len        uint64
	RegionType PciBarRegionType
}

// --- Interfaces ---

// PciDevice encapsulates the functionality of an emulated PCI device.
// Equivalent to Rust's `pub trait PciDevice: Send`.
type PciDevice interface {
	// AllocateBars allocates the needed PCI BARs space.
	// Returns a slice of PciBarConfiguration or an error.
	AllocateBars(
		allocator SystemAllocator, // Assumes concurrent safety or internal syncing
		mmio32Allocator AddressAllocator,
		mmio64Allocator AddressAllocator,
		resources []Resource,
	) ([]PciBarConfiguration, error)

	// FreeBars frees the PCI BARs previously allocated with AllocateBars.
	FreeBars(
		allocator SystemAllocator,
		mmio32Allocator AddressAllocator,
		mmio64Allocator AddressAllocator,
	) error

	// WriteConfigRegister sets a register in the configuration space.
	// Returns a slice of reprogramming actions and an optional synchronization channel.
	WriteConfigRegister(regIdx int, offset uint64, data []byte) ([]BarReprogrammingParams, chan struct{})

	// ReadConfigRegister gets a 32-bit value from the configuration space register.
	ReadConfigRegister(regIdx int) uint32

	// ReadBar reads from a BAR region mapped into the device.
	ReadBar(base uint64, offset uint64, data []byte)

	// WriteBar writes to a BAR region mapped into the device.
	WriteBar(base uint64, offset uint64, data []byte) chan struct{}

	// MoveBar relocates the BAR to a different address in guest address space.
	MoveBar(oldBase uint64, newBase uint64) error

	// AsAnyMut provides an interface placeholder for runtime type introspection.
	// Mimics Rust's `fn as_any_mut(&mut self) -> &mut dyn Any`.
	AsAnyMut() any

	// ID optionally returns a unique identifier. Second value is false if ID is missing.
	ID() (string, bool)
}

// DeviceRelocation defines a set of actions triggered whenever a PCI device topology is altered.
// Equivalent to Rust's `pub trait DeviceRelocation: Send + Sync`.
type DeviceRelocation interface {
	// MoveBar moves a BAR to a different location in the guest address space following a guest command.
	MoveBar(
		oldBase uint64,
		newBase uint64,
		length uint64,
		pciDev PciDevice,
		regionType PciBarRegionType,
	) error
}

// --- Mocking unresolved types from outer crates for compilation placeholder ---

type Resource any
type SystemAllocator any
type AddressAllocator any

// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE-BSD-3-Clause file.
//
// SPDX-License-Identifier: Apache-2.0 AND BSD-3-Clause

// PciInterruptPin defines the four standard PCI interrupt pins A->D.
type PciInterruptPin uint32

const (
	PciInterruptPinIntA PciInterruptPin = iota
	PciInterruptPinIntB
	PciInterruptPinIntC
	PciInterruptPinIntD
)

// ToMask returns the numeric mask value of the interrupt pin.
func (p PciInterruptPin) ToMask() uint32 {
	return uint32(p)
}

const (
	// PciConfigIoPort defines the x86 IO port for PCI Configuration Address (0xCF8).
	PciConfigIoPort uint64 = 0xcf8
	// PciConfigIoPortSize defines the size of the PCI IO port region.
	PciConfigIoPortSize uint64 = 0x8
)

// PciBdf represents a PCI Segment, Bus, Device, and Function topology tuple packed into a uint32.
type PciBdf uint32

// NewPciBdf creates and packs fields into a PciBdf identifier.
func NewPciBdf(segment uint16, bus uint8, device uint8, function uint8) PciBdf {
	return PciBdf(
		(uint32(segment) << 16) |
			(uint32(bus) << 8) |
			((uint32(device) & 0x1f) << 3) |
			(uint32(function) & 0x7),
	)
}

// Segment returns the 16-bit segment number.
func (bdf PciBdf) Segment() uint16 {
	return uint16((bdf >> 16) & 0xffff)
}

// Bus returns the 8-bit bus number.
func (bdf PciBdf) Bus() uint8 {
	return uint8((bdf >> 8) & 0xff)
}

// Device returns the 5-bit device/slot number.
func (bdf PciBdf) Device() uint8 {
	return uint8((bdf >> 3) & 0x1f)
}

// Function returns the 3-bit function number.
func (bdf PciBdf) Function() uint8 {
	return uint8(bdf & 0x7)
}

// Uint32 converts the PciBdf to its raw uint32 representation.
func (bdf PciBdf) Uint32() uint32 {
	return uint32(bdf)
}

// Uint16 converts the low 16-bit bus/dev/fn section of PciBdf to a uint16.
func (bdf PciBdf) Uint16() uint16 {
	return uint16(bdf & 0xffff)
}

// String implements the fmt.Stringer interface for standard hex representation: "ssss:bb:dd.f".
func (bdf PciBdf) String() string {
	return fmt.Sprintf("%04x:%02x:%02x.%01x", bdf.Segment(), bdf.Bus(), bdf.Device(), bdf.Function())
}

// ParsePciBdf parses a standard "ssss:bb:dd.f" string representation into a PciBdf instance.
func ParsePciBdf(s string) (PciBdf, error) {
	items := strings.Split(s, ".")
	if len(items) != 2 {
		return 0, fmt.Errorf("invalid PCI BDF format: missing function delimiter '.'")
	}

	functionVal, err := strconv.ParseUint(items[1], 16, 8)
	if err != nil {
		return 0, fmt.Errorf("failed parsing function: %w", err)
	}

	addrItems := strings.Split(items[0], ":")
	if len(addrItems) != 3 {
		return 0, fmt.Errorf("invalid PCI BDF address parts: expected segment:bus:device")
	}

	segmentVal, err := strconv.ParseUint(addrItems[0], 16, 16)
	if err != nil {
		return 0, fmt.Errorf("failed parsing segment: %w", err)
	}

	busVal, err := strconv.ParseUint(addrItems[1], 16, 8)
	if err != nil {
		return 0, fmt.Errorf("failed parsing bus: %w", err)
	}

	deviceVal, err := strconv.ParseUint(addrItems[2], 16, 8)
	if err != nil {
		return 0, fmt.Errorf("failed parsing device: %w", err)
	}

	return NewPciBdf(uint16(segmentVal), uint8(busVal), uint8(deviceVal), uint8(functionVal)), nil
}

// --- Serialization Interfaces ---

// MarshalJSON implements json.Marshaler interface to serialize BDF as a formatted string.
func (bdf PciBdf) MarshalJSON() ([]byte, error) {
	return json.Marshal(bdf.String())
}

// UnmarshalJSON implements json.Unmarshaler interface to deserialize BDF from a string.
func (bdf *PciBdf) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParsePciBdf(s)
	if err != nil {
		return err
	}
	*bdf = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler for key-based text conversions.
func (bdf PciBdf) MarshalText() ([]byte, error) {
	return []byte(bdf.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for key-based text conversions.
func (bdf *PciBdf) UnmarshalText(text []byte) error {
	parsed, err := ParsePciBdf(string(text))
	if err != nil {
		return err
	}
	*bdf = parsed
	return nil
}
