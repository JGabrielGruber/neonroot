# NeonRoot — Architecture

Design reference for the current system. For *why*, see `PRINCIPLES.md` and
`VISION.md`; for *history*, see `CHANGELOG.md`.

## The two data planes

**Cold — the vault** (a directory on a drive, or any write-controlled path). Holds:
- `index.toml` — the **catalog**: workspaces (name, root, images, mount, shell) with a
  `schema_version` and a monotonic `revision`.
- `workspaces/<name>.git` — a **bare git repo per workspace** (the content + history).
- `images/<name>/` — `Containerfile` (the definition) + `image.tar` (built data from
  `podman save`, so containers run offline).

**Hot — tmpfs/RAM.** Holds the live git clone of a loaded workspace, the Podman
graphroot (image layers), runtime state, and locks. Cleared on reboot; nothing durable
lives here.

**Catalog vs content split:** `index.toml`'s `revision` guards *structural* edits
(create/rm/rename); **git** owns *content* history and conflict handling. They don't
overlap.

## Vault kinds: local drive vs remote ssh

A vault is either **local** (a directory on a drive — `Vault.Path`) or **remote** (an
ssh server — `Vault.Remote`, an `ssh://user@host/path` or `user@host:path` target). Both
use the *same layout* (`index.toml` + `workspaces/*.git` + `images/*/image.tar`); only the
transport differs, so the whole system stays kind-agnostic through two seams:

- **`vault.Catalog`** reads/writes the index regardless of kind. Local = the on-drive
  `index.toml`. Remote = an `index.toml` tracked in a `_catalog.git` bare repo at the vault
  root, cloned into tmpfs on demand; a metadata write commits + pushes it, so two devices
  editing the catalog collide on **git non-fast-forward** (→ `ErrCommitConflict`) rather
  than silently clobbering — the same conflict model the workspace repos use.
- **`internal/remote`** is the ssh transport: `Addr` parses/joins ssh + scp-style targets
  (the `://` presence disambiguates an ssh *port* from an scp *path* colon), and `Transport`
  moves the non-git pieces — `Fetch`/`Upload` image tarballs over **scp**, `InitBare`/`Mkdir`
  set up repos/dirs over **ssh**. Every call goes through `platform.Runner`, so the exact
  scp/ssh argv is unit-tested without spawning.

**Availability is optimistic for remotes:** no network probe on `status`/`list`; a remote
op just fails on the actual ssh/scp exit (offline-first snappiness). Workspace clone/commit
need *no* special code — `git.Clone`'s origin is simply the ssh URL, so `commit`/`sync`
push there unchanged, non-ff handling included. Loading a containerized remote workspace =
clone repo (ssh) + fetch `image.tar` into tmpfs (scp) + the existing `podman load` path.

**Encryption** falls out for free: point a local vault's `Path` at a mounted
gocryptfs/LUKS filesystem — `vault.State` just checks the mount. **Multi-device sync** is
the same `Remote` configured on two machines, each with its own tmpfs clone, reconciled by
git non-ff. Neither needs new code.

## Secrets passthrough (opt-in, ephemeral)

A workspace can opt into carrying your identity into its container
(`create/set/load --secrets`; `internal/secrets`). Two halves, both into tmpfs, both wiped
on `stop` (the workspace state dir goes), never on the card:
- **Env vars** — merged from `bananenv list` (its own tmpfs env store; parsed `export
  K="V"`, optional dependency) and an optional `load --env-file <dotenv>` — written to a
  0600 env-file in the workspace state dir and passed as podman `--env-file`. Using a file
  (not `-e KEY=val`) keeps secret **values** out of the process argv (`ps`).
- **Identity** — the host `$SSH_AUTH_SOCK` is bind-mounted to `/ssh-agent` (with
  `SSH_AUTH_SOCK` set inside) and `~/.gitconfig` to `/root/.gitconfig:ro` (rootless podman
  maps container-root to the host uid, so `/root` is home). Only the **agent socket** is
  forwarded — the private key never enters the container. A missing agent/gitconfig warns,
  never fails.

Both ride the `domain.SessionOpts{EnvFile, Mounts}` added to `runtime.Run`/`Start`/
`StartPod`. It is opt-in and surfaced as a `(secrets)` marker in `list`/`status` because
the container then authenticates as you for its lifetime — on SELinux-enforcing hosts the
agent socket may need `--security-opt label=disable` (a target-hardware item).

## Isolation (agent sandboxes)

A workspace can invert the trusting-dev defaults for untrusted/agent use
(`create/set/load --sandbox|--isolated`; `spawn` for a one-shot). `domain.Sandbox` holds
the *intent* (domain stays argv-free); `domain.SandboxFor(profile)` maps a profile to a
preset; `runtime` translates it to podman flags:
- `sandbox` → `--cap-drop=ALL --security-opt=no-new-privileges --memory=2g --pids-limit=512`
  (network stays up so builds/tests fetch deps),
- `isolated` → the above **+ `--network=none`**.

It rides the `domain.SessionOpts.Sandbox` field into `runtime.Run`. The loader **refuses to
combine it with secrets** — a sandbox that mounts your ssh agent defeats the purpose — and
skips all identity/env injection when a profile is set. `spawn` composes the existing
seams (`createWorkspace` → `Loader` → a headless `podman exec` → `stopWorkspace`/
`removeWorkspace` reap) and propagates the command's exit code via `exitError`.

