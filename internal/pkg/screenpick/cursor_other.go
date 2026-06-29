//go:build !darwin && !windows

package screenpick

import "github.com/nic/devtoolkit/internal/pkg/apperr"

// cursorPos is unsupported on this platform.
func cursorPos() (int, int, error) {
	return 0, 0, apperr.New(apperr.Unsupported, "当前平台暂不支持屏幕取色")
}
