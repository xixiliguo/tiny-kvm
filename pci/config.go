package pci

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync"
)

// The number of 32bit registers in the config space, 4096 bytes.
const NumConfigurationRegisters = 1024

const (
	CommandReg                    = 1
	CommandRegMemorySpaceMask     = 0x00000002
	StatusReg                     = 1
	StatusRegCapabilitiesUsedMask = 0x00100000
	Bar0Reg                       = 4
	RomBarReg                     = 12
	RomBarIdx                     = 6
	BarIoAddrMask                 = 0xfffffffc
	BarMemAddrMask                = 0xfffffff0
	RomBarAddrMask                = 0xfffff800
	MsiCapabilityRegisterMask     = 0x00710000
	MsixCapabilityRegisterMask    = 0xc0000000
	NumBarRegs                    = 6
	CapabilityListHeadOffset      = 0x34
	FirstCapabilityOffset         = 0x40
	CapabilityMaxOffset           = 192

	InterruptLinePinReg = 15
)

const PciConfigurationID = "pci_configuration"

// PciHeaderType represents the types of PCI headers allowed in the configuration registers.
type PciHeaderType int

const (
	PciHeaderTypeDevice PciHeaderType = iota
	PciHeaderTypeBridge
)

// PciClassCode represents classes of PCI nodes.
type PciClassCode uint8

const (
	PciClassCodeTooOld                          PciClassCode = 0x00
	PciClassCodeMassStorage                     PciClassCode = 0x01
	PciClassCodeNetworkController               PciClassCode = 0x02
	PciClassCodeDisplayController               PciClassCode = 0x03
	PciClassCodeMultimediaController            PciClassCode = 0x04
	PciClassCodeMemoryController                PciClassCode = 0x05
	PciClassCodeBridgeDevice                    PciClassCode = 0x06
	PciClassCodeSimpleCommunicationController   PciClassCode = 0x07
	PciClassCodeBaseSystemPeripheral            PciClassCode = 0x08
	PciClassCodeInputDevice                     PciClassCode = 0x09
	PciClassCodeDockingStation                  PciClassCode = 0x0a
	PciClassCodeProcessor                       PciClassCode = 0x0b
	PciClassCodeSerialBusController             PciClassCode = 0x0c
	PciClassCodeWirelessController              PciClassCode = 0x0d
	PciClassCodeIntelligentIoController         PciClassCode = 0x0e
	PciClassCodeEncryptionController            PciClassCode = 0x0f
	PciClassCodeDataAcquisitionSignalProcessing PciClassCode = 0x10
	PciClassCodeOther                           PciClassCode = 0xff
)

func (c PciClassCode) GetRegisterValue() uint8 {
	return uint8(c)
}

// PciSubclass defines a PCI subclass interface. Each class in PciClassCode can specify a unique set of subclasses.
type PciSubclass interface {
	GetRegisterValue() uint8
}

// PciMultimediaSubclass represents subclasses of the MultimediaController class.
type PciMultimediaSubclass uint8

const (
	PciMultimediaSubclassVideoController PciMultimediaSubclass = 0x00
	PciMultimediaSubclassAudioController PciMultimediaSubclass = 0x01
	PciMultimediaSubclassTelephonyDevice PciMultimediaSubclass = 0x02
	PciMultimediaSubclassAudioDevice     PciMultimediaSubclass = 0x03
	PciMultimediaSubclassOther           PciMultimediaSubclass = 0x80
)

func (s PciMultimediaSubclass) GetRegisterValue() uint8 {
	return uint8(s)
}

// PciBridgeSubclass represents subclasses of the BridgeDevice.
type PciBridgeSubclass uint8

const (
	PciBridgeSubclassHostBridge                    PciBridgeSubclass = 0x00
	PciBridgeSubclassIsaBridge                     PciBridgeSubclass = 0x01
	PciBridgeSubclassEisaBridge                    PciBridgeSubclass = 0x02
	PciBridgeSubclassMcaBridge                     PciBridgeSubclass = 0x03
	PciBridgeSubclassPciToPciBridge                PciBridgeSubclass = 0x04
	PciBridgeSubclassPcmciaBridge                  PciBridgeSubclass = 0x05
	PciBridgeSubclassNuBusBridge                   PciBridgeSubclass = 0x06
	PciBridgeSubclassCardBusBridge                 PciBridgeSubclass = 0x07
	PciBridgeSubclassRacEwayBridge                 PciBridgeSubclass = 0x08
	PciBridgeSubclassPciToPciSemiTransparentBridge PciBridgeSubclass = 0x09
	PciBridgeSubclassInfiniBrandToPciHostBridge    PciBridgeSubclass = 0x0a
	PciBridgeSubclassOtherBridgeDevice             PciBridgeSubclass = 0x80
)

func (s PciBridgeSubclass) GetRegisterValue() uint8 {
	return uint8(s)
}

// PciSerialBusSubClass represents subclass of the SerialBus.
type PciSerialBusSubClass uint8

const (
	PciSerialBusSubClassFirewire  PciSerialBusSubClass = 0x00
	PciSerialBusSubClassAccessbus PciSerialBusSubClass = 0x01
	PciSerialBusSubClassSsa       PciSerialBusSubClass = 0x02
	PciSerialBusSubClassUsb       PciSerialBusSubClass = 0x03
)

func (s PciSerialBusSubClass) GetRegisterValue() uint8 {
	return uint8(s)
}

// PciMassStorageSubclass represents Mass Storage Sub Classes.
type PciMassStorageSubclass uint8

