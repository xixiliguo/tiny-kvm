package vmm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultVCPUs       uint32 = 1
	DefaultMaxPhysBits uint8  = 46
	DefaultMemoryBytes uint64 = 512 << 20
	DefaultCmdline            = "console=ttyS0 earlyprintk=serial"
)

// CpuTopology describes the guest CPU topology.
type CpuTopology struct {
	ThreadsPerCore uint16 `json:"threads_per_core,omitempty"`
	CoresPerDie    uint16 `json:"cores_per_die,omitempty"`
	DiesPerPackage uint16 `json:"dies_per_package,omitempty"`
	Packages       uint16 `json:"packages,omitempty"`
}

// CpusConfig contains the subset of Cloud Hypervisor's CPU configuration
// required by tiny-kvm.
type CpusConfig struct {
	BootVcpus   uint32       `json:"boot_vcpus"`
	MaxVcpus    uint32       `json:"max_vcpus"`
	Topology    *CpuTopology `json:"topology,omitempty"`
	KvmHyperv   bool         `json:"kvm_hyperv,omitempty"`
	MaxPhysBits uint8        `json:"max_phys_bits,omitempty"`
	Nested      bool         `json:"nested,omitempty"`
}

// DefaultCpusConfig returns the Cloud Hypervisor-compatible CPU defaults.
func DefaultCpusConfig() CpusConfig {
	return CpusConfig{
		BootVcpus:   DefaultVCPUs,
		MaxVcpus:    DefaultVCPUs,
		MaxPhysBits: DefaultMaxPhysBits,
		Nested:      true,
	}
}

// MemoryConfig describes guest RAM. Size is expressed in bytes.
type MemoryConfig struct {
	Size     uint64 `json:"size"`
	Shared   bool   `json:"shared,omitempty"`
	Prefault bool   `json:"prefault,omitempty"`
}

// DefaultMemoryConfig returns the Cloud Hypervisor-compatible memory defaults.
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{Size: DefaultMemoryBytes}
}

// PayloadConfig identifies the kernel and optional initramfs.
type PayloadConfig struct {
	Kernel    string `json:"kernel"`
	Cmdline   string `json:"cmdline,omitempty"`
	Initramfs string `json:"initramfs,omitempty"`
}

// Validate checks that the payload can be booted by the direct kernel path.
func (p PayloadConfig) Validate() error {
	if p.Kernel == "" {
		return errors.New("payload: kernel is required")
	}
	if p.Cmdline != "" && len(p.Cmdline)+1 > 64<<10 {
		return fmt.Errorf("payload: kernel command line exceeds %d bytes", 64<<10-1)
	}
	return nil
}

// ConsoleOutputMode selects where serial output is sent.
type ConsoleOutputMode string

const (
	ConsoleOutputNull   ConsoleOutputMode = "Null"
	ConsoleOutputStdout ConsoleOutputMode = "Stdout"
	ConsoleOutputFile   ConsoleOutputMode = "File"
)

// ConsoleConfig configures a serial or console device.
type ConsoleConfig struct {
	Mode ConsoleOutputMode `json:"mode,omitempty"`
	File string            `json:"file,omitempty"`
}

// DefaultSerialConfig sends the guest serial console to stdout.
func DefaultSerialConfig() ConsoleConfig {
	return ConsoleConfig{Mode: ConsoleOutputStdout}
}

// PlatformConfig supplies optional SMBIOS identity fields.
type PlatformConfig struct {
	SerialNumber string   `json:"serial_number,omitempty"`
	UUID         string   `json:"uuid,omitempty"`
	OEMStrings   []string `json:"oem_strings,omitempty"`
}

// VmConfig is the portable input used to create a VM.
type VmConfig struct {
	Cpus     CpusConfig      `json:"cpus"`
	Memory   MemoryConfig    `json:"memory"`
	Payload  PayloadConfig   `json:"payload"`
	Serial   ConsoleConfig   `json:"serial"`
	Platform *PlatformConfig `json:"platform,omitempty"`
}

// DefaultVmConfig returns a configuration with all required defaults filled.
func DefaultVmConfig() VmConfig {
	return VmConfig{
		Cpus:   DefaultCpusConfig(),
		Memory: DefaultMemoryConfig(),
		Serial: DefaultSerialConfig(),
	}
}

// Validate checks configuration invariants before KVM resources are touched.
func (c *VmConfig) Validate() error {
	if c.Cpus.BootVcpus == 0 {
		return errors.New("config: boot_vcpus must be greater than zero")
	}
	if c.Cpus.MaxVcpus == 0 {
		return errors.New("config: max_vcpus must be greater than zero")
	}
	if c.Cpus.BootVcpus > c.Cpus.MaxVcpus {
		return fmt.Errorf("config: boot_vcpus (%d) exceeds max_vcpus (%d)", c.Cpus.BootVcpus, c.Cpus.MaxVcpus)
	}
	if c.Cpus.MaxPhysBits == 0 || c.Cpus.MaxPhysBits > 63 {
		return fmt.Errorf("config: invalid max_phys_bits %d", c.Cpus.MaxPhysBits)
	}
	if c.Memory.Size == 0 {
		return errors.New("config: memory size must be greater than zero")
	}
	if c.Memory.Size < 2<<20 {
		return errors.New("config: guest memory must be at least 2 MiB")
	}
	if c.Memory.Size%uint64(os.Getpagesize()) != 0 {
		return fmt.Errorf("config: memory size must be a multiple of the %d-byte page size", os.Getpagesize())
	}
	if err := c.Payload.Validate(); err != nil {
		return err
	}
	if c.Serial.Mode == "" {
		c.Serial.Mode = ConsoleOutputStdout
	}
	switch c.Serial.Mode {
	case ConsoleOutputNull, ConsoleOutputStdout:
	case ConsoleOutputFile:
		if c.Serial.File == "" {
			return errors.New("config: serial file is required for File output")
		}
	default:
		return fmt.Errorf("config: unsupported serial output mode %q", c.Serial.Mode)
	}
	return nil
}

// ResolvePath returns an absolute path for payload and output files.
func ResolvePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	return filepath.Abs(path)
}
