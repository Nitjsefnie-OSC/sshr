<p align="center"><img src="assets/icons/sshr-256.png" width="128" height="128" alt="sshr logo"></p>

# sshr

## About

sshr is an SSH host manager for Linux, macOS and Windows, a desktop app (Gio) and a CLI in one binary, it keeps the server list on your machine (`~/.sshr/vault`, one 0600 JSON file per host) and connects through the system `ssh`, nothing is installed on the servers

- hosts with a key file, a key kept inside sshr, a key from an ssh agent or 1Password, or a password saved encrypted with a key from the system keychain, see [Passwords](#passwords), [Keys stored in sshr](#keys-stored-in-sshr), [Keys from an ssh agent](#keys-from-an-ssh-agent), [Passwords and keys in 1Password](#passwords-and-keys-in-1password)
- jump hosts, tmux or zellij on connect, named port forwards, shared connections and mosh, see [Jump hosts](#jump-hosts), [Keeping the session on the server](#keeping-the-session-on-the-server), [Port forwards](#port-forwards), [Reusing connections](#reusing-connections), [Connecting with mosh](#connecting-with-mosh)
- hosts in and out: [import](#importing-sshconfig) from and [export](#exporting-to-sshconfig) to `~/.ssh/config`, [sync between devices](#sync-between-devices) through your own end to end encrypted [sync server](#sync-server)
- [whether a host answers](#is-the-host-up), [clipboard images to a server](#sending-images-to-a-server)
- for other programs: an [MCP server](#ai-assistants-mcp) for AI assistants and a local [REST API](#rest-api)
- interface in Russian and English

Everything with a cost (another connection, a file of yours edited, commands from AI) is off by default and says what it costs next to its switch

## Install

On Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/klovbin/sshr/main/install.sh | sh
```

or `sh install.sh` from a checkout, options in `sh install.sh --help`

- it installs into `~/.local/bin` (`--prefix <dir>` or `SSHR_PREFIX` for another place), no root needed
- a release binary `sshr_<os>_<arch>` is used when the release has one and it matches `sha256sums.txt`, a binary without a checksum is not installed
- otherwise sshr is built from source, from the checkout the script runs in or from a fresh clone, it needs Go 1.21 or newer (go fetches the toolchain named in `go.mod` itself), the GUI also needs a C compiler and, on Linux, Wayland/X11 and Vulkan headers, without them the CLI is built and the script says so, `--cli-only` skips the GUI
- if the directory is not in `PATH`, one line marked `# added by the sshr installer` goes to the startup file of your shell (`~/.bashrc`, `~/.bash_profile` on macOS, `~/.zshrc`, `~/.config/fish/conf.d/sshr.fish` or `~/.profile`), `--no-modify-path` leaves them alone
- when the GUI is installed, the script runs `sshr desktop-entry`, which adds sshr with its icon to the application menu (below)
- `--uninstall` removes the binary, the menu entry and the PATH line, hosts in `~/.sshr` stay

### Icon and application menu

```sh
sshr desktop-entry            # add sshr to the menu with its icon
sshr desktop-entry --remove   # take it out
```

- Linux: `sshr.desktop` in `~/.local/share/applications` and the icon in sizes 16 to 512 in `~/.local/share/icons/hicolor`, the window gets the same icon, Gio uses the binary name `sshr` as its app id, the entry starts the binary it was made from
- macOS: `~/Applications/sshr.app` with the icon, signed ad hoc, the Dock shows a bundle icon only for a program inside the bundle, so the bundle holds a copy of sshr, run `sshr desktop-entry` again after an update (`install.sh` does it)
- Windows: the icon and version info are resources of `sshr.exe` (`cmd/sshr/rsrc_windows_*.syso`), `go build` links them in, the window and Explorer show it, nothing to install
- the icons are inside the binary, nothing is downloaded, the logo files are in `assets/icons`, after changing the logo run `go generate ./cmd/sshr` for the Windows resources

Packages for AUR, Homebrew, Scoop and nixpkgs are planned ([#17](https://github.com/klovbin/sshr/issues/17)), on Windows build with `go build -o sshr.exe ./cmd/sshr` for now

## Importing ~/.ssh/config

```sh
sshr import-ssh-config --dry-run   # show what would be added
sshr import-ssh-config             # add it
```

In the app: **Settings → Import → Import from ~/.ssh/config**

- every `Host` name of `~/.ssh/config` and the files it `Include`s (globs, relative paths from `~/.ssh`, a file that includes itself is read once) becomes a host, patterns with `*`, `?` or `!` are rules, not hosts, and are skipped
- each name is resolved with `ssh -G`, so ssh itself applies `Match`, `Include`, `Host *` and defaults, nothing is sent over the network, only your own config is read, `Match exec` runs what you wrote there, as on every connect
- carried over: `HostName`, `User`, `Port`, `IdentityFile` (the first that exists and is a private key, only when you set it), `IdentityAgent` (the 1Password socket becomes the `1password` agent), `ProxyJump`, `LocalForward`, `RemoteForward` and `DynamicForward` as named forwards `l1`, `r1`, `d1`
- a host with `ProxyCommand` is saved with the alias as its address, so ssh keeps applying that config block on connect, unix socket forwards are skipped, each such case is listed
- a host whose name, or address, user and port, sshr has already is skipped, running it again adds only what is new

## Exporting to ~/.ssh/config

```sh
sshr export-ssh-config               # print the hosts as ssh config
sshr export-ssh-config --install     # write ~/.sshr/ssh_config and include it
sshr export-ssh-config --uninstall   # remove both
```

In the app: **Settings → Import → Export to ~/.ssh/config**, off by default, it edits a file of yours

- `--install` writes `~/.sshr/ssh_config` and adds `Include ~/.sshr/ssh_config` as the first line of `~/.ssh/config`, a `Host` line before it would make ssh read the include as part of that block, from then on every change of the hosts rewrites the file, so plain `ssh my-box`, `scp`, `rsync` and editors with remote ssh reach the hosts by name
- the alias is the host name with everything except letters, digits, `.`, `_` and `-` turned into `-`, `my box` becomes `my-box`
- exported: `HostName`, `User`, `Port`, a key file as `IdentityFile` with `IdentitiesOnly yes`, an agent as `IdentityAgent` with the chosen key, jump hosts as `ProxyJump` with saved hops replaced by their aliases
- passwords and keys kept in sshr or 1Password stay in sshr, such a host gets its address and a comment, plain ssh asks for the password, forwards, sessions and shared connections stay sshr options
- a host whose alias `~/.ssh/config` already has is skipped, ssh takes the first value it reads and the export would shadow your own block, so a host imported from ssh config is not written back
- a value that ssh could read as a second option (a newline, a quote, a space in an address) skips the host, hosts synced from another device cannot add `ProxyCommand` to your config
- when `~/.ssh/config` is a link (home-manager, a dotfiles repo), it is not edited, the export is written and sshr prints the `Include` line to add there

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

## Is the host up

```sh
sshr ping                 # check every host, exits 1 when one is down
sshr ping web db          # only these
sshr list --ping          # the list with a STATUS column
```

With **Settings → Connection → Show whether hosts answer** (`"health_check": true`) the app shows a dot next to each host name, green with the round trip in ms, red when it is down, orange when the port answers but is not ssh, gray `?` when it cannot be checked from here, `sshr list` adds the column too (`--no-ping` turns it off for one run), it is **off by default**, because:

- each check connects to the ssh port and reads the banner, no login is tried, but sshd logs it as a connection closed before authentication, fail2ban counts those in `mode = ddos` and `mode = aggressive` (not in the default `normal`) and may ban your address
- the checks go out from your machine on their own, every 5 minutes by default (`"health_interval": 300` in seconds, 60 at least) and right after a host is added or changed

How it checks:

- the time shown is the TCP connect, one round trip, a host counts as up only when the SSH banner arrives within 3 s
- a host behind a jump host is not checked, a direct connection says nothing about the path ssh takes, a host saved as an alias of ssh config (`ProxyCommand`) that does not resolve is shown as unknown, not down

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

## Keys from an ssh agent

A host can take its key from an ssh agent instead of a key file, the agent of your session, the 1Password SSH agent or any other agent socket (Bitwarden, KeePassXC, a hardware key agent), the key never leaves the agent

```sh
sshr add web -u deploy --agent                          # the agent in SSH_AUTH_SOCK
sshr add web -u deploy --agent 1password --agent-key work
sshr add web -u deploy --agent /path/to/agent.sock --agent-key SHA256:GORl...
```

- `1password` is `~/.1password/agent.sock` on Linux, `~/Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock` on macOS and `\\.\pipe\openssh-ssh-agent` on Windows, as in the 1Password setup guide
- `--agent-key` picks one key by fingerprint, comment or a unique part of either, sshr asks the agent once and keeps that public key, ssh then gets `-i <id>.pub -o IdentitiesOnly=yes` and offers only it, OpenSSH servers stop after about six tries, so an agent with many keys fails without it
- without `--agent-key` and with several keys in the agent, `sshr add` lists them and asks which one, with no terminal every key is offered
- in the host form choose **SSH agent**, the agent field is empty, `1password` or a socket path, the key field takes a fingerprint or comment

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

## AI assistants (MCP)

`sshr mcp` is an [MCP](https://modelcontextprotocol.io) server, AI assistants such as Claude Code, Codex or Cursor start it and get your saved hosts as tools, so "look at the nginx logs on web" works without copying addresses and keys

```sh
claude mcp add --transport stdio sshr -- sshr mcp                    # Claude Code
claude mcp add --transport stdio sshr -- sshr mcp --allow-commands   # and let it run commands
```

Other clients take the usual block, `{"mcpServers": {"sshr": {"command": "sshr", "args": ["mcp"]}}}`

Tools:

- `list_hosts` - names, addresses, users, ports, how each host logs in, jump hosts, sessions and forwards, read only
- `upload_file` - copies a local file to `~/.cache/sshr/pasted/` on a host and returns the path there, removed after 15 minutes like a pasted image
- `run_command` - runs a command with `sh` on a host without a terminal and returns stdout, stderr (64 KiB each at most) and the exit code, stopped on the server after `timeout_seconds` (60 by default, 600 at most) when it has `timeout`, **off by default**, it needs `--allow-commands` or `"mcp_commands": true` in `~/.sshr/settings.json`, because an assistant that can run commands on every saved server can also break them, the tool is marked destructive, so clients ask before each call unless you turned that off

What it never does:

- passwords and private keys never leave sshr, a host logs in the same way as a CLI upload, with its key file, the sshr agent, an ssh agent or a saved password through the one-time askpass socket
- hosts set to ask for the password every time cannot be used, there is nobody to ask
- it listens on nothing, the assistant runs `sshr mcp` as its own child process and talks over stdin and stdout, so no port and no token
- every command is logged to stderr, which the assistant shows in its MCP logs

## REST API

`sshr serve` is a local HTTP API for other programs, editor plugins, panels, scripts, it runs until Ctrl+C

```sh
sshr serve                                  # unix socket $XDG_RUNTIME_DIR/sshr/api.sock (or ~/.sshr/cm/api.sock)
sshr serve --listen 127.0.0.1:7522          # a loopback port, needs the token
curl --unix-socket "$XDG_RUNTIME_DIR/sshr/api.sock" http://x/v1/hosts
curl -H "Authorization: Bearer $(cat ~/.sshr/api-token)" http://127.0.0.1:7522/v1/hosts
```

| Method and path | What it does |
|----|----|
| `GET /v1/hosts` | list hosts, the same fields as the MCP `list_hosts` |
| `GET /v1/hosts/{host}` | one host by name, address or ID |
| `POST /v1/hosts` | add a host: `name`, `host`, `user`, `port`, `key`, `agent`, `agent_key`, `ask_password`, `op_password`, `op_key`, `jump`, `session`, `forwards` |
| `DELETE /v1/hosts/{host}` | remove a host |
| `GET /v1/hosts/{host}/connect` | the command line that opens a session, `{"argv": [...]}`, run it in your terminal |
| `POST /v1/hosts/{host}/upload?ext=png` | body is the file, returns its path on the server, removed after 15 minutes |
| `POST /v1/hosts/{host}/run` | `{"command": "...", "timeout_seconds": 60}`, **off by default** (`--allow-commands` or `"api_commands": true`) |
| `POST /v1/hosts/{host}/tunnels` | `{"forwards": ["db"]}` opens saved forwards with `ssh -N` (all when empty), they live as long as the server |
| `GET /v1/tunnels`, `DELETE /v1/tunnels/{id}` | list and close tunnels |

Security:

- it never listens on the network, `--listen` takes only `127.0.0.1`, `::1` or `localhost`, the default unix socket is 0600 in a 0700 directory
- on a port any local program could connect, so a token is required, 64 hex characters in `~/.sshr/api-token` (0600), created on the first start
- a request with an `Origin` header (a web page) or a `Host` that is not a loopback name (DNS rebinding) is refused
- passwords, private keys and 1Password references are never returned, and are not taken either, a saved password or a pasted key is added in the app or with `sshr add`

## Sync between devices

Hosts sync through an `sshr-sync` server (see [Sync server](#sync-server)), end to end encrypted, the server gets only ciphertext

```sh
sshr sync init --server https://sync.example.com --name laptop   # first device
sshr sync join --server https://sync.example.com <code>          # on the new device, prints its fingerprint
sshr sync approve <id>                                           # on a device already in, after comparing fingerprints
sshr sync                                                        # pull, then push what changed here
sshr sync status                                                 # devices and a state fingerprint
sshr sync remove-device <id>                                     # the vault key changes
```

How it is protected:

- every device has its own keys from a 32-byte seed in the system keychain, an Ed25519 signing key and an ML-KEM-768 + X25519 (X-Wing) key, there is no master password to guess
- a random vault key encrypts every host (XChaCha20-Poly1305, the account, host ID and key epoch as associated data, padded to 512 bytes), the vault key reaches each device in its own HPKE envelope (Go's crypto/hpke)
- the manifest lists the hosts, the hash of each blob and the trusted devices, it is MAC'd with the vault key, signed by the device that pushed it and carries the hash of the previous manifest, a device accepts only a chain that continues the one it saw, signed by a device it already trusts
- a new device is trusted only after you compare its fingerprint on both screens, the invite code carries the fingerprint of the inviting device, so the server can neither add its own device nor pretend to be the inviting one
- saved passwords travel inside the encryption and are sealed again with the keychain key of the receiving machine, private keys stored in sshr never leave their device, other devices see the host with its key fingerprint and are asked to paste the key there
- removing a device changes the vault key, encrypts every host again and revokes its token

Limits:

- a device that is new or lost its state accepts whatever the server shows as the current state, a device that synced before refuses an older one (`--accept-reset` after a real server restore)
- a server could show two devices different histories, they keep syncing apart, compare the state fingerprint in `sshr sync status` on two screens, equal means the same history
- the server sees how many hosts there are (sizes are padded), when devices push, their addresses and device names, and it can hold back changes
- two devices changing the same host before syncing keep the later change in the host and the other one as a copy named `<name> (conflict <device> <date>)`, the copy syncs like any host, delete it when you have looked, nothing is lost silently
- a removed device keeps what it already downloaded
- key file paths sync but the files do not, jump hosts refer to names, sync needs the system keychain, it runs when you call `sshr sync`, there is no automatic sync yet

## Sync server

`sshr-sync` is the server devices sync through, a separate binary, it stores only device public keys, encrypted blobs and signed manifests, it cannot read hosts or passwords, it can only lose data or refuse service

```sh
go build -o sshr-sync ./cmd/sshr-sync
sshr-sync --autocert sync.example.com --listen :443 --data /var/lib/sshr-sync    # Let's Encrypt
sshr-sync --tls-cert cert.pem --tls-key key.pem --listen :8443 --data ./data
sshr-sync --plain-http --listen 127.0.0.1:8080 --data ./data                      # behind a TLS reverse proxy
```

- every device has its own bearer token, the server keeps only its SHA-256, new accounts and joins are limited to 10 an hour per address, a joined device can do nothing until an existing device approves it
- a manifest is replaced only with `If-Match` naming the current counter and only when every blob it names is stored, so two devices pushing at once cannot overwrite each other and a broken upload leaves the old state
- blobs are content-addressed (SHA-256, at most 1 MiB), blobs no manifest needs are removed after a day, the last 1000 manifests are kept for the history check
- one server per data directory (file lock), TLS 1.3 only, plain HTTP only on a loopback address

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

## Roadmap

What is done and what comes next, with links to the issues, is in [docs/ROADMAP.md](docs/ROADMAP.md), the board is [SSHR](https://github.com/users/klovbin/projects/3), reasons for each decision are in [docs/WHY.md](docs/WHY.md) (Russian)