const (
	PciMassStorageSubclassScsiStorage          PciMassStorageSubclass = 0x00
	PciMassStorageSubclassIdeInterface         PciMassStorageSubclass = 0x01
	PciMassStorageSubclassFloppyController     PciMassStorageSubclass = 0x02
	PciMassStorageSubclassIpiController        PciMassStorageSubclass = 0x03
	PciMassStorageSubclassRaidController       PciMassStorageSubclass = 0x04
	PciMassStorageSubclassAtaController        PciMassStorageSubclass = 0x05
	PciMassStorageSubclassSataController       PciMassStorageSubclass = 0x06
	PciMassStorageSubclassSerialScsiController PciMassStorageSubclass = 0x07
	PciMassStorageSubclassNvmController        PciMassStorageSubclass = 0x08
	PciMassStorageSubclassMassStorage          PciMassStorageSubclass = 0x80
)

func (s PciMassStorageSubclass) GetRegisterValue() uint8 {
	return uint8(s)
}

// PciNetworkControllerSubclass represents Network Controller Sub Classes.
type PciNetworkControllerSubclass uint8

const (
	PciNetworkControllerSubclassEthernetController   PciNetworkControllerSubclass = 0x00
	PciNetworkControllerSubclassTokenRingController  PciNetworkControllerSubclass = 0x01
	PciNetworkControllerSubclassFddiController       PciNetworkControllerSubclass = 0x02
	PciNetworkControllerSubclassAtmController        PciNetworkControllerSubclass = 0x03
	PciNetworkControllerSubclassIsdnController       PciNetworkControllerSubclass = 0x04
	PciNetworkControllerSubclassWorldFipController   PciNetworkControllerSubclass = 0x05
	PciNetworkControllerSubclassPicmgController      PciNetworkControllerSubclass = 0x06
	PciNetworkControllerSubclassInfinibandController PciNetworkControllerSubclass = 0x07
	PciNetworkControllerSubclassFabricController     PciNetworkControllerSubclass = 0x08
	PciNetworkControllerSubclassNetworkController    PciNetworkControllerSubclass = 0x80
)

func (s PciNetworkControllerSubclass) GetRegisterValue() uint8 {
	return uint8(s)
}

// PciProgrammingInterface defines a PCI class programming interface.
type PciProgrammingInterface interface {
	GetRegisterValue() uint8
}

// PciCapabilityID represents types of PCI capabilities.
type PciCapabilityID uint8

const (
	PciCapabilityIDListID                           PciCapabilityID = 0
	PciCapabilityIDPowerManagement                  PciCapabilityID = 0x01
	PciCapabilityIDAcceleratedGraphicsPort          PciCapabilityID = 0x02
	PciCapabilityIDVitalProductData                 PciCapabilityID = 0x03
	PciCapabilityIDSlotIdentification               PciCapabilityID = 0x04
	PciCapabilityIDMessageSignalledInterrupts       PciCapabilityID = 0x05
	PciCapabilityIDCompactPciHotSwap                PciCapabilityID = 0x06
	PciCapabilityIDPciX                             PciCapabilityID = 0x07
	PciCapabilityIDHyperTransport                   PciCapabilityID = 0x08
	PciCapabilityIDVendorSpecific                   PciCapabilityID = 0x09
	PciCapabilityIDDebugport                        PciCapabilityID = 0x0A
	PciCapabilityIDCompactPciCentralResourceControl PciCapabilityID = 0x0B
	PciCapabilityIDPciStandardHotPlugController     PciCapabilityID = 0x0C
	PciCapabilityIDBridgeSubsystemVendorDeviceIDID  PciCapabilityID = 0x0D
	PciCapabilityIDAgpTargetPciPcibridge            PciCapabilityID = 0x0E
	PciCapabilityIDSecureDevice                     PciCapabilityID = 0x0F
	PciCapabilityIDPciExpress                       PciCapabilityID = 0x10
	PciCapabilityIDMsiX                             PciCapabilityID = 0x11
	PciCapabilityIDSataDataIndexConf                PciCapabilityID = 0x12
	PciCapabilityIDPciAdvancedFeatures              PciCapabilityID = 0x13
	PciCapabilityIDPciEnhancedAllocation            PciCapabilityID = 0x14
)

func NewPciCapabilityID(c uint8) PciCapabilityID {
	if c <= 0x14 {
		return PciCapabilityID(c)
	}
	return PciCapabilityIDListID
}

// PciExpressCapabilityID represents types of PCI Express capabilities.
type PciExpressCapabilityID uint16

