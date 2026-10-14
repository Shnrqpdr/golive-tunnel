# Como funciona

## Por que WireGuard, e não proxy

O [DiscordGoLiveBypass](https://github.com/thomassolcia/DiscordGoLiveBypass) original abria o Discord com um proxy SOCKS ou HTTP: só a API e o gateway saíam por fora, e a mídia ia direto. Isso funcionava enquanto o bloqueio era decidido pelo IP do gateway.

Em outubro de 2026 os logs do Discord mostravam outra coisa. Com o gateway saindo pelos EUA e a mídia pelo Brasil:

- a transmissão caía num servidor de mídia de São Paulo (`c-gru*`);
- o servidor aceitava a conexão e repassava o áudio, mas o alvo do codificador de vídeo ficava em 0 bps: quem transmite captura a 30 fps e não codifica nada;
- quem assiste espera o primeiro quadro e recebe o erro **2012** (*Video Viewer Timeout*).

O servidor de mídia decide pelo IP de onde vem o UDP, e proxy SOCKS só carrega TCP. Pôr o processo inteiro do Discord num túnel WireGuard resolve, mas **dos dois lados**: com só quem transmite no túnel, o vídeo ainda não passa. Com os dois, a transmissão vai para servidores de mídia nos EUA (`c-iad*`, `c-ewr*`) e funciona. Isso foi validado nas duas direções.

## No Windows

O executável é compilado com o servidor embutido (`-ldflags -X main.server=... -X main.serverKey=...`). Um `go build` comum não tem servidor e se recusa a rodar.

### Primeira execução

Gera uma chave X25519 em `%LOCALAPPDATA%\golive-tunnel\wireguard.key`, mostra a linha `golive-wg <chave pública>`, copia para a área de transferência e para. Conectar agora daria um erro que não ensina nada, porque o servidor ainda não conhece a chave.

### Etapa de administrador, uma vez

Na execução seguinte, o programa compara a configuração que montaria agora com a instalada. Se for diferente, ou se ainda não houver nada, ele se reabre como administrador (`-setup-wiresock <chave> -sid <SID>`), sem janela, e:

1. baixa o instalador do WireSock SDK 3.4.8.1 do site oficial e confere o SHA256 fixado no código antes de rodar;
2. grava a configuração em `%ProgramData%\golive-tunnel\wiresock.conf`, numa pasta que só SYSTEM e administradores leem, porque o serviço roda como SYSTEM e a configuração leva a chave privada;
3. registra o serviço `wiresock-client-service` com início manual e `-network-lock disabled`: ligado, o *network lock* cortaria a internet do computador inteiro quando o túnel caísse;
4. acrescenta ao descritor de segurança do serviço uma entrada `(A;;LCRPWP;;;<SID>)`, que deixa só aquele usuário consultar, ligar e desligar o serviço.

A etapa de administrador só lê da pasta do usuário a chave; o resto vem do próprio executável. A configuração nunca leva `PostUp`, `PreUp` ou `DNS`, que o serviço executaria como SYSTEM.

O aviso volta a aparecer só quando muda algo que entra na configuração: a chave, o servidor ou o nome do executável.

### Cada abertura

Sem aviso nenhum:

1. acha o Discord em `%LOCALAPPDATA%` (stable, PTB ou Canary), na pasta `app-<versão>` completa mais nova;
2. liga o serviço pelo gerenciador de serviços do Windows;
3. prova que o túnel funciona: o Discord precisa responder `200` pelo túnel, e o país de saída (pelo `ifconfig.co`) não pode estar na lista de recusados (`-exclude`, `BR` por padrão). O WireGuard não avisa quando o servidor desconhece a chave; os pacotes só ficam sem resposta, então essa é a única prova;
4. fecha o Discord que já estava aberto, porque ele é instância única e um segundo processo só acordaria a janela antiga, fora do túnel;
5. abre o Discord sem nenhuma flag, minimiza a própria janela e fica vigiando.

Se o serviço cair, a vigia liga de novo. Quando o Discord fica fechado por duas checagens seguidas, ela desliga o túnel; uma checagem só não basta, porque o atualizador do Discord fecha e reabre o processo. Fechar a janela do programa também desliga o túnel.

Uma instância só desliga o túnel que ela mesma ligou, até começar a vigiar: um `-check` com o Discord aberto não derruba a conexão de quem está usando.

Entram no túnel só `Discord.exe`, `Update.exe` e o próprio executável (a linha `#@ws:AllowedApps` da configuração do WireSock).

### Versões antigas

Antes de virar `golive-tunnel`, o executável se chamava `DiscordGoLiveBypass.exe` e guardava a chave em `%LOCALAPPDATA%\DiscordGoLiveBypass\wireguard\privada`. O executável novo move essa chave para o caminho novo, então quem já tinha acesso continua com o mesmo endereço. A etapa de administrador apaga a cópia antiga em `%ProgramData%\DiscordGoLiveBypass`.

## No servidor

- `wg0` em `10.66.0.1/16`, UDP 51820.
- Tabela `nftables` `inet golive`: só as portas do Discord saem, sem acesso a redes internas, sem um amigo alcançar o outro e sem alcançar o próprio servidor pelo túnel. Detalhes em [`server.md`](server.md#o-firewall).
- `golive-peer` mantém um bloco `[Peer]` por amigo no `wg0.conf`, com o nome e quem liberou em comentários, e aplica a mudança na hora com `wg set`, sem reiniciar a interface.
- O endereço de cada amigo é `10.66.(h[0] % 254 + 1).(h[1] % 254 + 1)`, com `h` o SHA256 da chave pública. O teste `TestAddressFor` e o `golive-peer` usam a mesma chave de exemplo para as duas contas não divergirem.

## Validações repetidas de propósito

Os formatos de nome e de chave são conferidos em mais de um lugar, porque cada lado recebe dados de outra pessoa:

| formato | onde |
| --- | --- |
| chave WireGuard `^[A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=$` | `golive-peer`, `golive-admin.ps1`, `golive-delegate.ps1`, `scripts/build.sh` |
| nome `^[A-Za-z0-9_-]{1,32}$` | `golive-peer`, `golive-admin.ps1`, `golive-delegate.ps1` |
| chave SSH ed25519 | `golive-peer`, `golive-admin.ps1`, `scripts/build.sh` |

Mudou um, mude todos.
