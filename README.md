# golive-tunel

Abre o Discord por um túnel WireGuard até um servidor seu, devolvendo o Go Live, a câmera e a transmissão de tela para quem está no Brasil. Só o Discord entra no túnel: navegador, jogos e o resto do computador continuam na internet normal.

> Derivado do [DiscordGoLiveBypass](https://github.com/thomassolcia/DiscordGoLiveBypass), de Thomas Solcia, sob a mesma licença GPL-3.0. A busca de proxies, o lançador do Discord e a interface de console vêm de lá; o túnel WireGuard, o servidor e a gestão de amigos são desta versão.

## Por que um túnel WireGuard, e não proxy

Em agosto de 2026 a ANPD [mandou o Discord suspender transmissões ao vivo no Brasil](https://www.gov.br/anpd/pt-br/assuntos/noticias/em-medida-preventiva-anpd-determina-que-discord-suspenda-transmissoes-ao-vivo-no-brasil). O projeto original desviava só a conexão com a API e o gateway, por proxy SOCKS, e deixava a mídia sair direto. Isso funcionava porque o bloqueio era decidido pelo IP do gateway.

Em outubro de 2026 os logs mostravam outra coisa. Com o gateway saindo pelos EUA e a mídia pelo Brasil:

- a transmissão caía em servidor de mídia de São Paulo (`c-gru*`);
- o servidor aceitava a conexão e repassava o áudio, mas o vídeo ficava em 0 bps: quem transmite captura a 30 fps e não codifica nada;
- quem assiste espera o primeiro quadro e recebe o erro **2012** (*Video Viewer Timeout*).

Proxy SOCKS só carrega TCP, e a mídia do Discord vai por UDP. A solução é o processo inteiro do Discord no túnel, UDP incluído, **dos dois lados**: com só quem transmite no túnel, o vídeo ainda não passa. Com os dois, a transmissão vai para servidores nos EUA (`c-iad*`, `c-ewr*`) e funciona.

## Como funciona

**No servidor** (um VPS fora do Brasil):

- WireGuard na interface `wg0`, porta UDP 51820;
- firewall `nftables` que só deixa passar o que o Discord usa (HTTPS e as portas de mídia), sem acesso à rede interna, sem um amigo enxergar o outro e sem alcançar o próprio servidor;
- `golive-peer`, que libera, lista e corta amigos. O endereço de cada um no túnel sai da própria chave pública, então ninguém precisa combinar IP.

**No Windows**, o executável compilado com o servidor embutido:

1. Na primeira vez, gera uma chave X25519 em `%LOCALAPPDATA%\DiscordGoLiveBypass\wireguard` e mostra a linha `golive-wg ...` para mandar a quem cuida do servidor. A chave privada nunca sai do computador.
2. Depois da liberação, pede administrador **uma vez** para:
   - baixar o [WireSock](https://wiresock.net) do site oficial, conferido por SHA256, e instalar;
   - escrever a configuração em `%ProgramData%\DiscordGoLiveBypass`, numa pasta que só administrador lê, porque o serviço roda como SYSTEM e a configuração leva a chave;
   - registrar o serviço `wiresock-client-service` com `-network-lock disabled`, para nunca cortar a internet do resto do computador, e dar ao usuário o direito de ligá-lo e desligá-lo.
3. A cada abertura, sem aviso nenhum: liga o serviço, confere que o Discord responde pelo túnel com saída fora do Brasil, abre o Discord sem flags, minimiza a janela e vigia. Se o serviço cair, liga de novo; quando o Discord fecha, desliga o túnel.

Só `Discord.exe`, `Update.exe` e o próprio executável entram no túnel (`AllowedApps` do WireSock).

**O que fica visível.** Quem cuida do servidor vê o IP de quem usa, os horários e o volume de tráfego; o conteúdo vai cifrado pelo Discord. O WireSock é gratuito para uso pessoal, de código fechado, e a versão gratuita manda relatórios de falha para a Sentry sem opção de desligar.

## Montar o servidor

Testado em Ubuntu 24.04. Os arquivos estão em [`servidor/`](servidor/).

```sh
apt install wireguard-tools nftables

# Chave do servidor: a privada fica no servidor, a pública vai para o .env.
cd /etc/wireguard && umask 077
wg genkey > servidor.chave && wg pubkey < servidor.chave > servidor.pub

cp servidor/golive.nft /etc/wireguard/
sed "s|<gerada no servidor>|$(cat servidor.chave)|" servidor/wg0.conf.exemplo > /etc/wireguard/wg0.conf
cp servidor/90-golive-wg.conf /etc/sysctl.d/ && sysctl --system
install -m 700 servidor/golive-peer /usr/local/sbin/golive-peer
install -m 755 servidor/golive-liberador /usr/local/sbin/golive-liberador
systemctl enable --now wg-quick@wg0
```

Confira se a placa de rede se chama `eth0` (`ip -br link`); se não, troque no `golive.nft`.

Recomendado também:

- **SSH só por chave**: `servidor/00-endurecer.conf` em `/etc/ssh/sshd_config.d/`. Ele desliga a senha e ignora o `authorized_keys2`, onde alguns provedores deixam a chave do suporte deles.
- **IPv6 quebrado do provedor**: se o VPS recebe IPv6 mas o tráfego IPv6 não sai, acrescente `servidor/gai.conf.trecho` ao `/etc/gai.conf`.

## Compilar e distribuir

Precisa do [Go](https://go.dev/dl/) e de `python3`. Num WSL, os testes rodam no Windows pelo interop.

```sh
cp .env.example .env    # preencha o endereço e a chave pública do servidor
./compilar-amigos.sh
```

O pacote sai em `dist/DiscordGoLiveBypass-amigos-AAAA-MM-DD.zip`, com o executável, o [`COMO-USAR.txt`](COMO-USAR.txt), o `SHA256SUMS.txt` e a licença. Ao lado sai o `dist/Liberador-*.zip`, só para quem for liberador. Mande o SHA256 do executável por uma mensagem separada do zip. O build é reproduzível (`-trimpath -buildvcs=false`): o mesmo código e o mesmo `.env` dão o mesmo SHA256.

Sem as variáveis do `.env` (um `go build` comum), o executável não tem túnel embutido e funciona como o original, mas esse modo não devolve mais o Go Live.

## Liberar e cortar amigos

O `Liberar-amigo.cmd` roda no Windows de quem cuida do servidor, no PowerShell ou no Prompt de Comando, e entra pelo apelido `vps` do `~/.ssh/config`, como root e com chave:

```
.\Liberar-amigo.cmd -Nome joao -Chave 'golive-wg AAAA...='
.\Liberar-amigo.cmd -Listar
.\Liberar-amigo.cmd -Remover joao
```

O `.cmd` só chama o `.ps1` com a política de scripts liberada para aquela execução: o Windows, por padrão, recusa `.ps1` digitado direto. A linha que o amigo manda é validada aqui e de novo no servidor, porque termina numa linha de comando do root. O `-Listar` mostra quem liberou cada um e o último contato.

### Liberadores

Um liberador é um amigo que libera outros sozinho, sem acesso ao servidor: ele libera até o limite dele, vê a lista inteira e só corta quem ele mesmo liberou.

1. Mande para ele o `dist/Liberador-*.zip`. O `compilar-amigos.sh` gera o kit com o endereço e a identidade SSH do servidor tirados do `.env`.
2. Ele roda `.\Liberar-pessoal.cmd` uma vez. O script gera a chave dele e mostra uma linha `golive-liberador ssh-ed25519 ...`, que ele te manda.
3. Você dá a permissão: `.\Liberar-amigo.cmd -Liberador maria -Chave 'golive-liberador ssh-ed25519 ...' -Limite 20`.
4. Daí em diante ele usa `.\Liberar-pessoal.cmd -Nome`, `-Listar` e `-Remover` como você.

Você acompanha com `-Liberadores` e tira a permissão com `-TirarLiberador maria`. Quem ele liberou continua com acesso até você cortar.

No servidor, todos os liberadores entram pelo usuário `liberador`, sem senha. Cada chave tem no `authorized_keys` um comando forçado, `golive-liberador <nome>`: o SSH ignora o que for pedido e só roda esse script, então quem conecta não escolhe quem é. O script aceita só `add`, `remove` e `list`, com palavras simples, registra o pedido no log (`journalctl -t golive-liberador`) e chama `sudo golive-peer como <nome>`, a única coisa que o `sudo` permite a esse usuário. A chave tem `restrict`, então não abre terminal nem encaminha porta.

## Problemas comuns

<details>
<summary><strong>"o tunel ligou, mas nada passou por ele"</strong></summary>
<br>

O WireGuard não avisa quando o servidor não conhece a chave: os pacotes só ficam sem resposta. Quase sempre a chave ainda não foi liberada; a mensagem já deixa a linha `golive-wg` copiada para mandar de novo. Se ela já aparece no `-Listar`, confira se o servidor está ligado e se a porta UDP 51820 chega até ele.
</details>

<details>
<summary><strong>Go Live carregando para sempre, ou erro 2012</strong></summary>
<br>

Quem transmite e quem assiste precisam estar no túnel. Com só um dos dois, o servidor de mídia continua segurando o vídeo.
</details>

<details>
<summary><strong>"este programa foi aberto como administrador"</strong></summary>
<br>

Aberto como administrador, ele abriria o Discord como administrador também. Abra com duplo clique normal: quando precisa, o próprio programa pede o aviso do Windows.
</details>

<details>
<summary><strong>"o aviso de administrador foi recusado"</strong></summary>
<br>

O aviso só aparece na primeira vez, ou quando muda algo do túnel, como a chave, o servidor ou o nome do executável. É quando o programa instala o WireSock e prepara o serviço. Abra de novo e aceite.
</details>

<details>
<summary><strong>"ja existe um WireSock rodando fora do servico"</strong></summary>
<br>

Outro programa, ou um `wiresock-client run` aberto à mão, está usando o WireSock. Feche-o antes, porque dois ao mesmo tempo brigam pelo mesmo filtro de rede.
</details>

## Estrutura

| arquivo | conteúdo |
| --- | --- |
| `tunel*.go` | chave WireGuard, configuração do WireSock, serviço do Windows, etapa de administrador e vigia |
| `main.go` | opções e orquestração, do túnel embutido ao modo original |
| `proxy.go` | teste até a resposta 200 do Discord e o país de saída, por túnel ou por proxy |
| `source.go` | modo original: lista pública, Tor, cache |
| `discord.go` | detecção da instalação, instância única, abertura |
| `ui.go`, `console*.go` | etapas, caixas, cores e o comportamento da janela no duplo clique |
| `servidor/` | WireGuard, firewall, `golive-peer`, `golive-liberador` e endurecimento do VPS |
| `compilar-amigos.sh` | build com o servidor do `.env` e montagem do pacote |
| `Liberar-amigo.cmd` / `.ps1` | dono: liberar, listar e cortar amigos e liberadores, como root pelo SSH |
| `Liberar-pessoal.cmd` / `.ps1` | liberador: liberar, listar e cortar com a permissão que o dono deu (modelo; a versão pronta sai em `dist/`) |
| `COMO-USAR.txt` | guia que vai no pacote dos amigos |

## Licença

GPL-3.0, como o projeto original (ver [`LICENSE`](LICENSE)). Quem recebe o executável tem direito ao código-fonte correspondente.
