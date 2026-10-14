// Command golive-tunnel opens Discord through a WireGuard tunnel to the server compiled into it,
// so Go Live, camera and screen sharing work from Brazil. Only Discord goes into the tunnel.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Shnrqpdr/golive-tunnel/internal/discord"
	"github.com/Shnrqpdr/golive-tunnel/internal/netcheck"
	"github.com/Shnrqpdr/golive-tunnel/internal/tunnel"
	"github.com/Shnrqpdr/golive-tunnel/internal/ui"
)

// Set at build time with -ldflags -X (see scripts/build.sh). A plain go build leaves the server
// empty, and the program refuses to run: there is nothing to tunnel to.
var (
	version   = "dev"
	server    = "" // host:port of the server's WireGuard
	serverKey = "" // the server's WireGuard public key, in base64
)

// How long the process has to survive for the launch to count as successful. One that dies
// from a broken install dies well before that.
const survivalDelay = 3 * time.Second

type options struct {
	exclude string
	channel string
	exePath string
	force   bool
	check   bool
	extra   []string
}

func main() {
	opts := options{}

	flag.StringVar(&opts.exclude, "exclude", "BR", "codigos de pais recusados na saida, separados por virgula")
	flag.StringVar(&opts.channel, "channel", "auto", "canal do Discord: auto, stable, ptb ou canary")
	flag.StringVar(&opts.exePath, "exe", "", "caminho explicito do executavel, ignora a deteccao automatica")
	// The double click is the normal use, and it has to restart a Discord that is already open:
	// otherwise the old one keeps going out through the normal route.
	flag.BoolVar(&opts.force, "force", true, "reinicia o Discord que ja estiver aberto; -force=false recusa em vez disso")
	flag.BoolVar(&opts.check, "check", false, "so liga e confere o tunel, nao abre o Discord")
	setupKey := flag.String(tunnel.SetupFlag, "", "uso interno: etapa de administrador que instala o WireSock e prepara o servico do tunel")
	sid := flag.String("sid", "", "uso interno: usuario que pode ligar e desligar o servico do tunel")

	flag.Usage = usage
	flag.Parse()
	opts.extra = flag.Args()

	cfg := tunnel.Config{Endpoint: server, PublicKey: serverKey}

	// Elevated step, opened by the program itself through the administrator prompt. It runs
	// without a window, so the result goes to a file the original instance reads.
	if *setupKey != "" {
		if err := tunnel.SetupElevated(cfg, *setupKey, *sid); err != nil {
			os.Exit(1)
		}
		return
	}

	u := ui.New()
	cleanup := &shutdown{}
	cleanup.add(func() {
		u.Close()
		u.Print("\n  cancelado")
	})
	cleanup.listen()

	err := run(opts, cfg, u, cleanup)
	failed := err != nil && !errors.Is(err, tunnel.ErrNewKey)
	if failed {
		u.Fatal(err)
	}
	u.Close()

	// Opened by double click, the window goes away with the program: without the pause, the new
	// key or the error would disappear before anyone read them.
	if err != nil && keepWindow() {
		fmt.Fprint(os.Stdout, "\n  Pressione Enter para fechar ")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}

	if failed {
		os.Exit(1)
	}
}

// From a terminal, the terminal holds the window. Opened directly, the program holds it itself
// whenever someone is at the keyboard.
func keepWindow() bool {
	return ui.OwnsConsole() || ui.IsTerminal(os.Stdin)
}

func usage() {
	fmt.Fprintf(os.Stderr, `golive-tunnel: abre o Discord por um tunel WireGuard ate o servidor embutido.

uso: %s [flags] [-- argumentos extras para o Discord]

`, os.Args[0])
	flag.PrintDefaults()
}

// shutdown runs the cleanup steps on Ctrl+C and when the console window is closed (SIGTERM on
// Windows). One handler for everything: with one per package, whichever exited first could
// skip stopping the tunnel.
type shutdown struct {
	mu    sync.Mutex
	steps []func()
}

func (s *shutdown) add(step func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
}

func (s *shutdown) listen() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		s.mu.Lock()
		for i := len(s.steps) - 1; i >= 0; i-- {
			s.steps[i]()
		}
		os.Exit(130)
	}()
}

