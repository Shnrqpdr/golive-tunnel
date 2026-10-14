package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Servidor WireGuard gravado no executável pelo build, com -ldflags -X. Vazio, o programa segue
// como antes: -proxy, Tor ou -public. Preenchido, o duplo clique já põe o Discord inteiro no
// túnel, voz e vídeo incluídos.
//
// Túnel só de TCP não basta: o servidor de mídia do Discord olha o IP de onde vem o UDP, e com
// ele no Brasil o vídeo do Go Live não passa, mesmo com o gateway saindo de fora.
var (
	tunelServidor      = "" // host:porta do WireGuard
	tunelChaveServidor = "" // chave pública do servidor, em base64
)

const (
	tunelRetomada   = 5 * time.Second
	discordChecagem = 10 * time.Second
	tunelEspera     = 25 * time.Second

	// Prefixo da linha que o amigo manda: deixa claro, para quem recebe, do que se trata.
	prefixoChave = "golive-wg"
)

// A primeira execução só gera a chave: o servidor ainda não a conhece, e tentar conectar agora
// daria um erro que não ensina nada a quem acabou de baixar o programa.
var errChaveNova = errors.New("chave nova, falta a liberacao no servidor")

var sidRe = regexp.MustCompile(`^S-1-[0-9]+(-[0-9]+)+$`)

// Dá a um usuário o direito de consultar, ligar e desligar (LC, RP, WP) só o serviço do túnel,
// para o programa não precisar de administrador a cada abertura. A entrada vai na DACL, antes
// da parte de auditoria, e não se repete quando a preparação roda de novo.
func PermitirLigarDesligar(sddl, sid string) string {
	// O Windows devolve os direitos na ordem dele (LCRPWP), não na que foram gravados, então a
	// entrada existente é reconhecida pelos direitos e não pelo texto.
	existente := regexp.MustCompile(`\(A;;([A-Z]+);;;` + regexp.QuoteMeta(sid) + `\)`)
	for _, m := range existente.FindAllStringSubmatch(sddl, -1) {
		direitos := map[string]bool{}
		for i := 0; i+1 < len(m[1]); i += 2 {
			direitos[m[1][i:i+2]] = true
		}
		if direitos["LC"] && direitos["RP"] && direitos["WP"] {
			return sddl
		}
	}

	ace := "(A;;LCRPWP;;;" + sid + ")"
	if i := strings.Index(sddl, "S:"); i >= 0 {
		return sddl[:i] + ace + sddl[i:]
	}
	return sddl + ace
}

func TunelEmbutido() bool {
	return tunelServidor != "" && tunelChaveServidor != ""
}

// O endereço dentro do túnel sai da própria chave pública, com a mesma conta do golive-peer no
// servidor: assim ninguém precisa combinar IP. Fica em 10.66.1-254.1-254, longe do 10.66.0.1.
func EnderecoDaChave(publica []byte) string {
	h := sha256.Sum256(publica)
	return fmt.Sprintf("10.66.%d.%d", int(h[0])%254+1, int(h[1])%254+1)
}

// Só estes programas entram no túnel; o resto do computador segue na rede normal. Este próprio
// executável vai junto para conferir a saída antes de abrir o Discord.
func appsNoTunel() []string {
	apps := []string{"Discord", "Discord.exe", "Update.exe"}
	if exe, err := os.Executable(); err == nil {
		if nome := filepath.Base(exe); !strings.ContainsAny(nome, ",\r\n") {
			apps = append(apps, nome)
		}
	}
	return apps
}

// Configuração do WireSock: WireGuard comum mais a linha AllowedApps, que o WireSock lê de
// dentro de um comentário para o arquivo continuar válido para qualquer outro cliente.
func ConfigWireSock(privada []byte, apps []string) string {
	chave := ecdhPublica(privada)
	return fmt.Sprintf("[Interface]\r\n"+
		"PrivateKey = %s\r\n"+
		"Address = %s/32\r\n"+
		"\r\n"+
		"[Peer]\r\n"+
		"PublicKey = %s\r\n"+
		"AllowedIPs = 0.0.0.0/0, ::/0\r\n"+
		"Endpoint = %s\r\n"+
		"PersistentKeepalive = 25\r\n"+
		"#@ws:AllowedApps = %s\r\n",
		base64.StdEncoding.EncodeToString(privada), EnderecoDaChave(chave),
		tunelChaveServidor, tunelServidor, strings.Join(apps, ", "))
}

// Impressão digital da configuração instalada. Comparar isto evita pedir administrador a cada
// abertura: só muda quando muda a chave, o servidor ou o nome deste executável.
func DigitalConfig(config string) string {
	h := sha256.Sum256([]byte(config))
	return hex.EncodeToString(h[:])
}

func ecdhPublica(privada []byte) []byte {
	chave, err := ecdh.X25519().NewPrivateKey(privada)
	if err != nil {
		return nil
	}
	return chave.PublicKey().Bytes()
}

func pastaTunel() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "DiscordGoLiveBypass", "wireguard"), nil
}

// A chave privada fica na pasta do usuário e nunca sai dela: quem cuida do servidor só recebe a
// pública. O WireSock lê uma cópia montada pela etapa de administrador, em pasta protegida.
func LerChavePrivada(caminho string) ([]byte, error) {
	bruto, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	privada, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(bruto)))
	if err != nil || len(privada) != 32 || ecdhPublica(privada) == nil {
		return nil, fmt.Errorf("a chave em %s esta corrompida: apague o arquivo e gere outra", caminho)
	}
	return privada, nil
}

