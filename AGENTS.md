# AGENTS.md

Guidance for coding agents (and humans) working on this repository. User docs are in `README.md` and `docs/`, in Portuguese.

## What this is

`golive-tunnel` opens Discord on Windows through a WireGuard tunnel to a self-hosted server outside Brazil, so Go Live works again. Each server owner builds the executable with their server baked in and hands it to friends; only Discord's processes go into the tunnel, through the WireSock client service.

The one fact the whole design rests on: Discord's media server (SFU) decides by the IP the **media UDP** comes from. A SOCKS proxy or SSH tunnel (TCP only) moves the API abroad but leaves the media in Brazil, and viewers get error 2012. So the whole Discord process goes into the tunnel, UDP included, on **both** sides (streamer and viewer). Do not reintroduce a proxy mode: it was removed because it no longer works.

It is a GPL-3.0 fork of `thomassolcia/DiscordGoLiveBypass`.

## Layout

```
cmd/golive-tunnel/    main: flags, the open sequence, the single signal handler
internal/tunnel/      key, WireSock config, Windows service (SCM), elevated setup, Watch
internal/discord/     install detection (newest complete app-<version>), launch, single instance
internal/netcheck/    Probe (Discord 200 through the tunnel), exit country, error descriptions
internal/ui/          console output, Windows console mode, clipboard, country names
internal/process/     tasklist/taskkill/start by image name
server/               install.sh, golive-peer, golive-delegate-login, nftables, configs
scripts/build.sh      vet + tests + reproducible build with the server from .env, packages in dist/
scripts/windows/      golive-admin (owner) and golive-delegate (delegate kit template), .ps1 + .cmd
docs/                 Portuguese docs; friend-guide.txt ships as COMO-USAR.txt
```

## Commands

```sh
go vet ./... && GOOS=windows go vet ./...
go test ./...                    # portable tests
GOOS=windows go test ./...       # everything; under WSL the test .exe runs through interop
scripts/build.sh                 # needs .env (copy .env.example); output in dist/
```

CI (`.github/workflows/ci.yml`) runs vet/test/build on `windows-latest` and gofmt + shellcheck on Ubuntu.

Server scripts are tested in a disposable container, never on a real server:

```sh
docker run --rm --cap-add NET_ADMIN -v "$PWD/server:/src:ro" ubuntu:24.04 bash
# inside: apt-get install iproute2 curl openssh-server sudo shellcheck; ssh-keygen -A;
# stub /usr/local/bin/systemctl so "start wg-quick@wg0" runs "wg-quick up wg0" and
# "is-active -q wg-quick@wg0" checks "ip link show wg0"; then cp -r /src /work && /work/install.sh
```

The WSL2 kernel has WireGuard and nf_tables, so `wg0` and the firewall come up for real there.

## Conventions

- **Language:** identifiers, comments, file names, commit messages and this file in English. Everything a user reads at runtime (executable messages, script output, server script output) and the docs in `docs/` and `README.md` are in Portuguese. Runtime strings have **no accents** (old Windows consoles and code pages); docs use normal Portuguese.
- Comments explain **why**, not what. Match the density of the surrounding code.
- `gofmt`; no dependencies outside the standard library.
- `.ps1` and `.cmd` are CRLF (`.gitattributes` enforces it); everything else LF.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `refactor:`, `docs:`, `ci:`, `chore:`, with an optional scope (`feat(server): ...`). No emoji.
- No secrets or deployment details in the repo: server address, keys and peer names live in `.env` (gitignored) and on the server.

## Invariants

Breaking any of these breaks existing installs or opens a hole. Check before changing related code.

1. **Address derivation** is `10.66.(sha256(pub)[0] % 254 + 1).(sha256(pub)[1] % 254 + 1)` in both `tunnel.AddressFor` and `server/golive-peer`. `TestAddressFor` pins the sample key to `10.66.125.59`; the server must give the same.
2. **The WireSock config never carries anything that executes** (`PostUp`, `PreUp`, `PostDown`, `PreDown`, `DNS`): the service runs it as SYSTEM. `TestWireSockConfig` enforces it.
3. **Anything that changes `wireSockConfig` output** (key, server, executable name, `AllowedApps`) changes the fingerprint and makes every user accept a UAC prompt again. That is intended when it happens, never by accident.
4. **On-disk paths are part of the contract.** Key: `%LOCALAPPDATA%\golive-tunnel\wireguard.key`. Protected config: `%ProgramData%\golive-tunnel\wiresock.conf` (+ `.sha256`). Renaming any of them needs a migration like `moveLegacyKey`; a lost key means the friend must be let in again. A corrupt key is an error, never silently replaced.
5. **The program refuses to run elevated.** The elevated step (`-setup-wiresock`) reads only the key from the user's folder; everything else comes from the binary. The SID is validated before it goes into the SDDL.
6. **Tunnel ownership:** an instance stops only the service it started (`Tunnel.owned`), until `Watch` takes ownership. `-check` with a Discord open must not cut that Discord off.
7. **`-network-lock disabled`** on the service, always: enabled, it cuts the whole computer's internet when the tunnel drops.
8. **WireSock is pinned** by version and SHA256 in `setup_windows.go`. Bumping it means downloading the new installer, checking it, and updating both.
9. **Validation formats are duplicated on purpose** (each side receives data from someone else): the WireGuard key regex, the name regex and the ssh-ed25519 regex appear in `golive-peer`, `golive-admin.ps1`, `golive-delegate.ps1` and `scripts/build.sh`. Change one, change all.
10. **`golive-peer` runs as root**, reached by delegates through `sudo golive-peer as <name>`. No environment-based overrides, no new subcommands under `as` beyond `add|remove|list`, and the name `owner` stays reserved (it marks the owner's peers in `wg0.conf`).
11. The `KeyPrefix` `golive-wg` is what friends paste; the scripts strip it. The delegate line prefix is `golive-delegate` (the admin script also accepts the old `golive-liberador`).
12. Windows struct sizes (`SHELLEXECUTEINFOW`, `SERVICE_STATUS`, `QUERY_SERVICE_CONFIGW`) are pinned by `TestWindowsStructSizes`.

## Testing on a real Windows machine

The person running the agent is usually also a user of the tunnel. Do not disturb their setup:

- Run Windows binaries from a Windows path (`%TEMP%`), with `< /dev/null`.
- `-check` is safe even with Discord open. **`-force` defaults to true:** a run without `-check` kills the running Discord. For a full run, point `-exe` at a dummy program with a different image name.
- Use a fake profile so the real key is not touched: `LOCALAPPDATA='C:\...\fake' WSLENV=LOCALAPPDATA/w ./golive-tunnel.exe -check`. With a fake profile the run stops at the new-key notice; going further would trigger the elevated setup and reconfigure the real WireSock service.
- UAC prompts appear on the user's screen: warn before anything that elevates. An elevated process cannot be killed without elevation.
- The executable and the delegate script write to the clipboard. Save and restore it, and skip such tests if it holds something other than text (an image would be lost).
- From WSL, call PowerShell with `-File script.ps1`; `-Command -` drops multi-line blocks.
