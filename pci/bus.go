// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE-BSD-3-Clause file.
//
// SPDX-License-Identifier: Apache-2.0 AND BSD-3-Clause

package pci

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"reflect"
	"sync"
)

const (
	VendorIDIntel             uint16 = 0x8086
	DeviceIDIntelVirtPcieHost uint16 = 0x0d57
	NumDeviceIDs              int    = 32
)

// --- Errors for device manager ---

// PciRootError defines the base interface for PCI root specific errors.
type PciRootError interface {
	error
	isPciRootError()
}

// AllocateDeviceAddrsError represents: Could not allocate device address space for the device.
type AllocateDeviceAddrsError struct {
	Source error
}

func (e *AllocateDeviceAddrsError) Error() string {
	return fmt.Sprintf("Could not allocate device address space for the device: %v", e.Source)
}
func (e *AllocateDeviceAddrsError) Unwrap() error   { return e.Source }
func (e *AllocateDeviceAddrsError) isPciRootError() {}

// AllocateIrqError represents: Could not allocate an IRQ number.
type AllocateIrqError struct{}

func (e *AllocateIrqError) Error() string {
	return "Could not allocate an IRQ number"
}
func (e *AllocateIrqError) isPciRootError() {}

// PioInsertError represents: Could not add a device to the port io bus.
type PioInsertError struct {
	Source error
}

func (e *PioInsertError) Error() string {
	return fmt.Sprintf("Could not add a device to the port io bus: %v", e.Source)
}
func (e *PioInsertError) Unwrap() error   { return e.Source }
func (e *PioInsertError) isPciRootError() {}

// MmioInsertError represents: Could not add a device to the mmio bus.
type MmioInsertError struct {
	Source error
}

func (e *MmioInsertError) Error() string {
	return fmt.Sprintf("Could not add a device to the mmio bus: %v", e.Source)
}
func (e *MmioInsertError) Unwrap() error   { return e.Source }
func (e *MmioInsertError) isPciRootError() {}

// NoPciDeviceSlotAvailableError represents: Could not find an available device slot on the PCI bus.
type NoPciDeviceSlotAvailableError struct{}

func (e *NoPciDeviceSlotAvailableError) Error() string {
	return "Could not find an available device slot on the PCI bus"
}
func (e *NoPciDeviceSlotAvailableError) isPciRootError() {}

// InvalidPciDeviceSlotError represents: Invalid PCI device identifier provided.
type InvalidPciDeviceSlotError struct {
	Slot int
}

func (e *InvalidPciDeviceSlotError) Error() string {
	return fmt.Sprintf("Invalid PCI device identifier provided: %d", e.Slot)
}
func (e *InvalidPciDeviceSlotError) isPciRootError() {}

// AlreadyInUsePciDeviceSlotError represents: Valid PCI device identifier but already used.
type AlreadyInUsePciDeviceSlotError struct {
	Slot int
}

func (e *AlreadyInUsePciDeviceSlotError) Error() string {
	return fmt.Sprintf("Valid PCI device identifier but already used: %d", e.Slot)
}
func (e *AlreadyInUsePciDeviceSlotError) isPciRootError() {}

// --- PciRoot Emulation ---

// PciRoot emulates the PCI Root bridge device.
type PciRoot struct {
	mu     sync.Mutex
	config *PciConfiguration
}

// NewPciRoot creates an empty PCI root bridge.
func NewPciRoot(config *PciConfiguration) *PciRoot {
	if config != nil {
		return &PciRoot{
			config: config,
		}
	}

	return &PciRoot{
		config: NewPciConfiguration(
			VendorIDIntel,
			DeviceIDIntelVirtPcieHost,
			0,
			PciClassCodeBridgeDevice,
			PciBridgeSubclassHostBridge,
			nil,
			PciHeaderTypeDevice,
			0,
			0,
			nil,
			nil,
		),
	}
}

// --- BusDevice & PciDevice Interface Implementations ---

