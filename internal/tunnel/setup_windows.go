//go:build windows

package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Shnrqpdr/golive-tunnel/internal/netcheck"
	"github.com/Shnrqpdr/golive-tunnel/internal/process"
	"github.com/Shnrqpdr/golive-tunnel/internal/ui"
)

var (
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteExW = shell32.NewProc("ShellExecuteExW")
)

const (
	// Official WireSock SDK installer, pinned by version and by hash: a file different from the
	// checked one does not run, even when it comes from the right domain.
	wireSockURL    = "https://wiresock.net/_api/download-release.php?product=wiresock-secure-connect-sdk&platform=x64&version=3.4.8.1&channel=winget"
	wireSockSHA256 = "abfeebdc645de36b95fabbed00c7fdb0bf4d0c68c5518608450619c61876d33e"
	wireSockMaxMB  = 32

	errCancelled          = 1223
	seeMaskNoCloseProcess = 0x40
	swHide                = 0
	tokenElevation        = 20
	infinite              = 0xFFFFFFFF
)

// SHELLEXECUTEINFOW; only the fields in use have meaningful names.
type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     syscall.Handle
}

func findWireSock() string {
	base := os.Getenv("ProgramW6432")
	if base == "" {
		base = os.Getenv("ProgramFiles")
	}
	path := filepath.Join(base, "WireSock Secure Connect", "sdk", "wiresock-client.exe")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// The configuration the service reads lives where only administrators write: the service runs
// as SYSTEM, and a file in the user's folder would let any of their programs change what SYSTEM
// loads.
func protectedDir() (string, error) {
	base := os.Getenv("ProgramData")
	if base == "" {
		return "", errors.New("ProgramData nao esta definido")
	}
	return filepath.Join(base, "golive-tunnel"), nil
}

func serviceCommandLine(exe, dir string) string {
	return fmt.Sprintf(`"%s" service -config "%s" -log-level info -network-lock disabled`, exe, filepath.Join(dir, "wiresock.conf"))
}

// All set when WireSock exists, the service points to the protected configuration, it is the
// same one we would build now, and this user may start and stop the service without
// administrator.
func wireSockReady(config string) bool {
	exe := findWireSock()
	dir, err := protectedDir()
	if exe == "" || err != nil {
		return false
	}
	installed, err := os.ReadFile(filepath.Join(dir, "wiresock.conf.sha256"))
	if err != nil || strings.TrimSpace(string(installed)) != fingerprint(config) {
		return false
	}
	scm, service, err := openService(scManagerConnect, serviceQueryConfig|serviceStart|serviceStop|serviceQueryStatus)
	if err != nil {
		return false
	}
	defer closeService(scm, service)
	return serviceBinaryPath(service) == serviceCommandLine(exe, dir)
}

func ensureWireSock(keyPath, config string, u *ui.UI) error {
	if wireSockReady(config) {
		if !serviceRunning() && strayWireSock() {
			return errors.New("ja existe um WireSock rodando fora do servico, aberto por outro programa ou a mao. Feche-o e abra este de novo")
		}
		return nil
	}

	current, err := user.Current()
	if err != nil {
		return err
	}
	u.Busy("preparando o WireSock: aceite o aviso de administrador do Windows")
	result := setupResultPath(keyPath)
	os.Remove(result)

	code, err := elevate(fmt.Sprintf(`-%s "%s" -sid %s`, SetupFlag, keyPath, current.Uid))
	detail, _ := os.ReadFile(result)
	os.Remove(result)
	switch {
	case errors.Is(err, syscall.Errno(errCancelled)):
		return errors.New("o aviso de administrador foi recusado. Ele so aparece na primeira vez: e quando o programa instala o WireSock e prepara o servico do tunel")
	case err != nil:
		return fmt.Errorf("nao consegui pedir administrador: %w", err)
	case code != 0:
		return fmt.Errorf("a preparacao do WireSock falhou: %s", strings.TrimSpace(string(detail)))
	}

	if !wireSockReady(config) {
		return errors.New("a preparacao terminou, mas o servico do tunel nao ficou como esperado. Abra o programa de novo")
	}
	u.Ok("WireSock pronto, servico do tunel preparado")
	return nil
}

func strayWireSock() bool {
	running, err := process.Running("wiresock-client.exe")
	return err == nil && running
}

// The elevated step runs without a window, so it reports failures in a file next to the key,
// which the instance that asked for it reads.
func setupResultPath(keyPath string) string {
	return filepath.Join(filepath.Dir(keyPath), "setup-error.txt")
}

// elevate reopens this executable as administrator, without a window, and waits for it.
func elevate(args string) (uint32, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       utf16Ptr("runas"),
		lpFile:       utf16Ptr(exe),
		lpParameters: utf16Ptr(args),
		nShow:        swHide,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if _, err := call(procShellExecuteExW, uintptr(unsafe.Pointer(&info))); err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(info.hProcess)

	syscall.WaitForSingleObject(info.hProcess, infinite)
	var code uint32
	if err := syscall.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return 0, err
	}
	return code, nil
}

func IsElevated() bool {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	var elevated, n uint32
	if err := syscall.GetTokenInformation(token, tokenElevation, (*byte)(unsafe.Pointer(&elevated)), 4, &n); err != nil {
		return false
	}
	return elevated != 0
}

