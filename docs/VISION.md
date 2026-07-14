# NeonRoot — Product Vision & Next Evolution

> Forward-looking strategy. For the build history see `ROADMAP.md`.
> Market scan: July 2026.

## 1. What NeonRoot is today (honest snapshot)

A single ~8 MB static Go binary that manages **hot/cold storage of dev
workspaces**: a *vault* (a directory on an external drive — or any
write-controlled location) holds a **git repo per workspace** plus **container
image data**; `load` clones a workspace into **tmpfs/RAM** so the drive can be
unplugged; you work untethered; `commit` pushes changes back. Containers (rootless
Podman, graphroot on tmpfs) run the workspace; a workspace with several images
runs as a pod (app + sidecars). Ergonomics (nvim/LazyVim, tmux, starship,
powertools) live in **editable image/workspace templates**, not the binary.

It works — end-to-end, on real hardware. But it is currently a **power tool for
one expert user** (its author). The gap between "works" and "product" is UX,
onboarding, and a coherent toolbelt. This document is about crossing it.

## 2. The market map (2026)

The dev-environment space has split into four camps. None own NeonRoot's ground.

| Camp | Players | Model | Weakness NeonRoot exploits |
|------|---------|-------|-----------------------------|
| **Cloud CDEs** | Coder, GitHub Codespaces, Daytona, Gitpod→**Ona** | Env lives on remote infra; needs connectivity; per-seat or self-host infra cost | Useless offline / untethered; needs a control plane and a network |
| **Client-side devcontainers** | **DevPod** | No control plane; devcontainer spec on local Docker or a remote | Docker/cloud-oriented; assumes disk is always present; no portability/sync/write-control |
| **Local container dev** | **distrobox**, **Toolbx** | Rootless Podman, host-integrated (home, GPU, keys) | Not portable; no sync; no removable/ephemeral-storage model |
| **Reproducible envs** | **Nix**, **Devbox**, **Flox** | Deterministic packages on the host | Solve *sameness*, not *location*; no offline-carry story |
| **Mobile / on-the-go** | **Termux**, code-server, proot-distro | Full Linux on Android; iPad crippled | Phone-shaped; not "laptop + drive"; no cold/hot sync discipline |

Two 2026 shifts matter:
- **Gitpod rebranded to Ona and pivoted to AI-agent orchestration** (Classic shut
  down Oct 2025); **Daytona repositioned to AI sandboxes**. The incumbents are
  *leaving* the "developer's own environment" lane for the agent lane.
- The reproducibility crowd (Nix/Flox) proved developers want environments that
  are **portable and deterministic without a cloud** — but they stop at packages;
  they don't move the *whole working state* around.

## 3. NeonRoot's position — the empty quadrant

Two axes the incumbents never cross simultaneously:

```
                 needs connectivity ── OFFLINE-FIRST
                        │                    │
   cloud control plane  │  Coder/Ona/        │
   (server/daemon)      │  Codespaces        │   ← nobody
                        │  Daytona           │
        ────────────────┼────────────────────┼────────────
   no control plane     │  DevPod            │  ★ NeonRoot
   (just a binary)      │  distrobox/Toolbx  │    (portable, sync-based,
                        │  Nix/Devbox/Flox   │     write-controlled)
```

**NeonRoot is the only "no-control-plane + offline-first + portable + sync-based"
option.** Its defensible, unclaimed thesis:

> Cloud CDEs move your environment *to* the network. Nix makes it *reproducible*.
> distrobox makes it *local*. **NeonRoot makes it *portable, offline, and
> sync-controlled* — you carry a full dev environment on a drive, work with it
> unplugged, and decide when storage is written.** It's *git for your
> environment's location.*

Unique primitives no competitor has as a first-class concept:
- **Hot/cold split** — RAM is the working copy; the drive is cold storage; you
  control *when* persistence happens (SD-card/flash-wear, battery, privacy).
- **Untethered operation** — unplug and keep working; sync on reconnect. (Git
  makes this native; the vault makes it physical.)