// WriteConfigRegister updates the configuration register space.
// It returns bar reprogramming parameters and an optional synchronization channel (simulating Option<Arc<Barrier>>).
func (r *PciRoot) WriteConfigRegister(regIdx int, offset uint64, data []byte) ([]BarReprogrammingParams, chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.config.WriteConfigRegister(regIdx, offset, data), nil
}

// ReadConfigRegister reads a 32-bit value from the configuration space register.
func (r *PciRoot) ReadConfigRegister(regIdx int) uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.config.ReadReg(regIdx)
}

// AsAnyMut returns the underlying instance as an 'any' interface for dynamic type casting.
func (r *PciRoot) AsAnyMut() any {
	return r
}

// ID returns the unique identifier string of the device, if available.
func (r *PciRoot) ID() (string, bool) {
	return "", false
}

func (r *PciRoot) AllocateBars(
	allocator SystemAllocator, // Assumes concurrent safety or internal syncing
	mmio32Allocator AddressAllocator,
	mmio64Allocator AddressAllocator,
	resources []Resource,
) ([]PciBarConfiguration, error) {
	return nil, nil
}

func (r *PciRoot) FreeBars(
	allocator SystemAllocator,
	mmio32Allocator AddressAllocator,
	mmio64Allocator AddressAllocator,
) error {
	return nil
}

func (r *PciRoot) MoveBar(oldBase uint64, newBase uint64) error {
	return nil
}

func (r *PciRoot) ReadBar(base uint64, offset uint64, data []byte) {}

func (r *PciRoot) WriteBar(base uint64, offset uint64, data []byte) chan struct{} {
	c := make(chan struct{})
	return c
}

// Bus represents an abstract simulation of a system bus (simulating vm_device::Bus).
type Bus interface {
	Insert(dev any, addr uint64, size uint64) error
}

// PciBus manages all devices attached to a single PCI root domain.
type PciBus struct {
	mu sync.Mutex

	// Devices attached to this bus. Device 0 is host bridge.
	devices map[uint32]PciDevice

	// Relocation interface for dynamic BAR reassignment.
	deviceReloc DeviceRelocation

	// Bitmap tracking allocated device identifiers (slots).
	deviceIDs []bool
}

// NewPciBus creates a new PCI bus instance and initializes it with the host bridge at slot 0.
func NewPciBus(pciRoot *PciRoot, deviceReloc DeviceRelocation) *PciBus {
	devices := make(map[uint32]PciDevice)
	deviceIDs := make([]bool, NumDeviceIDs)

	// Device 0 is the host bridge (PciRoot)
	devices[0] = pciRoot
	deviceIDs[0] = true

	return &PciBus{
		devices:     devices,
		deviceReloc: deviceReloc,
		deviceIDs:   deviceIDs,
	}
}

// RegisterMapping registers the BAR address spaces of a device into the matching I/O or MMIO system buses.
func (b *PciBus) RegisterMapping(
	dev any, // Simulates Arc<dyn BusDeviceSync>
	ioBus Bus,
	mmioBus Bus,
	bars []PciBarConfiguration,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, bar := range bars {
		switch bar.GetRegionType() {
		case PciBarRegionTypeIoRegion:
			if err := ioBus.Insert(dev, bar.GetAddr(), bar.GetSize()); err != nil {
				return &PioInsertError{Source: err}
			}
		case PciBarRegionTypeMemory32BitRegion, PciBarRegionTypeMemory64BitRegion:
			if err := mmioBus.Insert(dev, bar.GetAddr(), bar.GetSize()); err != nil {
				return &MmioInsertError{Source: err}
			}
		}
	}
	return nil
}

// AddDevice inserts a PCI device into the specified device slot ID.
func (b *PciBus) AddDevice(deviceID uint32, device PciDevice) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.devices[deviceID] = device
	return nil
}

