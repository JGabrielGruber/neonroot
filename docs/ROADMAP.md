# NeonRoot — Vision & Roadmap

> Living document. Tracks the project's vision, guiding constraints, architecture,
> and phased delivery. Updated as each phase lands.

## Vision

NeonRoot is a portable, ephemeral dev-workspace manager for one concrete workflow:
boot a lightweight Arch image from a **write-sensitive SD card**, keep heavy dev
environments and data on an **external drive** (flash/HDD) that is only *briefly
plugged in*, hydrate a workspace into **tmpfs/RAM** so the drive can be unplugged,
work fully untethered, and later **explicitly commit** changes back to the drive
when re-plugged.

Core philosophy: **complex simplicity** — a small static binary, clear commands,
predictable behavior, minimal magic. Linux-first (Arch/Manjaro), leaning on native
Linux facilities rather than reinventing them.

## Two invariants (drive every decision)

1. **Never write to the SD card.** NeonRoot's own state/cache is redirected to
   tmpfs; only tiny, rarely-written *config* may live on the card. All path
   resolution is centralized in `internal/platform` so nothing strays.
2. **"Repo availability" is a first-class state, not an error path.** The external
   drive is usually absent; every repo-touching command resolves availability up
   front and treats "unavailable" as an expected branch — failing cleanly with a
   clear reason, never a silent overwrite or a crash.

## Decisions locked

- **Config + repo index: TOML for both** (`BurntSushi/toml`) — Arch-native,
  comment-friendly, hand-debuggable on the drive.
- **Domain types in `internal/domain/`; no `pkg/`** — a personal single binary has
  no external importers; `internal/` gives a compiler-enforced boundary for free.
- **Rich UX via the Charm stack** (`lipgloss` + `bubbles` + `bubbletea`) behind a
  `ui.Reporter` interface with a plain/`--quiet` fallback for non-TTY. Carries the
  neon/synthwave aesthetic via a shared Lip Gloss theme.
- **Explicit commit only** — never auto-saves (protects SD-card write endurance).
  Commit can target the same repo, a different repo, or a new name (`--as`).

## Architecture

```
cmd/                  thin Cobra commands (RunE only) → use-case methods
internal/
  domain/             pure types + sentinel errors, zero I/O
  platform/           SD-safe paths, mountinfo, flock, statfs, exec runner
  ui/                 Reporter interface; Lip Gloss neon theme; TTY + plain renderers
  config/             TOML user config + repo registry (name→path)
  repo/               repo resolution, availability (RepoState), index.toml, fingerprint
  hydration/          copy repo→tmpfs, build load-time manifest, progress events
  workspace/          "use case" orchestration: Load = repo + hydration + manifest
  session/            tmux adapter (interface + exec impl + fake)
  runtime/            podman adapter (interface + exec impl + fake); graphroot→tmpfs
  env/                bananenv adapter (interface + exec impl + fake)
  commit/             rescan+diff vs manifest, conflict detection, write-back, --as/--force
  state/              runtime state on tmpfs: what's loaded, locks, session map
bases/                base container images (arch-minimal)
repos/                example repo (index.toml)
```

**Conventions:** domain types in one package (avoids the hydration↔commit import
cycle); adapter interfaces defined in the consuming package, each with an exec impl
(via `platform.Runner`) plus a recording fake so orchestration is unit-testable
with no Podman/tmux present; real binaries only exercised in a `//go:build
integration` suite. `cmd/root.go` builds an `App` composition root; commands map
sentinel errors → messages + exit codes.

### Path layout (the SD-write guarantee)

| Data | Location | Backing |
|------|----------|---------|
| Config (`config.toml`) | `$XDG_CONFIG_HOME/neonroot` | SD card (only allowed write) |
| State, locks, manifests | `$XDG_RUNTIME_DIR/neonroot` → `/run/user/$UID` | tmpfs |
| Hydrated workspaces | `$TMPDIR/neonroot-$UID/workspaces` | tmpfs (`/tmp`) |
| Cache / scratch | `$TMPDIR/neonroot-$UID/cache` | tmpfs |

