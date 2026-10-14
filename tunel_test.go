package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// O build comum nao embute servidor: sem -ldflags, o programa tem de se comportar como antes.
func TestTunelEmbutidoPadrao(t *testing.T) {
	if TunelEmbutido() {
		t.Error("build sem -ldflags nao deveria ter tunel embutido")
	}
}

// Mesmo resultado do servidor/golive-peer para esta chave de exemplo: se as contas divergirem, o
// servidor libera um endereco e o programa usa outro, e nada passa.
func TestEnderecoDaChave(t *testing.T) {
	publica, err := base64.StdEncoding.DecodeString("iGzzH4MBfPTTXC5oQScRNZLXI7FOkqkCzDafbSaQrcI=")
	if err != nil {
		t.Fatal(err)
	}
	if got := EnderecoDaChave(publica); got != "10.66.125.59" {
		t.Errorf("EnderecoDaChave = %s, o golive-peer calcula 10.66.125.59", got)
	}
}

func TestConfigWireSock(t *testing.T) {
	servidor, chave := tunelServidor, tunelChaveServidor
	tunelServidor, tunelChaveServidor = "203.0.113.10:51820", "MrAXj9io3KrzWoF5LayHn62ca3yy3RgwKwsNEgtuJGI="
	defer func() { tunelServidor, tunelChaveServidor = servidor, chave }()

	privada := bytes.Repeat([]byte{7}, 32)
	config := ConfigWireSock(privada, []string{"Discord", "Discord.exe", "Update.exe"})

	for _, want := range []string{
		"PrivateKey = " + base64.StdEncoding.EncodeToString(privada) + "\r\n",
		"Address = " + EnderecoDaChave(ecdhPublica(privada)) + "/32\r\n",
		"PublicKey = MrAXj9io3KrzWoF5LayHn62ca3yy3RgwKwsNEgtuJGI=\r\n",
		"Endpoint = 203.0.113.10:51820\r\n",
		"AllowedIPs = 0.0.0.0/0, ::/0\r\n",
		"#@ws:AllowedApps = Discord, Discord.exe, Update.exe\r\n",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("faltou %q em:\n%s", want, config)
		}
	}
	// O servico roda como SYSTEM: nenhuma diretiva que execute coisa alguma pode entrar.
	for _, proibida := range []string{"PostUp", "PreUp", "PostDown", "PreDown", "DNS"} {
		if strings.Contains(config, proibida) {
			t.Errorf("a configuracao nao deveria ter %s:\n%s", proibida, config)
		}
	}
	if DigitalConfig(config) != DigitalConfig(ConfigWireSock(privada, []string{"Discord", "Discord.exe", "Update.exe"})) {
		t.Error("a mesma entrada deveria dar a mesma impressao digital")
	}
}

func TestGarantirChave(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "wireguard", "privada")

	primeira, nova, err := garantirChave(caminho)
	if err != nil || !nova || len(primeira) != 32 {
		t.Fatalf("garantirChave na primeira vez = %d bytes, nova=%v, err=%v", len(primeira), nova, err)
	}
	segunda, nova, err := garantirChave(caminho)
	if err != nil || nova || !bytes.Equal(primeira, segunda) {
		t.Fatalf("na segunda vez a chave deveria ser a mesma e nao nova (nova=%v, err=%v)", nova, err)
	}
	if linha := LinhaParaLiberar(primeira); !strings.HasPrefix(linha, "golive-wg ") || len(linha) != len("golive-wg ")+44 {
		t.Errorf("linha para liberar fora do formato: %q", linha)
	}

	if err := os.WriteFile(caminho, []byte("nao e chave"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := garantirChave(caminho); err == nil {
		t.Error("chave corrompida deveria dar erro, e nao virar chave nova em silencio")
	}
}

func TestPermitirLigarDesligar(t *testing.T) {
	sid := "S-1-5-21-1-2-3-1001"
	ace := "(A;;LCRPWP;;;" + sid + ")"

	// Como o Windows devolve depois de gravar: mesma entrada, direitos em outra ordem.
	if got := PermitirLigarDesligar("D:(A;;RPWPLC;;;"+sid+")S:(AU;FA;GA;;;WD)", sid); strings.Count(got, sid) != 1 {
		t.Errorf("entrada com os direitos em outra ordem deveria valer: %s", got)
	}
	// Entrada do mesmo usuario sem o direito de desligar nao basta.
	if got := PermitirLigarDesligar("D:(A;;LCRP;;;"+sid+")", sid); !strings.Contains(got, ace) {
		t.Errorf("faltando WP a permissao completa deveria entrar: %s", got)
	}

	comAuditoria := "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCLCSWLOCRRC;;;IU)S:(AU;FA;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;WD)"
	got := PermitirLigarDesligar(comAuditoria, sid)
	if !strings.Contains(got, "(A;;CCLCSWLOCRRC;;;IU)"+ace+"S:(AU;") {
		t.Errorf("a permissao deveria entrar na DACL, antes do S:: %s", got)
	}
	if again := PermitirLigarDesligar(got, sid); again != got {
		t.Errorf("rodar de novo nao deveria repetir a permissao: %s", again)
	}

	if got := PermitirLigarDesligar("D:(A;;CCLCSWLOCRRC;;;IU)", sid); got != "D:(A;;CCLCSWLOCRRC;;;IU)"+ace {
		t.Errorf("sem S: a permissao vai no fim: %s", got)
	}

	for _, sid := range []string{"S-1-5-21-1-2-3-1001", "S-1-5-18"} {
		if !sidRe.MatchString(sid) {
			t.Errorf("SID valido recusado: %s", sid)
		}
	}
	for _, sid := range []string{"", "S-1-5-21)(A;;GA;;;WD", "WD", "S-1-5-21-1 -2"} {
		if sidRe.MatchString(sid) {
			t.Errorf("SID invalido aceito: %q", sid)
		}
	}
}
