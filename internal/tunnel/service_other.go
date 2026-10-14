//go:build !windows

package tunnel

import (
	"errors"

	"github.com/Shnrqpdr/golive-tunnel/internal/ui"
)

// The per-app tunnel depends on WireSock, which only exists on Windows.
var errWindowsOnly = errors.New("o tunel WireGuard por programa so funciona no Windows")

func ensureWireSock(keyPath, config string, u *ui.UI) error { return errWindowsOnly }

func SetupElevated(cfg Config, keyPath, sid string) error { return errWindowsOnly }

func startService() error { return errWindowsOnly }

func stopService() {}

func serviceRunning() bool { return false }

func IsElevated() bool { return false }
