//go:build windows

package screenpick

import (
	"syscall"
	"unsafe"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
)

type winPoint struct {
	X int32
	Y int32
}

// cursorPos returns the global mouse cursor position in screen pixels.
func cursorPos() (int, int, error) {
	var p winPoint
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if r == 0 {
		return 0, 0, apperr.New(apperr.Unsupported, "无法获取鼠标位置")
	}
	return int(p.X), int(p.Y), nil
}
