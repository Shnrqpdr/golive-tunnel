//go:build windows

package main

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
)

var (
	advapi32                 = syscall.NewLazyDLL("advapi32.dll")
	shell32                  = syscall.NewLazyDLL("shell32.dll")
	procOpenSCManagerW       = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW         = advapi32.NewProc("OpenServiceW")
	procStartServiceW        = advapi32.NewProc("StartServiceW")
	procControlService       = advapi32.NewProc("ControlService")
	procQueryServiceStatus   = advapi32.NewProc("QueryServiceStatus")
	procQueryServiceConfigW  = advapi32.NewProc("QueryServiceConfigW")
	procChangeServiceConfigW = advapi32.NewProc("ChangeServiceConfigW")
	procCloseServiceHandle   = advapi32.NewProc("CloseServiceHandle")
	procShellExecuteExW      = shell32.NewProc("ShellExecuteExW")
)

const (
	// Instalador oficial do WireSock SDK, fixado por versão e por hash: um arquivo diferente
	// do conferido não roda, nem vindo do domínio certo.
	wiresockURL    = "https://wiresock.net/_api/download-release.php?product=wiresock-secure-connect-sdk&platform=x64&version=3.4.8.1&channel=winget"
	wiresockSHA256 = "abfeebdc645de36b95fabbed00c7fdb0bf4d0c68c5518608450619c61876d33e"
	wiresockMaxMB  = 32
	wiresockServ   = "wiresock-client-service"

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
	serviceRunning     = 4

	errServiceAlreadyRunning = 1056
	errServiceNotActive      = 1062
	errServiceDoesNotExist   = 1060
	errCancelled             = 1223

	seeMaskNoCloseProcess = 0x40
	swHide                = 0
	tokenElevation        = 20
	infinite              = 0xFFFFFFFF
)

type serviceStatus struct {
	ServiceType, CurrentState, ControlsAccepted, Win32ExitCode uint32
	ServiceSpecificExitCode, CheckPoint, WaitHint              uint32
}

// QUERY_SERVICE_CONFIGW. O Go alinha os campos como o C; o teste confere o tamanho.
type serviceConfig struct {
	ServiceType, StartType, ErrorControl uint32
	BinaryPathName, LoadOrderGroup       *uint16
	TagID                                uint32
	Dependencies, ServiceStartName       *uint16
	DisplayName                          *uint16
}

// SHELLEXECUTEINFOW, só os campos usados têm nome.
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

func chamar(p *syscall.LazyProc, args ...uintptr) (uintptr, error) {
	r, _, err := p.Call(args...)
	if r == 0 {
		return 0, err
	}
	return r, nil
}

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func abrirServico(acessoSCM, acessoServico uint32) (scm, servico uintptr, err error) {
	scm, err = chamar(procOpenSCManagerW, 0, 0, uintptr(acessoSCM))
	if err != nil {
		return 0, 0, err
	}
	servico, err = chamar(procOpenServiceW, scm, uintptr(unsafe.Pointer(u16(wiresockServ))), uintptr(acessoServico))
	if err != nil {
		procCloseServiceHandle.Call(scm)
		return 0, 0, err
	}
	return scm, servico, nil
}

func fecharServico(scm, servico uintptr) {
	procCloseServiceHandle.Call(servico)
	procCloseServiceHandle.Call(scm)
}

func estadoServico(servico uintptr) uint32 {
	var st serviceStatus
	if _, err := chamar(procQueryServiceStatus, servico, uintptr(unsafe.Pointer(&st))); err != nil {
		return 0
	}
	return st.CurrentState
}