// RemoveByDevice removes devices matching the provided instance pointer from the bus.
// It uses reflection to match the underlying memory pointer, mimicking Arc::ptr_eq.
func (b *PciBus) RemoveByDevice(device PciDevice) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	targetVal := reflect.ValueOf(device)
	if targetVal.Kind() != reflect.Ptr {
		return nil
	}
	targetPtr := targetVal.Pointer()

	// Mimics self.devices.retain(...)
	for id, dev := range b.devices {
		val := reflect.ValueOf(dev)
		if val.Kind() == reflect.Ptr && val.Pointer() == targetPtr {
			delete(b.devices, id)
		}
	}
	return nil
}

// NextDeviceID scans the allocation bitmap and claims the next available device slot ID.
func (b *PciBus) NextDeviceID() (uint32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for idx, allocated := range b.deviceIDs {
		if !allocated {
			b.deviceIDs[idx] = true
			return uint32(idx), nil
		}
	}

	return 0, &NoPciDeviceSlotAvailableError{}
}

// GetDeviceID explicitly allocates and marks a chosen device slot ID as active.
func (b *PciBus) GetDeviceID(id int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if id < 0 || id >= NumDeviceIDs {
		return &InvalidPciDeviceSlotError{Slot: id}
	}

	if b.deviceIDs[id] {
		return &AlreadyInUsePciDeviceSlotError{Slot: id}
	}

	b.deviceIDs[id] = true
	return nil
}

// PutDeviceID releases an allocated device slot ID back into the free pool.
func (b *PciBus) PutDeviceID(id int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if id < 0 || id >= NumDeviceIDs {
		return &InvalidPciDeviceSlotError{Slot: id}
	}

	b.deviceIDs[id] = false
	return nil
}

// PciConfigIo emulates access to the PCI configuration space via I/O ports (0xCF8 and 0xCFC).
type PciConfigIo struct {
	mu            sync.Mutex
	configAddress uint32
	pciBus        *PciBus
}

// NewPciConfigIo creates a new PciConfigIo instance.
func NewPciConfigIo(pciBus *PciBus) *PciConfigIo {
	return &PciConfigIo{
		configAddress: 0,
		pciBus:        pciBus,
	}
}

// ConfigSpaceRead reads a 32-bit value from the selected device's configuration space.
func (io *PciConfigIo) ConfigSpaceRead() uint32 {
	io.mu.Lock()
	addr := io.configAddress
	io.mu.Unlock()

	// Bit 31 is the Enable flag
	enabled := (addr & 0x80000000) != 0
	if !enabled {
		return 0xffffffff
	}

	bus, device, function, register := parseIoConfigAddress(addr &^ 0x80000000)

	// Only support a single root bus (Bus 0).
	if bus != 0 {
		return 0xffffffff
	}

	// Multi-function devices are not supported.
	if function > 0 {
		return 0xffffffff
	}

	io.pciBus.mu.Lock()
	dev, exists := io.pciBus.devices[uint32(device)]
	io.pciBus.mu.Unlock()

	if !exists {
		return 0xffffffff
	}

	return dev.ReadConfigRegister(register)
}

// ConfigSpaceWrite writes to the selected device's configuration space and triggers BAR relocation if required.
func (io *PciConfigIo) ConfigSpaceWrite(offset uint64, data []byte) chan struct{} {
	if offset+uint64(len(data)) > 4 {
		return nil
	}

	io.mu.Lock()
	addr := io.configAddress
	io.mu.Unlock()

	enabled := (addr & 0x80000000) != 0
	if !enabled {
		return nil
	}

	bus, device, _, register := parseIoConfigAddress(addr &^ 0x80000000)
	if bus != 0 {
		return nil
	}

	io.pciBus.mu.Lock()
	defer io.pciBus.mu.Unlock()

	dev, exists := io.pciBus.devices[uint32(device)]
	if !exists {
		return nil
	}

	// Write to the configuration register and retrieve possible BAR updates
	barReprogram, barrierChan := dev.WriteConfigRegister(register, offset, data)

	// Handle BAR relocation/reprogramming dynamically if needed
	for _, params := range barReprogram {
		if err := io.pciBus.deviceReloc.MoveBar(
			params.OldBase,
			params.NewBase,
			params.Len,
			dev,
			params.RegionType,
		); err != nil {
			log.Printf("Failed moving device BAR: %v: 0x%x->0x%x(0x%x)",
				err, params.OldBase, params.NewBase, params.Len)
		}
	}

	return barrierChan
}

