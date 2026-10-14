package tunnel

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Same result as server/golive-peer for this sample key: if the two diverge, the server lets in
// one address and the program uses another, and nothing flows.
func TestAddressFor(t *testing.T) {
	public, err := base64.StdEncoding.DecodeString("iGzzH4MBfPTTXC5oQScRNZLXI7FOkqkCzDafbSaQrcI=")
	if err != nil {
		t.Fatal(err)
	}
	if got := AddressFor(public); got != "10.66.125.59" {
		t.Errorf("AddressFor = %s, golive-peer computes 10.66.125.59", got)
	}
}

func TestConfigHost(t *testing.T) {
	cfg := Config{Endpoint: "203.0.113.10:51820", PublicKey: "x"}
	if got := cfg.Host(); got != "203.0.113.10" {
		t.Errorf("Host = %q", got)
	}
	if !cfg.Valid() || (Config{Endpoint: "203.0.113.10:51820"}).Valid() {
		t.Error("Valid should require both endpoint and key")
	}
}

func TestWireSockConfig(t *testing.T) {
	cfg := Config{Endpoint: "203.0.113.10:51820", PublicKey: "MrAXj9io3KrzWoF5LayHn62ca3yy3RgwKwsNEgtuJGI="}
	private := bytes.Repeat([]byte{7}, 32)
	apps := []string{"Discord", "Discord.exe", "Update.exe"}
	config := wireSockConfig(cfg, private, apps)

	for _, want := range []string{
		"PrivateKey = " + base64.StdEncoding.EncodeToString(private) + "\r\n",
		"Address = " + AddressFor(publicKey(private)) + "/32\r\n",
		"PublicKey = MrAXj9io3KrzWoF5LayHn62ca3yy3RgwKwsNEgtuJGI=\r\n",
		"Endpoint = 203.0.113.10:51820\r\n",
		"AllowedIPs = 0.0.0.0/0, ::/0\r\n",
		"#@ws:AllowedApps = Discord, Discord.exe, Update.exe\r\n",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("missing %q in:\n%s", want, config)
		}
	}
	// The service runs as SYSTEM: no directive that executes anything may go in.
	for _, forbidden := range []string{"PostUp", "PreUp", "PostDown", "PreDown", "DNS"} {
		if strings.Contains(config, forbidden) {
			t.Errorf("the configuration should not have %s:\n%s", forbidden, config)
		}
	}
	if fingerprint(config) != fingerprint(wireSockConfig(cfg, private, apps)) {
		t.Error("the same input should give the same fingerprint")
	}
}

func TestEnsureKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "golive-tunnel", "wireguard.key")
	legacy := legacyKeyPath(dir)

	first, created, err := ensureKey(path, legacy)
	if err != nil || !created || len(first) != 32 {
		t.Fatalf("first ensureKey = %d bytes, created=%v, err=%v", len(first), created, err)
	}
	second, created, err := ensureKey(path, legacy)
	if err != nil || created || !bytes.Equal(first, second) {
		t.Fatalf("second time the key should be the same and not new (created=%v, err=%v)", created, err)
	}
	if line := ShareLine(first); !strings.HasPrefix(line, KeyPrefix+" ") || len(line) != len(KeyPrefix)+1+44 {
		t.Errorf("share line out of format: %q", line)
	}

	if err := os.WriteFile(path, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ensureKey(path, legacy); err == nil {
		t.Error("a corrupted key should be an error, not silently become a new key")
	}
}

// A friend who already has access keeps the same key, and so the same address, after the rename.
func TestEnsureKeyMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "golive-tunnel", "wireguard.key")
	legacy := legacyKeyPath(dir)

	old := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(legacy), "preparo.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// Something that is not ours in the old folder must survive.
	other := filepath.Join(dir, "DiscordGoLiveBypass", "proxy.json")
	if err := os.WriteFile(other, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	private, created, err := ensureKey(path, legacy)
	if err != nil || created {
		t.Fatalf("ensureKey with a legacy key: created=%v, err=%v", created, err)
	}
	if got := base64.StdEncoding.EncodeToString(private); got != old {
		t.Errorf("migrated key = %s, want the legacy one", got)
	}
	if _, err := os.Stat(filepath.Dir(legacy)); !os.IsNotExist(err) {
		t.Errorf("the empty legacy wireguard folder should be gone: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a file that is not ours was removed: %v", err)
	}
}

func TestEnsureKeyRefusesCorruptLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := legacyKeyPath(dir)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ensureKey(filepath.Join(dir, "golive-tunnel", "wireguard.key"), legacy); err == nil {
		t.Error("a corrupted legacy key should be an error, not silently become a new key")
	}
}

func TestGrantStartStop(t *testing.T) {
	sid := "S-1-5-21-1-2-3-1001"
	ace := "(A;;LCRPWP;;;" + sid + ")"

	// The way Windows returns it after writing: same entry, rights in another order.
	if got := grantStartStop("D:(A;;RPWPLC;;;"+sid+")S:(AU;FA;GA;;;WD)", sid); strings.Count(got, sid) != 1 {
		t.Errorf("an entry with the rights in another order should count: %s", got)
	}
	// An entry for the same user without the right to stop is not enough.
	if got := grantStartStop("D:(A;;LCRP;;;"+sid+")", sid); !strings.Contains(got, ace) {
		t.Errorf("without WP the full grant should be added: %s", got)
	}

	withAudit := "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCLCSWLOCRRC;;;IU)S:(AU;FA;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;WD)"
	got := grantStartStop(withAudit, sid)
	if !strings.Contains(got, "(A;;CCLCSWLOCRRC;;;IU)"+ace+"S:(AU;") {
		t.Errorf("the grant should go in the DACL, before S:: %s", got)
	}
	if again := grantStartStop(got, sid); again != got {
		t.Errorf("running again should not repeat the grant: %s", again)
	}

	if got := grantStartStop("D:(A;;CCLCSWLOCRRC;;;IU)", sid); got != "D:(A;;CCLCSWLOCRRC;;;IU)"+ace {
		t.Errorf("without S: the grant goes at the end: %s", got)
	}

	for _, sid := range []string{"S-1-5-21-1-2-3-1001", "S-1-5-18"} {
		if !sidPattern.MatchString(sid) {
			t.Errorf("valid SID refused: %s", sid)
		}
	}
	for _, sid := range []string{"", "S-1-5-21)(A;;GA;;;WD", "WD", "S-1-5-21-1 -2"} {
		if sidPattern.MatchString(sid) {
			t.Errorf("invalid SID accepted: %q", sid)
		}
	}
}