const (
	PcieCapNullCapability                                 PciExpressCapabilityID = 0x0000
	PcieCapAdvancedErrorReporting                         PciExpressCapabilityID = 0x0001
	PcieCapVirtualChannelMultiFunctionVCNotPresent        PciExpressCapabilityID = 0x0002
	PcieCapDeviceSerialNumber                             PciExpressCapabilityID = 0x0003
	PcieCapPowerBudgeting                                 PciExpressCapabilityID = 0x0004
	PcieCapRootComplexLinkDeclaration                     PciExpressCapabilityID = 0x0005
	PcieCapRootComplexInternalLinkControl                 PciExpressCapabilityID = 0x0006
	PcieCapRootComplexEventCollectorEndpointAssociation   PciExpressCapabilityID = 0x0007
	PcieCapMultiFunctionVirtualChannel                    PciExpressCapabilityID = 0x0008
	PcieCapVirtualChannelMultiFunctionVCPresent           PciExpressCapabilityID = 0x0009
	PcieCapRootComplexRegisterBlock                       PciExpressCapabilityID = 0x000a
	PcieCapVendorSpecificExtendedCapability               PciExpressCapabilityID = 0x000b
	PcieCapConfigurationAccessCorrelation                 PciExpressCapabilityID = 0x000c
	PcieCapAccessControlServices                          PciExpressCapabilityID = 0x000d
	PcieCapAlternativeRoutingIdentificationInterpretation PciExpressCapabilityID = 0x000e
	PcieCapAddressTranslationServices                     PciExpressCapabilityID = 0x000f
	PcieCapSingleRootIoVirtualization                     PciExpressCapabilityID = 0x0010
	PcieCapDeprecatedMultiRootIoVirtualization            PciExpressCapabilityID = 0x0011
	PcieCapMulticast                                      PciExpressCapabilityID = 0x0012
	PcieCapPageRequestInterface                           PciExpressCapabilityID = 0x0013
	PcieCapReservedForAmd                                 PciExpressCapabilityID = 0x0014
	PcieCapResizeableBar                                  PciExpressCapabilityID = 0x0015
	PcieCapDynamicPowerAllocation                         PciExpressCapabilityID = 0x0016
	PcieCapThpRequester                                   PciExpressCapabilityID = 0x0017
	PcieCapLatencyToleranceReporting                      PciExpressCapabilityID = 0x0018
	PcieCapSecondaryPciExpress                            PciExpressCapabilityID = 0x0019
	PcieCapProtocolMultiplexing                           PciExpressCapabilityID = 0x001a
	PcieCapProcessAddressSpaceId                          PciExpressCapabilityID = 0x001b
	PcieCapLnRequester                                    PciExpressCapabilityID = 0x001c
	PcieCapDownstreamPortContainment                      PciExpressCapabilityID = 0x001d
	PcieCapL1PmSubstates                                  PciExpressCapabilityID = 0x001e
	PcieCapPrecisionTimeMeasurement                       PciExpressCapabilityID = 0x001f
	PcieCapPciExpressOverMphy                             PciExpressCapabilityID = 0x0020
	PcieCapFRSQueueing                                    PciExpressCapabilityID = 0x0021
	PcieCapReadinessTimeReporting                         PciExpressCapabilityID = 0x0022
	PcieCapDesignatedVendorSpecificExtendedCapability     PciExpressCapabilityID = 0x0023
	PcieCapVfResizeableBar                                PciExpressCapabilityID = 0x0024
	PcieCapDataLinkFeature                                PciExpressCapabilityID = 0x0025
	PcieCapPhysicalLayerSixteenGts                        PciExpressCapabilityID = 0x0026
	PcieCapLaneMarginingAtTheReceiver                     PciExpressCapabilityID = 0x0027
	PcieCapHierarchyId                                    PciExpressCapabilityID = 0x0028
	PcieCapNativePcieEnclosureManagement                  PciExpressCapabilityID = 0x0029
	PcieCapPhysicalLayerThirtyTwoGts                      PciExpressCapabilityID = 0x002a
	PcieCapAlternateProtocol                              PciExpressCapabilityID = 0x002b
	PcieCapSystemFirmwareIntermediary                     PciExpressCapabilityID = 0x002c
	PcieCapShadowFunctions                                PciExpressCapabilityID = 0x002d
	PcieCapDataObjectExchange                             PciExpressCapabilityID = 0x002e
	PcieCapReserved                                       PciExpressCapabilityID = 0x002f
	PcieCapExtendedCapabilitiesAbsence                    PciExpressCapabilityID = 0xffff
)

func NewPciExpressCapabilityID(c uint16) PciExpressCapabilityID {
	if c == 0xffff {
		return PcieCapExtendedCapabilitiesAbsence
	}
	if c <= 0x002e {
		return PciExpressCapabilityID(c)
	}
	return PcieCapReserved
}

// PciCapability defines a PCI capability list interface.
type PciCapability interface {
	Bytes() []byte
	ID() PciCapabilityID
}

func encode32BitsBarSize(barSize uint32) (uint32, bool) {
	if barSize > 0 {
		return ^(barSize - 1), true
	}
	return 0, false
}

func decode32BitsBarSize(barSize uint32) (uint32, bool) {
	if barSize > 0 {
		return ^barSize + 1, true
	}
	return 0, false
}

func encode64BitsBarSize(barSize uint64) (uint32, uint32, bool) {
	if barSize > 0 {
		result := ^(barSize - 1)
		resultHi := uint32(result >> 32)
		resultLo := uint32(result & 0xffffffff)
		return resultHi, resultLo, true
	}
	return 0, 0, false
}

func decode64BitsBarSize(barSizeHi, barSizeLo uint32) (uint64, bool) {
	barSize := (uint64(barSizeHi) << 32) | uint64(barSizeLo)
	if barSize > 0 {
		return ^barSize + 1, bool(true)
	}
	return 0, false
}

type PciBarRegionType int

const (
	PciBarRegionTypeMemory32BitRegion PciBarRegionType = 0
	PciBarRegionTypeIoRegion          PciBarRegionType = 0x01
	PciBarRegionTypeMemory64BitRegion PciBarRegionType = 0x04
)

type PciBar struct {
	Addr uint32            `json:"addr"`
	Size uint32            `json:"size"`
	Used bool              `json:"used"`
	Type *PciBarRegionType `json:"type"`
}

