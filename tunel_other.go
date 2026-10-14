//go:build !windows

package main

import "errors"

// O túnel por programa depende do WireSock, que só existe no Windows. Fora dele o servidor
// embutido não tem como ser usado.
var errSoWindows = errors.New("o tunel WireGuard por programa so funciona no Windows")

func prepararWireSock(caminhoChave, config string, ui *UI) error { return errSoWindows }

func PrepararWireSockElevado(caminhoChave, sid string) error { return errSoWindows }

func ligarServico() error { return errSoWindows }

func desligarServico() {}

func servicoLigado() bool { return false }

func isElevated() bool { return false }

func copiarTexto(texto string) error {
	return errors.New("area de transferencia so implementada no Windows")
}
