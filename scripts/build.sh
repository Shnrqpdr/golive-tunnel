#!/usr/bin/env bash
# Builds golive-tunnel.exe with the server from .env embedded, and assembles in dist/ the package
# for friends and the kit for delegates.
#
# usage: scripts/build.sh            (VERSION=x.y.z scripts/build.sh to set the version by hand)
#
# Messages are in Portuguese, like the rest of what the server owner reads.
set -euo pipefail
cd "$(dirname "$0")/.."

[ -f .env ] || { echo "falta o .env: copie o .env.example e preencha" >&2; exit 1; }
# shellcheck disable=SC1091
source .env

# .env becomes a build flag and the executable goes to other people: nothing outside the format.
[[ "${GOLIVE_SERVER:-}" =~ ^[A-Za-z0-9.-]+:[0-9]{1,5}$ ]] ||
    { echo "GOLIVE_SERVER invalido no .env: use host:porta" >&2; exit 1; }
[[ "${GOLIVE_SERVER_KEY:-}" =~ ^[A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=$ ]] ||
    { echo "GOLIVE_SERVER_KEY invalida no .env: e a saida de wg show wg0 public-key" >&2; exit 1; }
[[ "${GOLIVE_SSH_HOST_KEY:-}" =~ ^ssh-ed25519\ AAAAC3NzaC1lZDI1NTE5[A-Za-z0-9+/]{48}$ ]] ||
    { echo "GOLIVE_SSH_HOST_KEY invalida no .env: e o /etc/ssh/ssh_host_ed25519_key.pub do servidor, sem o comentario" >&2; exit 1; }

version=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
[[ "$version" =~ ^[A-Za-z0-9._+-]+$ ]] || { echo "versao invalida: $version" >&2; exit 1; }
package=dist/golive-tunnel
kit=dist/golive-delegate

go vet ./...
GOOS=windows go vet ./...
# Tests with Windows paths and structs only count when compiled for Windows. Under WSL, interop
# runs the test .exe; elsewhere the portable tests run here and CI runs the rest on Windows.
if [ -x /mnt/c/Windows/System32/cmd.exe ]; then
    GOOS=windows go test -count=1 ./...
else
    go test -count=1 ./...
    echo "aviso: sem Windows a mao, os testes so do Windows ficam para o CI" >&2
fi

rm -rf dist
mkdir -p "$package" "$kit"
# -trimpath and -buildvcs=false make the build reproducible: the same code and the same .env give
# the same SHA256, with or without pending changes in git.
GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=false \
    -ldflags "-X main.version=$version -X main.server=$GOLIVE_SERVER -X main.serverKey=$GOLIVE_SERVER_KEY" \
    -o "$package/golive-tunnel.exe" ./cmd/golive-tunnel

# Opened in Notepad, so it goes in CRLF. The version tells friends which source code it is.
sed -e "s|__VERSION__|$version|" -e 's/\r*$/\r/' docs/friend-guide.txt > "$package/COMO-USAR.txt"
cp LICENSE "$package/"
(cd "$package" && sha256sum golive-tunnel.exe | sed 's/$/\r/' > SHA256SUMS.txt)

# Only for delegates, not in everyone's zip: it carries the server's address and SSH identity.
sed -e "s|__SERVER__|${GOLIVE_SERVER%:*}|" -e "s|__HOST_KEY__|$GOLIVE_SSH_HOST_KEY|" scripts/windows/golive-delegate.ps1 |
    sed 's/\r*$/\r/' > "$kit/golive-delegate.ps1"
sed 's/\r*$/\r/' scripts/windows/golive-delegate.cmd > "$kit/golive-delegate.cmd"

(cd dist && python3 -I -m zipfile -c "golive-tunnel-$version.zip" golive-tunnel)
(cd dist && python3 -I -m zipfile -c "golive-delegate-$version.zip" golive-delegate)

echo
echo "pacote dos amigos:    dist/golive-tunnel-$version.zip"
echo "kit dos liberadores:  dist/golive-delegate-$version.zip"
echo "SHA256 do executavel, para mandar separado do zip:"
echo "    $(cut -d' ' -f1 "$package/SHA256SUMS.txt")"
