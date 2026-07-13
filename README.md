# NeonRoot

A lightweight, portable workspace manager for ephemeral development environments.

**Philosophy**: Complex simplicity — plug briefly, work untethered.

NeonRoot hydrates development workspaces from cold storage (an external drive)
into tmpfs so you can unplug and work untethered, then commit changes back to the
drive when you choose. It never writes to the SD card it boots from.

## Commands

| Command  | Status | Purpose |
|----------|--------|---------|
| `list`   | ✅ working | List configured repos (name → path) |
| `load`   | 🚧 Phase 2 | Hydrate a workspace from a repo into tmpfs |
| `status` | 🚧 Phase 1/4 | Show loaded workspaces and pending changes |
| `create` | 🚧 Phase 1 | Create a new empty workspace in a repo |
| `commit` | 🚧 Phase 4 | Write workspace changes back to a repo |

Global flags: `--quiet`/`-q` (warnings only), `--plain` (no color).

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
./neonroot list
```

Version: **0.0.2**
