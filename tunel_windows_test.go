//go:build windows

package main

import (
	"testing"
	"unsafe"
)

// Tamanho errado aqui faz a chamada ao Windows falhar ou ler lixo sem erro aparente.
func TestEstruturasDoWindows(t *testing.T) {
	ptr := unsafe.Sizeof(uintptr(0))
	cases := []struct {
		nome     string
		got      uintptr
		x64, x86 uintptr
	}{
		{"SHELLEXECUTEINFOW", unsafe.Sizeof(shellExecuteInfo{}), 112, 60},
		{"SERVICE_STATUS", unsafe.Sizeof(serviceStatus{}), 28, 28},
		{"QUERY_SERVICE_CONFIGW", unsafe.Sizeof(serviceConfig{}), 64, 36},
	}
	for _, c := range cases {
		want := c.x64
		if ptr == 4 {
			want = c.x86
		}
		if c.got != want {
			t.Errorf("%s com %d bytes, o Windows espera %d", c.nome, c.got, want)
		}
	}
}
