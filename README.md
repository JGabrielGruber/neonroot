# NeonRoot

A lightweight, portable workspace manager for ephemeral development environments.

**Philosophy**: Complex simplicity — plug briefly, work untethered.

NeonRoot hydrates development workspaces from cold storage (an external drive)
into tmpfs so you can unplug and work untethered, then commit changes back to the
drive when you choose. It never writes to the SD card it boots from.

NeonRoot is **workspace-first**: you name a workspace, and it uses a **vault**
(where it's stored — a git repo per workspace) and optionally an image (what it
runs in). You configure a vault once and then work by workspace name.

## Commands

Workspace commands (the everyday surface):

| Command   | Purpose |
|-----------|---------|
| `list`    | List your workspaces (vault, image, loaded state) |
| `create <name>` | Create a workspace (bare git repo) — default template or `--from <ws>`, optional `--image` |
| `load <name>`   | `git clone` a workspace into tmpfs; container (if image) + tmux session |
| `attach <name>` | Attach to a loaded workspace's session (inside its container) |
| `commit <name>` | `git commit` + `git push` back (refuses on conflict; `--rebase`/`--as`/`--force`) |
| `status [name]` | Vault overview, or a workspace's live git state (dirty/ahead/behind) |
| `snapshot <name> <label>` | Tag a durable point-in-time copy of the workspace |
| `stop <name>`   | Stop the container/pod + session and drop the tmpfs copy |
| `rm <name>`     | Delete a workspace from its vault (stop it first) |

Attaching recreates the session if you exited it (Ctrl-D) — the container keeps
running until `stop`, so you can always `attach` back in.

Image management (`neonroot image …`):

| Command | Purpose |
|---------|---------|
| `image create <name>` | Scaffold a Containerfile in the vault |
| `image build <name>`  | Build (online) + save the image's data into the vault |
| `image ls` / `rm <name>` | List / remove vault images |
| `image snapshot <ws>` | Commit a running container back into its vault image |

A workspace with one image runs as a container; with several, as a podman pod
(primary + sidecars sharing localhost).

Vault setup (one-time):

| Command | Purpose |
|---------|---------|
| `vault add <name> <path>` | Register a vault; the first becomes the default |
| `vault list`              | List vaults and availability |
| `vault set-default <name>`| Change the default vault |
| `vault rm <name>`         | Unregister a vault (drive data untouched) |

Workspace commands default to the configured default vault (no `--vault` needed);
pass `--vault` to target another. Global: `--quiet`/`-q`, `--plain`.
`load` takes `--no-session`, `--no-container`, and `--clean` (re-clone fresh);
`commit` takes `-m <message>`.

Requires `git` on PATH; `tmux`/`podman` optional (degrade to host-only).
Integration tests run with `go test -tags integration ./...`.

## Architecture

```
cmd/                  thin Cobra commands (RunE) → use-case methods
internal/
  domain/             pure types + sentinel errors (no I/O)
  platform/           SD-safe paths, mountinfo, flock, statfs, exec runner
  ui/                 Reporter interface + Lip Gloss neon theme
  config/             TOML user config + vault registry
  vault/              resolution, availability, index.toml (the catalog)
  git/                git adapter: workspaces are bare repos in the vault
  workspace/          load orchestration (clone + session/container seams)
  session/ runtime/   tmux / podman adapters (via platform.Runner)
  template/           embedded default workspace skeleton
```

Config lives on the SD card (`$XDG_CONFIG_HOME/neonroot/config.toml`); all state,
workspaces, and locks are redirected to tmpfs (`/run/user/$UID`, `/tmp`).
The vault stores the catalog (`index.toml`) and a bare git repo per workspace.

## Quick Start

```bash
go build -o neonroot .

neonroot vault add ext /mnt/ext/neonroot   # one-time: becomes the default vault
neonroot create webapp --image arch-minimal
neonroot load webapp                       # git clone + container + tmux; unplug the drive
# ... work untethered (commit locally offline) ...
neonroot commit webapp                     # re-plug, push changes back
```

Version: **0.0.2**
