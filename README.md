# hetzner_auto_orchestrator (`hao`)

A tool for managing Hetzner Cloud projects without living in the terminal.
It comes as a desktop app (`hao-gui`, built with Wails) and a command-line
tool (`hao`). Both use the same encrypted store and the same active project.

**Status:** both list what exists in a project. Both also create, power,
back up, snapshot, protect and delete **servers** (see
[Servers](#servers-changes-things-at-hetzner)). Every other resource type is
read-only.

## Getting started

Everything below is for Windows, in PowerShell, from the repository folder.

### 1. Install the tools

| Tool | Needed for | Check |
|---|---|---|
| [Go](https://go.dev/dl/) 1.27 or newer | both | `go version` |
| [Node.js](https://nodejs.org/) (LTS, includes npm) | desktop app | `node -v`, `npm -v` |
| Wails CLI **v2.14.0** | desktop app | `wails version` |
| Microsoft WebView2 runtime | desktop app | already part of Windows 11 |

Install the Wails CLI with Go. The version must match the one in `go.mod`:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
wails doctor        # checks Go, Node, npm and WebView2
```

If `wails` is not found, add `%USERPROFILE%\go\bin` to your `PATH` and open a
new terminal. `wails doctor` may say `upx` and `nsis` are missing. Both are
optional (exe compression and an installer) and not needed.

### 2. Build the desktop app

```powershell
cd cmd/hao-gui
wails build
```

The first build also runs `npm install` and takes a minute or two. The app is
`cmd/hao-gui/build/bin/hao-gui.exe`. Start it by double-clicking it.

While working on the app itself, `wails dev` (in the same folder) opens it with
live reload instead.

### 3. Build the command-line tool (optional)

```powershell
go build -o bin/hao.exe ./cmd/hao
```

On a fresh clone, `go build ./...` (all packages at once) fails until
`wails build` has run once: the desktop app embeds the web files that
`wails build` produces. Building only `./cmd/hao`, as above, always works.

### 4. First run

1. **Create the store.** In the app, click **Create encrypted store**. Or run `hao init`.
   This creates the encrypted file for your tokens and puts its key in the
   Windows Credential Manager.
2. **Get a token.** In the Hetzner Console, create an API token for your
   project with **Read & Write** permission (`hao token` prints the Console
   link). Each Hetzner project has its own tokens.
3. **Add it.** In the app, open the **Contexts** tab, enter a name (e.g. the
   project's name) and paste the token. Or run `hao context add <name>`.
   Hetzner checks the token before it is saved. The first one you add becomes
   the active project.
4. **Look around.** The app's tabs (Servers, Load Balancers, ..., Preflight)
   show the active project. Switch projects with **Use** on the Contexts tab.
   The app and `hao` always show the same active project.

Already using the `hcloud` CLI? `hao import` adopts its contexts (see below).

## Where your tokens live

- Tokens are kept in one encrypted file:
  `%AppData%\hetzner_auto_orchestrator\contexts.age` (age encryption).
- The key that decrypts it is kept in the OS keychain (Windows Credential
  Manager, entry `hetzner_auto_orchestrator`), never in a file on disk.
- `HCLOUD_TOKEN` in the environment is **ignored**. `hao` warns you if it is set.
- `hao` never writes to the hcloud CLI's `cli.toml`.

## Contexts

A context is a named Hetzner Cloud project token, like in the `hcloud` CLI.
One context holds one token, and it should be a **Read & Write** token: the
server commands need write access. `hao` does not check a token's scope; a
read-only token fails at the first write with "token is read-only".

## Commands

### `hao init`

Creates the encrypted store and its key. Run once. It refuses to run if a
store already exists, so it cannot overwrite your tokens.

### `hao context add <name>`

Adds a context. Prompts for the token (input is hidden in a normal console
window), checks it against the Hetzner API, and saves it only if Hetzner
accepts it.

```powershell
hao context add myproject
token:            # paste the token, press Enter
```

- The token is never taken as a command-line argument, so it does not end up in
  your shell history.
- A name that already exists is rejected; nothing is overwritten.
- If the token is rejected, or Hetzner cannot be reached, nothing is saved.
- The first context you add becomes the active one.

### `hao context delete <name>`

Removes a context from hao's store. Asks first (`y/N`, default no), because
the store may hold your only copy of that token.

- This does **not** revoke the token at Hetzner. It keeps working until you
  delete it in the Hetzner Console.
- Deleting the active context leaves no context active; pick one with
  `hao context use <name>`.

### `hao import`

Reads contexts from the `hcloud` CLI (`%AppData%\hcloud\cli.toml`), lists them,
and asks which to adopt (`1,3`, `all`, or blank to cancel). Contexts you already
have are marked and skipped. Imported tokens are **not** validated.

`cli.toml` stores tokens in plaintext. After importing, consider removing them
there with `hcloud context delete <name>`.

### `hao context list`

Lists your contexts. The active one is marked with `*`.

### `hao context use <name>`

Makes `<name>` the active context. The choice is saved and survives restarts.

## Listing resources

Every listing reads from the **active** context's project. Names match the
`hcloud` CLI.

| Command | One line per item |
|---|---|
| `hao server list` | name, status, type, location, public IPv4 |
| `hao load-balancer list` | name, type, location, public IPv4, services / targets |
| `hao network list` | name, IP range, subnets / attached servers |
| `hao firewall list` | name, rule count, number of resources it applies to |
| `hao floating-ip list` | name, IPv4/IPv6, address, home location, assigned server |
| `hao volume list` | name, status, size, location, attached server |
| `hao ssh-key list` | name, fingerprint |
| `hao certificate list` | name, type, expiry date, domains |
| `hao zone list` | DNS zone name, status, mode, record count |

A `-` means the field is empty (e.g. a volume attached to nothing). A project
with none of a resource prints `no <resources> in this project`; that is an
answer, not an error.

## Server type availability

```powershell
hao preflight                  # every server type in every location
hao preflight ccx13 ash        # one yes/no answer
hao preflight fsn1, nbg1, hel1 # what is available in these locations
```

The location form lists only the server types you can create **right now**,
grouped per location with its city (`fsn1 (Falkenstein): 12 server types
available`). Commas, spaces or both work as separators, and case doesn't
matter. Location codes: `fsn1` Falkenstein, `nbg1` Nuremberg, `hel1` Helsinki,
`ash` Ashburn, `hil` Hillsboro, `sin` Singapore. `hao preflight` with no
arguments shows them all.

With exactly two words, the first decides: a location code means two
locations, anything else means `<type> <location>`.

This answers **"does Hetzner offer this type in this location right now"**. It
is **not** a quota check: a type shown as available can still fail at create
time if your project has hit a limit. Hetzner has no API for remaining quota.

A misspelled type or location, or a type not offered in that location, is an
error, never a plain "not available", so a typo can't pass for "out of stock".

## API tokens and members

```powershell
hao token
hao member
```

Hetzner has no API for either, so these print a link to the Hetzner Console
(`https://console.hetzner.com/`) instead.

## Servers (changes things at Hetzner)

These commands act on the **active** project, print `project: <name>` first,
and ask before doing anything. If the active project changes between the
question and your answer (e.g. `hao context use` in another window), the
command is refused and nothing changes.

```powershell
hao server create web-1 cpx02 fsn1 ubuntu-24.04 my-key   # several keys: key-a,key-b
hao server poweron  web-1
hao server shutdown web-1      # the OS shuts down cleanly
hao server poweroff web-1      # cuts power; unsaved data is lost
hao server reboot   web-1
hao server backup   web-1      # manual backup now
hao server snapshot web-1 before upgrade
hao server protect   web-1     # delete + rebuild protection on
hao server unprotect web-1     # type the name
hao server delete    web-1     # type the name
```

A server can be named or given by its ID.

In the desktop app the same actions are on the **Servers** tab: **Create
server** above the list, and an **Actions** button on each row. The rules
below are the same, checked by the same code. Buttons that would be refused
are greyed out; hover them to see why.

- **Create costs money.** Before every create, `hao` fetches the live prices
  and shows the hourly price, the monthly cap, the primary IPv4, and an
  estimate for 30 days running 24/7. You type `yes` to go ahead. A server is
  billed while it **exists**, even when powered off.
- **An existing SSH key is required** to create a server. `hao` does not
  create SSH keys and never sets a root password. Add keys in the Hetzner
  Console; `hao ssh-key list` shows them.
- **Delete only works on servers `hao` created.** They carry the label
  `managed-by=hao`. Others are refused: delete them in the Console. You also
  type the server's name. A typo cancels.
- **Protection** turns delete and rebuild protection on or off together
  (Hetzner does not allow one without the other). A protected server cannot
  be deleted until you `unprotect` it, which also asks for the name.
- **Backup** needs backups switched on for the server. `hao` never switches
  them on or off; do that in the Console. Hetzner keeps up to 7 backups and
  deletes the oldest when a new one is created and all are taken.
- **Snapshot** shows its price per GB and month first. Snapshots are kept
  until you delete them in the Console.
- **"Still running at Hetzner; refresh later"** is not an error: Hetzner
  accepted the action, it just took longer than the 75 seconds `hao` waits.
  Check with `hao server list`. Large snapshots often end this way.
- **"Outcome unknown"** means the connection dropped while the change was
  being sent. It may or may not have happened; check before retrying.

## Quick start (command line)

```powershell
hao init
hao context add myproject        # or: hao import
hao context list
hao server list
hao preflight
```

## Not yet

- Writes for every other resource type (load balancers, networks, firewalls,
  volumes, ...).
- Deleting snapshots and backups, rebuild, rescue, resize: use the Console.
