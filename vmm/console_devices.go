package vmm

import (
	"fmt"
	"io"
	"os"
)

// consoleOutput is an owned output sink for a console device.
type consoleOutput struct {
	writer io.Writer
	file   *os.File
}

func newConsoleOutput(config ConsoleConfig) (*consoleOutput, error) {
	switch config.Mode {
	case "", ConsoleOutputStdout:
		return &consoleOutput{writer: os.Stdout}, nil
	case ConsoleOutputNull:
		return &consoleOutput{writer: io.Discard}, nil
	case ConsoleOutputFile:
		file, err := os.OpenFile(config.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("console: open %q: %w", config.File, err)
		}
		return &consoleOutput{writer: file, file: file}, nil
	default:
		return nil, fmt.Errorf("console: unsupported mode %q", config.Mode)
	}
}

func (c *consoleOutput) Write(data []byte) (int, error) { return c.writer.Write(data) }

func (c *consoleOutput) Close() error {
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}