- **Removable/ephemeral storage as the model**, not an edge case.
- **One binary, no daemon, no account.** Sovereign by construction.

## 4. Who this is for (market)

Not the enterprise-fleet buyer (that's Coder's). NeonRoot's beachheads:

1. **Nomadic / constrained-hardware devs** — travel light: minimal Arch on an
   SD-card SBC/laptop, heavy env on a USB/SSD. Battery- and write-conscious.
2. **Sovereign / air-gapped / privacy devs** — no cloud, no telemetry, works in a
   Faraday cage. Journalists, defense, regulated, offline-grid.
3. **Homelab / self-host / tinkerer** — the distrobox/Nix crowd who want
   *portability* on top of local. Arch/Fedora-Silverblue-adjacent.
4. **The "second machine" case** — carry your exact env to a borrowed/loaner box:
   plug in, `load`, work, `commit`, unplug — leaving nothing behind.
5. **(Emerging) AI-agent operators** — disposable, isolated, syncable per-task
   workspaces (see §7).

This is a **wedge, not a TAM grab**: own "portable offline dev env" for the
Linux-CLI-native crowd, then expand along the toolbelt (§6) and reach (§7).

## 5. Human-first: from power tool to product

The single biggest lever. Priority order:

- **A TUI home screen (the product-defining move).** `neonroot` with no args opens
  an interactive dashboard: vaults (mounted?), workspaces (hot/cold, dirty/ahead),
  images (built?), RAM headroom — with one-key **load / attach / commit / stop**.
  This is what turns a CLI into a *cockpit*. (Bubble Tea was scoped and can return;
  the `ui.Reporter` seam already anticipates it.)
- **`neonroot init` onboarding wizard** — first run: detect git/podman/tmux, pick a
  vault path, offer to build the `arch-dev` image, create a first workspace. Zero
  mental-model prerequisite.
- **`neonroot doctor`** — preflight: tools present? drive mounted? RAM headroom for
  a load? any workspace dirty/unpushed *before you unplug*?
- **Safety nets (the trust layer).** The whole promise is "unplug freely" — so
  NeonRoot must never let you lose work silently:
  - **`neonroot sync`** — commit + push every dirty/ahead loaded workspace at once.
  - **Unplug guard** — a `pre-unmount`/`status --unsafe` that screams if anything
    is uncommitted or unpushed.
  - **Auto-snapshot to a *non-drive* tmpfs checkpoint** on an interval (opt-in), so
    a crash mid-session isn't total loss — without touching the SD card.
- **Discoverability** — shell completion, `--json` everywhere (scriptable), a man
  page, richer `--help` with examples.
- **Editor freedom, surfaced.** A killer latent feature: **the workspace is a normal
  directory on the host** (`WorkspaceRoot`). Any host editor — VS Code, Zed,
  JetBrains — can open it directly while the container runs the toolchain. Document
  and add `neonroot path <ws>` / `neonroot code <ws>` so it's not a secret.

## 6. The mobile fullstack toolbelt (built-ins & integrations)

The `arch-dev` image is the seed. To be "a fullstack dev toolbelt on the fly":

- **A library of image templates** — `node`, `python`, `go`, `rust`, `ruby`,
  `php-laravel`, `elixir`, plus `arch-dev` (editor). Shipped as named
  `image --template`s; community-shareable. This is the "toolbelt."
