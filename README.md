# hetzner_auto_orchestrator (`hao`)

A tool for managing Hetzner Cloud projects without living in the terminal.
The goal is a desktop GUI (Wails). Right now it is a command-line tool, `hao`,
that proves the core works.

**Status: Phase 1, read-only.** `hao` stores Hetzner project tokens encrypted
and lists what exists in a project. It does not create, change or delete
anything in Hetzner.

## Build

Requires Go 1.27+.

```powershell
go build -o bin/hao.exe ./cmd/hao
```

## Where your tokens live

- Tokens are kept in one encrypted file:
  `%AppData%\hetzner_auto_orchestrator\contexts.age` (age encryption).
- The key that decrypts it is kept in the OS keychain (Windows Credential
  Manager, entry `hetzner_auto_orchestrator`), never in a file on disk.
- `HCLOUD_TOKEN` in the environment is **ignored**. `hao` warns you if it is set.
- `hao` never writes to the hcloud CLI's `cli.toml`.

## Contexts

A context is a named Hetzner Cloud project token, like in the `hcloud` CLI.
One context holds one token, and it should be a **Read & Write** token.
Everything today only reads, but later phases create and delete resources and
will expect write access. `hao` does not check a token's scope.

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

## Quick start

```powershell
hao init
hao context add myproject        # or: hao import
hao context list
hao server list
hao preflight
```

## Not in Phase 1

Everything is read-only. Creating, changing or deleting Hetzner resources, and
the desktop GUI, come in later phases.