func run(opts options, cfg tunnel.Config, u *ui.UI, cleanup *shutdown) error {
	if !cfg.Valid() {
		return errors.New("este executavel foi compilado sem servidor embutido. Compile com scripts/build.sh, com o .env preenchido")
	}

	excluded := netcheck.ParseCountries(opts.exclude)
	if len(excluded) == 0 {
		return errors.New("a lista de paises recusados ficou vazia, informe algo como -exclude BR")
	}

	// Opened as administrator, the program would open Discord as administrator too. When it
	// needs to, it asks for the Windows prompt itself, once, to install WireSock.
	if tunnel.IsElevated() {
		return errors.New("este programa foi aberto como administrador, e assim o Discord abriria como administrador tambem. Feche e abra com duplo clique normal: quando precisar, ele mesmo pede o aviso do Windows")
	}

	steps := 4
	if opts.check {
		steps = 3
	}
	u.Plan(steps)
	u.Banner(version)

	// Finding the executable before the tunnel avoids setting everything up only to find out
	// Discord is not even installed.
	u.Step("Procurando a instalacao do Discord")
	channel, binary, err := resolveTarget(opts, u)
	if err != nil {
		if !opts.check {
			return err
		}
		u.Warn("%v", err)
	}

	if binary != "" {
		if _, err := os.Stat(binary); err != nil {
			return fmt.Errorf("nao consegui usar o executavel %s: %w", binary, err)
		}
		u.Ok("%s encontrado", channel.Friendly)
		u.Detail("%s", binary)

		running, err := discord.IsRunning(channel)
		switch {
		case err != nil && opts.check:
			u.Warn("nao consegui checar se o %s ja esta aberto: %v", channel.Friendly, err)
		case err != nil:
			return fmt.Errorf("nao consegui checar se o %s ja esta aberto: %w. Sem essa resposta nao da para abrir: se ele estiver de pe, o processo novo so acorda a janela antiga, que continua fora do tunel, e a tela mostraria sucesso do mesmo jeito", channel.Friendly, err)
		case running && opts.check:
			u.Warn("o %s esta aberto agora, uma abertura de verdade o reiniciaria", channel.Friendly)
		case running && !opts.force:
			return fmt.Errorf("o %s ja esta aberto. Ele e instancia unica, entao abrir de novo so acorda a janela antiga, que continua fora do tunel. Feche-o ou rode sem -force=false", channel.Friendly)
		case running:
			u.Warn("o %s ja esta aberto, sera reiniciado no fim", channel.Friendly)
		}
	}

	u.Step("Abrindo o tunel ate o servidor")
	t, err := tunnel.Open(cfg, u)
	if err != nil {
		return err
	}
	// On an error from here on, at the end of -check and on Ctrl+C the tunnel goes down with
	// the program. Opening Discord, Watch only returns when it closes.
	defer t.Close()
	cleanup.add(t.Close)

	u.Step("Conferindo a saida pelo tunel")
	exit, err := t.Validate(excluded, u)
	if err != nil {
		return err
	}

	summary := [][2]string{
		{"saida", ui.CountryLabel(exit.Country)},
		{"servidor", cfg.Endpoint},
		{"resposta", fmt.Sprintf("%d ms", exit.Latency.Milliseconds())},
		{"rota", "o Discord inteiro, voz e video incluidos"},
	}

	if opts.check {
		u.Summary("tunel validado", append(summary, [2]string{"discord", "nao foi aberto, modo -check"}))
		return nil
	}

	// Only kill the old instance now, with a working tunnel at hand. Killing it earlier would
	// leave the person with no Discord at all if the tunnel failed.
	u.Step("Abrindo o %s", channel.Friendly)
	if err := terminateIfRunning(channel, u); err != nil {
		return err
	}

	// No flag at all: the whole process already goes out through the server.
	u.Busy("subindo o %s", channel.Friendly)
	pid, err := discord.Launch(binary, opts.extra)
	if err != nil {
		return fmt.Errorf("nao consegui abrir o %s: %w", channel.Friendly, err)
	}
	// Start only proves the process was born. Without checking that it survived, a broken
	// install would show the green box with nothing open.
	u.Busy("conferindo se o %s continua de pe", channel.Friendly)
	time.Sleep(survivalDelay)
	if alive, err := discord.IsRunning(channel); err == nil && !alive {
		return fmt.Errorf("o %s abriu (pid %d) e morreu logo em seguida, sem chegar a mostrar janela. Costuma ser instalacao incompleta: abra o %s normalmente uma vez, deixe ele terminar de se atualizar e rode de novo", channel.Friendly, pid, channel.Friendly)
	}

	u.Ok("%s aberto, pid %d", channel.Friendly, pid)
	u.Summary("tudo pronto", append(summary, [2]string{"discord", fmt.Sprintf("%s, pid %d", channel.Friendly, pid)}))

	return t.Watch(channel.Friendly, func() (bool, error) { return discord.IsRunning(channel) }, u)
}

func resolveTarget(opts options, u *ui.UI) (discord.Channel, string, error) {
	if opts.channel != "auto" {
		channel, err := discord.LookupChannel(opts.channel)
		if err != nil {
			return discord.Channel{}, "", err
		}
		if opts.exePath != "" {
			return channel, opts.exePath, nil
		}

		u.Busy("procurando o %s dentro de %%LOCALAPPDATA%%", channel.Friendly)
		binary, err := discord.Locate(channel)
		return channel, binary, err
	}

	if opts.exePath != "" {
		return discord.ChannelForBinary(opts.exePath), opts.exePath, nil
	}

	u.Busy("procurando dentro de %%LOCALAPPDATA%%")
	return discord.DetectChannel()
}

func terminateIfRunning(channel discord.Channel, u *ui.UI) error {
	running, err := discord.IsRunning(channel)
	if err != nil {
		return fmt.Errorf("nao consegui checar se o %s esta aberto para encerra-lo: %w", channel.Friendly, err)
	}
	if !running {
		return nil
	}

	u.Busy("encerrando o %s que ja estava aberto", channel.Friendly)
	if err := discord.Terminate(channel); err != nil {
		return err
	}
	u.Ok("instancia antiga encerrada")

	// Windows takes a moment to release the single-instance lock. Starting on top of it makes
	// the new process think it is a duplicate and quit on its own.
	u.Busy("esperando o Windows soltar a trava de instancia unica")
	time.Sleep(2 * time.Second)
	return nil
}