- **Databases & services as sidecars (fullstack, today's pods).** We already run
  pods — make it ergonomic: `create app --image node --with postgres,redis`. App +
  DB + cache in one workspace, reachable over localhost. This is the differentiator
  distrobox/devcontainers make painful.
- **Port forwarding / live preview.** `neonroot up <ws>` → run the project's dev
  server and forward the port to the host (Podman does it; surface it). "Plug in,
  `up`, open localhost:3000."
- **Secrets & identity.** The real gap for ephemeral envs: inject SSH keys, git
  identity, tokens into a loaded workspace from the vault (or host agent) without
  persisting them on the SD card; wipe on `stop`. Table stakes for "real work."
- **Version managers.** Bake `mise`/`asdf` into templates so per-project toolchains
  are declarative.
- **Vault encryption.** `gocryptfs`/LUKS-backed vault — you're literally carrying a
  dev environment on a USB; make it safe to lose the stick.
- **Remote vaults (the cloud bridge).** A vault addressable over `ssh://`/`https://`
  git — sync to a server *when online*, work offline otherwise. This bridges
  NeonRoot toward the CDE world **without becoming a CDE** (no control plane, no
  always-on). Offline-first, cloud-optional.
- **Multi-device sync.** Because it's git + a drive, "same env on laptop and SBC"
  is nearly free — formalize it.

## 7. The asymmetric bet: the offline substrate for AI agents

The incumbents (Ona, Daytona) just vacated developer-owned environments *to chase
agents in the cloud*. NeonRoot can take the **inverse, un-contested** position:
**local, disposable, syncable sandboxes for coding agents.**

- A workspace is already a **cheap, isolated, throwaway git clone in a container.**
  That is exactly what you want per agent task: spin up, let the agent work,
  **commit the good ones, `rm` the rest** — no cloud bill, no data leaving the box.
- The user's own `$HOME/.claude`-as-a-vault insight generalizes: run agents against
  RAM copies, sync deliberately, keep the SD card and your real home clean.
- Positioning: *"Cloud agent platforms rent you sandboxes. NeonRoot gives you an
  unlimited local fleet you own — offline, disposable, and git-native."*

This is speculative but cheap to reach from here (parallel workspaces + pods +
git already exist) and rides the exact wave the incumbents are betting on — from
the sovereign/local side they've abandoned.

## 8. Phased next evolution

- **E1 — Human-first (the product leap).** TUI home screen · `init` wizard ·
  `doctor` · `sync` + unplug-guard · completion/`--json` · `path`/editor surface.
  *Turns the power tool into a product.*
- **E2 — Fullstack toolbelt.** Image-template library (languages) · `--with`
  sidecars (databases) · `up` port-forward/preview · secrets injection · `mise`.
  *Makes it useful for real fullstack work, not just editing.*
- **E3 — Reach & safety.** Remote vaults (ssh/git) · vault encryption ·
  multi-device sync. *Bridges offline-first to cloud-optional; makes carrying an
  env safe.*
- **E4 — Agent substrate.** Disposable per-task workspaces · fleet view · agent
  isolation defaults. *The asymmetric bet.*

## 9. What could kill it (risks to steer by)

- **Audience too narrow.** Mitigation: the toolbelt (E2) and editor-freedom broaden
  it beyond CLI purists; remote vaults (E3) reach the "sometimes online" majority.
- **Podman/tmpfs fragility across distros/kernels.** Mitigation: `doctor`, graceful
  host-only degradation (already there), integration suite on target images.
- **"Why not just Nix/distrobox + a USB + git?"** — the honest competitor is a
  *DIY script*. NeonRoot wins only if the **UX + safety nets + toolbelt** make the
  assembled experience meaningfully better than rolling your own. That is exactly
  what E1/E2 must deliver; it's the whole ballgame.
- **Linux-only.** Fine as a wedge; macOS (via a Linux VM) is a later question.

## 10. One line

**NeonRoot is git for your dev environment's *location*: carry a full, containerized
fullstack toolbelt on a drive, work with it unplugged, and sync when you choose —
no cloud, no daemon, no account.** The nomad's, the sovereign dev's, and (next) the
local AI-agent operator's environment manager.

---

### Sources (market scan, July 2026)
- Coder vs Gitpod vs DevPod review 2026 — devopsboys.com
- Gitpod→Ona pivot / Classic shutdown — bunnyshell.com, openalternative.co
- DevPod client-side / no control plane — pistack.xyz, devtune.ai
- distrobox / Toolbx local Podman dev — tecmint.com, linux-magazine.com, hackandslash.blog
- Devbox vs Dev Containers vs Nix / Flox — devtoolreviews.com, flox.dev, crafteo.io
- Termux / on-the-go dev — thenewstack.io, dev.to
