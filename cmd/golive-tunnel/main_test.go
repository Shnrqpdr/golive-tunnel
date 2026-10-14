package main

import (
	"testing"

	"github.com/Shnrqpdr/golive-tunnel/internal/tunnel"
)

// A plain go build embeds no server, and the program has to refuse to run instead of guessing.
func TestNoServerByDefault(t *testing.T) {
	if (tunnel.Config{Endpoint: server, PublicKey: serverKey}).Valid() {
		t.Error("a build without -ldflags should not have a server")
	}
}
