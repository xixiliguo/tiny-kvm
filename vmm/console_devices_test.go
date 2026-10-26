package vmm

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestTerminalConsoleOutputRestoresMode(t *testing.T) {
	slave := openPTYSlave(t)
	fd := int(slave.Fd())

	before, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatalf("get initial terminal mode: %v", err)
	}
	output, err := newTerminalConsoleOutput(slave)
	if err != nil {
		t.Fatalf("new terminal console output: %v", err)
	}
	if output.terminalState == nil {
		t.Fatal("terminal console output did not enter raw mode")
	}

	raw, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatalf("get raw terminal mode: %v", err)
	}
	if raw.Lflag&unix.ICANON != 0 || raw.Lflag&unix.ECHO != 0 {
		t.Fatalf("terminal is not in raw mode: lflag=%#x", raw.Lflag)
	}

	if err := output.Close(); err != nil {
		t.Fatalf("close terminal console output: %v", err)
	}
	after, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatalf("get restored terminal mode: %v", err)
	}
	if *after != *before {
		t.Fatalf("terminal mode was not restored: before=%+v after=%+v", *before, *after)
	}
}

func openPTYSlave(t *testing.T) *os.File {
	t.Helper()

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("open pseudo-terminal: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })

	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlock pseudo-terminal: %v", err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("get pseudo-terminal number: %v", err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open pseudo-terminal slave: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return slave
}