type PciCapabilityBounds struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
}

type PciConfigurationState struct {
	Registers      []uint32             `json:"registers"`
	WritableBits   []uint32             `json:"writable_bits"`
	Bars           []PciBar             `json:"bars"`
	RomBarAddr     uint32               `json:"rom_bar_addr"`
	RomBarSize     uint32               `json:"rom_bar_size"`
	RomBarUsed     bool                 `json:"rom_bar_used"`
	LastCapability *PciCapabilityBounds `json:"last_capability"`
	MsixCapRegIdx  *int                 `json:"msix_cap_reg_idx"`
}

// PciConfiguration contains the configuration space of a PCI node.
// The configuration space is accessed with DWORD reads and writes from the guest.
type PciConfiguration struct {
	mu                  sync.Mutex
	registers           [NumConfigurationRegisters]uint32
	writableBits        [NumConfigurationRegisters]uint32
	bars                [NumBarRegs]PciBar
	romBarAddr          uint32
	romBarSize          uint32
	romBarUsed          bool
	lastCapability      *PciCapabilityBounds
	msixCapRegIdx       *int
	msixConfig          *MsixConfig // Guarded by external logic or internal mutex if needed
	pendingBarReprogram []BarReprogrammingParams
}

// Errors matching Rust's `thiserror` mapping definitions
var (
	ErrCapabilityEmpty = errors.New("empty capabilities are invalid")
	ErrDecode32BarSize = errors.New("failed to decode 32 bits BAR size")
	ErrDecode64BarSize = errors.New("failed to decode 64 bits BAR size")
	ErrEncode32BarSize = errors.New("failed to encode 32 bits BAR size")
	ErrEncode64BarSize = errors.New("failed to encode 64 bits BAR size")
)

type BarAddressInvalidError struct {
	Addr uint64
	Size uint64
}

func (e *BarAddressInvalidError) Error() string {
	return fmt.Sprintf("address %d size %d too big", e.Addr, e.Size)
}

type BarInUseError struct {
	Idx int
}

func (e *BarInUseError) Error() string {
	return fmt.Sprintf("bar %d already used", e.Idx)
}

type BarInUse64Error struct {
	Idx int
}

func (e *BarInUse64Error) Error() string {
	return fmt.Sprintf("64bit bar %d already used (requires two regs)", e.Idx)
}

type BarInvalidError struct {
	Idx int
}

func (e *BarInvalidError) Error() string {
	return fmt.Sprintf("bar %d invalid, max %d", e.Idx, NumBarRegs-1)
}

type BarInvalid64Error struct {
	Idx int
}

func (e *BarInvalid64Error) Error() string {
	return fmt.Sprintf("64bitbar %d invalid, requires two regs, max %d", e.Idx, NumBarRegs-1)
}

type BarSizeInvalidError struct {
	Size uint64
}

func (e *BarSizeInvalidError) Error() string {
	return fmt.Sprintf("bar address %d not a power of two", e.Size)
}

type CapabilityLengthInvalidError struct {
	Length int
}

func (e *CapabilityLengthInvalidError) Error() string {
	return fmt.Sprintf("Invalid capability length %d", e.Length)
}

type CapabilitySpaceFullError struct {
	Size int
}

func (e *CapabilitySpaceFullError) Error() string {
	return fmt.Sprintf("capability of size %d doesn't fit", e.Size)
}

type RomBarAddressInvalidError struct {
	Addr uint64
	Size uint64
}

func (e *RomBarAddressInvalidError) Error() string {
	return fmt.Sprintf("address %d size %d too big", e.Addr, e.Size)
}

type RomBarInUseError struct {
	Idx int
}

func (e *RomBarInUseError) Error() string {
	return fmt.Sprintf("rom bar %d already used", e.Idx)
}

type RomBarInvalidError struct {
	Idx int
}

func (e *RomBarInvalidError) Error() string {
	return fmt.Sprintf("rom bar %d invalid, max %d", e.Idx, NumBarRegs-1)
}

type RomBarSizeInvalidError struct {
	Size uint64
}

func (e *RomBarSizeInvalidError) Error() string {
	return fmt.Sprintf("rom bar address %d not a power of two", e.Size)
}

// NewPciConfiguration creates a new PCI Configuration or restores it from a saved state.
func NewPciConfiguration(
	vendorID uint16,
	deviceID uint16,
	revisionID uint8,
	classCode PciClassCode,
	subclass PciSubclass,
	programmingInterface PciProgrammingInterface,
	headerType PciHeaderType,
	subsystemVendorID uint16,
	subsystemID uint16,
	msixConfig *MsixConfig,
	state *PciConfigurationState,
) *PciConfiguration {
	pci := &PciConfiguration{
		msixConfig:          msixConfig,
		pendingBarReprogram: make([]BarReprogrammingParams, 0),
	}

	if state != nil {
		copy(pci.registers[:], state.Registers)
		copy(pci.writableBits[:], state.WritableBits)
		copy(pci.bars[:], state.Bars)
		pci.romBarAddr = state.RomBarAddr
		pci.romBarSize = state.RomBarSize
		pci.romBarUsed = state.RomBarUsed
		pci.lastCapability = state.LastCapability
		pci.msixCapRegIdx = state.MsixCapRegIdx
	} else {
		pci.registers = [NumConfigurationRegisters]uint32{}
		pci.writableBits = [NumConfigurationRegisters]uint32{}

		pci.registers[0] = (uint32(deviceID) << 16) | uint32(vendorID)
		// TODO: Status should be write-1-to-clear
		pci.writableBits[1] = 0x0000ffff // Status (r/o), command (r/w)

		var pi uint8 = 0
		if programmingInterface != nil {
			pi = programmingInterface.GetRegisterValue()
		}

		pci.registers[2] = (uint32(classCode.GetRegisterValue()) << 24) |
			(uint32(subclass.GetRegisterValue()) << 16) |
			(uint32(pi) << 8) |
			uint32(revisionID)

		pci.writableBits[3] = 0x000000ff // Cacheline size (r/w)

		switch headerType {
		case PciHeaderTypeDevice:
			pci.registers[3] = 0x00000000     // Header type 0 (device)
			pci.writableBits[15] = 0x000000ff // Interrupt line (r/w)
		case PciHeaderTypeBridge:
			pci.registers[3] = 0x00010000     // Header type 1 (bridge)
			pci.writableBits[9] = 0xfff0fff0  // Memory base and limit
			pci.writableBits[15] = 0xffff00ff // Bridge control (r/w), interrupt line (r/w)
		}

		pci.registers[11] = (uint32(subsystemID) << 16) | uint32(subsystemVendorID)

		for i := 0; i < NumBarRegs; i++ {
			pci.bars[i] = PciBar{}
		}
	}

	return pci
}

