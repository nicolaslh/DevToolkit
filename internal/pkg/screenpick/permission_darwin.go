//go:build darwin

package screenpick

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

// hasScreenAccess reports whether the app already holds Screen Recording
// permission. Without it, macOS silently captures only the wallpaper and the
// app's own windows (other apps' windows are excluded).
func hasScreenAccess() bool {
	return bool(C.CGPreflightScreenCaptureAccess())
}

// requestScreenAccess triggers the system permission prompt and registers the
// app in System Settings › Privacy & Security › Screen Recording. The grant
// only takes effect after the app is restarted.
func requestScreenAccess() bool {
	return bool(C.CGRequestScreenCaptureAccess())
}
