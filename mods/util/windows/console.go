//go:build windows

package windows

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type proc interface {
	Call(a ...uintptr) (r1, r2 uintptr, lastErr error)
}

var (
	kernel32                        = windows.NewLazySystemDLL("kernel32")
	getConsoleScreenBufferInfo proc = kernel32.NewProc("GetConsoleScreenBufferInfo")
	setConsoleScreenBufferSize proc = kernel32.NewProc("SetConsoleScreenBufferSize")
)

type Size struct {
	Width  int
	Height int
}

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximunWindowSize coord
}

func GetTerminalSize(fp *os.File) (s Size, err error) {
	csbi := consoleScreenBufferInfo{}
	ret, _, err := getConsoleScreenBufferInfo.Call(uintptr(windows.Handle(fp.Fd())), uintptr(unsafe.Pointer(&csbi)))
	if ret == 0 {
		return
	}
	err = nil
	s = Size{
		Width:  int(csbi.size.x),
		Height: int(csbi.size.y),
	}
	return
}

func SetTerminalSize(fp *os.File, s Size) error {
	c := coord{x: int16(s.Width), y: int16(s.Height)}
	ret, _, err := setConsoleScreenBufferSize.Call(uintptr(windows.Handle(fp.Fd())), uintptr(unsafe.Pointer(&c)))
	if ret == 0 {
		return err
	}
	return nil
}
