//go:build !windows

package ui

import "os"

// Outside Windows the terminal arrives ready: only TERM=dumb needs care.
func prepareConsole(f *os.File) (ansi bool, wide bool) {
	if !IsTerminal(f) || os.Getenv("TERM") == "dumb" {
		return false, false
	}
	return true, true
}

// OwnsConsole is always false outside Windows: the terminal is never the program's own, so it
// neither disappears at the end nor should be minimized.
func OwnsConsole() bool { return false }

func MinimizeConsole() {}
