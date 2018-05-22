// +build !darwin
// +build !dragonfly
// +build !freebsd
// +build !linux
// +build !netbsd
// +build !openbsd
// +build !windows

package glog

import (
	"errors"
	"os"
)

func WriteFileWithColor(file *os.File, data []byte, s severity) {
	file.Write(data)
}

func IsTerminal(fd uintptr) bool {
	return false
}

func RedirectStderrTo(file *os.File) error {
	return errors.New("not implemented")
}
