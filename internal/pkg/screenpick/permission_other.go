//go:build !darwin

package screenpick

// Non-macOS platforms do not gate screen capture behind a runtime permission.
func hasScreenAccess() bool { return true }

func requestScreenAccess() bool { return true }
