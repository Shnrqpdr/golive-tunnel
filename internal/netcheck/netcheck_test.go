package netcheck

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestParseCountries(t *testing.T) {
	got := ParseCountries(" br , AR, x, 123, ")
	want := map[string]bool{"BR": true, "AR": true}

	if len(got) != len(want) {
		t.Fatalf("ParseCountries = %v, want %v", got, want)
	}
	for code := range want {
		if !got[code] {
			t.Errorf("missing %s in %v", code, got)
		}
	}

	if empty := ParseCountries(""); len(empty) != 0 {
		t.Errorf("ParseCountries(\"\") = %v, want empty", empty)
	}
}

func TestCountryFromJSON(t *testing.T) {
	got, err := countryFromJSON(strings.NewReader(`{"ip":"203.0.113.10","country":"United States","country_iso":"us"}`))
	if err != nil || got != "US" {
		t.Errorf("countryFromJSON = %q, %v; want US", got, err)
	}

	for _, body := range []string{`{"ip":"203.0.113.10"}`, `{"country_iso":"USA"}`, `<html>`} {
		if got, err := countryFromJSON(strings.NewReader(body)); err == nil {
			t.Errorf("countryFromJSON(%s) = %q, want error", body, got)
		}
	}
}

type fakeTimeout struct{}

func (fakeTimeout) Error() string   { return "i/o timeout" }
func (fakeTimeout) Timeout() bool   { return true }
func (fakeTimeout) Temporary() bool { return true }

// The error arrives the way net/http builds it: url.Error over OpError over the system code.
func dialFailure(cause error) error {
	return &url.Error{
		Op:  "Get",
		URL: discordURL,
		Err: &net.OpError{
			Op:   "dial",
			Net:  "tcp",
			Addr: &net.TCPAddr{IP: net.IPv4(162, 159, 128, 233), Port: 443},
			Err:  os.NewSyscallError("connectex", cause),
		},
	}
}

func TestDescribeBlockedByMachine(t *testing.T) {
	for name, cause := range map[string]error{
		"windows": winSocketForbidden,
		"others":  syscall.EACCES,
	} {
		got := Describe(dialFailure(cause))
		if !strings.Contains(got, "antivirus ou firewall") {
			t.Errorf("%s: Describe = %q, want the antivirus hint", name, got)
		}
		if strings.Contains(got, "discord.com") {
			t.Errorf("%s: Describe = %q, still carries the raw URL", name, got)
		}
	}
}

func TestDescribeOtherCauses(t *testing.T) {
	dns := &url.Error{Op: "Get", URL: discordURL, Err: &net.DNSError{Err: "no such host", Name: "discord.com"}}
	if got := Describe(dns); !strings.Contains(got, "discord.com") || !strings.Contains(got, "DNS") {
		t.Errorf("Describe(dns) = %q, want DNS and the host", got)
	}

	if got := Describe(dialFailure(fakeTimeout{})); !strings.Contains(got, "demorou demais") {
		t.Errorf("Describe(timeout) = %q, want the delay hint", got)
	}

	if got := Describe(dialFailure(syscall.ECONNREFUSED)); !strings.Contains(got, "recusada do outro lado") {
		t.Errorf("Describe(refused) = %q, want the remote refusal hint", got)
	}

	// Without a known cause the inner message is left, already without the URL.
	plain := &url.Error{Op: "Get", URL: discordURL, Err: errors.New("HTTP 503")}
	if got := Describe(plain); got != "HTTP 503" {
		t.Errorf("Describe(generic) = %q", got)
	}

	if got := Describe(nil); got != "" {
		t.Errorf("Describe(nil) = %q, want empty", got)
	}
}
