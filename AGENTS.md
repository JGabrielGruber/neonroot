# AGENTS.md

Guidance for AI agents. NeonRoot is unusual: it's **a tool agents can use** (a safe,
disposable sandbox) *and* a codebase agents help build. This file covers both.

---

## Part 1 — Using NeonRoot as a sandbox

NeonRoot gives an agent a **safe, disposable, local, git-native environment** to do work
in — the sovereign counterpart to a cloud CDE sandbox. A workspace is a git clone in a
container living in tmpfs: spin one up per task, run in it, keep the good results, throw
the rest away. Nothing leaves the box.

### The one-shot: `spawn`

The atomic agent task — create a throwaway box, run a command, propagate its exit code,
and reap it:

```bash
# run a project's tests in a fresh, locked-down box seeded from the current dir:
neonroot spawn --image ci --sandbox --seed . -- go test ./...

# run something you don't trust with NO network:
neonroot spawn --image ci --isolated -- ./unknown-build.sh

# keep the box afterward to inspect / commit the result:
neonroot spawn task-42 --image ci --sandbox --seed . --keep -- make build
```

- Everything after `--` is the command; an optional name may precede it (else one is
  generated: `spawn-<hex>`).
- The command's **exit code becomes `spawn`'s exit code** — a failed command fails your
  pipeline. Output streams live.
- Defaults to the **scratch vault** (tmpfs, wiped on reboot), so throwaway boxes cost
  nothing. `--image` is required (you need a container to run in).

### Isolation tiers — a sandbox *distrusts the code*

| | `--sandbox` | `--isolated` |
|---|---|---|
| Host identity (ssh agent, gitconfig) | withheld | withheld |
| Capabilities | `--cap-drop=ALL` | `--cap-drop=ALL` |
| Privilege escalation | `no-new-privileges` | `no-new-privileges` |
| Memory / PID limits | 2g / 512 | 2g / 512 |
| **Network** | **up** (fetch deps) | **off** (`--network=none`) |

Use `--sandbox` for building/testing (needs the network for deps); `--isolated` for code
you don't trust. Isolation is mutually exclusive with `--secrets` — a sandbox must never
carry your identity.

### Persistent boxes and the review loop

For iterative work, make it a normal workspace and keep it:

```bash
neonroot create work --image ci --sandbox --seed .   # or --isolated
neonroot load work                                   # clone into RAM + start the container
neonroot run work -- pytest -q                       # headless, exit code propagated
neonroot attach work                                 # a shell inside (interactive)
neonroot logs work                                   # container/pod logs (debugging)
# happy with it? keep the work:
neonroot commit work --as agent/task-42              # push to a branch (no conflict)
# not happy? throw it away:
neonroot stop work && neonroot rm work
```

`list`/`status` show a `(sandbox)`/`(isolated)` marker so you always know a box is locked
down. This is the **"commit the good runs, `rm` the rest"** model.

### Honest boundary — NOT a VM

Rootless podman + dropped caps + no-new-privs + optional no-network is **strong
defense-in-depth, not a hermetic guarantee**. Do not treat an agent box as bulletproof
against actively hostile code. (No `--read-only` rootfs or seccomp tuning yet.)

### Requirements

`git` + `podman` (for containers). Remote vaults also use `ssh`/`scp` (or `rsync`).
Everything runs on the local machine, offline-capable.

---

## Part 2 — Developing NeonRoot

### Shape

A single static Go binary — a **thin engine**. Opinionated content (dev environments,
scaffolding) lives in **editable data**, not Go: image/workspace templates under
`internal/template/files/`. When adding an "environment" feature, prefer a template.

```
cmd/          thin Cobra commands (RunE) → orchestrate via the App composition root
internal/
  domain/     pure types + sentinel errors, zero I/O
  platform/   SD-safe paths, mountinfo, flock, the exec Runner seam (+ runnertest)
  config/     TOML user config + vault registry
  vault/      catalog (index.toml / remote _catalog.git), availability, image layout
  git/        git adapter (workspaces are git repos)
  remote/     ssh vault addressing (Addr) + scp/ssh/rsync transport
  secrets/    opt-in env-file + ssh-agent/gitconfig passthrough
  runtime/    podman adapter: graphroot→tmpfs, images, pods, sandbox flags
  workspace/  load orchestration + loaded-workspace state
  session/    tmux adapter (host-only sessions)
  template/   embedded + user templates
  tui/        Bubble Tea cockpit (no import of cmd; driven via Deps)
```

See `docs/ARCHITECTURE.md` for the design, `docs/VISION.md` for the why, `docs/ROADMAP.md`
for what's next, `CHANGELOG.md` for what shipped.

### Build, test, gate

Before every commit, all of these must be clean:

```bash
gofmt -l cmd/ internal/     # prints nothing when clean
go build ./...
go vet ./...
go test ./...
```

Integration tests (real podman/ssh) are behind a build tag and skip when a dep is absent:

```bash
go test -tags integration ./...
```

### Conventions

- **Adapters go through `platform.Runner`.** Unit-test them by asserting the exact argv via
  `runnertest.Recorder` — never spawn a real binary in a unit test. Real tools are exercised
  only in the `//go:build integration` suite.
- **Consumer-defined interfaces:** e.g. `workspace.Loader` declares small `Git`/`Runtime`/
  `Sessions` seams; concrete adapters satisfy them structurally.
- **Sentinel errors → exit codes** in `cmd/root.go` (`exitCode`): unavailable→3, locked→4,
  conflict→5.
- **Never write the SD card.** Only `config.toml` lives there; all state/clones/locks/secrets
  are redirected to tmpfs (`internal/platform/paths.go` is the single source of truth).
- **Staged commits on `master`.** Plan an epic as a numbered, independently-shippable stage
  sequence; commit each stage the moment it's green. Update `docs/ROADMAP.md` status,
  `CHANGELOG.md`, and the READMEs/ARCHITECTURE as part of the epic's commits. End every commit
  message with:
  ```
  Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01Jw2XVj6VffygFFzg2oDdYP
  ```

### Dogfood to validate

The strongest validation is running NeonRoot *through NeonRoot*. Build a `ci` image, seed
the repo as a workspace, and run the integration suite inside the container's own sshd —
no host ssh config touched:

```bash
neonroot image create ci --template ci && neonroot image build ci
neonroot create nr --image ci --seed .
neonroot load nr
neonroot run nr -- sh -c 'ensure-sshd && go test -tags integration ./...'
neonroot stop nr && neonroot rm nr
```

This flow already caught a real bug the mocked unit tests missed — prefer it for anything
touching the ssh/rsync/catalog paths.

### Adding things

- **A dev environment / toolchain** → a new image template dir under
  `internal/template/files/imagedefs/<name>/` (a `Containerfile`, plus any files). It appears
  in `image create --template <name>` automatically. Version managers, editors, sshd, etc.
  belong here, not in Go.
- **A command** → a small `cmd/<verb>.go` with a Cobra command; orchestrate via the `App`
  helpers (`resolveVault`, `requireAvailable`, `catalog`, `podman`, `lock`). Reuse the
  factored helpers (`createWorkspace`, `stopWorkspace`, `removeWorkspace`).
