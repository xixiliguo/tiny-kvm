package vmm

import (
	"io"
	"sync"

	"github.com/xixiliguo/tiny-kvm/device"
	"github.com/xixiliguo/tiny-kvm/device/legacy"
)

const (
	LegacySerialPort     uint64 = 0x3f8
	LegacySerialPortSize uint64 = 0x8
	LegacySerialIRQ             = 4
	DebugPort            uint64 = 0xe9
)

// SerialManager owns the legacy 8250 UART and the 0xe9 debug port.
type SerialManager struct {
	mu         sync.Mutex
	uart       *legacy.Serial
	output     *consoleOutput
	controller device.InterruptController
	closed     bool
}

// NewSerialManager creates the COM1 UART and its output sink.
func NewSerialManager(controller device.InterruptController, config ConsoleConfig) (*SerialManager, error) {
	output, err := newConsoleOutput(config)
	if err != nil {
		return nil, err
	}
	manager := &SerialManager{
		output:     output,
		controller: controller,
	}
	manager.uart = legacy.NewSerial("serial0", controller, LegacySerialIRQ, output)
	return manager, nil
}

// Read delegates UART reads.
func (s *SerialManager) Read(base, offset uint64, data []byte) {
	s.uart.Read(base, offset, data)
}

// Write delegates UART writes.
func (s *SerialManager) Write(base, offset uint64, data []byte) error {
	s.uart.Write(base, offset, data)
	return nil
}

// QueueInputBytes feeds host input into the UART receive FIFO.
func (s *SerialManager) QueueInputBytes(data []byte) error {
	return s.uart.QueueInputBytes(data)
}

// WriteDebugPort implements the Linux early console byte sink.
func (s *SerialManager) WriteDebugPort(_ uint64, _ uint64, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.output.Write(data)
	return err
}

// ReadDebugPort implements the write-only debug port as an empty source.
func (s *SerialManager) ReadDebugPort(_ uint64, _ uint64, data []byte) {
	clear(data)
}

// Close flushes and closes the serial output.
func (s *SerialManager) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	_ = s.uart.FlushOutput()
	return s.output.Close()
}

var _ io.Writer = (*consoleOutput)(nil)
