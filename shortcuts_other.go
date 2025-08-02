//go:build !darwin && !windows && !linux

package main

// getDeleteShortcutText returns the default delete shortcut text for other operating systems
func getDeleteShortcutText() string {
	return "Alt+Backspace: delete"
}