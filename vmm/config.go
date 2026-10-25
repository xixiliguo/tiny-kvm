package vmm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// ParseConfig decodes a JSON VmConfig. Missing values retain the defaults,
// matching Cloud Hypervisor's serde defaults closely enough for the supported
// fields.
func ParseConfig(data []byte) (VmConfig, error) {
	config := DefaultVmConfig()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&config); err != nil {
		return VmConfig{}, fmt.Errorf("vmm: decode config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return VmConfig{}, err
	}
	return config, nil
}

// LoadConfig reads and validates a JSON VmConfig.
func LoadConfig(path string) (VmConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return VmConfig{}, fmt.Errorf("vmm: read config %q: %w", path, err)
	}
	return ParseConfig(data)
}

// Clone returns an independent copy of a VM configuration.
func (c VmConfig) Clone() VmConfig {
	clone := c
	if c.Cpus.Topology != nil {
		topology := *c.Cpus.Topology
		clone.Cpus.Topology = &topology
	}
	if c.Platform != nil {
		platform := *c.Platform
		platform.OEMStrings = append([]string(nil), c.Platform.OEMStrings...)
		clone.Platform = &platform
	}
	return clone
}