// State exports the current state of the PCI Configuration.
func (p *PciConfiguration) State() PciConfigurationState {
	p.mu.Lock()
	defer p.mu.Unlock()

	regs := make([]uint32, NumConfigurationRegisters)
	wBits := make([]uint32, NumConfigurationRegisters)
	bars := make([]PciBar, NumBarRegs)

	copy(regs, p.registers[:])
	copy(wBits, p.writableBits[:])
	copy(bars, p.bars[:])

	return PciConfigurationState{
		Registers:      regs,
		WritableBits:   wBits,
		Bars:           bars,
		RomBarAddr:     p.romBarAddr,
		RomBarSize:     p.romBarSize,
		RomBarUsed:     p.romBarUsed,
		LastCapability: p.lastCapability,
		MsixCapRegIdx:  p.msixCapRegIdx,
	}
}

// ReadReg reads a 32bit register from `regIdx` in the register map.
func (p *PciConfiguration) ReadReg(regIdx int) uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if regIdx < 0 || regIdx >= NumConfigurationRegisters {
		return 0xffffffff
	}
	return p.registers[regIdx]
}

// WriteReg writes a 32bit register to `regIdx` in the register map.
func (p *PciConfiguration) WriteReg(regIdx int, value uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.writeRegInternal(regIdx, value)
}

// writeRegInternal handles internal register writes without taking the lock.
func (p *PciConfiguration) writeRegInternal(regIdx int, value uint32) {
	if regIdx < 0 || regIdx >= NumConfigurationRegisters {
		log.Printf("warn: bad PCI register write %d", regIdx)
		return
	}

	mask := p.writableBits[regIdx]

	if regIdx >= Bar0Reg && regIdx < Bar0Reg+NumBarRegs {
		// Handle very specific case where the BAR is being written with
		// all 1's to retrieve the BAR size during next BAR reading.
		if value == 0xffffffff {
			mask &= p.bars[regIdx-4].Size
		}
	} else if regIdx == RomBarReg {
		// Handle very specific case where the BAR is being written with
		// all 1's on bits 31-11 to retrieve the BAR size during next BAR reading.
		if value&RomBarAddrMask == RomBarAddrMask {
			mask &= p.romBarSize
		}
	}

	p.registers[regIdx] = (p.registers[regIdx] & ^p.writableBits[regIdx]) | (value & mask)
}

// WriteWord writes a 16bit word to `offset`. `offset` must be 16bit aligned.
func (p *PciConfiguration) WriteWord(offset int, value uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var shift uint32
	switch offset % 4 {
	case 0:
		shift = 0
	case 2:
		shift = 16
	default:
		log.Printf("warn: bad PCI config write offset %d", offset)
		return
	}
	regIdx := offset / 4

	if regIdx < 0 || regIdx >= NumConfigurationRegisters {
		log.Printf("warn: bad PCI config write offset %d", offset)
		return
	}

	writableMask := p.writableBits[regIdx]
	mask := (uint32(0xffff) << shift) & writableMask
	shiftedValue := (uint32(value) << shift) & writableMask
	p.registers[regIdx] = (p.registers[regIdx] & ^mask) | shiftedValue
}

// WriteByte writes a byte to `offset`.
func (p *PciConfiguration) WriteByte(offset int, value uint8) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.writeByteInternal(offset, value, true)
}

// writeByteInternal writes a byte to `offset`, optionally enforcing read-only bits.
func (p *PciConfiguration) writeByteInternal(offset int, value uint8, applyWritableMask bool) {
	shift := uint32((offset % 4) * 8)
	regIdx := offset / 4

	if regIdx < 0 || regIdx >= NumConfigurationRegisters {
		log.Printf("warn: bad PCI config write offset %d", offset)
		return
	}

	var writableMask uint32 = 0xffffffff
	if applyWritableMask {
		writableMask = p.writableBits[regIdx]
	}

	mask := (uint32(0xff) << shift) & writableMask
	shiftedValue := (uint32(value) << shift) & writableMask
	p.registers[regIdx] = (p.registers[regIdx] & ^mask) | shiftedValue
}

