// Package tunnel puts Discord, and only Discord, inside a WireGuard tunnel to the server built
// into the executable, through the WireSock client service on Windows.
//
// A TCP-only tunnel is not enough: Discord's media server looks at the IP the UDP comes from,
// and with it in Brazil the Go Live video does not flow even with the gateway going out abroad.
package tunnel

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the server baked into the executable at build time.
type Config struct {
	Endpoint  string // host:port of the server's WireGuard
	PublicKey string // the server's public key, in base64
}

func (c Config) Valid() bool {
	return c.Endpoint != "" && c.PublicKey != ""
}

// Host is the endpoint without the port, for messages.
func (c Config) Host() string {
	if i := strings.LastIndex(c.Endpoint, ":"); i > 0 {
		return c.Endpoint[:i]
	}
	return c.Endpoint
}

// AddressFor derives the tunnel address from the public key, with the same math as golive-peer
// on the server: nobody has to agree on an IP. It stays in 10.66.1-254.1-254, away from the
// server's 10.66.0.1.
func AddressFor(public []byte) string {
	h := sha256.Sum256(public)
	return fmt.Sprintf("10.66.%d.%d", int(h[0])%254+1, int(h[1])%254+1)
}

// Only these programs go into the tunnel; the rest of the computer stays on the normal network.
// This executable goes along to check the exit before opening Discord.
func tunneledApps() []string {
	apps := []string{"Discord", "Discord.exe", "Update.exe"}
	if exe, err := os.Executable(); err == nil {
		if name := filepath.Base(exe); !strings.ContainsAny(name, ",\r\n") {
			apps = append(apps, name)
		}
	}
	return apps
}

// Plain WireGuard plus the AllowedApps line, which WireSock reads from inside a comment so the
// file stays valid for any other client. Nothing that runs commands (PostUp and friends) ever
// goes in: the service loads this as SYSTEM.
func wireSockConfig(cfg Config, private []byte, apps []string) string {
	return fmt.Sprintf("[Interface]\r\n"+
		"PrivateKey = %s\r\n"+
		"Address = %s/32\r\n"+
		"\r\n"+
		"[Peer]\r\n"+
		"PublicKey = %s\r\n"+
		"AllowedIPs = 0.0.0.0/0, ::/0\r\n"+
		"Endpoint = %s\r\n"+
		"PersistentKeepalive = 25\r\n"+
		"#@ws:AllowedApps = %s\r\n",
		base64.StdEncoding.EncodeToString(private), AddressFor(publicKey(private)),
		cfg.PublicKey, cfg.Endpoint, strings.Join(apps, ", "))
}

// Fingerprint of the installed configuration. Comparing it avoids asking for administrator on
// every run: it only changes with the key, the server or the name of this executable.
func fingerprint(config string) string {
	h := sha256.Sum256([]byte(config))
	return hex.EncodeToString(h[:])
}
