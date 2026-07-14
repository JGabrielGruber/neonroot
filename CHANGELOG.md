# Changelog

Notable changes, newest first. NeonRoot is pre-1.0; the model has evolved
deliberately (see `docs/VISION.md` for where it's going).

## Unreleased

### polish

- **TUI CRUD** — the cockpit now deletes (`d`, with a y/N confirm), renames (`e`),
  and creates with an image (`n` accepts `name [image]`), on top of the existing
  load/attach/commit/stop/sync.
- **Remote image `snapshot`/`rm`** — both now work against a remote vault (over
  scp/rsync + ssh); only `image set --rename` stays local-only.
- **`mise` in arch-dev** — declarative toolchains: a workspace's `.mise.toml` /
  `.tool-versions` is restored on every load.
- **Fix** — `load` no longer fails with "container name already in use" when a
  container lingered from a past session; `run`/`pod create` use `--replace`.

### E3.6 — secrets passthrough + rsync

- **Secrets / identity passthrough** — opt-in, per-workspace, ephemeral. On load
  (`create --secrets`, `set --secrets`, or `load --secrets`), NeonRoot injects
  environment variables from **bananenv** (github.com/JGabrielGruber/bananenv —
  its tmpfs env store) and/or a `load --env-file <dotenv>`, and bind-mounts the
  host SSH agent socket + read-only `~/.gitconfig` so in-container `git push` over
  ssh works. Env vars go via podman `--env-file` (values never appear in `ps`);
  the private key never enters the container (only the agent socket). Nothing lands
  on the card — the env-file is tmpfs, wiped on `stop`. `list`/`status` show a
  `(secrets)` marker. bananenv is optional (absent → skipped).
- **rsync transport (opt-in)** — `vault add --rsync` / `vault set --rsync` makes a
  remote vault prefer rsync (resume + skip-unchanged) over scp for image transfers,
  falling back to scp when rsync is absent on either end.

### E3 — reach & safety

- **Remote vaults** — a vault can live on an ssh server, not just a local drive:
  `neonroot vault add cloud ssh://user@host/path` (or `user@host:path`). A remote
  vault uses the *same layout* as a local one — catalog, git workspaces, image
  tarballs — reached over git + scp instead of the filesystem.
  `create`/`load`/`commit`/`sync`, `image create`/`build`/`ls`, and `list` all
  work against it. Availability is **optimistic** (no network probe — offline-first
  and snappy); ops fail lazily if the host is unreachable. Cross-device catalog and
  workspace writes reconcile on **git non-fast-forward** (`ErrCommitConflict`),
  never a silent clobber — the catalog is itself a `_catalog.git` bare repo.
  `image snapshot`/`set`/`rm` against a remote are deferred with a clear message.
- **Encryption & multi-device** *(docs)* — both fall out of the model: a vault path
  can be a mounted gocryptfs/LUKS filesystem, and the same remote on two machines
  syncs through git non-ff. See `docs/ARCHITECTURE.md`.

### E2 — fullstack toolbelt
- **Language + database image templates** — `node`, `python`, `go`, `rust`
  (language) and `postgres`, `redis` (sidecar services), alongside `arch-dev`
  and `minimal`. `image create <name> --template <t>`.
- **Sidecars in one command** — `create app --image node --with postgres,redis`
  runs the app + database + cache as a podman pod sharing localhost.
- **Ports + `up`** — `create --port 3000,5432` publishes to the host;
  `neonroot up <ws> [-- cmd]` runs the dev command inside the container (or the
  workspace's declared `--up` command). Plug in, `up`, open localhost:3000.

### E1 — human-first
- **Interactive TUI cockpit** — `neonroot` with no args: vaults/workspaces/images
  dashboard with one-key load/attach/commit/sync/stop, live refresh. `--no-tui`
  falls back to help.
- **Safety layer** — `sync` (push all pending work), `doctor` (preflight),
  `guard [vault]` (scriptable unplug gate), `path`/`code` (open a workspace in
  any host editor).

### image templates & session model

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