// AddPciBar adds a region specified by `config`. Configures the specified BAR(s) to
// report this region and size to the guest kernel. Enforces a few constraints.
func (p *PciConfiguration) AddPciBar(config *PciBarConfiguration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	barIdx := config.Idx
	regIdx := Bar0Reg + barIdx

	if barIdx >= NumBarRegs {
		return &BarInvalidError{Idx: barIdx}
	}

	if p.bars[barIdx].Used {
		return &BarInUseError{Idx: barIdx}
	}

	// Inline Go power-of-two check
	if config.Size == 0 || (config.Size&(config.Size-1)) != 0 {
		return &BarSizeInvalidError{Size: config.Size}
	}

	endAddr := config.Addr + (config.Size - 1)
	if endAddr < config.Addr { // Check overflow
		return &BarAddressInvalidError{Addr: config.Addr, Size: config.Size}
	}

	switch config.RegionType {
	case PciBarRegionTypeMemory32BitRegion, PciBarRegionTypeIoRegion:
		if endAddr > 0xffffffff {
			return &BarAddressInvalidError{Addr: config.Addr, Size: config.Size}
		}

		encodedSize, ok := encode32BitsBarSize(uint32(config.Size))
		if !ok {
			return ErrEncode32BarSize
		}
		p.bars[barIdx].Size = encodedSize

	case PciBarRegionTypeMemory64BitRegion:
		if barIdx+1 >= NumBarRegs {
			return &BarInvalid64Error{Idx: barIdx}
		}

		if p.bars[barIdx+1].Used {
			return &BarInUse64Error{Idx: barIdx}
		}

		barSizeHi, barSizeLo, ok := encode64BitsBarSize(config.Size)
		if !ok {
			return ErrEncode64BarSize
		}

		p.registers[regIdx+1] = uint32(config.Addr >> 32)
		p.writableBits[regIdx+1] = 0xffffffff
		p.bars[barIdx+1].Addr = p.registers[regIdx+1]
		p.bars[barIdx].Size = barSizeLo
		p.bars[barIdx+1].Size = barSizeHi
		p.bars[barIdx+1].Used = true
	}

	var mask uint32
	var lowerBits uint32

	switch config.RegionType {
	case PciBarRegionTypeMemory32BitRegion, PciBarRegionTypeMemory64BitRegion:
		mask = BarMemAddrMask
		var pfBits uint32 = 0
		if config.Prefetchable {
			pfBits = 0x08
		}
		lowerBits = pfBits | uint32(config.RegionType)
	case PciBarRegionTypeIoRegion:
		mask = BarIoAddrMask
		lowerBits = uint32(config.RegionType)
	}

	p.registers[regIdx] = (uint32(config.Addr) & mask) | lowerBits
	p.writableBits[regIdx] = mask
	p.bars[barIdx].Addr = p.registers[regIdx]
	p.bars[barIdx].Used = true
	p.bars[barIdx].Type = &config.RegionType

	return nil
}

// AddPciRomBar adds expansion ROM BAR.
func (p *PciConfiguration) AddPciRomBar(config *PciBarConfiguration, active uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	barIdx := config.Idx
	regIdx := RomBarReg

	if p.romBarUsed {
		return &RomBarInUseError{Idx: barIdx}
	}

	if config.Size == 0 || (config.Size&(config.Size-1)) != 0 {
		return &RomBarSizeInvalidError{Size: config.Size}
	}

	if barIdx != RomBarIdx {
		return &RomBarInvalidError{Idx: barIdx}
	}

	endAddr := config.Addr + (config.Size - 1)
	if endAddr < config.Addr || endAddr > 0xffffffff {
		return &RomBarAddressInvalidError{Addr: config.Addr, Size: config.Size}
	}

	p.registers[regIdx] = uint32(config.Addr) | active
	p.writableBits[regIdx] = RomBarAddrMask
	p.romBarAddr = p.registers[regIdx]

	encodedSize, ok := encode32BitsBarSize(uint32(config.Size))
	if !ok {
		return ErrEncode32BarSize
	}
	p.romBarSize = encodedSize
	p.romBarUsed = true

	return nil
}

// GetBarAddr returns the address of the given BAR region.
func (p *PciConfiguration) GetBarAddr(barNum int) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	barIdx := Bar0Reg + barNum
	addr := uint64(p.bars[barNum].Addr & p.writableBits[barIdx])

	if p.bars[barNum].Type != nil && *p.bars[barNum].Type == PciBarRegionTypeMemory64BitRegion {
		addr |= uint64(p.bars[barNum+1].Addr) << 32
	}

	return addr
}

func (p *PciConfiguration) SetIrq(line uint8, pin PciInterruptPin) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pinIdx := uint32(pin) + 1
	p.registers[InterruptLinePinReg] = (p.registers[InterruptLinePinReg] & 0xffff0000) |
		(pinIdx << 8) |
		uint32(line)
}

