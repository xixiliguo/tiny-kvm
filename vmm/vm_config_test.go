package vmm

import (
	"strings"
	"testing"
)

func TestDefaultVmConfigAndValidation(t *testing.T) {
	config := DefaultVmConfig()
	config.Payload.Kernel = "/tmp/vmlinuz"
	if err := config.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if config.Cpus.BootVcpus != 1 || config.Cpus.MaxPhysBits != 46 {
		t.Fatalf("unexpected CPU defaults: %+v", config.Cpus)
	}
	if config.Memory.Size != 512<<20 {
		t.Fatalf("unexpected memory default: %d", config.Memory.Size)
	}
}

func TestParseConfigAppliesDefaults(t *testing.T) {
	config, err := ParseConfig([]byte(`{
		"memory": {"size": 1073741824},
		"payload": {"kernel": "/tmp/vmlinuz"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if config.Cpus.BootVcpus != 1 || config.Serial.Mode != ConsoleOutputStdout {
		t.Fatalf("defaults not applied: %+v", config)
	}
	if config.Memory.Size != 1<<30 {
		t.Fatalf("memory size = %d", config.Memory.Size)
	}
}

func TestVmConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*VmConfig)
		want string
	}{
		{"missing kernel", func(c *VmConfig) { c.Payload.Kernel = "" }, "kernel"},
		{"too many vcpus", func(c *VmConfig) { c.Cpus.BootVcpus = 2 }, "boot_vcpus"},
		{"small memory", func(c *VmConfig) { c.Memory.Size = 4096 }, "at least"},
		{"file mode without path", func(c *VmConfig) { c.Serial = ConsoleConfig{Mode: ConsoleOutputFile} }, "serial file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultVmConfig()
			config.Payload.Kernel = "/tmp/vmlinuz"
			test.edit(&config)
			err := config.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.want)
			}
		})
	}
}
