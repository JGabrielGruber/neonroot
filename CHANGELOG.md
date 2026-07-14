# Changelog

Notable changes, newest first. NeonRoot is pre-1.0; the model has evolved
deliberately (see `docs/VISION.md` for where it's going).

## Unreleased — image templates & session model

- **arch-dev image template** — a batteries-included terminal dev environment
  (nvim + LazyVim with plugins pre-synced, tmux with continuum session saving,
  starship, just, and the modern CLI kit). `image create --template arch-dev`.
- **Templates for images too** — named image definitions (`minimal`, `arch-dev`),
  mirroring workspace templates; user templates in `$XDG_CONFIG_HOME/neonroot/`.
- **Sessions live in the image, not the binary** — `attach` execs a plain shell
  into the container; the image's dotfiles decide whether to start tmux. No host
  tmux nesting, no imposed multiplexer. Per-workspace `--shell` override.
- **CRUD completeness** — `set` (edit `--rename`/`--image`/`--mount`/`--shell`/…),
  `rm`, `vault rm`, `vault set`, `image set --rename` (re-tags stored data).
- **Repo cleanup** — removed dead code and stale scaffolding; docs restructured
  around the product (this changelog, `PRINCIPLES`, `ARCHITECTURE`, `VISION`).

## Evolution v0.1 — vault, git-backed workspaces, image data

The pivot from a hand-rolled engine to battle-tested tools.

- **`repo` → `vault`** rename throughout (workspaces are git repos *inside* a vault).
- **Git-backed workspaces** — a bare git repo per workspace; `load` = `git clone`,
  `commit` = commit + push, conflicts = git's real merge/rebase (`--rebase`/`--as`/
  `--force-with-lease`). Retired the custom manifest/diff/conflict engine.
- **Image data in the vault** — `image build` → `podman save` an `image.tar`;
  `load` runs it offline via `podman load` straight from the drive.
- **Sidecar pods** — a workspace with multiple images runs as a podman pod
  (primary + sidecars over localhost). **Snapshots** — workspace git tags; image
  `podman commit` → save.
- **Workspace-first UX** — `list` shows workspaces; a vault is one-time setup.

## Foundation v0.0 — the engine

- Domain types + sentinel errors; SD-safe path resolution; `/proc/self/mountinfo`
  availability; flock; statfs pre-flight.
- Hot/cold storage manager: hydrate a workspace into tmpfs, work untethered, commit
  back. Rootless Podman with graphroot on tmpfs; tmux sessions; TOML config +
  vault registry; Lip Gloss neon UI.
- Cobra command surface with an `App` composition root; adapters via a testable
  `platform.Runner` seam; a `//go:build integration` suite for real git/podman/tmux.
