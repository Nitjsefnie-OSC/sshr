# sshr

## About

sshr is a desktop SSH hosts manager that keeps your server list local and under your control. It runs as a native Gio UI on your machine, stores hosts as JSON in a local vault (`~/.sshr/vault`), and is built to grow into CLI commands and optional git-based sync across devices — without requiring an agent or special setup on remote systems.

It currently supports:

- Adding and listing SSH hosts (name, host/IP, user, port)
- CLI commands: `list`, `add`, `rm`, `connect`, `paste`, `forward`, `tunnel`, `close`, `help`
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
- Optional connection reuse, off by default (see [Reusing connections](#reusing-connections))

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

## Reusing connections

With **Settings → Connection → Reuse connections** (`"reuse": true` in `~/.sshr/settings.json`) sessions to the same host share one ssh connection (`ControlMaster`), a second window, `paste` or `tunnel` opens at once without a new login, in a test the second connect took about 0.1 s instead of 0.65 s, it is **off by default**, because:

- if the shared connection drops, every session on it drops too
- while it is up, a password is not asked again, anyone with your user account can open a session through its socket, so sockets live in a 0700 directory, `$XDG_RUNTIME_DIR/sshr` or `~/.sshr/cm`
- Windows OpenSSH has no `ControlMaster`, there it does nothing

```sh
sshr connect --reuse web      # share this time, with any command that connects
sshr connect --no-reuse web   # a separate connection even if the setting is on
sshr close web                # end the shared connection now
```

- the shared connection stays up 10 minutes after the last session (`ControlPersist=10m`)
- if your ssh config already sets `ControlMaster` for the host, sshr leaves it to your config
- the socket path has a length limit (about 104 bytes), if no directory is short enough sshr says so and connects without reuse

## Passwords

Every host with a password has one of two modes:

- **saved** (Save in sshr, `sshr add --no-key`), the password is encrypted in the host file with XChaCha20-Poly1305, the vault key is 32 random bytes kept in the system keychain (Secret Service on Linux, Keychain on macOS, Credential Manager on Windows), not on disk
- **ask** (Ask every time, `sshr add --ask-password`), nothing is stored, ssh asks on the terminal on every connect

```sh
sshr add db.example.com -u admin --no-key          # asks for the password once, saves it encrypted
sshr add db.example.com -u admin --ask-password    # ssh asks every time
```

How a saved password reaches ssh:

- ssh runs sshr itself as `SSH_ASKPASS`, the helper gets the password over a unix socket that sshr serves in a 0700 directory for this one connection, the password is never written to a file, put in the environment or on the command line
- the socket answers once, a wrong password is not retried (`NumberOfPasswordPrompts=1`), three failures in a row are enough for fail2ban
- only a password prompt gets the password, the host key question, key passphrases and one-time codes are asked on the terminal
- `sshr list` and the host form never decrypt anything, the form shows an empty field, leave it empty to keep the saved password
- with no keychain (a server without a desktop, a locked keyring that nobody answers within 5 s) a saved password cannot be added, sshr says so, ask mode and key files keep working
- hosts saved by older versions with a plain `"password"` field keep working, the password is encrypted on the next save that can reach the keychain
- an older sshr that edits a host drops the encrypted password, the host then needs the password again
- if the keychain is reset or `~/.sshr` is copied to another machine, a saved password no longer opens, sshr asks to save it again

## Keys stored in sshr

A host can keep its private key inside sshr instead of a key file (**Key in sshr** in the host form, `--key-stdin` or choice 5 in `sshr add`), the key is sealed in the host file the same way as a saved password, no key file is written to `~/.ssh`

```sh
sshr add web -u deploy --key-stdin < key   # asks for the passphrase on the terminal if the key has one
```

- a key with a passphrase is opened once when you add it and stored without the passphrase, the vault encryption takes its place
- on connect sshr opens the key and serves it from an ssh agent of its own process for as long as ssh runs, on a unix socket in the 0700 `ControlDir` (a named pipe on Windows), the private key is only in memory
- ssh gets `-i <host>.pub -o IdentitiesOnly=yes -o IdentityAgent=SSH_AUTH_SOCK`, so it offers only this key, not the keys of your own agent or `~/.ssh`, agent forwarding is turned off for the host so the server cannot reach the key
- the public half is kept next to the host file (`~/.sshr/vault/hosts/<id>.pub`), the host form and `sshr add` show its fingerprint
- a jump hop gets your own `SSH_AUTH_SOCK` back

## Passwords and keys in 1Password

A host can keep its password or key in 1Password instead of this machine, sshr reads it with the [1Password CLI](https://developer.1password.com/docs/cli/get-started/) (`op`) on every connect and keeps only the reference, it is **off by default**, a host uses it only when you choose 1Password for it, because it needs `op` installed and signed in, and each connect waits for `op`, a locked app asks for a fingerprint or the account password

```sh
sshr add web -u deploy --op-password op://Private/web/password          # use an existing item
sshr add web -u deploy --no-key --store 1password --op-vault Servers   # type the password, sshr creates a Login item
sshr add web -u deploy --op-key "op://Private/web key/private key"     # the key of an SSH Key item
```

- in the host form choose **1Password** for the password (give an `op://` reference or type a password for a new item) or **Key in 1Password** for the key
- a new item is a Login item titled `sshr <name>` with the tag `sshr`, in the vault from `--op-vault` or `"op_vault"` in `~/.sshr/settings.json`, the account's default vault when neither is set, the password goes to `op item create` as a JSON template on stdin, not in its arguments
- the password then reaches ssh the same way as a saved one, through the one-time askpass socket, a key is read in OpenSSH format (`?ssh-format=openssh`) and served from the agent of sshr, see [Keys stored in sshr](#keys-stored-in-sshr)
- the public half of a 1Password key is read once when the host is saved, so the host form can show its fingerprint
- if `op` is missing or locked, a password host falls back to ssh asking for the password on the terminal, a key host stops with the error from `op`, the app, which has no terminal, shows the error
- a new password for a host linked to an item is changed in 1Password, not in sshr
- `op` cannot import an existing private key, add it to 1Password in the app (New Item, SSH Key, Add Private Key, Import a Key File), then link it with `--op-key`

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
- the CLI can ask for a password on the terminal, the app has no terminal, so it uses `BatchMode` and works with a key, ssh-agent or a saved password, a host in ask mode tells you to use `sshr paste`
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
   - connection reuse with `ControlMaster`/`ControlPersist`, off by default - done ([#22](https://github.com/klovbin/sshr/issues/22)),
   - sending clipboard images to the server — done ([#26](https://github.com/klovbin/sshr/issues/26));
   - mosh, off by default — done ([#23](https://github.com/klovbin/sshr/issues/23)).
3. Password handling: saved encrypted with a key from the system keychain or asked every time - done ([#6](https://github.com/klovbin/sshr/issues/6)), keys stored inside sshr - done ([#12](https://github.com/klovbin/sshr/issues/12)), passwords and keys in 1Password - done ([#13](https://github.com/klovbin/sshr/issues/13))
4. Cross-platform installer with OS auto-detection and PATH setup ([#11](https://github.com/klovbin/sshr/issues/11))
5. Sync with key services: 1Password, Bitwarden, ssh-agent ([#14](https://github.com/klovbin/sshr/issues/14))
6. REST API ([#12](https://github.com/klovbin/sshr/issues/12)) and MCP server ([#13](https://github.com/klovbin/sshr/issues/13)) for integrations and AI assistants
7. GitHub / git vault sync across machines (see the zero-knowledge idea below)

Not planned: Eternal Terminal (another server daemon for what tmux and mosh already cover) and autossh (meant for permanent tunnels, not interactive work).

Future ideas:

- Zero-knowledge sync with device-bound keys ([#10](https://github.com/klovbin/sshr/issues/10)) — the sync backend stores only ciphertext; each device keeps its own key pair (TPM where available) and the vault key is encrypted to every device, so there is no master password to brute-force; new devices are approved from an existing one by fingerprint / QR; paper recovery key; SSH private keys never leave their device
- Sync transport for it: direct device-to-device with no server at all — local network or the user's own WireGuard tunnel, TLS 1.3 with mutual auth and device keys pinned at pairing; git / own server only as a later opt-in fallback
- Optional hardware keys (YubiKey / FIDO2 `ed25519-sk`) — not required, buying a dedicated device just for sshr is a big ask
