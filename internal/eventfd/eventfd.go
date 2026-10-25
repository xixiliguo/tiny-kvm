package eventfd

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

// Eventfd represents a Linux eventfd object.
type EventFd struct {
	fd int
}

// NewEventfd returns an initialized eventfd.
func NewEventfd() (*EventFd, error) {
	fd, _, err := unix.RawSyscall(unix.SYS_EVENTFD2, 0, 0, 0)
	if err != 0 {
		return nil, fmt.Errorf("failed to create eventfd: %v", error(err))
	}
	if err := unix.SetNonblock(int(fd), true); err != nil {
		unix.Close(int(fd))
		return nil, err
	}
	return &EventFd{fd: int(fd)}, nil
}

// Close releases the eventfd.
func (ev *EventFd) Close() error {
	if ev == nil || ev.fd < 0 {
		return nil
	}
	err := unix.Close(ev.fd)
	ev.fd = -1
	return err
}

// Write writes a specific value to the eventfd.
func (ev EventFd) Write(val uint64) error {
	var buf [8]byte
	binary.NativeEndian.PutUint64(buf[:], val)

	for {
		n, err := unix.Write(ev.fd, buf[:])
		if err == unix.EINTR {
			continue
		}
		if err != nil || n != 8 {
			panic(fmt.Sprintf("bad write to eventfd: got %d bytes, wanted %d with error %v", n, 8, err))
		}
		return err
	}
}
