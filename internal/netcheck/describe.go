package netcheck

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
)

// WSAEACCES. Windows returns this code when the socket was refused by the machine itself, not
// by the network: antivirus, firewall or a filtering layer installed in between. The number only
// exists on Windows, so elsewhere the comparison never matches and EACCES covers the same case.
const winSocketForbidden = syscall.Errno(10013)

// Describe turns a network error into its cause and, when possible, the next step. The raw
// net/http message carries the whole URL and the resolved IP, which fill the error box without
// saying what to do.
func Describe(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, winSocketForbidden) || errors.Is(err, syscall.EACCES) {
		return "a conexao foi bloqueada pela sua propria maquina, antes de sair para a internet. Quase sempre e antivirus ou firewall barrando o golive-tunnel.exe, que nao tem assinatura digital: libere o programa na lista de excecoes e rode de novo"
	}

	var dns *net.DNSError
	if errors.As(err, &dns) {
		return fmt.Sprintf("nao consegui resolver o endereco %s, o que costuma ser DNS ou internet fora do ar", dns.Name)
	}

	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "a resposta demorou demais, a conexao pode estar instavel"
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return "a conexao foi recusada do outro lado"
	}

	var wrapped *url.Error
	if errors.As(err, &wrapped) {
		return wrapped.Err.Error()
	}
	return err.Error()
}
