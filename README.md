# NeonRoot

**Git for your dev environment's *location*.** Carry a full, containerized fullstack
toolbelt on a drive, work with it unplugged, and sync when you choose — no cloud, no
daemon, no account.

> *Complex simplicity — plug briefly, work untethered.*

---

## What it is

NeonRoot is a **hot/cold storage manager for dev workspaces**. A **vault** (a directory
on an external drive — or any write-controlled location) holds a git repo per workspace
plus its container image data. `load` clones a workspace into **tmpfs/RAM** so you can
unplug the drive and work untethered; `commit` pushes your changes back when you
reconnect. It boots from a minimal Arch SD card and **never writes to that card**.

The one thing nobody else does: cloud CDEs (Coder, Codespaces) move your environment
*to the network*; Nix/Devbox make it *reproducible*; distrobox makes it *local* —
NeonRoot makes it **portable, offline, and sync-controlled.** It's a single ~8 MB static
binary. (See [`docs/VISION.md`](docs/VISION.md) for the full market position.)

## Who it's for

Nomadic and constrained-hardware developers (minimal OS on an SD card, heavy env on a
USB/SSD); sovereign / air-gapped / privacy work; homelab tinkerers who want portability
on top of local; anyone who wants to plug into a borrowed machine, work, and leave
nothing behind.

## Quick start

```bash
go build -o neonroot .

neonroot vault add ext /mnt/ext/neonroot     # one-time; the first vault becomes default

# Optional: a batteries-included dev image (nvim+LazyVim, tmux, starship, powertools)
neonroot image create dev --template arch-dev
neonroot image build dev                     # build once (online) → stored in the vault

neonroot create webapp --image dev           # or omit --image for host-only
neonroot load webapp                         # git clone into RAM + start its container
neonroot attach webapp                       # a shell in the workspace
#   … unplug the drive, work untethered, commit locally offline …
neonroot sync                                # re-plug: push all your pending work back
```

Run `neonroot` with no arguments for the interactive cockpit (`--no-tui` to skip).

**On a server instead of a drive?** A vault can live over ssh — same layout, same
commands, still offline-first:

```bash
neonroot vault add cloud ssh://you@host/srv/neonroot   # or you@host:srv/neonroot
neonroot create webapp --image dev                     # inits the repo on the server
neonroot load webapp                                    # git-clone over ssh into RAM
#   … work, commit; a mirror on another machine 'load's the same vault …
```

Availability is optimistic (no network probe — commands just work offline and fail
lazily if the host is down), and two machines writing the same vault reconcile on git's
non-fast-forward, never a silent overwrite. Encrypted vaults (point a vault at a mounted
gocryptfs/LUKS path) and multi-device sync fall out of the same model — see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Add `--rsync` to a remote vault for
resumable, skip-unchanged image transfers (falls back to scp).

**Secrets, ephemerally.** Opt a workspace into identity passthrough and NeonRoot injects
your env vars and git/ssh identity into the container on load — into RAM, never on the
card, wiped on `stop`:

```bash
bananenv set STRIPE_KEY=sk_test_…                # your tmpfs env store (optional)
neonroot create webapp --image dev --secrets     # or: set --secrets / load --secrets
neonroot load webapp                             # container gets $STRIPE_KEY,
                                                 #   your ssh agent + gitconfig (ro)
neonroot load webapp --env-file .env.local       # or inject an extra dotenv
```

Env values go via podman `--env-file` (never in `ps`); only the SSH **agent socket** is
forwarded, so your private key never enters the container. It's opt-in and shown as a
`(secrets)` marker in `list`/`status`, because the workspace then carries your identity.

## CI / self-test in a box

The `ci` image (alpine + Go + git + a self-contained passwordless-localhost sshd) plus
`--seed` and `run` turn any repo into a hermetic, containerized test sandbox — this is how
NeonRoot tests *itself*:

```bash
neonroot image create ci --template ci && neonroot image build ci
neonroot create proj --image ci --seed .        # import this repo as a workspace
neonroot load proj
neonroot run proj -- sh -c 'ensure-sshd && go test -tags integration ./...'
```

Because the container carries its own sshd, NeonRoot's ssh/rsync integration tests run
*inside* it without touching the host's ssh config. (Dogfooding this way already caught a
real catalog-over-ssh bug the mocked unit tests missed.)

## Agent sandboxes

