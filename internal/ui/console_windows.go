//go:build windows

package ui

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

// OwnsConsole reports whether the console window belongs to this process alone. A double click
// in Explorer creates a console just for it; from cmd or a script the console is shared. Only in
// the first case the window disappears at the end, and only then can it be minimized without
// touching anyone else's terminal.
func OwnsConsole() bool {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}

func MinimizeConsole() {
	if hwnd, _, _ := procGetConsoleWindow.Call(); hwnd != 0 {
		procShowWindow.Call(hwnd, swMinimize)
	}
}

// The Windows console starts without ANSI and on an old code page. Both have to be turned on by
// hand, and whatever fails only downgrades the looks: without ANSI the output falls back to
// plain lines, without UTF-8 to ASCII.
func prepareConsole(f *os.File) (ansi bool, wide bool) {
	if !IsTerminal(f) {
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
