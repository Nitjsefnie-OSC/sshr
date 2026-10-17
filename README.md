# sshr

## About

sshr is a desktop SSH hosts manager that keeps your server list local and under your control. It runs as a native Gio UI on your machine, stores hosts as JSON in a local vault (`~/.sshr/vault`), and is built to grow into CLI commands and optional git-based sync across devices — without requiring an agent or special setup on remote systems.

It currently supports:

- Adding and listing SSH hosts (name, host/IP, user, port)
- CLI commands: `list`, `add`, `rm`, `connect`, `paste`, `forward`, `tunnel`, `help`
- Private key support (`-k` flag or interactive selection)
- Auto-discovery of SSH keys in `~/.ssh/`
- Interactive authentication method prompt when adding hosts; `--no-key` for password hosts in scripts (with no answer on stdin `add` stops instead of guessing)
- Local vault storage per host file (0600 permissions)
- Settings with interface language (Russian / English)
- Resizable sidebar layout and icon navigation
- Optional mosh transport, off by default (see [Connecting with mosh](#connecting-with-mosh))
- Sending a clipboard image to a server, from the CLI and the app (see [Sending images to a server](#sending-images-to-a-server))
- Jump hosts per host, including chains (see [Jump hosts](#jump-hosts))
- Attaching to a tmux or zellij session on connect, off by default (see [Keeping the session on the server](#keeping-the-session-on-the-server))
- Named port forwards per host, opened with a session or as a tunnel (see [Port forwards](#port-forwards))

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

## Jump hosts

Some servers are reachable only through another one (a bastion), give the host a jump:

```sh
sshr add 10.0.0.7 -u app -k ~/.ssh/app -J bastion          # a saved host
sshr add 10.0.0.7 -u app -k ~/.ssh/app -J ops@gw:2222      # or user@host:port
sshr add 10.0.0.7 -u app -k ~/.ssh/app -J gw,bastion       # a chain: gw first, then bastion
```

In the app it is the **Jump host** field of the host form, `connect` and `paste` both go through it

- If every hop is a plain `user@host:port`, sshr passes it to ssh as `-J`
- If a hop is a saved host, ssh runs sshr itself as `ProxyCommand` (`sshr __proxy …`), because `-J` uses only your ssh config and default keys for the hops and would ignore the key or password saved for the bastion, through `ProxyCommand` every hop logs in with its own saved key or password
- A saved jump host may have its own jump, so a chain can also be built by saving hosts one behind another, a chain that points back at itself stops after 8 hops with an error
- mosh is not used through a jump host, it needs a direct UDP path to the server, sshr says so and connects with ssh

## Keeping the session on the server

A host can open a tmux or zellij session on connect instead of a plain shell, then a dropped connection does not kill your work, `sshr connect` again and you are back where you were, it is **off by default**

```sh
sshr add 10.0.0.7 -u app -k ~/.ssh/app --session tmux
sshr connect web            # attaches to the session "sshr", creates it the first time
sshr connect --plain web    # a plain shell this time
```

In the app it is **On connect open** in the host form: plain shell, tmux or zellij

- sshr runs `tmux new-session -A -s sshr` (or `zellij attach --create sshr`) under `sh` on the server, so a fish or zsh login shell is fine
- the tool has to be installed on the server, without it sshr says so and opens a plain login shell
- it works over plain ssh on every OS, keeps port forwarding and terminal images working, and through mosh as well
- a login script that starts its own tmux on the server still runs in the plain shell, sshr does not change it

## Port forwards

A host can keep named forwards, so "open the database" is one command instead of remembering `-L 5432:localhost:5432` every time

```sh
sshr forward add web db -L 5432:localhost:5432    # local 5432 -> the server's 5432
sshr forward add web app -R 8080:localhost:3000   # the server's 8080 -> your 3000
sshr forward add web socks -D 1080                # SOCKS proxy through the server
sshr forward web                                  # list them
sshr forward rm web socks

sshr tunnel web db            # only the forward, no shell, Ctrl+C closes it
sshr tunnel web               # all forwards of the host
sshr connect -F db web        # a shell with the forward open, -F all for every one
```

In the app it is the **Port forwards** field of the host form, one forward per line in the same `name -L spec` form

- the spec is what ssh takes after `-L`, `-R` or `-D`, `[bind:]port:host:hostport`, or `[bind:]port` for `-D`, it is checked when you save it
- `sshr tunnel` runs `ssh -N -o ExitOnForwardFailure=yes`, so a port that is already taken stops the tunnel with an error instead of leaving it open and useless
- forwards go through jump hosts like the session does, mosh carries no forwards, so `-F` switches that connection to ssh

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
   - jump hosts via `ProxyJump` - done ([#19](https://github.com/klovbin/sshr/issues/19)),
   - attach to tmux/zellij on connect - done ([#20](https://github.com/klovbin/sshr/issues/20)),
   - saved port forwards per host - done ([#21](https://github.com/klovbin/sshr/issues/21)),
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
