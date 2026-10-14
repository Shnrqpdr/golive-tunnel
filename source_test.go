package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseExcluded(t *testing.T) {
	got := ParseExcluded(" br , AR, x, 123, ")
	want := map[string]bool{"BR": true, "AR": true}

	if len(got) != len(want) {
		t.Fatalf("ParseExcluded = %v, queria %v", got, want)
	}
	for code := range want {
		if !got[code] {
			t.Errorf("faltou %s em %v", code, got)
		}
	}

	if empty := ParseExcluded(""); len(empty) != 0 {
		t.Errorf("ParseExcluded(\"\") = %v, queria vazio", empty)
	}
}

// Um descartado por filtro, e os dois que sobram na ordem em que serao testados.
func TestRankFreeProxies(t *testing.T) {
	body := `{"proxies":[
		{"proxy":"socks5://1.1.1.1:1080","alive":true,"uptime":99,"timeout":300,"ip_data":{"countryCode":"DE"}},
		{"proxy":"socks5://2.2.2.2:1080","alive":true,"uptime":95,"timeout":200,"ip_data":{"countryCode":"BR"}},
		{"proxy":"socks5://3.3.3.3:4145","alive":true,"uptime":99,"timeout":100,"ip_data":{"countryCode":"US"}},
		{"proxy":"socks5://4.4.4.4:1080","alive":false,"uptime":99,"timeout":100,"ip_data":{"countryCode":"US"}},
		{"proxy":"socks5://5.5.5.5:1080","alive":true,"uptime":80,"timeout":100,"ip_data":{"countryCode":"US"}},
		{"proxy":"socks5://6.6.6.6:1080","alive":true,"uptime":99,"timeout":9000,"ip_data":{"countryCode":"US"}},
		{"proxy":"socks5://7.7.7.7:1080","alive":true,"uptime":100,"timeout":800,"ip_data":{"countryCode":"NL"}}
	]}`

	ranked, err := rankFreeProxies(body, map[string]bool{"BR": true})
	if err != nil {
		t.Fatalf("rankFreeProxies: %v", err)
	}

	want := []string{"socks5://7.7.7.7:1080", "socks5://1.1.1.1:1080"}
	if len(ranked) != len(want) {
		t.Fatalf("sobraram %d candidatas (%v), queria %v", len(ranked), ranked, want)
	}
	for i, endpoint := range ranked {
		if endpoint.String() != want[i] {
			t.Errorf("posicao %d = %s, queria %s", i, endpoint, want[i])
		}
	}
}

func TestRankFreeProxiesBadJSON(t *testing.T) {
	if _, err := rankFreeProxies("<html>nao sou json</html>", nil); err == nil {
		t.Error("queria erro, veio lista")
	}
}

func TestRankProbed(t *testing.T) {
	pool := []probeResult{
		{Endpoint{"socks5", "1.1.1.1", 1080}, 2000 * time.Millisecond},
		{Endpoint{"socks5", "2.2.2.2", 1080}, 300 * time.Millisecond},
		{Endpoint{"socks5", "3.3.3.3", 1080}, 900 * time.Millisecond},
	}
	asked := map[string]bool{"socks5://2.2.2.2:1080": true}

	got := rankProbed(pool, preferredLatency, asked)
	if len(got) != 1 || got[0].endpoint.Host != "3.3.3.3" {
		t.Fatalf("sob o limite preferido = %v, queria so a de 900 ms", got)
	}

	got = rankProbed(pool, probeTimeout, asked)
	want := []string{"3.3.3.3", "1.1.1.1"}
	if len(got) != len(want) {
		t.Fatalf("sem limite apertado sobraram %d, queria %v", len(got), want)
	}
	for i, result := range got {
		if result.endpoint.Host != want[i] {
			t.Errorf("posicao %d = %s, queria %s", i, result.endpoint.Host, want[i])
		}
	}
}

// O cache vive em os.UserCacheDir, que cada sistema tira de uma variavel diferente.
func isolateCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestCachedProxySource(t *testing.T) {
	isolateCache(t)

	saved := choice{Endpoint{"socks5", "127.0.0.1", 1080}, time.Second, "US", sourceManual}
	if err := StoreCachedProxy(saved); err != nil {
		t.Fatalf("StoreCachedProxy: %v", err)
	}

	endpoint, source, ok := ReadCachedProxy()
	if !ok || endpoint != saved.endpoint || source != sourceManual {
		t.Errorf("ReadCachedProxy = %v, %q, %v; queria %v, %q, true", endpoint, source, ok, saved.endpoint, sourceManual)
	}
}

// Cache de antes da origem existir nao pode virar passe livre para uma proxy publica.
func TestCachedProxySemOrigem(t *testing.T) {
	isolateCache(t)

	path, err := cachePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := fmt.Sprintf(`{"proxy":"socks5://1.2.3.4:1080","at":%d}`, time.Now().UnixMilli())
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, source, ok := ReadCachedProxy(); !ok || source != sourcePublic {
		t.Fatalf("cache sem origem leu como %q (ok=%v), queria %q", source, ok, sourcePublic)
	}

	// Sem -public a recusa vem antes de qualquer teste de rede.
	if _, ok := ReuseCachedProxy(map[string]bool{"BR": true}, false, NewUI()); ok {
		t.Error("reaproveitou uma proxy publica sem -public")
	}
}

func TestIsLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1":             true,
		"127.0.0.2":             true,
		"localhost":             true,
		"1.2.3.4":               false,
		"localhost.exemplo.com": false,
	} {
		if got := IsLoopback(host); got != want {
			t.Errorf("IsLoopback(%q) = %v, queria %v", host, got, want)
		}
	}
}