// setConfigAddress updates the tracked configuration address register using partial byte masking.
func (io *PciConfigIo) setConfigAddress(offset uint64, data []byte) {
	if offset+uint64(len(data)) > 4 {
		return
	}

	var mask, value uint32

	switch len(data) {
	case 1:
		mask = 0x000000ff << (offset * 8)
		value = uint32(data[0]) << (offset * 8)
	case 2:
		mask = 0x0000ffff << (offset * 16)
		value = ((uint32(data[1]) << 8) | uint32(data[0])) << (offset * 16)
	case 4:
		mask = 0xffffffff
		value = binary.LittleEndian.Uint32(data)
	default:
		return
	}

	io.mu.Lock()
	defer io.mu.Unlock()
	io.configAddress = (io.configAddress &^ mask) | value
}

// --- BusDevice Interface Implementation ---

// Read traps hardware port reads relative to the 0xCF8 base port.
// Offsets 0..=3 correspond to 0xCF8 (CONFIG_ADDRESS), offsets 4..=7 correspond to 0xCFC (CONFIG_DATA).
func (io *PciConfigIo) Read(base uint64, offset uint64, data []byte) {
	var value uint32

	switch offset {
	case 0, 1, 2, 3:
		io.mu.Lock()
		value = io.configAddress
		io.mu.Unlock()
	case 4, 5, 6, 7:
		value = io.ConfigSpaceRead()
	default:
		value = 0xffffffff
	}

	// Handle size-specific serialization (partial or unaligned register chunk reads)
	start := int(offset % 4)
	end := start + len(data)

	if end <= 4 {
		for i := start; i < end; i++ {
			data[i-start] = byte(value >> (i * 8))
		}
	} else {
		for i := range data {
			data[i] = 0xff
		}
	}
}

// Write traps hardware port writes relative to the 0xCF8 base port.
func (io *PciConfigIo) Write(base uint64, offset uint64, data []byte) chan struct{} {
	switch offset {
	case 0, 1, 2, 3:
		io.setConfigAddress(offset, data)
		return nil
	case 4, 5, 6, 7:
		return io.ConfigSpaceWrite(offset-4, data)
	default:
		return nil
	}
}

// PciConfigMmio emulates the PCI memory-mapped configuration access mechanism (ECAM).
type PciConfigMmio struct {
	mu     sync.Mutex
	pciBus *PciBus
}

// NewPciConfigMmio creates a new PciConfigMmio instance.
func NewPciConfigMmio(pciBus *PciBus) *PciConfigMmio {
	return &PciConfigMmio{
		pciBus: pciBus,
	}
}

// configSpaceRead handles reading from the memory-mapped PCI configuration space.
func (mmio *PciConfigMmio) configSpaceRead(configAddress uint32) uint32 {
	bus, device, _, register := parseMmioConfigAddress(configAddress)

	// Only support a single root bus (Bus 0).
	if bus != 0 {
		return 0xffffffff
	}

	mmio.pciBus.mu.Lock()
	dev, exists := mmio.pciBus.devices[uint32(device)]
	mmio.pciBus.mu.Unlock()

	if !exists {
		return 0xffffffff
	}

	return dev.ReadConfigRegister(register)
}

