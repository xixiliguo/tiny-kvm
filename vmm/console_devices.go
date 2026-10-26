package vmm

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// consoleOutput is an owned output sink for a console device.
type consoleOutput struct {
	writer        io.Writer
	file          *os.File
	terminalFD    int
	terminalState *term.State
}

func newConsoleOutput(config ConsoleConfig) (*consoleOutput, error) {
	switch config.Mode {
	case "":
		return newTerminalConsoleOutput(os.Stdout)
	case ConsoleOutputStdout:
		return newTerminalConsoleOutput(os.Stdout)
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

func newTerminalConsoleOutput(file *os.File) (*consoleOutput, error) {
	output := &consoleOutput{writer: file}
	fd := int(file.Fd())
	// if !term.IsTerminal(fd) {
	// 	return output, nil
	// }

	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("console: set terminal raw mode: %w", err)
	}
	output.terminalFD = fd
	output.terminalState = state
	return output, nil
}

func (c *consoleOutput) Write(data []byte) (int, error) {
	return c.writer.Write(data)
}

func (c *consoleOutput) Close() error {
	var first error
	if c.terminalState != nil {
		if err := term.Restore(c.terminalFD, c.terminalState); err != nil {
			first = fmt.Errorf("console: restore terminal mode: %w", err)
		}
		c.terminalState = nil
	}
	if c.file != nil {
		file := c.file
		c.file = nil
		if err := file.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