func esperarEstado(servico uintptr, estado uint32, prazo time.Duration) bool {
	limite := time.Now().Add(prazo)
	for estadoServico(servico) != estado {
		if time.Now().After(limite) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
	return true
}

func servicoLigado() bool {
	scm, servico, err := abrirServico(scManagerConnect, serviceQueryStatus)
	if err != nil {
		return false
	}
	defer fecharServico(scm, servico)
	return estadoServico(servico) == serviceRunning
}

func ligarServico() error {
	scm, servico, err := abrirServico(scManagerConnect, serviceStart|serviceQueryStatus)
	if err != nil {
		return fmt.Errorf("nao consegui abrir o servico do tunel: %w", err)
	}
	defer fecharServico(scm, servico)

	if _, err := chamar(procStartServiceW, servico, 0, 0); err != nil && !errors.Is(err, syscall.Errno(errServiceAlreadyRunning)) {
		return fmt.Errorf("o servico do tunel nao ligou: %w", err)
	}
	if !esperarEstado(servico, serviceRunning, 30*time.Second) {
		return errors.New("o servico do tunel nao ficou de pe em 30 s")
	}
	return nil
}

func desligarServico() {
	scm, servico, err := abrirServico(scManagerConnect, serviceStop|serviceQueryStatus)
	if err != nil {
		return
	}
	defer fecharServico(scm, servico)
	var st serviceStatus
	procControlService.Call(servico, serviceControlStop, uintptr(unsafe.Pointer(&st)))
	esperarEstado(servico, serviceStopped, 15*time.Second)
}

func caminhoServico(servico uintptr) string {
	var precisa uint32
	procQueryServiceConfigW.Call(servico, 0, 0, uintptr(unsafe.Pointer(&precisa)))
	if precisa == 0 {
		return ""
	}
	buf := make([]byte, precisa)
	if _, err := chamar(procQueryServiceConfigW, servico, uintptr(unsafe.Pointer(&buf[0])), uintptr(precisa), uintptr(unsafe.Pointer(&precisa))); err != nil {
		return ""
	}
	cfg := (*serviceConfig)(unsafe.Pointer(&buf[0]))
	if cfg.BinaryPathName == nil {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(cfg.BinaryPathName, len(buf)/2))
}

func acharWireSock() string {
	base := os.Getenv("ProgramW6432")
	if base == "" {
		base = os.Getenv("ProgramFiles")
	}
	caminho := filepath.Join(base, "WireSock Secure Connect", "sdk", "wiresock-client.exe")
	if _, err := os.Stat(caminho); err == nil {
		return caminho
	}
	return ""
}

// A configuração que o serviço lê fica onde só administrador escreve: o serviço roda como
// SYSTEM, e um arquivo na pasta do usuário deixaria qualquer programa dele mexer no que o
// SYSTEM carrega.
func pastaProtegida() string {
	return filepath.Join(os.Getenv("ProgramData"), "DiscordGoLiveBypass")
}

func linhaServico(exe string) string {
	conf := filepath.Join(pastaProtegida(), "wiresock-discord.conf")
	return fmt.Sprintf(`"%s" service -config "%s" -log-level info -network-lock disabled`, exe, conf)
}

// Tudo certo quando o WireSock existe, o serviço aponta para a configuração protegida, ela é a
// mesma que montaríamos agora e este usuário pode ligar e desligar o serviço sem administrador.
func wireSockPronto(config string) bool {
	exe := acharWireSock()
	if exe == "" {
		return false
	}
	digital, err := os.ReadFile(filepath.Join(pastaProtegida(), "wiresock-discord.sha256"))
	if err != nil || strings.TrimSpace(string(digital)) != DigitalConfig(config) {
		return false
	}
	scm, servico, err := abrirServico(scManagerConnect, serviceQueryConfig|serviceStart|serviceStop|serviceQueryStatus)
	if err != nil {
		return false
	}
	defer fecharServico(scm, servico)
	return caminhoServico(servico) == linhaServico(exe)
}

func prepararWireSock(caminhoChave, config string, ui *UI) error {
	if wireSockPronto(config) {
		if !servicoLigado() && wireSockSolto() {
			return errors.New("ja existe um WireSock rodando fora do servico, aberto por outro programa ou a mao. Feche-o e abra este de novo")
		}
		return nil
	}

	atual, err := user.Current()
	if err != nil {
		return err
	}
	ui.Busy("preparando o WireSock: aceite o aviso de administrador do Windows")
	resultado := filepath.Join(filepath.Dir(caminhoChave), "preparo.txt")
	os.Remove(resultado)

	codigo, err := elevar(fmt.Sprintf(`-preparar-wiresock "%s" -sid %s`, caminhoChave, atual.Uid))
	detalhe, _ := os.ReadFile(resultado)
	os.Remove(resultado)
	switch {
	case errors.Is(err, syscall.Errno(errCancelled)):
		return errors.New("o aviso de administrador foi recusado. Ele so aparece na primeira vez: e quando o programa instala o WireSock e prepara o servico do tunel")
	case err != nil:
		return fmt.Errorf("nao consegui pedir administrador: %w", err)
	case codigo != 0:
		return fmt.Errorf("a preparacao do WireSock falhou: %s", strings.TrimSpace(string(detalhe)))
	}

	if !wireSockPronto(config) {
		return errors.New("a preparacao terminou, mas o servico do tunel nao ficou como esperado. Abra o programa de novo")
	}
	ui.Ok("WireSock pronto, servico do tunel preparado")
	return nil
}

func wireSockSolto() bool {
	solto, err := IsRunning(Channel{Binary: "wiresock-client.exe"})
	return err == nil && solto
}

// Reabre este executável como administrador, sem janela, e espera ele terminar.
func elevar(argumentos string) (uint32, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       u16("runas"),
		lpFile:       u16(exe),
		lpParameters: u16(argumentos),
		nShow:        swHide,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if _, err := chamar(procShellExecuteExW, uintptr(unsafe.Pointer(&info))); err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(info.hProcess)

	syscall.WaitForSingleObject(info.hProcess, infinite)
	var codigo uint32
	if err := syscall.GetExitCodeProcess(info.hProcess, &codigo); err != nil {
		return 0, err
	}
	return codigo, nil
}

func isElevated() bool {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	var elevado, n uint32
	if err := syscall.GetTokenInformation(token, tokenElevation, (*byte)(unsafe.Pointer(&elevado)), 4, &n); err != nil {
		return false
	}
	return elevado != 0
}

// Roda como administrador, chamado por elevar. Monta tudo a partir da chave e do que está
// gravado neste executável; nada do que estiver na pasta do usuário além da chave é usado.
func PrepararWireSockElevado(caminhoChave, sid string) (err error) {
	resultado := filepath.Join(filepath.Dir(caminhoChave), "preparo.txt")
	defer func() {
		if err != nil {
			os.WriteFile(resultado, []byte(err.Error()), 0o600)
		}
	}()

	if !sidRe.MatchString(sid) {
		return fmt.Errorf("identificador de usuario invalido: %q", sid)
	}
	privada, err := LerChavePrivada(caminhoChave)
	if err != nil {
		return err
	}
	config := ConfigWireSock(privada, appsNoTunel())

	exe := acharWireSock()
	if exe == "" {
		if err := instalarWireSock(); err != nil {
			return err
		}
		if exe = acharWireSock(); exe == "" {
			return errors.New("o instalador do WireSock terminou, mas o programa nao apareceu em Arquivos de Programas")
		}
	}

	dir := pastaProtegida()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Só SYSTEM e administradores: a configuração leva a chave privada do túnel.
	if out, err := exec.Command("icacls", dir, "/inheritance:r", "/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F").CombinedOutput(); err != nil {
		return fmt.Errorf("nao consegui proteger %s: %v: %s", dir, err, strings.TrimSpace(string(out)))
	}
	conf := filepath.Join(dir, "wiresock-discord.conf")
	if err := os.WriteFile(conf, []byte(config), 0o600); err != nil {
		return err
	}
	digital := filepath.Join(dir, "wiresock-discord.sha256")
	if err := os.WriteFile(digital, []byte(DigitalConfig(config)), 0o644); err != nil {
		return err
	}
	// A impressão digital não é segredo, e o programa sem administrador precisa lê-la.
	if out, err := exec.Command("icacls", digital, "/grant", "*S-1-5-32-545:R").CombinedOutput(); err != nil {
		return fmt.Errorf("nao consegui liberar a leitura de %s: %v: %s", digital, err, strings.TrimSpace(string(out)))
	}

	scm, servico, err := abrirServico(scManagerAll, serviceAll)
	if errors.Is(err, syscall.Errno(errServiceDoesNotExist)) {
		out, ierr := exec.Command(exe, "install", "-start-type", "3", "-config", conf, "-log-level", "info", "-network-lock", "disabled").CombinedOutput()
		scm, servico, err = abrirServico(scManagerAll, serviceAll)
		if err != nil {
			return fmt.Errorf("o WireSock nao criou o servico (%v): %s", ierr, strings.TrimSpace(string(out)))
		}
	} else if err != nil {
		return fmt.Errorf("nao consegui abrir o servico do WireSock: %w", err)
	}
	defer fecharServico(scm, servico)

	var st serviceStatus
	procControlService.Call(servico, serviceControlStop, uintptr(unsafe.Pointer(&st)))
	esperarEstado(servico, serviceStopped, 30*time.Second)

	// O serviço pode ter vindo de outro programa: a linha de comando é sempre refeita para
	// apontar para a configuração protegida, com o network-lock desligado. Ligado, ele corta
	// a internet do computador inteiro quando o túnel cai.
	if _, err := chamar(procChangeServiceConfigW, servico, serviceNoChange, serviceDemandStart, serviceNoChange,
		uintptr(unsafe.Pointer(u16(linhaServico(exe)))), 0, 0, 0, 0, 0, 0); err != nil {
		return fmt.Errorf("nao consegui configurar o servico do WireSock: %w", err)
	}

	sddl, err := exec.Command("sc.exe", "sdshow", wiresockServ).Output()
	if err != nil {
		return fmt.Errorf("nao consegui ler as permissoes do servico: %w", err)
	}
	if out, err := exec.Command("sc.exe", "sdset", wiresockServ, PermitirLigarDesligar(strings.TrimSpace(string(sddl)), sid)).CombinedOutput(); err != nil {
		return fmt.Errorf("nao consegui liberar o servico para este usuario: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func instalarWireSock() error {
	dir, err := os.MkdirTemp(filepath.Join(os.Getenv("SystemRoot"), "Temp"), "golive-wiresock-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	instalador := filepath.Join(dir, "wiresock-sdk.exe")

	cliente := &http.Client{Timeout: 2 * time.Minute}
	res, err := cliente.Get(wiresockURL)
	if err != nil {
		return fmt.Errorf("nao consegui baixar o WireSock: %s", DescribeNetwork(err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("o site do WireSock respondeu HTTP %d", res.StatusCode)
	}

	arquivo, err := os.Create(instalador)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(arquivo, h), io.LimitReader(res.Body, wiresockMaxMB<<20))
	arquivo.Close()
	if err != nil {
		return fmt.Errorf("o download do WireSock foi interrompido: %w", err)
	}
	if hex.EncodeToString(h.Sum(nil)) != wiresockSHA256 {
		return errors.New("o instalador baixado do WireSock nao e o conferido por este programa. Nada foi instalado")
	}

	err = exec.Command(instalador, "/quiet", "/norestart").Run()
	var saida *exec.ExitError
	if errors.As(err, &saida) && saida.ExitCode() == 3010 {
		return nil // instalado, só pede reinício para trocar arquivos em uso
	}
	if err != nil {
		return fmt.Errorf("o instalador do WireSock falhou: %w", err)
	}
	return nil
}

func copiarTexto(texto string) error {
	cmd := exec.Command("clip")
	cmd.Stdin = strings.NewReader(texto)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
