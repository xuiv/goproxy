// +build windows

package glog

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode          = kernel32.NewProc("GetConsoleMode")
	procSetConsoleTextAttribute = kernel32.NewProc("SetConsoleTextAttribute")
	procSetStdHandle            = kernel32.NewProc("SetStdHandle")
)

func WriteFileWithColor(file *os.File, data []byte, s severity) {
	switch s {
	case fatalLog:
		procSetConsoleTextAttribute.Call(file.Fd(), uintptr(0x04))
	case errorLog:
		procSetConsoleTextAttribute.Call(file.Fd(), uintptr(0x04))
	case warningLog:
		procSetConsoleTextAttribute.Call(file.Fd(), uintptr(0x06))
	case infoLog:
		procSetConsoleTextAttribute.Call(file.Fd(), uintptr(0x03))
	}
	file.Write(data[:5])
	procSetConsoleTextAttribute.Call(file.Fd(), uintptr(0x07))
	file.Write(data[5:])
}

func IsTerminal(fd uintptr) bool {
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, fd, uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}

func RedirectStderrTo(file *os.File) error {
	var handle int32 = syscall.STD_ERROR_HANDLE

	r, _, e := syscall.Syscall(procSetStdHandle.Addr(), 2, uintptr(handle), uintptr(file.Fd()), 0)
	if r == 0 {
		if e != 0 {
			return error(e)
		}
		return syscall.EINVAL
	}

	os.Stderr = file

	return nil
}
