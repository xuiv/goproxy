// +build linux

package glog

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	colorCyan   = []byte{'\x1b', '[', '3', '6', 'm'}
	colorRed    = []byte{'\x1b', '[', '3', '1', 'm'}
	colorYellow = []byte{'\x1b', '[', '3', '3', 'm'}
	colorReset  = []byte{'\x1b', '[', '0', 'm'}
)

func WriteFileWithColor(file *os.File, data []byte, s severity) {
	var colorPrefix []byte
	switch s {
	case fatalLog:
		colorPrefix = colorRed
	case errorLog:
		colorPrefix = colorRed
	case warningLog:
		colorPrefix = colorYellow
	case infoLog:
		colorPrefix = colorCyan
	}
	file.Write(colorPrefix)
	file.Write(data[:5])
	file.Write(colorReset)
	file.Write(data[5:])
}

func IsTerminal(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

func RedirectStderrTo(file *os.File) error {
	os.Stderr = file
	return syscall.Dup3(int(file.Fd()), 2, 0)
}