The same throwaway-container loop is a **safe, disposable environment for an AI agent** —
NeonRoot's take on the "agent substrate" the cloud CDEs chased, but local, offline, and
sovereign. Where a dev workspace *trusts you*, a sandbox *distrusts the code*: no host
identity, dropped capabilities, resource limits, and (optionally) no network.

```bash
# create a throwaway box, run a command in it, propagate the exit code, reap it:
neonroot spawn --image ci --sandbox --seed . -- go test ./...

# --isolated also cuts the network (for untrusted code); --keep retains the box to review:
neonroot spawn --image ci --isolated -- ./suspicious-build.sh
```

- `--sandbox` = no `SSH_AUTH_SOCK`/gitconfig, `--cap-drop=ALL`, `--security-opt=no-new-privileges`,
  memory + pids limits — **network stays up** so builds/tests can fetch deps.
- `--isolated` = sandbox **+ `--network=none`**, for running code you don't trust.
- Mutually exclusive with `--secrets` (a sandbox must not carry your identity). Also a
  persistent trait: `create/set --sandbox|--isolated`, shown as a marker in `list`/`status`/TUI.

**Honest boundary:** rootless podman + dropped caps + no-new-privs + no-network is a *strong*
isolation boundary — defense in depth — **not a VM**. Don't treat an agent box as a hermetic
security guarantee for actively hostile code.

## How it works

```
  VAULT (cold, on the drive)                HOT (tmpfs / RAM)
  ├─ index.toml         (catalog)   load →  git clone  ──► work untethered
  ├─ workspaces/<w>.git (content)   commit  git push   ◄──  (commit offline)
  └─ images/<w>/image.tar (env)     load →  podman load ──► container / pod
```

A workspace is a normal git repo and a normal host directory — open it in any editor. A
workspace with one image runs as a container; with several, as a podman pod (app +
sidecars over localhost). Ergonomics (editor, shell, tmux) live in **editable image and
workspace templates**, not the binary.

## Commands

**Workspaces** (the everyday surface):
`create` · `spawn` · `load` · `attach` · `up` · `run` · `logs` · `commit` · `sync` ·
`status` · `snapshot` · `set` · `stop` · `rm` · `reap` · `list` (`--json`/`--loaded`) ·
`path`/`code` · `doctor` · `guard`

`create` takes `--image`, `--with postgres,redis` (sidecars), `--port 3000`
(publish to host), `--up "npm run dev"` (dev command for `neonroot up`), and
`--seed <dir>` (turn an existing project directory into a workspace). `run <ws> --
<cmd>` runs a command in the container and propagates its exit code (the CI/scripting
primitive); `logs <ws>` shows container/pod logs. Image templates: `node` · `python` ·
`go` · `rust` · `postgres` · `redis` · `arch-dev` · `ci` · `minimal`.

**Vaults** (one-time setup): `vault add` · `list` · `set-default` · `set` · `rm`

**Images**: `image create --template <minimal|arch-dev|ci>` · `build` · `ls` ·
`set --rename` · `rm` · `snapshot`

**Templates**: `template ls` · `new` · `path` — plus `create --template <name>` and
`--from <workspace>`.

Conflicts on `commit` resolve with `--rebase` / `--merge` / `--as <branch>` / `--force`
(→ `git push --force-with-lease`, never a bare force). Full flags via `--help`.

## Requirements

Linux (Arch/Manjaro-first). **`git`** required; **`tmux`** / **`podman`** optional —
NeonRoot degrades to host-only when they're absent. Remote (ssh) vaults additionally use
**`ssh`** / **`scp`** (or **`rsync`** with `--rsync`) and expect key-based auth to the
host. Secrets passthrough uses your running **ssh-agent** and, optionally,
**`bananenv`** for env vars. Config lives on the card; all state, clones, locks, and
injected secrets are redirected to tmpfs.

## Docs

- [`AGENTS.md`](AGENTS.md) — for AI agents: using NeonRoot as a sandbox, and developing it
- [`VISION.md`](docs/VISION.md) — market position & where this is going
- [`ROADMAP.md`](docs/ROADMAP.md) — what's next (E1–E4)
- [`PRINCIPLES.md`](docs/PRINCIPLES.md) — the design tenets
- [`ARCHITECTURE.md`](docs/ARCHITECTURE.md) — how it's built
- [`CHANGELOG.md`](CHANGELOG.md) — what's shipped

---

Pre-1.0 · Linux-first · MIT
