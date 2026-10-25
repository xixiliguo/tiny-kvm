// minimal example
// ref: https://lwn.net/Articles/658511/
package main

import (
	"bytes"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/xixiliguo/tiny-kvm/hyper"
	"golang.org/x/sys/unix"
)

const RamSize = 0x1000 // ram 4K

type simpleOps struct {
	buf bytes.Buffer
}

func (s *simpleOps) GuestMemWrite(gpa uint64, buf []byte) (int, error) {
	return 0, nil
}

func (s *simpleOps) GuestMemRead(gpa uint64, buf []byte) (int, error) {
	return 0, nil
}
func (s *simpleOps) MmioRead(gpa uint64, data []byte) error {
	return nil
}
func (s *simpleOps) MmioWrite(gpa uint64, data []byte) error {
	return nil
}
func (s *simpleOps) PioRead(port uint64, data []byte) error {
	return nil
}
func (s *simpleOps) PioWrite(port uint64, data []byte) error {
	// fmt.Printf("port: %x data %+v\n", port, string(data))
	s.buf.Write(data)
	if data[0] == '\n' {
		fmt.Printf("%s", s.buf.String())
	}
	return nil
}

func main() {

	kvm, err := hyper.NewKvmHyper()

	vm, err := kvm.CreateVM(RamSize)

	if err != nil {
		fmt.Printf("fail to create vm: %v\n", err)
		return
	}

	guestMem, err := unix.Mmap(-1, 0, RamSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_ANONYMOUS)
	if err != nil {
		fmt.Printf("fail to mmap guest memory: %v\n", err)
		return
	}

	defer unix.Munmap(guestMem)

	code := []byte{
		0xba, 0xf8, 0x03, /* mov $0x3f8, %dx */
		0x00, 0xd8, /* add %bl, %al */
		0x04, '0', /* add $'0', %al */
		0xee,       /* out %al, (%dx) */
		0xb0, '\n', /* mov $'\n', %al */
		0xee, /* out %al, (%dx) */
		0xf4, /* hlt */
	}

	copy(guestMem, code)

	if err := vm.CreateUserMemoryRegion(0, 0x1000, RamSize, uint64(uintptr(unsafe.Pointer(&guestMem[0])))); err != nil {
		fmt.Printf("KvmSetUserMemoryRegion: %v\n", err)
		return
	}
	ops := &simpleOps{}
	vcpu, err := vm.CreateVcpu(0, ops)
	if err != nil {
		fmt.Printf("fail to create vcpu: %v\n", err)
		return
	}

	sregs, err := vcpu.GetSregs()
	if err != nil {
		fmt.Printf("fail to get sregs: %v\n", err)
		return
	}
	sregs.Cs.Base = 0
	sregs.Cs.Selector = 0
	if err := vcpu.SetSregs(&sregs); err != nil {
		fmt.Printf("fail to set sregs: %v\n", err)
		return
	}

	regs := &hyper.StandardRegisters{
		Rip:    0x1000,
		Rflags: 0x2,
		Rax:    2,
		Rbx:    3,
	}
	if err := vcpu.SetRegs(regs); err != nil {
		fmt.Printf("fail to set regs: %v\n", err)
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for {
		exit, err := vcpu.Run()
		if err != nil {
			fmt.Printf("run error: %s", err)
			break
		}
		fmt.Printf("vm exit reason: %s\n", exit)
		if exit == hyper.VmExitIgnore {
			continue
		} else {
			break
		}
	}
}
