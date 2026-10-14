//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	user32                    = syscall.NewLazyDLL("user32.dll")
	procGetConsoleMode        = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode        = kernel32.NewProc("SetConsoleMode")
	procSetConsoleOutputCP    = kernel32.NewProc("SetConsoleOutputCP")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procShowWindow            = user32.NewProc("ShowWindow")
)

const (
	enableVirtualTerminal = 0x0004
	codePageUTF8          = 65001
	swMinimize            = 6
)

// Duplo clique no Explorer cria um console só para este processo; pelo cmd ou por um script ele é
// compartilhado. Só no primeiro caso a janela some no fim e só nele dá para minimizá-la sem
// mexer no terminal de ninguém.
func ownsConsole() bool {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}

func minimizeConsole() {
	if hwnd, _, _ := procGetConsoleWindow.Call(); hwnd != 0 {
		procShowWindow.Call(hwnd, swMinimize)
	}
}

// O console do Windows nasce sem interpretar ANSI e numa codepage antiga. Os dois
// precisam ser ligados na mao, e o que falhar apenas rebaixa a aparencia: quem nao
// consegue ANSI cai para linhas simples, quem nao consegue UTF-8 cai para ASCII.
func prepareConsole(f *os.File) (ansi bool, wide bool) {
	if !charDevice(f) {
		return false, false
	}

	handle := syscall.Handle(f.Fd())
	var mode uint32
	if ok, _, _ := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return false, false
	}

	if mode&enableVirtualTerminal == 0 {
		if ok, _, _ := procSetConsoleMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminal)); ok == 0 {
			return false, false
		}
	}

	ok, _, _ := procSetConsoleOutputCP.Call(codePageUTF8)
	return true, ok != 0
}