// AddCapability adds the capability `capData` to the list of capabilities.
func (p *PciConfiguration) AddCapability(capData PciCapability) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	totalLen := len(capData.Bytes())
	if totalLen == 0 {
		return 0, ErrCapabilityEmpty
	}

	var capOffset int
	var tailOffset int

	if p.lastCapability != nil {
		// nextDword inline equivalent
		next := p.lastCapability.Offset + p.lastCapability.Size
		capOffset = (next + 3) & ^3
		tailOffset = p.lastCapability.Offset + 1
	} else {
		capOffset = FirstCapabilityOffset
		tailOffset = CapabilityListHeadOffset
	}

	endOffset := capOffset + totalLen
	if endOffset > CapabilityMaxOffset {
		return 0, &CapabilitySpaceFullError{Size: totalLen}
	}

	p.registers[StatusReg] |= StatusRegCapabilitiesUsedMask
	p.writeByteInternal(tailOffset, uint8(capOffset), false)
	p.writeByteInternal(capOffset, uint8(capData.ID()), false)
	p.writeByteInternal(capOffset+1, 0, false) // Next pointer

	for i, byteVal := range capData.Bytes() {
		p.writeByteInternal(capOffset+i+2, byteVal, false)
	}

	p.lastCapability = &PciCapabilityBounds{Offset: capOffset, Size: totalLen}

	switch capData.ID() {
	case PciCapabilityIDMessageSignalledInterrupts:
		p.writableBits[capOffset/4] = MsiCapabilityRegisterMask
	case PciCapabilityIDMsiX:
		idx := capOffset / 4
		p.msixCapRegIdx = &idx
		p.writableBits[*p.msixCapRegIdx] = MsixCapabilityRegisterMask
	}

	return capOffset, nil
}

// WriteConfigRegister handles routing guest programmatic writes inside the PCI space.
func (p *PciConfiguration) WriteConfigRegister(regIdx int, offset uint64, data []byte) []BarReprogrammingParams {
	p.mu.Lock()
	defer p.mu.Unlock()

	if int(offset)+len(data) > 4 {
		return make([]BarReprogrammingParams, 0)
	}

	if p.msixCapRegIdx != nil && p.msixConfig != nil {
		msixRegIdx := *p.msixCapRegIdx
		if msixRegIdx == regIdx && offset == 2 && len(data) == 2 {
			p.msixConfig.SetMsgCtl(binary.LittleEndian.Uint16(data))
		} else if msixRegIdx == regIdx && offset == 0 && len(data) == 4 {
			p.msixConfig.SetMsgCtl(uint16(binary.LittleEndian.Uint32(data) >> 16))
		}
	}

	switch len(data) {
	case 1:
		p.writeByteInternal(regIdx*4+int(offset), data[0], true)
	case 2:
		val := uint16(data[0]) | (uint16(data[1]) << 8)
		// Simulating write_word bounds check inline logic natively without secondary locks
		var shift uint32 = 0
		if offset%4 == 2 {
			shift = 16
		}
		writableMask := p.writableBits[regIdx]
		mask := (uint32(0xffff) << shift) & writableMask
		shiftedValue := (uint32(val) << shift) & writableMask
		p.registers[regIdx] = (p.registers[regIdx] & ^mask) | shiftedValue
	case 4:
		p.writeRegInternal(regIdx, binary.LittleEndian.Uint32(data))
	}

	if param, ok := p.detectBarRepprogrammingInternal(regIdx, data); ok {
		p.pendingBarReprogram = append(p.pendingBarReprogram, param)
	}

	if len(p.pendingBarReprogram) > 0 {
		if p.registers[CommandReg]&CommandRegMemorySpaceMask == CommandSpaceMask {
			log.Printf("info: BAR reprogramming parameter is returned: %+v", p.pendingBarReprogram)
			// Simulates Rust's .drain(..).collect()
			ret := make([]BarReprogrammingParams, len(p.pendingBarReprogram))
			copy(ret, p.pendingBarReprogram)
			p.pendingBarReprogram = p.pendingBarReprogram[:0]
			return ret
		}
		log.Printf("info: MSE bit is disabled. No BAR reprogramming parameter is returned: %+v", p.pendingBarReprogram)
	}

	return make([]BarReprogrammingParams, 0)
}

func (p *PciConfiguration) ReadConfigRegister(regIdx int) uint32 {
	return p.ReadReg(regIdx)
}

// Global scope alias placeholder mapping to CommandRegMemorySpaceMask
const CommandSpaceMask = 0x00000002

