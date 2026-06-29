//go:build darwin

package screenpick

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

// cursorPos returns the global mouse cursor position in display points.
func cursorPos() (int, int, error) {
	var src C.CGEventSourceRef
	e := C.CGEventCreate(src)
	p := C.CGEventGetLocation(e)
	if e != 0 {
		C.CFRelease(C.CFTypeRef(e))
	}
	return int(p.x), int(p.y), nil
}
