//go:build windows

package tunnel

import (
	"testing"
	"unsafe"
)

// A wrong size here makes the Windows call fail, or read garbage with no visible error.
func TestWindowsStructSizes(t *testing.T) {
	ptr := unsafe.Sizeof(uintptr(0))
	cases := []struct {
		name     string
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
			t.Errorf("%s has %d bytes, Windows expects %d", c.name, c.got, want)
		}
	}
}