// SetupElevated runs as administrator, called through elevate. It builds everything from the key
// and from what is compiled into this executable; nothing else in the user's folder is used.
func SetupElevated(cfg Config, keyPath, sid string) (err error) {
	defer func() {
		if err != nil {
			os.WriteFile(setupResultPath(keyPath), []byte(err.Error()), 0o600)
		}
	}()

	if !sidPattern.MatchString(sid) {
		return fmt.Errorf("identificador de usuario invalido: %q", sid)
	}
	private, err := readPrivateKey(keyPath)
	if err != nil {
		return err
	}
	config := wireSockConfig(cfg, private, tunneledApps())

	exe := findWireSock()
	if exe == "" {
		if err := installWireSock(); err != nil {
			return err
		}
		if exe = findWireSock(); exe == "" {
			return errors.New("o instalador do WireSock terminou, mas o programa nao apareceu em Arquivos de Programas")
		}
	}

	dir, err := protectedDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// SYSTEM and administrators only: the configuration carries the tunnel's private key.
	if out, err := exec.Command("icacls", dir, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F").CombinedOutput(); err != nil {
		return fmt.Errorf("nao consegui proteger %s: %v: %s", dir, err, strings.TrimSpace(string(out)))
	}
	conf := filepath.Join(dir, "wiresock.conf")
	if err := os.WriteFile(conf, []byte(config), 0o600); err != nil {
		return err
	}
	sum := filepath.Join(dir, "wiresock.conf.sha256")
	if err := os.WriteFile(sum, []byte(fingerprint(config)), 0o644); err != nil {
		return err
	}
	// The fingerprint is no secret, and the program without administrator needs to read it.
	if out, err := exec.Command("icacls", sum, "/grant", "*S-1-5-32-545:R").CombinedOutput(); err != nil {
		return fmt.Errorf("nao consegui liberar a leitura de %s: %v: %s", sum, err, strings.TrimSpace(string(out)))
	}
	// Builds from before the rename kept a copy of the private key here.
	os.RemoveAll(filepath.Join(filepath.Dir(dir), "DiscordGoLiveBypass"))

	scm, service, err := openService(scManagerAll, serviceAll)
	if errors.Is(err, syscall.Errno(errServiceDoesNotExist)) {
		out, ierr := exec.Command(exe, "install", "-start-type", "3", "-config", conf, "-log-level", "info", "-network-lock", "disabled").CombinedOutput()
		scm, service, err = openService(scManagerAll, serviceAll)
		if err != nil {
			return fmt.Errorf("o WireSock nao criou o servico (%v): %s", ierr, strings.TrimSpace(string(out)))
		}
	} else if err != nil {
		return fmt.Errorf("nao consegui abrir o servico do WireSock: %w", err)
	}
	defer closeService(scm, service)

	sendStop(service)
	waitState(service, serviceStopped, 30*time.Second)

	// The service may have come from another program: its command line is always rewritten to
	// point to the protected configuration, with network-lock off. On, it cuts the internet of
	// the whole computer when the tunnel drops.
	if _, err := call(procChangeServiceConfigW, service, serviceNoChange, serviceDemandStart, serviceNoChange,
		uintptr(unsafe.Pointer(utf16Ptr(serviceCommandLine(exe, dir)))), 0, 0, 0, 0, 0, 0); err != nil {
		return fmt.Errorf("nao consegui configurar o servico do WireSock: %w", err)
	}

	sddl, err := exec.Command("sc.exe", "sdshow", serviceName).Output()
	if err != nil {
		return fmt.Errorf("nao consegui ler as permissoes do servico: %w", err)
	}
	if out, err := exec.Command("sc.exe", "sdset", serviceName, grantStartStop(strings.TrimSpace(string(sddl)), sid)).CombinedOutput(); err != nil {
		return fmt.Errorf("nao consegui liberar o servico para este usuario: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func installWireSock() error {
	dir, err := os.MkdirTemp(filepath.Join(os.Getenv("SystemRoot"), "Temp"), "golive-wiresock-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	installer := filepath.Join(dir, "wiresock-sdk.exe")

	client := &http.Client{Timeout: 2 * time.Minute}
	res, err := client.Get(wireSockURL)
	if err != nil {
		return fmt.Errorf("nao consegui baixar o WireSock: %s", netcheck.Describe(err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("o site do WireSock respondeu HTTP %d", res.StatusCode)
	}

	file, err := os.Create(installer)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(file, h), io.LimitReader(res.Body, wireSockMaxMB<<20))
	file.Close()
	if err != nil {
		return fmt.Errorf("o download do WireSock foi interrompido: %w", err)
	}
	if hex.EncodeToString(h.Sum(nil)) != wireSockSHA256 {
		return errors.New("o instalador baixado do WireSock nao e o conferido por este programa. Nada foi instalado")
	}

	err = exec.Command(installer, "/quiet", "/norestart").Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3010 {
		return nil // installed, it only asks for a reboot to replace files in use
	}
	if err != nil {
		return fmt.Errorf("o instalador do WireSock falhou: %w", err)
	}
	return nil
}
