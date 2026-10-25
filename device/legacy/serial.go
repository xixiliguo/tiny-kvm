package legacy

import (
	"io"
	"sync"

	"github.com/xixiliguo/tiny-kvm/device"
)

// UART Register Sizing Configurations
const LoopSize = 0x40

// UART Register Offsets (mapped to x86 I/O ports base+offset)
const (
	Data uint8 = 0 // Transmitter Holding Register / Receiver Buffer Register
	Ier  uint8 = 1 // Interrupt Enable Register
	Iir  uint8 = 2 // Interrupt Identification Register
	Lcr  uint8 = 3 // Line Control Register
	Mcr  uint8 = 4 // Modem Control Register
	Lsr  uint8 = 5 // Line Status Register
	Msr  uint8 = 6 // Modem Status Register
	Scr  uint8 = 7 // Scratch Register
)

const (
	DlabLow  uint8 = 0 // Divisor Latch Low Byte (when DLAB bit is set)
	DlabHigh uint8 = 1 // Divisor Latch High Byte (when DLAB bit is set)
)

// Interrupt Enable Register Flags
const (
	IerRecvBit  uint8 = 0x1
	IerThrBit   uint8 = 0x2
	IerFifoBits uint8 = 0x0f
)

// Interrupt Identification Register Flags
const (
	IirFifoBits uint8 = 0xc0
	IirNoneBit  uint8 = 0x1
	IirThrBit   uint8 = 0x2
	IirRecvBit  uint8 = 0x4
)

// Line Control Register Flags
const LcrDlabBit uint8 = 0x80

// Line Status Register Flags
const (
	LsrDataBit  uint8 = 0x1
	LsrEmptyBit uint8 = 0x20
	LsrIdleBit  uint8 = 0x40
)

// Modem Control Register Flags
const McrLoopBit uint8 = 0x10

// Reset Default Core Line Parameters
const (
	DefaultInterruptIdentification uint8  = IirNoneBit
	DefaultLineStatus              uint8  = LsrEmptyBit | LsrIdleBit
	DefaultLineControl             uint8  = 0x3                // 8-bits per character configuration
	DefaultModemControl            uint8  = 0x8                // Auxiliary output 2
	DefaultModemStatus             uint8  = 0x20 | 0x10 | 0x80 // Data ready, CTS, CD
	DefaultBaudDivisor             uint16 = 12                 // 9600 bps base line scale
)

// Serial emulates traditional x86 COM ports (e.g., 0x3f8, 0x2f8).
type Serial struct {
	mu                      sync.Mutex // Guard state mutations across concurrent I/O threads
	id                      string
	interruptEnable         uint8
	interruptIdentification uint8
	lineControl             uint8
	lineStatus              uint8
	modemControl            uint8
	modemStatus             uint8
	scratch                 uint8
	baudDivisor             uint16
	inBuffer                []byte // Implemented as a slice-backed FIFO ring queue
	interrupt               device.InterruptController
	pin                     int
	out                     io.Writer
}

// NewSerial initializes a functional UART container instance.
func NewSerial(id string, interrupt device.InterruptController, pin int, out io.Writer) *Serial {
	s := &Serial{
		id:        id,
		interrupt: interrupt,
		pin:       pin,
		out:       out,
	}

	s.interruptEnable = 0
	s.interruptIdentification = DefaultInterruptIdentification
	s.lineControl = DefaultLineControl
	s.lineStatus = DefaultLineStatus
	s.modemControl = DefaultModemControl
	s.modemStatus = DefaultModemStatus
	s.scratch = 0
	s.baudDivisor = DefaultBaudDivisor
	s.inBuffer = make([]byte, 0)

	return s
}

// QueueInputBytes appends external host console input into the guest's read-ahead buffer.
func (s *Serial) QueueInputBytes(c []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isLoop() {
		s.inBuffer = append(s.inBuffer, c...)
		return s.recvData()
	}
	return nil
}

// FlushOutput enforces downstream line clear actions onto the configured device backend.
func (s *Serial) FlushOutput() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.out != nil {
		if flusher, ok := s.out.(interface{ Flush() error }); ok {
			return flusher.Flush()
		}
	}
	return nil
}

