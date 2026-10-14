package tunnel

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Shnrqpdr/golive-tunnel/internal/netcheck"
	"github.com/Shnrqpdr/golive-tunnel/internal/ui"
)

const (
	serviceRetry     = 5 * time.Second
	appCheck         = 10 * time.Second
	firstTrafficWait = 25 * time.Second
)

// SetupFlag is the hidden flag the executable reopens itself with, as administrator, to run
// SetupElevated.
const SetupFlag = "setup-wiresock"

// ErrNewKey stops the first run right after the key is created: the server does not know it
// yet, and trying to connect now would give an error that teaches nothing to someone who just
// downloaded the program.
var ErrNewKey = errors.New("chave nova, falta a liberacao no servidor")

type Tunnel struct {
	cfg  Config
	line string
	// When the service was already running, it belongs to another instance watching an open
	// Discord: this one may only stop what it started, until Watch takes ownership.
	owned atomic.Bool
}

// Exit is what Validate found on the other side of the tunnel.
type Exit struct {
	Latency time.Duration
	Country string
}

// Open prepares the key, WireSock and the service, and starts the tunnel. The first time it
// stops earlier, with ErrNewKey, after showing the line that has to be let in on the server.
func Open(cfg Config, u *ui.UI) (*Tunnel, error) {
	path, legacy, err := keyPaths()
	if err != nil {
		return nil, err
	}

	private, created, err := ensureKey(path, legacy)
	if err != nil {
		return nil, err
	}
	t := &Tunnel{cfg: cfg, line: ShareLine(private)}
	if created {
		showKeyLine(u, "Primeira vez neste computador", t.line)
		return nil, ErrNewKey
	}

	if err := ensureWireSock(path, wireSockConfig(cfg, private, tunneledApps()), u); err != nil {
		return nil, err
	}

	u.Busy("ligando o tunel ate %s", cfg.Host())
	t.owned.Store(!serviceRunning())
	if err := startService(); err != nil {
		return nil, err
	}
	u.Ok("tunel WireGuard ligado ate %s", cfg.Host())
	return t, nil
}

func showKeyLine(u *ui.UI, title, line string) {
	lines := []string{
		"Mande a linha abaixo para quem cuida do servidor. Nao e senha.",
		"",
		line,
		"",
	}
	if ui.Copy(line) == nil {
		lines = append(lines, "Ela ja esta copiada: e so colar na conversa.")
	}
	lines = append(lines, "Quando liberarem, abra o programa de novo.")
	u.Notice(title, lines)
}

// Validate proves the tunnel works. WireGuard does not say when the server does not know the
// key: packets just get no answer. So the proof is Discord answering through the tunnel, and
// the exit country being outside the refused ones.
func (t *Tunnel) Validate(excluded map[string]bool, u *ui.UI) (Exit, error) {
	u.Busy("testando o Discord pelo tunel")
	deadline := time.Now().Add(firstTrafficWait)
	var latency time.Duration
	var err error
	for {
		if latency, err = netcheck.Probe(netcheck.Timeout); err == nil {
			break
		}
		if time.Now().After(deadline) {
			ui.Copy(t.line)
			return Exit{}, fmt.Errorf("o tunel ligou, mas nada passou por ele em %s. Se voce acabou de configurar, confirme se quem cuida do servidor ja liberou esta linha (ela ja esta copiada): %s", firstTrafficWait, t.line)
		}
		time.Sleep(time.Second)
	}

	country, err := netcheck.Accepts(excluded, netcheck.Timeout)
	if err != nil {
		return Exit{}, fmt.Errorf("o tunel funciona, mas a saida nao serve: %w", err)
	}

	u.Ok("o Discord respondeu em %d ms pelo tunel, saida em %s", latency.Milliseconds(), ui.CountryLabel(country))
	return Exit{Latency: latency, Country: country}, nil
}

// Close stops the tunnel if this instance owns it. Safe to call more than once and from the
// signal handler: stopping a tunnel that was already up would cut off whoever is using it now.
func (t *Tunnel) Close() {
	if t.owned.Load() {
		stopService()
	}
}

// Watch keeps the tunnel alive while the app is open: if the service stops it starts it again,
// and when the app closes the tunnel goes down with it. From here on this instance owns the
// tunnel, because the Discord it launched depends on it.
func (t *Tunnel) Watch(name string, alive func() (bool, error), u *ui.UI) error {
	t.owned.Store(true)

	u.Info("deixe esta janela aberta: ela mantem o tunel e fecha sozinha junto com o %s", name)
	if ui.OwnsConsole() {
		time.Sleep(2 * time.Second)
		ui.MinimizeConsole()
	}

	missing := 0
	lastCheck := time.Now()
	for {
		time.Sleep(serviceRetry)

		// Two checks in a row without the app: Discord's updater closes and reopens the
		// process, and stopping the tunnel in between would send the new version out through
		// Brazil.
		if time.Since(lastCheck) >= appCheck {
			lastCheck = time.Now()
			running, err := alive()
			switch {
			case err != nil:
			case running:
				missing = 0
			default:
				missing++
			}
			if missing >= 2 {
				t.Close()
				u.Ok("%s fechado, tunel desligado", name)
				return nil
			}
		}

		if serviceRunning() {
			continue
		}
		u.Warn("o tunel parou, ligando de novo")
		if err := startService(); err != nil {
			u.Detail("%v", err)
			continue
		}
		u.Ok("tunel de volta")
	}
}
