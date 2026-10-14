# golive-tunnel

Abre o Discord por um túnel WireGuard até um servidor seu fora do Brasil, devolvendo o Go Live, a câmera e a transmissão de tela. Só o Discord entra no túnel: navegador, jogos e o resto do computador continuam na internet normal.

Você monta o servidor uma vez, compila o executável com o endereço dele embutido e manda para os amigos. Eles clicam duas vezes e o Discord abre pelo túnel.

> Derivado do [DiscordGoLiveBypass](https://github.com/thomassolcia/DiscordGoLiveBypass), de Thomas Solcia, sob a mesma licença GPL-3.0. O lançador do Discord e a interface de console vêm de lá; o túnel WireGuard, o servidor e a gestão de acessos são desta versão.

## Por que um túnel, e não um proxy

Em agosto de 2026 a ANPD [mandou o Discord suspender transmissões ao vivo no Brasil](https://www.gov.br/anpd/pt-br/assuntos/noticias/em-medida-preventiva-anpd-determina-que-discord-suspenda-transmissoes-ao-vivo-no-brasil). O bloqueio não olha só o IP de onde vem a conexão com a API: o servidor de mídia do Discord olha o IP de onde vem o UDP do vídeo. Um proxy SOCKS só carrega TCP, então a API sai pelos EUA, o vídeo continua saindo do Brasil e quem assiste recebe o erro 2012.

A saída é pôr o processo inteiro do Discord no túnel, UDP incluído, **dos dois lados**: quem transmite e quem assiste. A história completa está em [`docs/architecture.md`](docs/architecture.md).

## O que você precisa

- **Um VPS fora do Brasil**, de qualquer provedor. O instalador foi testado em Ubuntu 24.04 e serve para Debian. O que pesa é a banda: a voz e o vídeo de quem usa passam todos pelo servidor.
- **UDP liberado** na porta do WireGuard (51820, por padrão) no firewall do provedor.
- **Para compilar:** [Go](https://go.dev/dl/) 1.22 ou mais novo e `python3`, em Linux, macOS ou WSL.
- **Para usar:** Windows 10 ou 11, 64 bits. O túnel por programa depende do [WireSock](https://wiresock.net), que só existe para Windows.

## Passo a passo

### 1. Montar o servidor

No VPS, como root:

```sh
git clone https://github.com/Shnrqpdr/golive-tunnel.git
cd golive-tunnel
sudo server/install.sh
```

O instalador põe o WireGuard e o firewall no ar e termina mostrando três linhas `GOLIVE_...`. Detalhes, instalação manual e endurecimento opcional em [`docs/server.md`](docs/server.md).

### 2. Compilar o executável

No seu computador, com o repositório clonado:

```sh
cp .env.example .env    # cole as três linhas que o install.sh mostrou
scripts/build.sh
```

O pacote sai em `dist/golive-tunnel-<versão>.zip`, com o `golive-tunnel.exe`, o guia [`COMO-USAR.txt`](docs/friend-guide.txt), o `SHA256SUMS.txt` e a licença. Mande o SHA256 que o script mostra por uma mensagem separada do zip, para o amigo conferir o arquivo.

O executável leva o endereço do seu servidor: mande só para quem você vai liberar.

### 3. Liberar os amigos

1. O amigo abre o `golive-tunnel.exe`. Na primeira vez ele mostra uma linha `golive-wg ...`, já copiada, que o amigo te manda.
2. Você libera, do Windows:

   ```powershell
   .\scripts\windows\golive-admin.cmd -Add joao -Key 'golive-wg AAAA...='
   ```

   ou de qualquer terminal com acesso SSH ao servidor:

   ```sh
   ssh root@seu-servidor golive-peer add joao AAAA...=
   ```

3. O amigo abre o programa de novo e aceita o aviso de administrador, só dessa vez. Daí em diante é duplo clique.

Para listar, cortar e dar a amigos de confiança o poder de liberar outros (os *liberadores*), veja [`docs/access.md`](docs/access.md). Se algo der errado, comece por [`docs/troubleshooting.md`](docs/troubleshooting.md).

## O que cada um vê

- **Quem cuida do servidor** vê o IP de quem usa, os horários e o volume de tráfego. O conteúdo vai cifrado pelo Discord.
- **O Discord** vê o IP do servidor, não o de quem usa.
- **O firewall do servidor limita portas, não destinos.** O executável só põe o Discord no túnel, mas quem tem uma chave liberada consegue mandar HTTPS para qualquer site pelo servidor. Libere só quem você conhece.
- **O WireSock** é gratuito para uso pessoal e de código fechado, e a versão gratuita manda relatórios de falha para a Sentry, sem opção de desligar. O executável baixa o instalador do site oficial e confere o SHA256 antes de rodar.

## Estrutura do repositório

| caminho | conteúdo |
| --- | --- |
| [`cmd/golive-tunnel/`](cmd/golive-tunnel) | o executável: opções e a sequência de abertura |
| [`internal/tunnel/`](internal/tunnel) | chave WireGuard, configuração do WireSock, serviço do Windows, etapa de administrador e vigia |
| [`internal/discord/`](internal/discord) | instalação do Discord, instância única, abertura |
| [`internal/netcheck/`](internal/netcheck) | prova de que o Discord responde pelo túnel e de que a saída não é no Brasil |
| [`internal/ui/`](internal/ui), [`internal/process/`](internal/process) | console e processos do Windows |
| [`server/`](server) | `install.sh`, `golive-peer`, firewall e configurações do servidor |
| [`scripts/build.sh`](scripts/build.sh) | build com o servidor do `.env` e montagem dos pacotes |
| [`scripts/windows/`](scripts/windows) | `golive-admin` (dono) e `golive-delegate` (liberador) |
| [`docs/`](docs) | servidor, acessos, arquitetura, problemas comuns e o guia que vai no zip |

## Desenvolvimento

```sh
go test ./...                     # partes portáveis
GOOS=windows go test ./...        # tudo; no WSL roda pelo interop do Windows
```

O CI roda os testes num Windows de verdade e o `shellcheck` nos scripts do servidor. Convenções de código, de idioma e de commit estão em [`AGENTS.md`](AGENTS.md).

## Licença

GPL-3.0, como o projeto original (ver [`LICENSE`](LICENSE)). Quem recebe o executável tem direito ao código-fonte correspondente; o `COMO-USAR.txt` aponta para este repositório e para a versão compilada.