func (s *Serial) isDlabSet() bool {
	return (s.lineControl & LcrDlabBit) != 0
}
func (s *Serial) isRecvIntrEnabled() bool {
	return (s.interruptEnable & IerRecvBit) != 0
}
func (s *Serial) isThrIntrEnabled() bool {
	return (s.interruptEnable & IerThrBit) != 0
}
func (s *Serial) isLoop() bool {
	return (s.modemControl & McrLoopBit) != 0
}

func (s *Serial) addIntrBit(bit uint8) {
	s.interruptIdentification &= ^IirNoneBit
	s.interruptIdentification |= bit
}

func (s *Serial) delIntrBit(bit uint8) {
	s.interruptIdentification &= ^bit
	if s.interruptIdentification == 0x0 {
		s.interruptIdentification = DefaultInterruptIdentification
	}
}

func (s *Serial) thrEmpty() error {
	if s.isThrIntrEnabled() {
		s.addIntrBit(IirThrBit)
		return s.triggerInterrupt()
	}
	return nil
}

func (s *Serial) recvData() error {
	if s.isRecvIntrEnabled() {
		s.addIntrBit(IirRecvBit)
		if err := s.triggerInterrupt(); err != nil {
			return err
		}
	}
	s.lineStatus |= LsrDataBit
	return nil
}

func (s *Serial) triggerInterrupt() error {
	return s.interrupt.ServiceIrq(s.pin)
}

func (s *Serial) iirReset() {
	s.interruptIdentification = DefaultInterruptIdentification
}

func (s *Serial) handleWrite(offset uint8, v uint8) error {
	switch offset {
	case DlabLow:
		if s.isDlabSet() {
			s.baudDivisor = (s.baudDivisor & 0xff00) | uint16(v)
		} else {
			return s.handleDataWrite(v)
		}
	case DlabHigh:
		if s.isDlabSet() {
			s.baudDivisor = (s.baudDivisor & 0x00ff) | (uint16(v) << 8)
		} else {
			s.interruptEnable = v & IerFifoBits
		}
	case Lcr:
		s.lineControl = v
	case Mcr:
		s.modemControl = v
	case Scr:
		s.scratch = v
	}
	return nil
}

func (s *Serial) handleDataWrite(v uint8) error {
	if s.isLoop() {
		if len(s.inBuffer) < LoopSize {
			s.inBuffer = append(s.inBuffer, v)
			return s.recvData()
		}
	} else {
		if s.out != nil {
			if _, err := s.out.Write([]byte{v}); err != nil {
				return err
			}
			if flusher, ok := s.out.(interface{ Flush() error }); ok {
				_ = flusher.Flush()
			}
		}
		return s.thrEmpty()
	}
	return nil
}

// Read processes trapped MMIO/PIO reads executed by the guest OS kernel.
func (s *Serial) Read(base uint64, offset uint64, data []byte) {
	if len(data) != 1 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch uint8(offset) {
	case DlabLow:
		if s.isDlabSet() {
			data[0] = uint8(s.baudDivisor)
		} else {
			s.delIntrBit(IirRecvBit)
			if len(s.inBuffer) <= 1 {
				s.lineStatus &= ^LsrDataBit
			}
			if len(s.inBuffer) > 0 {
				data[0] = s.inBuffer[0]
				s.inBuffer = s.inBuffer[1:]
			} else {
				data[0] = 0
			}
		}
	case DlabHigh:
		if s.isDlabSet() {
			data[0] = uint8(s.baudDivisor >> 8)
		} else {
			data[0] = s.interruptEnable
		}
	case Iir:
		data[0] = s.interruptIdentification | IirFifoBits
		s.iirReset()
	case Lcr:
		data[0] = s.lineControl
	case Mcr:
		data[0] = s.modemControl
	case Lsr:
		data[0] = s.lineStatus
	case Msr:
		data[0] = s.modemStatus
	case Scr:
		data[0] = s.scratch
	default:
		data[0] = 0
	}
}

// Write intercepts trapped MMIO/PIO configuration registers modifications.
func (s *Serial) Write(base uint64, offset uint64, data []byte) {
	if len(data) != 1 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.handleWrite(uint8(offset), data[0])
}
