//go:build windows

package ui

import (
	"os/exec"
	"strings"
	"syscall"
)

// Copy puts text on the clipboard, through clip.exe without flashing a window.
func Copy(text string) error {
	cmd := exec.Command("clip")
	cmd.Stdin = strings.NewReader(text)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
