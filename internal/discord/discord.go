// Package discord finds the Discord installation and starts, checks and stops it.
package discord

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Shnrqpdr/golive-tunnel/internal/process"
)

// Channel is one Discord release channel: its folder under %LOCALAPPDATA% and its executable.
type Channel struct {
	Key      string
	Folder   string
	Binary   string
	Friendly string
}

var channels = map[string]Channel{
	"stable": {"stable", "Discord", "Discord.exe", "Discord"},
	"ptb":    {"ptb", "DiscordPTB", "DiscordPTB.exe", "Discord PTB"},
	"canary": {"canary", "DiscordCanary", "DiscordCanary.exe", "Discord Canary"},
}

var channelOrder = []string{"stable", "ptb", "canary"}

func LookupChannel(key string) (Channel, error) {
	channel, ok := channels[strings.ToLower(strings.TrimSpace(key))]
	if !ok {
		return Channel{}, fmt.Errorf("canal desconhecido %q, use auto, stable, ptb ou canary", key)
	}
	return channel, nil
}

// DetectChannel returns the first installed channel, in the order stable, ptb, canary.
func DetectChannel() (Channel, string, error) {
	var checked []string
	for _, key := range channelOrder {
		channel := channels[key]
		binary, err := Locate(channel)
		if err == nil {
			return channel, binary, nil
		}
		checked = append(checked, channel.Friendly)
	}

	return Channel{}, "", fmt.Errorf("nao achei nenhum Discord instalado. Procurei por %s dentro de %%LOCALAPPDATA%%. Se o seu esta em outro lugar, aponte com -exe", strings.Join(checked, ", "))
}

// ChannelForBinary names the channel of an executable passed by hand, or a "custom" one.
func ChannelForBinary(binary string) Channel {
	name := filepath.Base(binary)
	for _, key := range channelOrder {
		if strings.EqualFold(name, channels[key].Binary) {
			return channels[key]
		}
	}
	return Channel{Key: "custom", Binary: name, Friendly: name}
}

// Sorting the folders as text would put app-1.0.998 ahead of app-1.0.9184, and the launcher
// would open a version the updater already retired.
func versionOf(name string) []int {
	raw := strings.TrimPrefix(name, "app-")
	var parts []int
	for _, piece := range strings.Split(raw, ".") {
		number, err := strconv.Atoi(piece)
		if err != nil {
			return nil
		}
		parts = append(parts, number)
	}
	return parts
}

func newerThan(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return len(a) > len(b)
}

// Locate points to the Discord.exe inside the newest complete app-<version> folder, not to the
// Update.exe at the root, so the process started (and the pid reported) is Discord itself.
func Locate(channel Channel) (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", fmt.Errorf("LOCALAPPDATA nao esta definido, informe o caminho com -exe")
	}

	root := filepath.Join(local, channel.Folder)
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("nao achei a instalacao do %s em %s", channel.Friendly, root)
	}

	var versions [][]int
	byVersion := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "app-") {
			continue
		}

		binary := filepath.Join(root, entry.Name(), channel.Binary)
		if info, err := os.Stat(binary); err != nil || info.IsDir() {
			continue
		}

		version := versionOf(entry.Name())
		if version == nil {
			continue
		}

		versions = append(versions, version)
		byVersion[fmt.Sprint(version)] = binary
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("nenhum %s instalado em %s", channel.Binary, root)
	}

	sort.Slice(versions, func(i, j int) bool { return newerThan(versions[i], versions[j]) })
	for _, version := range versions {
		binary := byVersion[fmt.Sprint(version)]
		if complete(filepath.Dir(binary)) {
			return binary, nil
		}
	}

	return "", fmt.Errorf("achei %s em %s, mas nenhuma versao esta completa. Isso e update interrompido: abra o %s normalmente uma vez, deixe ele terminar de se atualizar e rode de novo", channel.Binary, root, channel.Friendly)
}

// An interrupted update leaves the new version folder with the executable but without the files
// Chromium needs to start. The process is born and dies on its own, so the newest folder is not
// good just for being the newest.
var runtimeFiles = []string{"icudtl.dat", "resources.pak", "locales"}

func complete(dir string) bool {
	for _, name := range runtimeFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// IsRunning matters because Discord is single instance: a second process only wakes the window
// of the first one, which keeps going out through the old route.
func IsRunning(channel Channel) (bool, error) {
	return process.Running(channel.Binary)
}

func Terminate(channel Channel) error {
	return process.Kill(channel.Binary)
}

func Launch(binary string, args []string) (int, error) {
	return process.Start(binary, args)
}
