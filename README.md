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
`create` · `load` · `attach` · `commit` · `sync` · `status` · `snapshot` · `set` ·
`stop` · `rm` · `list` · `path`/`code` · `doctor` · `guard`

**Vaults** (one-time setup): `vault add` · `list` · `set-default` · `set` · `rm`

**Images**: `image create --template <minimal|arch-dev>` · `build` · `ls` · `set --rename`
· `rm` · `snapshot`

**Templates**: `template ls` · `new` · `path` — plus `create --template <name>` and
`--from <workspace>`.

Conflicts on `commit` resolve with `--rebase` / `--merge` / `--as <branch>` / `--force`
(→ `git push --force-with-lease`, never a bare force). Full flags via `--help`.

## Requirements

Linux (Arch/Manjaro-first). **`git`** required; **`tmux`** / **`podman`** optional —
NeonRoot degrades to host-only when they're absent. Config lives on the card; all state,
clones, and locks are redirected to tmpfs.

## Docs

- [`VISION.md`](docs/VISION.md) — market position & where this is going
- [`ROADMAP.md`](docs/ROADMAP.md) — what's next (E1–E4)
- [`PRINCIPLES.md`](docs/PRINCIPLES.md) — the design tenets
- [`ARCHITECTURE.md`](docs/ARCHITECTURE.md) — how it's built
- [`CHANGELOG.md`](CHANGELOG.md) — what's shipped

---

Pre-1.0 · Linux-first · MIT
