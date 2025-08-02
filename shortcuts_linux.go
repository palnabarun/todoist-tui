//go:build linux

package main

// getDeleteShortcutText returns the Linux-specific delete shortcut text
func getDeleteShortcutText() string {
	return "Win+Backspace: delete"
}