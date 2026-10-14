package discord

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewerThan(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"bigger last field", "app-1.0.9184", "app-1.0.998", true},
		{"smaller last field", "app-1.0.998", "app-1.0.9184", false},
		{"same version", "app-1.0.9253", "app-1.0.9253", false},
		{"extra field breaks the tie", "app-1.0.9253.1", "app-1.0.9253", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := newerThan(versionOf(c.a), versionOf(c.b)); got != c.want {
				t.Errorf("newerThan(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestVersionOf(t *testing.T) {
	for _, name := range []string{"app-1.0.beta", "app-", "Update.exe"} {
		if got := versionOf(name); got != nil {
			t.Errorf("versionOf(%q) = %v, want nil", name, got)
		}
	}
}

func TestChannelForBinary(t *testing.T) {
	if got := ChannelForBinary(filepath.Join("Discord", "app-1.0.1", "DiscordPTB.exe")); got.Key != "ptb" {
		t.Errorf("channel of DiscordPTB.exe = %q, want ptb", got.Key)
	}
	if got := ChannelForBinary(filepath.Join("Other", "Vesktop.exe")); got.Key != "custom" {
		t.Errorf("channel of Vesktop.exe = %q, want custom", got.Key)
	}
}

func TestLocate(t *testing.T) {
	channel := channels["stable"]

	t.Run("prefers the complete one even when older", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("LOCALAPPDATA", root)

		complete := filepath.Join(root, channel.Folder, "app-1.0.998")
		install(t, complete, channel.Binary, true)
		install(t, filepath.Join(root, channel.Folder, "app-1.0.9184"), channel.Binary, false)

		binary, err := Locate(channel)
		if err != nil {
			t.Fatalf("Locate: %v", err)
		}
		if want := filepath.Join(complete, channel.Binary); binary != want {
			t.Errorf("picked %s, want %s", binary, want)
		}
	})

	t.Run("refuses when none is complete", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("LOCALAPPDATA", root)

		install(t, filepath.Join(root, channel.Folder, "app-1.0.9184"), channel.Binary, false)

		if binary, err := Locate(channel); err == nil {
			t.Errorf("want error, got %s", binary)
		}
	})
}

// An incomplete install has the executable and lacks the Chromium files.
func install(t *testing.T, dir, binary string, complete bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	names := []string{binary}
	if complete {
		names = append(names, runtimeFiles...)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
