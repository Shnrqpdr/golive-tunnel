# Servidor

O servidor é um VPS fora do Brasil com WireGuard numa interface `wg0` e um firewall `nftables` que só deixa passar o que o Discord usa. Cada amigo é um *peer* WireGuard, liberado pelo `golive-peer`.

## Requisitos

- Ubuntu 24.04 (testado) ou Debian, com acesso root.
- Um IPv4 público. Em nuvens com NAT (AWS, GCP, Oracle), o IP público é o do painel do provedor, não o da placa de rede.
- A porta UDP do WireGuard (51820, por padrão) liberada no firewall do provedor, se ele tiver um.

## Instalação

```sh
git clone https://github.com/Shnrqpdr/golive-tunnel.git
cd golive-tunnel
sudo server/install.sh             # ou: sudo server/install.sh --port 443
```

O `install.sh`:

1. instala `wireguard-tools` e `nftables`;
2. descobre a interface da rota padrão (`eth0`, `ens3`...) e grava o firewall em `/etc/wireguard/golive.nft` com ela;
3. gera a chave do servidor e o `/etc/wireguard/wg0.conf`, se ele ainda não existir;
4. liga o encaminhamento de pacotes (`/etc/sysctl.d/90-golive-wg.conf`);
5. instala `golive-peer` e `golive-delegate-login` em `/usr/local/sbin`;
6. habilita e liga o `wg-quick@wg0`;
7. mostra as três linhas do `.env`: endereço, chave pública do WireGuard e identidade SSH do servidor.

Ele pode rodar de novo a qualquer momento: um `wg0.conf` que já existe, com a chave e os amigos liberados, é mantido, e com o `wg0` no ar ele só recarrega o firewall, sem derrubar quem está conectado. Para atualizar o servidor depois de um `git pull`, é só rodar o `install.sh` outra vez.

Se o `ufw` estiver ativo, o instalador avisa: ele descarta pacotes encaminhados, e um descarte em qualquer tabela vence o aceite da nossa. Os dois comandos para liberar aparecem no aviso.

### Instalação manual

Para outra distribuição, ou para entender cada peça:

```sh
apt install wireguard-tools nftables

install -d -m 700 /etc/wireguard
sed 's|^define wan = .*|define wan = "eth0"|' server/golive.nft > /etc/wireguard/golive.nft   # sua interface
(umask 077 && sed "s|^PrivateKey = .*|PrivateKey = $(wg genkey)|" server/wg0.conf.example > /etc/wireguard/wg0.conf)

cp server/sysctl-golive.conf /etc/sysctl.d/90-golive-wg.conf && sysctl --system
install -m 700 server/golive-peer /usr/local/sbin/golive-peer
install -m 755 server/golive-delegate-login /usr/local/sbin/golive-delegate-login
systemctl enable --now wg-quick@wg0
```

Os valores do `.env`:

```sh
wg show wg0 public-key                                 # GOLIVE_SERVER_KEY
cut -d' ' -f1,2 /etc/ssh/ssh_host_ed25519_key.pub      # GOLIVE_SSH_HOST_KEY
```

## O firewall

O `golive.nft` cria a tabela `inet golive` e só filtra o que entra pelo `wg0`:

| regra | por quê |
| --- | --- |
| TCP 443, 2053, 2083, 2087, 2096, 8443 | HTTPS do Discord e as portas alternativas da Cloudflare usadas pelo RTC |
| UDP 443, 19294-19344, 50000-65535 | mídia de voz e vídeo |
| nada para redes internas (10/8, 172.16/12, 192.168/16, 100.64/10...) | quem está no túnel não alcança a rede do provedor |
| nada de `wg0` para `wg0` | um amigo não enxerga o outro |
| nada de entrada pelo `wg0` | pelo túnel ninguém alcança o próprio servidor |
| NAT na saída | o tráfego sai com o IP do servidor |

O resto do servidor (SSH, por exemplo) não passa por essas regras.

## Endurecimento opcional

**SSH só por chave.** O [`sshd-hardening.conf`](../server/sshd-hardening.conf) desliga o login por senha, deixa o root entrar só com chave e ignora o `authorized_keys2`, onde alguns provedores instalam a chave do suporte deles. Antes de instalar, confirme que você entra com chave:

```sh
cp server/sshd-hardening.conf /etc/ssh/sshd_config.d/00-golive-hardening.conf
sshd -t && systemctl reload ssh
```

**IPv6 que não roteia.** Alguns provedores anunciam IPv6 sem rotear o tráfego, e o servidor fica pendurado tentando IPv6 em destinos que têm os dois. Para preferir IPv4:

```sh
cat server/gai.conf.snippet >> /etc/gai.conf
```

## O que fica instalado

| caminho | conteúdo |
| --- | --- |
| `/etc/wireguard/wg0.conf` | chave do servidor e um bloco `[Peer]` por amigo, com `# name:` e `# added-by:` |
| `/etc/wireguard/golive.nft` | o firewall, carregado pelo `PostUp` do `wg0` |
| `/etc/wireguard/delegates` | um liberador por linha, com o limite dele |
| `/etc/sysctl.d/90-golive-wg.conf` | `net.ipv4.ip_forward = 1` |
| `/usr/local/sbin/golive-peer` | libera, lista e corta amigos e liberadores |
| `/usr/local/sbin/golive-delegate-login` | comando forçado das chaves dos liberadores |
| `/home/golive-delegate/.ssh/authorized_keys` | uma chave por liberador, criada no primeiro `delegate-add` |
| `/etc/sudoers.d/golive-delegate` | deixa o usuário `golive-delegate` rodar só `golive-peer as ...` |

O que cada um faz fica no journal:

```sh
journalctl -t golive-peer -t golive-delegate
```

## Remover tudo

```sh
systemctl disable --now wg-quick@wg0
userdel -r golive-delegate 2>/dev/null
rm -f /usr/local/sbin/golive-peer /usr/local/sbin/golive-delegate-login \
      /etc/sudoers.d/golive-delegate /etc/sysctl.d/90-golive-wg.conf
rm -rf /etc/wireguard        # leva junto a chave do servidor e a lista de amigos
```
