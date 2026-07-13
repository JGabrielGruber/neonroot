# NeonRoot

A lightweight, portable workspace manager for ephemeral development environments.

**Philosophy**: Complex simplicity — plug briefly, work untethered.

NeonRoot hydrates development workspaces from cold storage (an external drive)
into tmpfs so you can unplug and work untethered, then commit changes back to the
drive when you choose. It never writes to the SD card it boots from.

## Commands

| Command    | Status | Purpose |
|------------|--------|---------|
| `list`     | ✅ working | List repos; `list workspaces` lists workspaces |
| `status`   | ✅ working | Show repos, availability, and contents |
| `create`   | ✅ working | Create a workspace (template or `--from`), optional `--image` |
| `repo add` | ✅ working | Register a repo path in config |
| `load`     | ✅ working | Hydrate a workspace into tmpfs + start a tmux session |
| `attach`   | ✅ working | Attach to a loaded workspace's tmux session |
| `stop`     | ✅ working | Kill the session and drop the tmpfs copy |
| `commit`   | ✅ working | Write workspace changes back to a repo |
| `status`   | ✅ working | Repo availability, or a workspace's pending diff |

Global flags: `--quiet`/`-q` (warnings only), `--plain` (no color).
`create`/`load` take `--repo`/`-r` to target a repo (defaults to the configured
default); `load --no-session` skips starting tmux.
`commit` takes `--repo` (target a different repo), `--as <name>` (save a copy
under a new name), and `--force` (override a conflict). `status <workspace>`
shows that workspace's pending changes.

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
./neonroot list
```

Version: **0.0.2**
