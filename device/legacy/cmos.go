package legacy

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xixiliguo/tiny-kvm/internal/eventfd"
)

const (
	IndexMask   uint8  = 0x7f
	IndexOffset uint64 = 0x0
	DataOffset  uint64 = 0x1
	DataLen     int    = 128
)

// Cmos emulates a CMOS/RTC device commonly targeted on x86 I/O ports 0x70/0x71.
type Cmos struct {
	mu                 sync.Mutex
	index              uint8
	data               [DataLen]uint8
	resetEvt           *eventfd.EventFd
	vcpusKillSignalled *atomic.Pointer[atomic.Bool] // Corresponds to Option<Arc<AtomicBool>>
}

// NewCmos constructs a functional CMOS/RTC virtual hardware component.
// memBelow4g: Total physical memory bytes allocated below the 32-bit PCI hole gap.
// memAbove4g: Total physical memory bytes allocated above the 4GB memory layout line.
func NewCmos(memBelow4g uint64, memAbove4g uint64, resetEvt *eventfd.EventFd, killSignal *atomic.Pointer[atomic.Bool]) *Cmos {
	c := &Cmos{
		resetEvt:           resetEvt,
		vcpusKillSignalled: killSignal,
	}

	// Calculate and report base conventional extended memory limits (16MB to 4GB) in 64KB increments
	var extMem uint64
	if memBelow4g > 16*1024*1024 {
		extMem = (memBelow4g - 16*1024*1024) / (64 * 1024)
	}
	if extMem > 0xFFFF {
		extMem = 0xFFFF
	}
	c.data[0x34] = uint8(extMem)
	c.data[0x35] = uint8(extMem >> 8)

	// Calculate high memory limits (> 4GB) reported in 64KB increments
	highMem := memAbove4g / (64 * 1024)
	if highMem > 0x00FF_FFFF {
		highMem = 0x00FF_FFFF
	}
	c.data[0x5b] = uint8(highMem)
	c.data[0x5c] = uint8(highMem >> 8)
	c.data[0x5d] = uint8(highMem >> 16)

	return c
}

// Write captures downstream Port IO modifications (OUT instruction targets).
func (c *Cmos) Write(base uint64, offset uint64, data []byte) {
	if len(data) != 1 {
		log.Printf("Invalid write size on CMOS device: %d", len(data))
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	val := data[0]
	switch offset {
	case IndexOffset:
		c.index = val
	case DataOffset:
		// Index 0x8f with value 0 triggers an architectural hard system reset command.
		if c.index == 0x8f && val == 0 {
			log.Println("CMOS reset triggered")
			if err := c.resetEvt.Write(1); err != nil {
				log.Printf("Failed to signal reset eventfd: %v", err)
			}

			// If a vcpu kill reference exists, safely swing ownership out and spin wait.
			if c.vcpusKillSignalled != nil {
				killPtr := c.vcpusKillSignalled.Swap(nil)
				if killPtr != nil {
					// Spin and yield execution to prevent vCPU re-entry windows during a panic/shutdown.
					for !killPtr.Load() {
						time.Sleep(1 * time.Millisecond)
					}
				}
			}
		} else {
			c.data[c.index&IndexMask] = val
		}
	default:
		log.Printf("bad write offset on CMOS device: %d", offset)
	}
}

// Read retrieves runtime configuration metadata or clock time variables (IN instruction targets).
func (c *Cmos) Read(base uint64, offset uint64, data []byte) {
	if len(data) != 1 {
		log.Printf("Invalid read size on CMOS device: %d", len(data))
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch offset {
	case IndexOffset:
		data[0] = c.index
	case DataOffset:
		now := time.Now().UTC()

		// Calculate Update In Progress (UIP) flag status.
		// CMOS standard holds this bit high for the last 244us of every single second increment cycle.
		const nanosecondsPerSecond = 1_000_000_000
		const uipHoldLength = 8 * nanosecondsPerSecond / 32768
		currentNanoseconds := now.Nanosecond()
		updateInProgress := currentNanoseconds >= (nanosecondsPerSecond - uipHoldLength)

		switch c.index {
		case 0x00:
			data[0] = toBcd(uint8(now.Second()))
		case 0x02:
			data[0] = toBcd(uint8(now.Minute()))
		case 0x04:
			data[0] = toBcd(uint8(now.Hour()))
		case 0x06:
			// RTC weekday representation scale ranges from 1 (Sunday) through 7 (Saturday).
			data[0] = toBcd(uint8(now.Weekday() + 1))
		case 0x07:
			data[0] = toBcd(uint8(now.Day()))
		case 0x08:
			data[0] = toBcd(uint8(now.Month()))
		case 0x09:
			data[0] = toBcd(uint8(now.Year() % 100))
		case 0x0a:
			// Bit 5 signals a stable 32kHz oscillator clock. Bit 7 reflects UIP status.
			var uipMask uint8
			if updateInProgress {
				uipMask = 1 << 7
			}
			data[0] = (1 << 5) | uipMask
		case 0x0d:
			// Bit 7 must be set high indicating full internal battery power availability.
			data[0] = 1 << 7
		case 0x32:
			data[0] = toBcd(uint8((now.Year() + 1900) / 100))
		default:
			data[0] = c.data[c.index&IndexMask]
		}
	default:
		log.Printf("bad read offset on CMOS device: %d", offset)
		data[0] = 0
	}
}

// toBcd converts a standard binary integer into Binary Coded Decimal notation format.
func toBcd(v uint8) uint8 {
	if v >= 100 {
		panic("cmos: bcd input constraint validation out of bounds")
	}
	return ((v / 10) << 4) | (v % 10)
}
