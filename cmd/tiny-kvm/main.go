package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/xixiliguo/tiny-kvm/vmm"
)

func main() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s
EXAMPLES:
	tiny-kvm --kernel bzImage --initramfs initramfs.img --cpu 1 --memory 512
	`, cli.AppHelpTemplate)
	app := &cli.App{
		Name:    "tiny-kvm",
		Usage:   "A tiny VMM based on KVM",
		Version: "0.0.1",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "kernel",
				Aliases:  []string{"k"},
				Required: true,
				Usage:    "Path to kernel to load",
			},
			&cli.StringFlag{
				Name:     "initramfs",
				Aliases:  []string{"i"},
				Required: true,
				Usage:    "Path to initramfs image",
			},
			&cli.IntFlag{
				Name:     "cpu",
				Aliases:  []string{"c"},
				Required: true,
				Value:    1,
				Usage:    "count of logical cpu",
			},
			&cli.IntFlag{
				Name:     "memory",
				Aliases:  []string{"m"},
				Required: true,
				Value:    1024,
				Usage:    "guest memory size, unit: MB",
			},
			&cli.StringFlag{
				Name:  "cmdline",
				Usage: "Linux kernel command line",
			},
		},
		Action: func(ctx *cli.Context) error {
			memoryMB := ctx.Int("memory")
			if memoryMB <= 0 {
				return fmt.Errorf("memory must be greater than zero")
			}
			config := vmm.DefaultVmConfig()
			config.Cpus.BootVcpus = uint32(ctx.Int("cpu"))
			config.Cpus.MaxVcpus = config.Cpus.BootVcpus
			config.Memory.Size = uint64(memoryMB) << 20
			config.Payload.Kernel = ctx.String("kernel")
			config.Payload.Initramfs = ctx.String("initramfs")
			config.Payload.Cmdline = ctx.String("cmdline")

			manager, err := vmm.New()
			if err != nil {
				return fmt.Errorf("open KVM: %w", err)
			}
			machine, err := manager.CreateVM(config)
			if err != nil {
				return err
			}
			defer machine.Close()

			fmt.Fprintf(os.Stderr, "booting tiny-kvm: %d vCPU(s), %d MiB RAM\n", config.Cpus.BootVcpus, memoryMB)
			return machine.Run()
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
