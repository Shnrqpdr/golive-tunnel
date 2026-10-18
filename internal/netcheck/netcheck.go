// Package netcheck proves that the path this process gets to the internet reaches Discord and
// exits outside the refused countries.
package netcheck

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	discordURL = "https://discord.com/api/v9/gateway"

	// Timeout is how long a single check may take before it counts as failed.
	Timeout = 6 * time.Second

	maxResponseBytes = 64 * 1024
)

// A fresh connection per request, never through an HTTP(S)_PROXY from the environment: what is
// being tested is the route WireSock gives this process, and a reused connection could be one
// opened before the tunnel came up.
func client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:             nil,
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

func get(url string, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "*/*")
	return client(timeout).Do(req)
}

// Probe goes all the way to Discord's 200 answer: a path that only completes the TCP handshake
// passes any simpler check and still leaves Discord without a connection.
func Probe(timeout time.Duration) (time.Duration, error) {
	started := time.Now()

	res, err := get(discordURL, timeout)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, maxResponseBytes))

	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("resposta inesperada do Discord: %s", res.Status)
	}
	return time.Since(started), nil
}

// geoSources are tried in order: a free geolocation service that is slow or rate limited must
// not fail a tunnel that already proved itself with Discord's 200.
var geoSources = []struct{ url, field string }{
	{"https://ifconfig.co/json", "country_iso"},
	{"https://api.country.is", "country"},
	{"https://ipinfo.io/json", "country"},
}

// ExitCountry returns the ISO code of the country this process appears to come from.
func ExitCountry(timeout time.Duration) (string, error) {
	var errs []error
	for _, source := range geoSources {
		country, err := countryFrom(source.url, source.field, timeout)
		if err == nil {
			return country, nil
		}
		errs = append(errs, err)
	}
	return "", errors.Join(errs...)
}

func countryFrom(url, field string, timeout time.Duration) (string, error) {
	res, err := get(url, timeout)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("o servico de geolocalizacao respondeu %s", res.Status)
	}
	return countryFromJSON(io.LimitReader(res.Body, maxResponseBytes), field)
}

func countryFromJSON(r io.Reader, field string) (string, error) {
	var payload map[string]any
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return "", errors.New("nao achei o pais na resposta do servico de geolocalizacao")
	}
	code, _ := payload[field].(string)
	if code = strings.ToUpper(code); !isCountryCode(code) {
		return "", errors.New("nao achei o pais na resposta do servico de geolocalizacao")
	}
	return code, nil
}

// Accepts returns the exit country, with an error when it is one of the refused ones.
func Accepts(excluded map[string]bool, timeout time.Duration) (string, error) {
	country, err := ExitCountry(timeout)
	if err != nil {
		return "", err
	}
	if excluded[country] {
		return country, fmt.Errorf("saida em %s, que esta na lista de recusados", country)
	}
	return country, nil
}

// ParseCountries reads a comma-separated list of ISO country codes, skipping anything else.
func ParseCountries(raw string) map[string]bool {
	codes := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if code := strings.ToUpper(strings.TrimSpace(part)); isCountryCode(code) {
			codes[code] = true
		}
	}
	return codes
}

func isCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
