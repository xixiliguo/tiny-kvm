package hyper

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func ioctl(fd uintptr, req uint) (int, error) {
	var (
		ret  uintptr
		err  error
		err1 unix.Errno
	)
	for {
		ret, _, err1 = unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), 0)
		if err1 == 0 {
			break
		}
		if err1 != unix.EINTR {
			err = err1
			break
		}
	}
	return int(ret), err
}

// ioctlNoRetry performs an ioctl once. KVM_RUN must not transparently retry
// EINTR because the signal is how a userspace VMM requests immediate exit.
func ioctlNoRetry(fd uintptr, req uint) (int, error) {
	ret, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), 0)
	if errno != 0 {
		return int(ret), errno
	}
	return int(ret), nil
}

func ioctlInt(fd uintptr, req uint, arg int) (int, error) {
	var (
		ret  uintptr
		err  error
		err1 unix.Errno
	)
	for {
		ret, _, err1 = unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg))
		if err1 == 0 {
			break
		}
		if err1 != unix.EINTR {
			err = err1
			break
		}
	}
	return int(ret), err
}

func ioctlPtr(fd uintptr, req uint, arg unsafe.Pointer) (err error) {
	for {
		_, _, err1 := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(arg))
		if err1 == 0 {
			break
		}
		if err1 != unix.EINTR {
			err = err1
			break
		}
	}
	return err
}

func ioctlBytes(fd uintptr, req uint, arg []byte) (err error) {
	for {
		_, _, err1 := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&arg[0])))
		if err1 == 0 {
			break
		}
		if err1 != unix.EINTR {
			err = err1
			break
		}
	}
	return err
}