Deliberately **not** a VM: no `--read-only` rootfs yet (image-brittle), no seccomp/userns
tuning. This is defense-in-depth (rootless podman + dropped caps + no-new-privs + optional
no-network), not a hermetic guarantee for actively hostile code — stated plainly so it
isn't over-trusted. Read-only + tmpfs and finer profiles are the follow-on.

## Path layout (the SD-write guarantee)

All resolution is centralized in `internal/platform/paths.go`.

| Data | Location | Backing |
|------|----------|---------|
| Config (`config.toml`) | `$XDG_CONFIG_HOME/neonroot` | **SD card** (the only allowed write) |
| State, locks | `$XDG_RUNTIME_DIR/neonroot` → `/run/user/$UID` | tmpfs |
| Loaded workspaces (git clones) | `$TMPDIR/neonroot-$UID/workspaces` | tmpfs (`/tmp`) |
| Podman graphroot / cache | `$TMPDIR/neonroot-$UID/{containers,cache}` | tmpfs |

## The core loop

- **`load`** — gate on vault availability → `git clone --no-hardlinks --single-branch`
  the bare repo into tmpfs → (if the workspace declares images) `podman load` each
  `image.tar` **straight from the drive** and start a container (or a **pod** for
  multiple images: primary + sidecars sharing localhost). Non-destructive reuse of an
  already-loaded clone; `--clean` re-clones.
- **work untethered** — the clone is a normal git repo + host directory; commit locally
  offline.
- **`commit` / `sync`** — availability gate → `git commit` + `git push`. A non-fast-forward
  push is **refused** (the vault moved ahead); resolve with `--rebase`/`--merge`/`--as
  <branch>`/`--force` (→ `--force-with-lease`).
- **`attach`** — execs a shell into the running container (or host tmux for host-only).
  NeonRoot imposes no tmux; the image's dotfiles do (see `PRINCIPLES.md` §5/§6).

## Package layout

```
cmd/                thin Cobra commands (RunE) → orchestrate via the App composition root
internal/
  domain/           pure types + sentinel errors, zero I/O (Vault, Index, Workspace, …)
  platform/         SD-safe paths, /proc mountinfo, flock, statfs, the exec Runner seam
                    (+ runnertest: a recording Runner for spawning-free adapter tests)
  ui/               Reporter interface + Lip Gloss neon theme; TTY + plain renderers
  config/           TOML user config + vault registry
  vault/            vault resolution, availability (VaultState via mountinfo),
                    index.toml read/write + version gate, image layout
  git/              git adapter (clone/commit/push/status/snapshot) — workspaces are git
  remote/           ssh vault addressing (Addr) + scp/ssh/rsync transport (Transport)
  secrets/          opt-in env-file (bananenv/dotenv) + ssh-agent/gitconfig passthrough
  workspace/        load orchestration + loaded-workspace state (List/ReadState/HotSize)
  session/          tmux adapter (host-only sessions)
  runtime/          podman adapter: graphroot→tmpfs, images, pods
  template/         embedded + user templates (workspace skeletons and image definitions)
```

## Conventions

- **Adapters go through `platform.Runner`** (git/tmux/podman), so every one is
  unit-tested by asserting the command + args via `runnertest.Recorder` — no real binary
  spawned. Real tools are exercised only in a `//go:build integration` suite.
- **Consumer-defined interfaces:** `workspace.Loader` declares small `Git`/`Sessions`/
  `Runtime` seams; the concrete adapters satisfy them structurally. Keeps orchestration
  testable and decoupled.
- **`App` composition root** (`cmd/root.go`) wires config, paths, the reporter, and the
  runner once in `PersistentPreRunE`; commands map sentinel errors → messages + exit codes
  (unavailable→3, locked→4, conflict→5).
- **Graceful degradation:** missing tmux/podman → host-only; missing git is the one hard
  requirement for workspace ops.

## Validated hard problems

- **Rootless Podman graphroot on tmpfs** — validated on `/dev/shm` (overlay driver);
  containers *and* pods run. Image layers live in RAM; unplugging never strands them.
- **Git filesystem-remote offline round-trip** — clone (`--no-hardlinks`, so the tmpfs
  clone is drive-independent), commit with the drive gone, push on replug; non-ff rejected;
  `--force-with-lease` refuses a concurrent writer.
- **Mount detection across re-plugs** — `/proc/self/mountinfo` + a readable `index.toml`
  distinguish a mounted drive from a stale mountpoint directory.
- **Remote vaults + secrets over ssh** — the `//go:build integration` suite exercises the
  real ssh/scp/rsync transport, git init-bare/seed/clone over ssh, a cross-device catalog
  non-fast-forward, and (with podman) that a container actually receives the `--env-file`
  vars and the `:ro` identity bind-mount. The secrets/container path is validated; the ssh
  paths run wherever passwordless localhost ssh (or a real host) is configured.

## Running the integration suite

Excluded from the default build; run on target hardware:

```
go test -tags integration ./...
```

Each test skips cleanly when its dependency is absent (`podman`, or passwordless
`ssh localhost`), so a partial environment still runs what it can.