## Native Linux facilities (use vs skip)

| Facility | Decision | Why |
|---|---|---|
| XDG base dirs | Use | Config on card; state/cache redirected to tmpfs — the no-SD-write split. |
| `/run/user/$UID` (`XDG_RUNTIME_DIR`) | Use | Per-user tmpfs (0700) for state, locks, Podman graphroot. |
| `flock` (LOCK_EX\|LOCK_NB) | Use | Guard load/commit/state mutation; friendly "already running". |
| `unix.Statfs` free-space pre-flight | Use | Fail before hydration, not mid-copy into RAM. |
| `/proc/self/mountinfo` + device compare | Use | Backs `RepoState`; stale mountpoint dirs make `Stat` wrong. Match a re-plugged drive by a stable marker, not mountpoint path. |
| Rootless Podman graphroot on tmpfs | Use — **risky** | Layers in RAM so unplugging doesn't strand storage; user-ns overlay on tmpfs is finicky — prototype early. |
| inotify / fsnotify | Skip | Needs a daemon, misses offline changes; commit-time rescan is better. |
| overlayfs upperdir as diff | Skip (v1) | Privilege/whiteout complexity; possible v2 optimization. |
| systemd user units | Skip (v1) | Commands are one-shot; revisit for udev plug detection later. |

## Dirty-state / commit-diff strategy

**Hydrate-time manifest, rescanned at commit** (no daemon, no privileges, survives
the whole unplugged session; the same walk powers `status`):

- **On `load`:** record per file `relpath → size, mtime, content-hash` (fast
  non-crypto hash; mtime-first, hash lazily). Persist to tmpfs. Also record a
  **source fingerprint** of the origin repo (`index.toml` `revision` + `updated_at`).
- **On `commit`:** re-walk tmpfs, diff vs load manifest → **added / modified /
  deleted** (mtime mismatch confirmed by hash to avoid tmpfs↔drive granularity
  false positives). Show the diff before writing; copy only changed files, remove
  deletions, preserve mtimes.
- **Conflict ("drive changed underneath you"):** compare the target repo's current
  `revision`/fingerprint to the stored source fingerprint. Match → fast path.
  Differ → **never silently overwrite**; report per-file conflicts and funnel to
  `--as <newname>` or explicit `--force`. NeonRoot **detects and redirects; it does
  not merge** (out of scope).

## Phased delivery

Ordered by importance to the structural foundation. Each phase is an independently
testable deliverable (unit-testable with fakes; no drive/Podman needed until Phase 3).

- **Phase 0 — Foundations.** ✅ **Done.** `internal/domain`, `internal/platform`
  (SD-safe xdg, statfs, flock, mountinfo), `internal/config` (TOML + registry),
  `internal/ui` (Reporter + neon theme), `App` composition root, all commands →
  `RunE`, `pkg/` removed, deps added. `list` works. Binary ~7 MB.
- **Phase 1 — Repo resolution & availability.** ✅ **Done.** `internal/repo`:
  `index.toml` read/write with `schema_version` rejection, `RepoState` via
  mountinfo (distinct-mount vs stale-mountpoint), `Fingerprint`, atomic writes,
  `Bump`. `list` shows availability, `status` shows repo contents, `create`
  initializes a repo + adds a workspace (flock-guarded), `repo add` registers a
  repo path in config. Clean `ErrRepoUnavailable`/`ErrRepoNotFound` exit codes.
- **Phase 2 — Hydration.** ✅ **Done.** `internal/hydration`: statfs pre-flight,
  walk-copy repo→tmpfs preserving mode/mtime, single-read fnv64 content hashing,
  per-byte progress via `ui.Reporter`, symlink handling, atomic manifest I/O.
  `internal/workspace`: `Loader.Load` orchestrates (availability → index lookup →
  double-load guard → hydrate → persist manifest + state with source fingerprint,
  rollback on failure) plus loaded-workspace tracking. `load` works; `status`
  lists loaded workspaces. Verified: load → unplug → workspace still usable.
