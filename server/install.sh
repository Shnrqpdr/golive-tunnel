#!/usr/bin/env bash
# Sets up, or updates, the golive-tunnel exit server on Ubuntu or Debian: WireGuard on wg0, the
# nftables firewall and the access scripts. Safe to run again: an existing wg0.conf, with the
# server key and the friends already let in, is kept, and connected friends are not dropped.
#
# usage: sudo ./install.sh [--port 51820]
#
# Messages are in Portuguese, like the rest of what the server owner reads.
set -euo pipefail
cd "$(dirname "$0")"

die() { echo "erro: $*" >&2; exit 1; }

port=51820
while [ $# -gt 0 ]; do
    case "$1" in
    --port) port=${2:-}; shift 2 || die "falta o numero depois de --port" ;;
    -h|--help) sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "opcao desconhecida: $1 (use --help)" ;;
    esac
done
if ! [[ "$port" =~ ^[0-9]{1,5}$ ]] || [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
    die "porta invalida: $port"
fi

[ "$(id -u)" -eq 0 ] || die "rode como root: sudo $0"
command -v apt-get >/dev/null || die "este instalador so conhece Ubuntu e Debian (apt-get)"

# Forwarding and NAT only go out through the interface that holds the default route.
wan=$(ip -o -4 route show to default | awk '{ print $5; exit }')
[ -n "$wan" ] || die "nao achei a rota padrao IPv4: o servidor esta sem internet?"

echo "==> instalando wireguard-tools e nftables"
export DEBIAN_FRONTEND=noninteractive
apt-get update -q >/dev/null
apt-get install -y -q wireguard-tools nftables >/dev/null

echo "==> firewall do tunel, saindo por $wan"
install -d -m 700 /etc/wireguard
sed "s|^define wan = .*|define wan = \"$wan\"|" golive.nft > /etc/wireguard/golive.nft
nft -c -f /etc/wireguard/golive.nft || die "o nftables recusou o /etc/wireguard/golive.nft"

conf=/etc/wireguard/wg0.conf
if [ -f "$conf" ]; then
    echo "==> mantendo o $conf que ja existe (chave do servidor e amigos)"
    port=$(awk '$1 == "ListenPort" { print $3 }' "$conf")
else
    echo "==> gerando a chave do servidor"
    (umask 077 && sed -e "s|^PrivateKey = .*|PrivateKey = $(wg genkey)|" -e "s|^ListenPort = .*|ListenPort = $port|" wg0.conf.example > "$conf")
fi

echo "==> scripts de acesso e encaminhamento de pacotes"
install -m 644 sysctl-golive.conf /etc/sysctl.d/90-golive-wg.conf
sysctl -q -p /etc/sysctl.d/90-golive-wg.conf
install -m 700 golive-peer /usr/local/sbin/golive-peer
install -m 755 golive-delegate-login /usr/local/sbin/golive-delegate-login

echo "==> ligando o wg0"
systemctl enable -q wg-quick@wg0
if systemctl is-active -q wg-quick@wg0; then
    # Already up: reload only the firewall, without dropping whoever is connected.
    nft -f /etc/wireguard/golive.nft
else
    systemctl start wg-quick@wg0
fi

# ufw drops forwarded packets by default, and a drop in any table wins over the accept in ours.
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q '^Status: active'; then
    cat >&2 <<EOF

aviso: o ufw esta ativo e vai barrar o tunel. Libere com:
    ufw allow $port/udp
    ufw route allow in on wg0 out on $wan
EOF
fi

# Services that answer with the plain address; a NAT'd cloud server only knows its private one.
address=$(curl -4 -fsS --max-time 5 https://ifconfig.co 2>/dev/null || true)
if ! [[ "$address" =~ ^[0-9]+(\.[0-9]+){3}$ ]]; then
    address=$(ip -o -4 addr show dev "$wan" | awk '{ split($4, a, "/"); print a[1]; exit }')
fi
public_key=$(awk '$1 == "PrivateKey" { print $3 }' "$conf" | wg pubkey)
host_key=$(cut -d' ' -f1,2 /etc/ssh/ssh_host_ed25519_key.pub 2>/dev/null || echo "ssh-ed25519 (nao achei /etc/ssh/ssh_host_ed25519_key.pub)")

cat <<EOF

pronto. Coloque no .env de quem compila o executavel:

GOLIVE_SERVER=$address:$port
GOLIVE_SERVER_KEY=$public_key
GOLIVE_SSH_HOST_KEY="$host_key"

Confira se $address e mesmo o IP publico deste servidor, e se o firewall do
provedor deixa entrar UDP na porta $port.
EOF
