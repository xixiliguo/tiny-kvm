package hyper

import (
	"encoding/binary"
	"os"

	"golang.org/x/sys/unix"
)

// EventFd wraps a Linux eventfd.
//
// An eventfd acts like a counter: writes add to the counter, reads retrieve
// the counter and reset it to zero. Reads also block if the counter is zero.
//
// See man 2 eventfd.
type EventFd struct {
	file *os.File
	// prefer raw over file.Fd(), since the latter puts the file into blocking
	// mode.
	raw int
}

func NewEventFd(id string) (*EventFd, error) {
	fd, err := unix.Eventfd(0, unix.O_CLOEXEC|unix.O_NONBLOCK)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "eventfd-"+id)
	return &EventFd{file, fd}, nil
}

func (efd *EventFd) Close() error {
	return efd.file.Close()
}

func (efd *EventFd) Write(n uint64) error {
	var buf [8]byte
	binary.NativeEndian.PutUint64(buf[:], n)
	_, err := efd.file.Write(buf[:])
	return err
}

func (efd *EventFd) Read() (uint64, error) {
	var buf [8]byte
	_, err := efd.file.Read(buf[:])
	return binary.NativeEndian.Uint64(buf[:]), err
}

func (efd *EventFd) Fd() int {
	return efd.raw
}
