package tunnel

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeyPrefix starts the line a friend sends to whoever runs the server, so it is clear what it is.
const KeyPrefix = "golive-wg"

// dataDir is %LOCALAPPDATA%\golive-tunnel. The private key lives there and never leaves it:
// the server only gets the public one. WireSock reads a copy made by the administrator step,
// in a protected folder.
func dataDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "golive-tunnel"), nil
}

func keyPaths() (path, legacy string, err error) {
	dir, err := dataDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "wireguard.key"), legacyKeyPath(filepath.Dir(dir)), nil
}

// Builds from before the rename to golive-tunnel kept the key here.
func legacyKeyPath(base string) string {
	return filepath.Join(base, "DiscordGoLiveBypass", "wireguard", "privada")
}

func publicKey(private []byte) []byte {
	key, err := ecdh.X25519().NewPrivateKey(private)
	if err != nil {
		return nil
	}
	return key.PublicKey().Bytes()
}

func readPrivateKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	private, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(private) != 32 || publicKey(private) == nil {
		return nil, fmt.Errorf("a chave em %s esta corrompida: apague o arquivo e gere outra", path)
	}
	return private, nil
}

// ensureKey loads the key, brings it over from the legacy path, or creates it. A corrupted key
// is an error and never silently becomes a new one: a new key means asking for access again.
func ensureKey(path, legacy string) (private []byte, created bool, err error) {
	private, err = readPrivateKey(path)
	if err == nil {
		return private, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}

	if _, err := os.Stat(legacy); err == nil {
		if err := moveLegacyKey(legacy, path); err != nil {
			return nil, false, fmt.Errorf("nao consegui trazer a chave da versao anterior, em %s: %w", legacy, err)
		}
		private, err = readPrivateKey(path)
		return private, false, err
	}

	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key.Bytes())), 0o600); err != nil {
		return nil, false, err
	}
	return key.Bytes(), true, nil
}

// Moving the key keeps the address the server already knows.
func moveLegacyKey(legacy, path string) error {
	if _, err := readPrivateKey(legacy); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Rename(legacy, path); err != nil {
		return err
	}

	// Leftovers of the old layout. os.Remove only takes empty folders, so anything else that
	// lives there, like the original DiscordGoLiveBypass cache, stays.
	dir := filepath.Dir(legacy)
	os.Remove(filepath.Join(dir, "preparo.txt"))
	os.Remove(dir)
	os.Remove(filepath.Dir(dir))
	return nil
}

// ShareLine is what the friend sends to be let in: the prefix plus the public key.
func ShareLine(private []byte) string {
	return KeyPrefix + " " + base64.StdEncoding.EncodeToString(publicKey(private))
}
