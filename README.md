# NeonRoot

A lightweight, portable workspace manager for ephemeral development environments.

**Philosophy**: Complex simplicity — plug briefly, work untethered.

NeonRoot hydrates development workspaces from cold storage (an external drive)
into tmpfs so you can unplug and work untethered, then commit changes back to the
drive when you choose. It never writes to the SD card it boots from.

NeonRoot is **workspace-first**: you name a workspace, and it uses a repo (where
it's stored) and optionally an image (what it runs in). You configure a repo once
and then work by workspace name.

## Commands

Workspace commands (the everyday surface):

| Command   | Purpose |
|-----------|---------|
| `list`    | List your workspaces (repo, image, loaded state) |
| `create <name>` | Create a workspace — default template or `--from <ws>`, optional `--image` |
| `load <name>`   | Hydrate into tmpfs; start a container (if image) + tmux session |
| `attach <name>` | Attach to a loaded workspace's session (inside its container) |
| `commit <name>` | Write changes back — `--as <name>`, `--force`, `--repo` |
| `status [name]` | Repo overview, or a workspace's pending diff |
| `stop <name>`   | Stop the container/session and drop the tmpfs copy |

Repo setup (one-time):

| Command | Purpose |
|---------|---------|
| `repo add <name> <path>` | Register a repo; the first becomes the default |
| `repo list`              | List repos and availability |
| `repo set-default <name>`| Change the default repo |

Workspace commands default to the configured default repo (no `--repo` needed);
pass `--repo`/`-r` to target another. Global: `--quiet`/`-q`, `--plain`.
`load` takes `--no-session` and `--no-container`.

Integration tests (real tmux/Podman) run with `go test -tags integration ./...`.

## Architecture

```
cmd/                  thin Cobra commands (RunE) → use-case methods
internal/
  domain/             pure types + sentinel errors (no I/O)
  platform/           SD-safe paths, mountinfo, flock, statfs, exec runner
  ui/                 Reporter interface + Lip Gloss neon theme
  config/             TOML user config + repo registry
  repo/               (Phase 1) resolution, availability, index.toml
  hydration/          (Phase 2) copy repo → tmpfs + manifest
  workspace/          (Phase 2) load orchestration
  session/ runtime/ env/   (Phase 3) tmux / podman / bananenv adapters
  commit/             (Phase 4) diff, conflict detection, write-back
```

Config lives on the SD card (`$XDG_CONFIG_HOME/neonroot/config.toml`); all state,
workspaces, and locks are redirected to tmpfs (`/run/user/$UID`, `/tmp`).

## Quick Start

```bash
go build -o neonroot .

neonroot repo add ext /mnt/ext/neonroot   # one-time: becomes the default repo
neonroot create webapp --image arch-minimal
neonroot load webapp                       # hydrate + container + tmux; unplug the drive
# ... work untethered ...
neonroot commit webapp                     # re-plug, write changes back
```

Version: **0.0.2**
