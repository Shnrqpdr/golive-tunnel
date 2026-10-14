package ui

import "os"

// IsTerminal reports whether f is an interactive console. Output redirected to a file or a pipe
// is not one: spinner and color would turn into garbage, so the UI falls back to plain lines.
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
