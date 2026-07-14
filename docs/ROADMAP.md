# NeonRoot — Roadmap

Where NeonRoot is going. For *why* (market, positioning) see `VISION.md`; for what
already shipped see `../CHANGELOG.md`.

The engine is done — NeonRoot carries a full containerized dev environment on a drive,
works untethered, and syncs via git. The remaining work turns a **power tool into a
product**: human-first UX, then a fullstack toolbelt, reach, and an AI-agent substrate.

---

## E1 — Human-first ✅ shipped

Turned the CLI into something you can live in — safety-first, then the cockpit.
All items below are shipped.

- **`workspace.Report` helper** — one reusable "is this loaded workspace dirty/ahead/
  behind, and how big?" predicate (reusing `git.Status.HasPendingWork`), consumed by
  everything below.
- **`neonroot sync`** — commit + push every loaded workspace with pending work in one
  go; refuses on conflict (never force). The "before I unplug" button.
- **`neonroot doctor`** — preflight: git/podman/tmux present, vault availability, tmpfs
  headroom, and any unpushed/dirty workspace.
- **`neonroot guard [vault]`** — a scriptable unplug gate: exit 0 when it's safe to
  remove the drive, non-zero when a loaded workspace has unsynced work (wire into a
  udev/eject hook).
- **`neonroot path` / `code`** — surface editor freedom: print the workspace dir /
  open `$EDITOR` on it, so any host editor works on a loaded workspace.
- **TUI cockpit** — `neonroot` with no args opens an interactive dashboard (vaults,
  workspaces hot/cold + dirty/ahead, images) with one-key load/attach/commit/sync/stop.
  A Bubble Tea view over the same CLI verbs. The product-defining move.

## E2 — Fullstack toolbelt (mostly shipped)

Make it useful for real fullstack work, not just editing.

- ✅ **Language image templates** — `node`, `python` (+uv), `go`, `rust` (plus `arch-dev`
  editor, `minimal`), shipped and community-shareable (the "toolbelt").
- ✅ **Databases as sidecars** — over the pod engine:
  `create app --image node --with postgres,redis` (app + DB + cache, reachable over
  localhost). Ships `postgres`/`redis` image templates.
- ✅ **Ports + `neonroot up`** — `create --port 3000` publishes to the host (on the
  pod/container); `neonroot up <ws> [-- cmd]` runs the dev command in the container
  (or the declared `--up` command). "plug in, `up`, open localhost:3000."
- **Version managers** *(next)* — `mise`/`asdf` baked into templates for declarative
  toolchains.

Secrets & identity passthrough moved to **E3** (below) — it's most useful now that remotes
push over ssh.

## E3 — Reach & safety (mostly shipped)

Broaden beyond the local drive; make carrying an environment reach a server.

- ✅ **Remote vaults** — a vault hosted over ssh (`vault add cloud ssh://user@host/path`
  or `user@host:path`). Same layout as a local vault — catalog (`_catalog.git`), git
  workspaces, image tarballs — reached over git + scp. `create`/`load`/`commit`/`sync`,
  `image create`/`build`/`ls`, and `list` all work against a remote; availability is
  optimistic (no network probe), so it's offline-first and cloud-optional. Cross-device
  writes reconcile on git non-fast-forward, not a lock.
- ✅ **Vault encryption** *(docs)* — point a vault's path at a mounted gocryptfs/LUKS
  filesystem; `vault.State` just checks the mount. No special code (see `ARCHITECTURE.md`).
- ✅ **Multi-device sync** *(docs)* — the same remote configured on two machines, each with
  its own tmpfs clone, reconciled by git non-ff (see `ARCHITECTURE.md`).
- ✅ **Secrets & identity** — opt-in, per-workspace, ephemeral passthrough: env vars from
  **bananenv** and/or a `--env-file`, plus the SSH-agent socket + read-only `~/.gitconfig`
  bind-mounted into the container. Into RAM (podman `--env-file`), never on the card, wiped
  on `stop`; surfaced as a `(secrets)` marker. `create/set/load --secrets`.
- ✅ **rsync transport** — `vault --rsync` prefers rsync (resume + skip-unchanged) over scp
  for image transfers, with scp fallback.
- **Remote images: `snapshot`/`set`/`rm`** *(next)* — currently local-vault only; remote
  vaults defer these with a clear message.

## E4 — Agent substrate (the asymmetric bet)

The incumbents (Ona, Daytona) left developer-owned environments to chase agents in the
cloud. Take the inverse, uncontested position: **local, disposable, git-native sandboxes
for AI coding agents.** A workspace is already a cheap throwaway clone-in-a-container —
spin up per task, commit the good ones, `rm` the rest, nothing leaves the box.

- Disposable per-task workspaces; a fleet view; agent-isolation defaults.
- Positioning: *cloud agent platforms rent you sandboxes; NeonRoot gives you an
  unlimited local fleet you own — offline, disposable, sovereign.*