func (p *PciConfiguration) detectBarRepprogrammingInternal(regIdx int, data []byte) (BarReprogrammingParams, bool) {
	if len(data) != 4 {
		return BarReprogrammingParams{}, false
	}

	value := binary.LittleEndian.Uint32(data)
	mask := p.writableBits[regIdx]

	if regIdx >= Bar0Reg && regIdx < Bar0Reg+NumBarRegs {
		if value == 0xffffffff {
			return BarReprogrammingParams{}, false
		}

		barIdx := regIdx - 4
		if p.bars[barIdx].Type != nil {
			barType := *p.bars[barIdx].Type

			if barType == PciBarRegionTypeMemory64BitRegion {
				return BarReprogrammingParams{}, false
			}

			if (value & mask) == (p.bars[barIdx].Addr & mask) {
				return BarReprogrammingParams{}, false
			}

			log.Printf("info: Detected BAR reprogramming: (BAR %d) 0x%x -> 0x%x", barIdx, p.bars[barIdx].Addr, value)
			oldBase := uint64(p.bars[barIdx].Addr & mask)
			newBase := uint64(value & mask)

			size, _ := decode32BitsBarSize(p.bars[barIdx].Size)
			length := uint64(size)

			p.bars[barIdx].Addr = value

			return BarReprogrammingParams{
				OldBase:    oldBase,
				NewBase:    newBase,
				Len:        length,
				RegionType: barType,
			}, true

		} else if regIdx > Bar0Reg {
			prevRegWritableMask := p.writableBits[regIdx-1]
			prevBarIdx := barIdx - 1

			if (p.registers[regIdx-1]&prevRegWritableMask) != (p.bars[prevBarIdx].Addr&prevRegWritableMask) ||
				(value&mask) != (p.bars[barIdx].Addr&mask) {

				log.Printf("info: Detected BAR reprogramming: (BAR %d) 0x%x -> 0x%x", barIdx, p.bars[barIdx].Addr, value)
				oldBase := (uint64(p.bars[barIdx].Addr&mask) << 32) | uint64(p.bars[prevBarIdx].Addr&prevRegWritableMask)
				newBase := (uint64(value&mask) << 32) | uint64(p.registers[regIdx-1]&prevRegWritableMask)

				length, _ := decode64BitsBarSize(p.bars[barIdx].Size, p.bars[prevBarIdx].Size)
				p.bars[barIdx].Addr = value
				p.bars[prevBarIdx].Addr = p.registers[regIdx-1]

				return BarReprogrammingParams{
					OldBase:    oldBase,
					NewBase:    newBase,
					Len:        length,
					RegionType: PciBarRegionTypeMemory64BitRegion,
				}, true
			}
		}
	} else if regIdx == RomBarReg && (value&mask) != (p.romBarAddr&mask) {
		if value&RomBarAddrMask == RomBarAddrMask {
			return BarReprogrammingParams{}, false
		}

		log.Printf("info: Detected ROM BAR reprogramming: (Expansion ROM BAR) 0x%x -> 0x%x", p.romBarAddr, value)
		oldBase := uint64(p.romBarAddr & mask)
		newBase := uint64(value & mask)

		size, _ := decode32BitsBarSize(p.romBarSize)
		length := uint64(size)

		p.romBarAddr = value

		return BarReprogrammingParams{
			OldBase:    oldBase,
			NewBase:    newBase,
			Len:        length,
			RegionType: PciBarRegionTypeMemory32BitRegion,
		}, true
	}

	return BarReprogrammingParams{}, false
}

func (p *PciConfiguration) PendingBarReprogram() []BarReprogrammingParams {
	p.mu.Lock()
	defer p.mu.Unlock()

	ret := make([]BarReprogrammingParams, len(p.pendingBarReprogram))
	copy(ret, p.pendingBarReprogram)
	return ret
}

func (p *PciConfiguration) ClearPendingBarReprogram() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pendingBarReprogram = make([]BarReprogrammingParams, 0)
}

// PciBarPrefetchable represents the prefetchable attribute of a PCI BAR.
type PciBarPrefetchable bool

const (
	Prefetchable    PciBarPrefetchable = true
	NotPrefetchable PciBarPrefetchable = false
)

// PciBarConfiguration contains configuration properties for a PCI Base Address Register.
type PciBarConfiguration struct {
	Idx          int
	Addr         uint64
	Size         uint64
	RegionType   PciBarRegionType
	Prefetchable PciBarPrefetchable
}

// DefaultPciBarConfiguration simulates `impl Default` in Rust.
func DefaultPciBarConfiguration() PciBarConfiguration {
	return PciBarConfiguration{
		Idx:          0,
		Addr:         0,
		Size:         0,
		RegionType:   PciBarRegionTypeMemory64BitRegion,
		Prefetchable: NotPrefetchable,
	}
}

// NewPciBarConfiguration creates a new configuration with initial required properties.
func NewPciBarConfiguration(
	idx int,
	size uint64,
	regionType PciBarRegionType,
	prefetchable PciBarPrefetchable,
) PciBarConfiguration {
	return PciBarConfiguration{
		Idx:          idx,
		Addr:         0,
		Size:         size,
		RegionType:   regionType,
		Prefetchable: prefetchable,
	}
}

// --- Builder Methods (Fluent Interface) ---

// SetIndex updates the index and returns the pointer for method chaining.
func (c *PciBarConfiguration) SetIndex(idx int) *PciBarConfiguration {
	c.Idx = idx
	return c
}

// SetAddress updates the address and returns the pointer for method chaining.
func (c *PciBarConfiguration) SetAddress(addr uint64) *PciBarConfiguration {
	c.Addr = addr
	return c
}

// SetSize updates the size and returns the pointer for method chaining.
func (c *PciBarConfiguration) SetSize(size uint64) *PciBarConfiguration {
	c.Size = size
	return c
}

// SetRegionType updates the region type and returns the pointer for method chaining.
func (c *PciBarConfiguration) SetRegionType(regionType PciBarRegionType) *PciBarConfiguration {
	c.RegionType = regionType
	return c
}

// SetPrefetchable updates the prefetchable flag and returns the pointer for method chaining.
func (c *PciBarConfiguration) SetPrefetchable(prefetchable PciBarPrefetchable) *PciBarConfiguration {
	c.Prefetchable = prefetchable
	return c
}

// --- Getter Methods ---

// GetIdx returns the index of the BAR.
func (c *PciBarConfiguration) GetIdx() int {
	return c.Idx
}

// GetAddr returns the base address of the BAR.
func (c *PciBarConfiguration) GetAddr() uint64 {
	return c.Addr
}

// GetSize returns the allocated memory size of the BAR.
func (c *PciBarConfiguration) GetSize() uint64 {
	return c.Size
}

// GetRegionType returns the PciBarRegionType (e.g., IO or Memory space).
func (c *PciBarConfiguration) GetRegionType() PciBarRegionType {
	return c.RegionType
}

// GetPrefetchable returns whether the BAR region is prefetchable.
func (c *PciBarConfiguration) GetPrefetchable() PciBarPrefetchable {
	return c.Prefetchable
}
