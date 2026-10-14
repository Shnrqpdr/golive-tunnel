//go:build windows

package tunnel

import (
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	advapi32                 = syscall.NewLazyDLL("advapi32.dll")
	procOpenSCManagerW       = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW         = advapi32.NewProc("OpenServiceW")
	procStartServiceW        = advapi32.NewProc("StartServiceW")
	procControlService       = advapi32.NewProc("ControlService")
	procQueryServiceStatus   = advapi32.NewProc("QueryServiceStatus")
	procQueryServiceConfigW  = advapi32.NewProc("QueryServiceConfigW")
	procChangeServiceConfigW = advapi32.NewProc("ChangeServiceConfigW")
	procCloseServiceHandle   = advapi32.NewProc("CloseServiceHandle")
)

const (
	serviceName = "wiresock-client-service"

	scManagerConnect   = 0x0001
	scManagerAll       = 0xF003F
	serviceQueryConfig = 0x0001
	serviceQueryStatus = 0x0004
	serviceStart       = 0x0010
	serviceStop        = 0x0020
	serviceAll         = 0xF01FF
	serviceNoChange    = 0xFFFFFFFF
	serviceDemandStart = 3
	serviceControlStop = 1
	serviceStopped     = 1
	serviceRunningCode = 4

	errServiceAlreadyRunning = 1056
	errServiceDoesNotExist   = 1060
)

// SERVICE_STATUS.
type serviceStatus struct {
	ServiceType, CurrentState, ControlsAccepted, Win32ExitCode uint32
	ServiceSpecificExitCode, CheckPoint, WaitHint              uint32
}

// QUERY_SERVICE_CONFIGW. Go lays out the fields like C does; a test checks the size.
type serviceConfig struct {
	ServiceType, StartType, ErrorControl uint32
	BinaryPathName, LoadOrderGroup       *uint16
	TagID                                uint32
	Dependencies, ServiceStartName       *uint16
	DisplayName                          *uint16
}

func call(p *syscall.LazyProc, args ...uintptr) (uintptr, error) {
	r, _, err := p.Call(args...)
	if r == 0 {
		return 0, err
	}
	return r, nil
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func openService(scmAccess, serviceAccess uint32) (scm, service uintptr, err error) {
	scm, err = call(procOpenSCManagerW, 0, 0, uintptr(scmAccess))
	if err != nil {
		return 0, 0, err
	}
	service, err = call(procOpenServiceW, scm, uintptr(unsafe.Pointer(utf16Ptr(serviceName))), uintptr(serviceAccess))
	if err != nil {
		procCloseServiceHandle.Call(scm)
		return 0, 0, err
	}
	return scm, service, nil
}

func closeService(scm, service uintptr) {
	procCloseServiceHandle.Call(service)
	procCloseServiceHandle.Call(scm)
}

func serviceState(service uintptr) uint32 {
	var st serviceStatus
	if _, err := call(procQueryServiceStatus, service, uintptr(unsafe.Pointer(&st))); err != nil {
		return 0
	}
	return st.CurrentState
}

func waitState(service uintptr, state uint32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for serviceState(service) != state {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
	return true
}

func sendStop(service uintptr) {
	var st serviceStatus
	procControlService.Call(service, serviceControlStop, uintptr(unsafe.Pointer(&st)))
}

func serviceRunning() bool {
	scm, service, err := openService(scManagerConnect, serviceQueryStatus)
	if err != nil {
		return false
	}
	defer closeService(scm, service)
	return serviceState(service) == serviceRunningCode
}

func startService() error {
	scm, service, err := openService(scManagerConnect, serviceStart|serviceQueryStatus)
	if err != nil {
		return fmt.Errorf("nao consegui abrir o servico do tunel: %w", err)
	}
	defer closeService(scm, service)

	if _, err := call(procStartServiceW, service, 0, 0); err != nil && !errors.Is(err, syscall.Errno(errServiceAlreadyRunning)) {
		return fmt.Errorf("o servico do tunel nao ligou: %w", err)
	}
	if !waitState(service, serviceRunningCode, 30*time.Second) {
		return errors.New("o servico do tunel nao ficou de pe em 30 s")
	}
	return nil
}

func stopService() {
	scm, service, err := openService(scManagerConnect, serviceStop|serviceQueryStatus)
	if err != nil {
		return
	}
	defer closeService(scm, service)
	sendStop(service)
	waitState(service, serviceStopped, 15*time.Second)
}

func serviceBinaryPath(service uintptr) string {
	var needed uint32
	procQueryServiceConfigW.Call(service, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return ""
	}
	buf := make([]byte, needed)
	if _, err := call(procQueryServiceConfigW, service, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed))); err != nil {
		return ""
	}
	cfg := (*serviceConfig)(unsafe.Pointer(&buf[0]))
	if cfg.BinaryPathName == nil {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(cfg.BinaryPathName, len(buf)/2))
}
