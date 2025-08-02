//go:build windows

package main

// getDeleteShortcutText returns the Windows-specific delete shortcut text
func getDeleteShortcutText() string {
	return "Win+Backspace: delete"
}