// Package process checks, stops and starts Windows processes by image name.
package process

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Running reports whether any process with this image name (for example "Discord.exe") is alive.
func Running(image string) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, fmt.Errorf("checagem de processo so implementada no Windows")
	}

	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image, "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(image)), nil
}

// Kill force-stops every process with this image name, children included.
func Kill(image string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("encerrar processo so implementado no Windows")
	}

	out, err := exec.Command("taskkill", "/IM", image, "/F", "/T").CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill falhou: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Start launches binary from its own folder and returns its pid without waiting for it.
func Start(binary string, args []string) (int, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = filepath.Dir(binary)

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	pid := cmd.Process.Pid
	// Release the handle: the launched program keeps running after this one exits.
	if err := cmd.Process.Release(); err != nil {
		return pid, err
	}
	return pid, nil
}
