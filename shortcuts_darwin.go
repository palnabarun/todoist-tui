//go:build darwin

package main

// getDeleteShortcutText returns the macOS-specific delete shortcut text
func getDeleteShortcutText() string {
	return "Option+Backspace: delete"
}