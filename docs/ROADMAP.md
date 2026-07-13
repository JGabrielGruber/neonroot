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
- **Phase 2 — Hydration.** `internal/hydration`: copy→tmpfs with per-file progress,
  build manifest, statfs pre-flight. `internal/workspace.Load` orchestrates. `load`
  fully works.
- **Phase 3 — Session & runtime.** `internal/session` (tmux) + `internal/runtime`
  (podman) interfaces + fakes + exec impls; Podman graphroot→tmpfs; wire session
  into `load`. **Flag:** validate graphroot-on-tmpfs on the real Arch image.
- **Phase 4 — Commit & dirty-tracking.** `internal/commit`: rescan+diff, conflict
  detection, write-back, `--as`/`--force`; clean `ErrRepoUnavailable` when drive
  absent. `commit` + diff-mode `status` real.
- **Phase 5 — Polish.** Multi-bar hydration, `--quiet`/`--json`, `env`/Bananenv
  provisioning hook, shell completion, richer `status`/interactive `list`, docs.
  Deferred: overlayfs experiment, udev plug detection.

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
