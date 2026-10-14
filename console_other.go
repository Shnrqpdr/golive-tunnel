//go:build !windows

package main

import "os"

// Fora do Windows o terminal ja chega pronto: so o TERM=dumb precisa de cuidado.
func prepareConsole(f *os.File) (ansi bool, wide bool) {
	if !charDevice(f) || os.Getenv("TERM") == "dumb" {
		return false, false
	}
	return true, true
}

// Terminal fora do Windows nunca é do programa: não some no fim nem deve ser minimizado.
func ownsConsole() bool { return false }

func minimizeConsole() {}