- **Phase 3 — Session & runtime.** ✅ **Done.** `internal/session` (tmux) and
  `internal/runtime` (podman) adapters over the `platform.Runner` seam, with
  recording fakes and a shared `runnertest.Recorder`; Podman pins graphroot→/tmp
  and runroot→/run/user on every call. `load` starts a tmux session (graceful
  degrade if tmux absent); added `attach` (stdio handover via syscall.Exec) and
  `stop` (kill session + drop tmpfs copy). Real-Podman-on-tmpfs validation lives
  in a `//go:build integration` suite. **Flag still open:** run that suite on the
  Arch image to confirm rootless overlay-on-tmpfs behavior.
- **Phase 4 — Commit & dirty-tracking.** ✅ **Done.** `internal/commit`: `Diff`
  (added/modified/deleted, mtime-then-hash), `HasConflict` vs source fingerprint,
  in-place `ApplyDiff` (delta-only write-back), `UpdateManifest` re-baseline, and
  a `Committer` handling in-place vs save-as. `commit <ws> [--repo][--as][--force]`
  and diff-mode `status <ws>` are real. Verified end-to-end: edit → status diff →
  commit → drive updated → clean; conflict → exit 5; `--as` copy; `--force`
  override. hydration refactored to share identity/copy helpers with commit.
- **Phase 5 — Model completion + polish.** ✅ **Core done.**
  - Content model: shipped default template (go:embed) + `create --from` copying
    an existing workspace; optional `image` on a workspace; `list workspaces`.
  - Runtime: `load` starts a container for workspaces that declare an image
    (`--pull=never`, workspace bind-mounted at `/workspace`), tmux execs a shell
    inside it; `--no-container` and graceful degrade to host-only. `stop` stops
    the container.
  - **Risk retired:** validated rootless Podman with graphroot on tmpfs
    (`/dev/shm`, overlay driver) runs containers successfully (podman 5.8.3).
  - **Still polish/backlog:** multi-bar hydration, `--json`, `env`/Bananenv hook,
    shell completion, interactive Bubble Tea `list`.

## Follow-ups surfaced during Phase 5 (tmpfs container storage)

1. **Base images must be populated into the tmpfs graphroot each boot** — a
   relocated graphroot starts empty and does not persist. NeonRoot needs to build
   from `bases/` or `podman load` base images into the tmpfs store before `load`
   can start a container after a fresh boot. This is the missing link that makes
   declared images actually runnable. (Not yet built.)
2. **Cleaning the rootless graphroot needs `podman unshare rm` / `podman system
   reset`**, not `os.RemoveAll` — layer dirs are owned by subuid-mapped users.
   Moot across reboots (tmpfs clears), but relevant to any in-session wipe.

- **Phase 6 — Workspace-first UX.** ✅ **Done.** The workspace is the primary
  object: bare `neonroot list` shows workspaces (repo/image/loaded); repo listing
  moved to `repo list`. Repo is one-time setup — `repo add` makes the first real
  repo the default (replacing the scratch placeholder), plus `repo set-default`,
  so workspace commands need no `--repo`. A workspace *uses* a repo (storage) and
  optionally an image (runtime), handled git-remote / docker-reference style.

## Out of scope (at least initially)

Full GUI · cloud sync / remote execution · package management inside workspaces ·
complex multi-container networking · automatic background syncing · Windows/macOS.

## Hard / risky problems being tracked

1. Rootless Podman graphroot on tmpfs (user-ns overlay behavior is image-dependent).
2. Reliable mount/unmount detection across re-plugs (match by stable repo identity,
   not mountpoint path).
3. Conflict detection without merge (detect + redirect to `--as`/`--force` only).
4. mtime fidelity across tmpfs↔drive filesystems (hash-confirm mandatory).
5. SD-write avoidance is only as good as path discipline (all path resolution
   centralized in `internal/platform`, guarded by test).
