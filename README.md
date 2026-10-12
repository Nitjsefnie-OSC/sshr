# sshr

## About

sshr is a desktop SSH hosts manager that keeps your server list local and under your control. It runs as a native Gio UI on your machine, stores hosts as JSON in a local vault (`~/.sshr/vault`), and is built to grow into CLI commands and optional git-based sync across devices — without requiring an agent or special setup on remote systems.

It currently supports:

- Adding and listing SSH hosts (name, host/IP, user, port)
- CLI commands: `list`, `add`, `rm`, `connect`, `paste`, `help`
- Private key support (`-k` flag or interactive selection)
- Auto-discovery of SSH keys in `~/.ssh/`
- Interactive authentication method prompt when adding hosts; `--no-key` for password hosts in scripts (with no answer on stdin `add` stops instead of guessing)
- Local vault storage per host file (0600 permissions)
- Settings with interface language (Russian / English)
- Resizable sidebar layout and icon navigation
- Optional mosh transport, off by default (see [Connecting with mosh](#connecting-with-mosh))
- Sending a clipboard image to a server, from the CLI and the app (see [Sending images to a server](#sending-images-to-a-server))

## Connecting with mosh

[mosh](https://mosh.org) keeps a session alive when your network changes or your laptop sleeps, and shows what you type immediately even on a slow link. It is **off by default**, because it has real costs:

- needs `mosh` on your computer, and `mosh-server` plus open UDP ports 60000–61000 on the server;
- no port forwarding and no agent forwarding;
- no scrollback unless you run tmux/screen on the server;
- terminal image protocols (kitty, sixel) do not pass through;
- no native Windows client.

Turn it on in **Settings → Connection → Connect with mosh** (stored as `"mosh": true` in `~/.sshr/settings.json`), or per call:

```sh
sshr connect --mosh web   # use mosh this time
sshr connect --ssh web    # plain ssh even if mosh is on
```

If mosh is not installed, or it fails within the first 20 seconds (no mosh-server, UDP blocked), sshr falls back to plain ssh and says so. A key path with spaces cannot be passed through mosh, so such hosts always use ssh.

## Sending images to a server

Pasting a screenshot into a remote session gives the server a path on *your* disk, which it cannot open. Remote Claude Code, an editor or a script just gets a useless string. sshr uploads the image to the host instead and gives you the path on the server.

```sh
sshr paste web              # clipboard image -> server, prints the remote path
sshr paste -c web           # same, and puts that path into the clipboard
sshr paste -f shot.png web  # upload a file instead of the clipboard
```

In the app, every host has a **Send image** button. The remote path goes straight into the clipboard, so you paste it into the session.

How it works:

- a separate `ssh` with the host's saved key and port writes the file to `~/.cache/sshr/pasted/` on the server and prints its path;
- the file is removed after 15 minutes by a detached `sleep`, and every upload also deletes older leftovers, in case that `sleep` was killed by a logout;
- the server needs only `sh`; the command runs under `sh -c`, so a fish or zsh login shell is fine;
- the CLI can ask for a password on the terminal. The app has no terminal, so it uses `BatchMode` and works only with a key or ssh-agent;
- `-c` is opt-in because it replaces the image in your clipboard with the path.

Reading the clipboard uses tools the OS already has, so the binary needs no cgo:

| OS | read image | copy path (`-c`) |
|----|------------|------------------|
| Linux, Wayland | `wl-paste` (wl-clipboard) | `wl-copy` |
| Linux, X11 | `xclip` | `xclip` |
| macOS | `osascript` (built in) | `pbcopy` |
| Windows | PowerShell (built in) | `clip` |

On Linux, install `wl-clipboard` or `xclip` if you don't have them. The same scheme already works in the author's terminal (MyCustomTerm).

## Planned

In the order we intend to build them (reasons in [docs/WHY.md](docs/WHY.md)):

1. Real-server check of the CLI ([#17](https://github.com/klovbin/sshr/issues/17))
2. Connect / session workflows on top of system SSH ([#16](https://github.com/klovbin/sshr/issues/16)):
   - jump hosts via `ProxyJump` ([#19](https://github.com/klovbin/sshr/issues/19)) — some servers are only reachable through a bastion;
   - attach to tmux/zellij on connect ([#20](https://github.com/klovbin/sshr/issues/20)) — the remote session survives a dropped connection, over plain ssh, on every OS;
   - saved port forwards per host ([#21](https://github.com/klovbin/sshr/issues/21)) — "open DB", "open admin panel" in one click;
   - connection reuse with `ControlMaster`/`ControlPersist` ([#22](https://github.com/klovbin/sshr/issues/22)) — a second session to the same host opens instantly;
   - sending clipboard images to the server — done ([#26](https://github.com/klovbin/sshr/issues/26));
   - mosh, off by default — done ([#23](https://github.com/klovbin/sshr/issues/23)).
3. Password handling: local secrets, then encrypted vault ([#15](https://github.com/klovbin/sshr/issues/15))
4. Cross-platform installer with OS auto-detection and PATH setup ([#11](https://github.com/klovbin/sshr/issues/11))
5. Sync with key services: 1Password, Bitwarden, ssh-agent ([#14](https://github.com/klovbin/sshr/issues/14))
6. REST API ([#12](https://github.com/klovbin/sshr/issues/12)) and MCP server ([#13](https://github.com/klovbin/sshr/issues/13)) for integrations and AI assistants
7. GitHub / git vault sync across machines (see the zero-knowledge idea below)

Not planned: Eternal Terminal (another server daemon for what tmux and mosh already cover) and autossh (meant for permanent tunnels, not interactive work).

Future ideas:

- Zero-knowledge sync with device-bound keys ([#10](https://github.com/klovbin/sshr/issues/10)) — the sync backend stores only ciphertext; each device keeps its own key pair (TPM where available) and the vault key is encrypted to every device, so there is no master password to brute-force; new devices are approved from an existing one by fingerprint / QR; paper recovery key; SSH private keys never leave their device
- Sync transport for it: direct device-to-device with no server at all — local network or the user's own WireGuard tunnel, TLS 1.3 with mutual auth and device keys pinned at pairing; git / own server only as a later opt-in fallback
- Optional hardware keys (YubiKey / FIDO2 `ed25519-sk`) — not required, buying a dedicated device just for sshr is a big ask