func garantirChave(caminho string) (privada []byte, nova bool, err error) {
	privada, err = LerChavePrivada(caminho)
	if err == nil {
		return privada, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}

	chave, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(caminho, []byte(base64.StdEncoding.EncodeToString(chave.Bytes())), 0o600); err != nil {
		return nil, false, err
	}
	return chave.Bytes(), true, nil
}

func LinhaParaLiberar(privada []byte) string {
	return prefixoChave + " " + base64.StdEncoding.EncodeToString(ecdhPublica(privada))
}

type Tunel struct {
	privada []byte
	linha   string
	// Se o serviço já estava ligado, é de outra instância que vigia um Discord aberto: esta só
	// pode desligar o que ela mesma ligou.
	ligouAgora bool
}

func (t *Tunel) Endpoint() Endpoint {
	host, port := tunelServidor, 0
	if i := strings.LastIndex(tunelServidor, ":"); i > 0 {
		host = tunelServidor[:i]
		fmt.Sscan(tunelServidor[i+1:], &port)
	}
	return Endpoint{Scheme: "wireguard", Host: host, Port: port}
}

// Prepara a chave, o WireSock e o serviço, e liga o túnel. Na primeira vez para antes, com
// errChaveNova, depois de mostrar a linha que precisa ser liberada no servidor.
func AbrirTunel(ui *UI) (*Tunel, error) {
	dir, err := pastaTunel()
	if err != nil {
		return nil, err
	}
	caminho := filepath.Join(dir, "privada")

	privada, nova, err := garantirChave(caminho)
	if err != nil {
		return nil, err
	}
	t := &Tunel{privada: privada, linha: LinhaParaLiberar(privada)}
	if nova {
		mostrarChave(ui, "Primeira vez neste computador", t.linha)
		return nil, errChaveNova
	}

	if err := prepararWireSock(caminho, ConfigWireSock(privada, appsNoTunel()), ui); err != nil {
		return nil, err
	}

	ui.Busy("ligando o tunel ate %s", t.Endpoint().Host)
	t.ligouAgora = !servicoLigado()
	if err := ligarServico(); err != nil {
		return nil, err
	}
	ui.Ok("tunel WireGuard ligado ate %s", t.Endpoint().Host)
	return t, nil
}

func mostrarChave(ui *UI, titulo, linha string) {
	copiada := copiarTexto(linha) == nil
	rows := [][2]string{
		{"", "Mande a linha abaixo para quem cuida do servidor. Nao e senha."},
		{"", ""},
		{"", linha},
		{"", ""},
	}
	if copiada {
		rows = append(rows, [2]string{"", "Ela ja esta copiada: e so colar na conversa."})
	}
	rows = append(rows, [2]string{"", "Quando liberarem, abra o programa de novo."})
	ui.box(cYellow, titulo, rows)
}

// O WireGuard não avisa quando o servidor desconhece a chave: o pacote só não tem resposta. Por
// isso a prova é o Discord responder por dentro do túnel, e o país vir de fora.
func (t *Tunel) Validar(excluded map[string]bool, ui *UI) (choice, error) {
	ui.Busy("testando o Discord pelo tunel")
	limite := time.Now().Add(tunelEspera)
	var latency time.Duration
	var err error
	for {
		if latency, err = Probe(t.Endpoint(), probeTimeout); err == nil {
			break
		}
		if time.Now().After(limite) {
			copiarTexto(t.linha)
			return choice{}, fmt.Errorf("o tunel ligou, mas nada passou por ele em %s. Se voce acabou de configurar, confirme se quem cuida do servidor ja liberou esta linha (ela ja esta copiada): %s", tunelEspera, t.linha)
		}
		time.Sleep(time.Second)
	}

	country, err := Accepts(t.Endpoint(), excluded, probeTimeout)
	if err != nil {
		return choice{}, fmt.Errorf("o tunel funciona, mas a saida nao serve: %w", err)
	}

	ui.Ok("o Discord respondeu em %d ms pelo tunel, saida em %s", latency.Milliseconds(), CountryLabel(country))
	return choice{t.Endpoint(), latency, country, sourceTunnel}, nil
}

// Para o fim do -check e para os erros: desligar um túnel que já estava de pé derrubaria o
// Discord de quem está usando agora.
func (t *Tunel) Fechar() {
	if t.ligouAgora {
		desligarServico()
	}
}

// O túnel vive enquanto o Discord estiver aberto: se o serviço parar, liga de novo, e quando o
// Discord fecha ele desliga junto. Fechar esta janela também desliga.
func (t *Tunel) Vigiar(channel Channel, ui *UI) error {
	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sinais
		desligarServico()
		os.Exit(0)
	}()

	ui.Info("deixe esta janela aberta: ela mantem o tunel e fecha sozinha junto com o %s", channel.Friendly)
	if ownsConsole() {
		time.Sleep(2 * time.Second)
		minimizeConsole()
	}

	ausente := 0
	ultimaChecagem := time.Now()
	for {
		time.Sleep(tunelRetomada)

		// Duas checagens seguidas sem o Discord: o update dele fecha e reabre o processo, e
		// desligar o túnel nesse meio tempo deixaria a versão nova saindo pelo Brasil.
		if time.Since(ultimaChecagem) >= discordChecagem {
			ultimaChecagem = time.Now()
			aberto, err := IsRunning(channel)
			switch {
			case err != nil:
			case aberto:
				ausente = 0
			default:
				ausente++
			}
			if ausente >= 2 {
				desligarServico()
				ui.Ok("%s fechado, tunel desligado", channel.Friendly)
				return nil
			}
		}

		if servicoLigado() {
			continue
		}
		ui.Warn("o tunel parou, ligando de novo")
		if err := ligarServico(); err != nil {
			ui.Detail("%v", err)
			continue
		}
		ui.Ok("tunel de volta")
	}
}