// configSpaceWrite handles writing to the memory-mapped PCI configuration space and relocates BARs if needed.
func (mmio *PciConfigMmio) configSpaceWrite(configAddress uint32, offset uint64, data []byte) {
	if offset+uint64(len(data)) > 4 {
		return
	}

	bus, device, _, register := parseMmioConfigAddress(configAddress)

	// Only support a single root bus (Bus 0).
	if bus != 0 {
		return
	}

	mmio.pciBus.mu.Lock()
	defer mmio.pciBus.mu.Unlock()

	dev, exists := mmio.pciBus.devices[uint32(device)]
	if !exists {
		return
	}

	// Write to the configuration register and get BAR reprogramming updates
	barReprogram, _ := dev.WriteConfigRegister(register, offset, data)

	// Handle dynamic BAR reassignment/relocation
	for _, params := range barReprogram {
		if err := mmio.pciBus.deviceReloc.MoveBar(
			params.OldBase,
			params.NewBase,
			params.Len,
			dev,
			params.RegionType,
		); err != nil {
			log.Printf("Failed moving device BAR: %v: 0x%x->0x%x(0x%x)",
				err, params.OldBase, params.NewBase, params.Len)
		}
	}
}

// --- BusDevice Interface Implementation ---

// Read traps memory-mapped configuration space reads.
func (mmio *PciConfigMmio) Read(base uint64, offset uint64, data []byte) {
	start := int(offset % 4)
	end := start + len(data)

	// Only allow reads within the 32-bit register boundary and valid 32-bit offset limits.
	if end > 4 || offset > math.MaxUint32 {
		for i := range data {
			data[i] = 0xff
		}
		return
	}

	value := mmio.configSpaceRead(uint32(offset))
	for i := start; i < end; i++ {
		data[i-start] = byte(value >> (i * 8))
	}
}

// Write traps memory-mapped configuration space writes.
func (mmio *PciConfigMmio) Write(base uint64, offset uint64, data []byte) chan struct{} {
	if offset > math.MaxUint32 {
		return nil
	}

	mmio.configSpaceWrite(uint32(offset), offset%4, data)
	return nil
}

func shiftAndMask(value uint32, offset uint, mask uint32) int {
	return int((value >> offset) & mask)
}

// parseMmioConfigAddress parses the MMIO address offset to a (bus, device, function, register) tuple.
// See section 7.2.2 PCI Express Enhanced Configuration Access Mechanism (ECAM)
// from the Pci Express Base Specification Revision 5.0 Version 1.0.
func parseMmioConfigAddress(configAddress uint32) (bus, device, function, register int) {
	const (
		busNumberOffset      = 20
		busNumberMask        = 0x00ff
		deviceNumberOffset   = 15
		deviceNumberMask     = 0x1f
		functionNumberOffset = 12
		functionNumberMask   = 0x07
		registerNumberOffset = 2
		registerNumberMask   = 0x3ff
	)

	bus = shiftAndMask(configAddress, busNumberOffset, busNumberMask)
	device = shiftAndMask(configAddress, deviceNumberOffset, deviceNumberMask)
	function = shiftAndMask(configAddress, functionNumberOffset, functionNumberMask)
	register = shiftAndMask(configAddress, registerNumberOffset, registerNumberMask)
	return
}

// parseIoConfigAddress parses the CONFIG_ADDRESS register to a (bus, device, function, register) tuple.
func parseIoConfigAddress(configAddress uint32) (bus, device, function, register int) {
	const (
		busNumberOffset      = 16
		busNumberMask        = 0x00ff
		deviceNumberOffset   = 11
		deviceNumberMask     = 0x1f
		functionNumberOffset = 8
		functionNumberMask   = 0x07
		registerNumberOffset = 2
		registerNumberMask   = 0x3f
	)

	bus = shiftAndMask(configAddress, busNumberOffset, busNumberMask)
	device = shiftAndMask(configAddress, deviceNumberOffset, deviceNumberMask)
	function = shiftAndMask(configAddress, functionNumberOffset, functionNumberMask)
	register = shiftAndMask(configAddress, registerNumberOffset, registerNumberMask)
	return
}
