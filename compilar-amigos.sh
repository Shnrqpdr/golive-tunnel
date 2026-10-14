#!/usr/bin/env bash
# Compila o executavel dos amigos, com o servidor do .env embutido, e monta o pacote em dist/.
# uso: ./compilar-amigos.sh
set -euo pipefail
cd "$(dirname "$0")"

[ -f .env ] || { echo "falta o .env: copie o .env.example e preencha" >&2; exit 1; }
# shellcheck disable=SC1091
source .env

# O .env vira flag de compilacao e o executavel vai para outras pessoas: nada fora do formato.
[[ "${GOLIVE_SERVIDOR:-}" =~ ^[A-Za-z0-9.-]+:[0-9]{1,5}$ ]] ||
    { echo "GOLIVE_SERVIDOR invalido no .env: use host:porta" >&2; exit 1; }
[[ "${GOLIVE_CHAVE_SERVIDOR:-}" =~ ^[A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=$ ]] ||
    { echo "GOLIVE_CHAVE_SERVIDOR invalida no .env: e a saida de wg show wg0 public-key" >&2; exit 1; }
[[ "${GOLIVE_SSH_IDENTIDADE:-}" =~ ^ssh-ed25519\ AAAAC3NzaC1lZDI1NTE5[A-Za-z0-9+/]{48}$ ]] ||
    { echo "GOLIVE_SSH_IDENTIDADE invalida no .env: e o ssh_host_ed25519_key.pub do VPS, sem o comentario" >&2; exit 1; }

versao="amigos-$(date +%Y-%m-%d)"
pacote="dist/DiscordGoLiveBypass"

go vet ./...
GOOS=windows go vet ./...
# Os testes com caminho do Windows so valem compilados para ele. No WSL o interop roda o .exe
# de teste; fora dele, o CI do GitHub roda os mesmos testes num Windows de verdade.
if [ -x /mnt/c/Windows/System32/cmd.exe ]; then
    GOOS=windows go test -count=1 ./...
else
    echo "aviso: sem Windows a mao, os testes ficam para o CI" >&2
fi

rm -rf dist
mkdir -p "$pacote"
# -trimpath e -buildvcs=false deixam o build reproduzivel: o mesmo codigo e o mesmo .env dao
# o mesmo SHA256, com ou sem mudanca pendente no git.
GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=false \
    -ldflags "-X main.version=$versao -X main.tunelServidor=$GOLIVE_SERVIDOR -X main.tunelChaveServidor=$GOLIVE_CHAVE_SERVIDOR" \
    -o "$pacote/DiscordGoLiveBypass.exe" .

# O COMO-USAR e aberto no Bloco de Notas, entao vai em CRLF.
sed 's/\r*$/\r/' COMO-USAR.txt > "$pacote/COMO-USAR.txt"
cp LICENSE "$pacote/"
(cd "$pacote" && sha256sum DiscordGoLiveBypass.exe | sed 's/$/\r/' > SHA256SUMS.txt)
(cd dist && python3 -I -m zipfile -c "DiscordGoLiveBypass-$versao.zip" DiscordGoLiveBypass)

# Script de quem libera amigos sozinho: so vai para os liberadores, nao no zip de todo mundo.
host=${GOLIVE_SERVIDOR%:*}
mkdir -p dist/Liberador
sed -e "s|__SERVIDOR__|$host|" -e "s|__IDENTIDADE__|$GOLIVE_SSH_IDENTIDADE|" Liberar-pessoal.ps1 |
    sed 's/\r*$/\r/' > dist/Liberador/Liberar-pessoal.ps1
cp Liberar-pessoal.cmd dist/Liberador/
(cd dist && python3 -I -m zipfile -c "Liberador-$versao.zip" Liberador)

echo
echo "pacote: dist/DiscordGoLiveBypass-$versao.zip"
echo "kit de liberador: dist/Liberador-$versao.zip"
echo "SHA256 do executavel, para mandar separado do zip:"
echo "    $(cut -d' ' -f1 "$pacote/SHA256SUMS.txt")"
