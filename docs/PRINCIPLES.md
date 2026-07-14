# NeonRoot — Principles

The compass. Every design decision is checked against these. If a change fights a
principle, the change is usually wrong.

## 1. Never write to the SD card
NeonRoot boots from write-sensitive flash. Its own state, cache, locks, and
workspaces live in **tmpfs**; only tiny, rarely-changed *config* (`config.toml`)
may touch the card. All path resolution is centralized in `internal/platform` so
nothing strays — and a unit test asserts it. You decide *when* persistent storage
is written (that's what `commit`/`sync` are); NeonRoot never writes behind your back.

## 2. Offline-first, untethered
The drive is usually **absent**. Working with it unplugged is the default case, not
an error path. Every vault-touching command resolves availability up front and treats
"unavailable" as an expected branch — a clear message, never a crash or silent
overwrite. Git makes the sync model native: clone plugged, commit offline, push on
reconnect.

## 3. No control plane
One static binary. No daemon, no server, no account, no telemetry. NeonRoot is
sovereign by construction — it runs in a Faraday cage. This is the line that separates
it from every cloud CDE.

## 4. Non-destructive by default
Uncommitted or **unpushed** work is precious and is never discarded implicitly.
Conflicts are detected and *refused* (`--force` maps to `git push --force-with-lease`,
never a bare force); destructive actions require an explicit flag. "Dirty" always
means working-tree changes **or** unpushed commits.

## 5. Ergonomics live in data, not the binary
The tool is a thin orchestration *engine*. Editor configs, a `.tmux.conf`, language
toolchains, a whole LazyVim setup — these live in **templates and images**, which are
editable, shareable files. The binary stays small (~8 MB) and the ergonomics are
infinitely extensible without recompiling. Want to change how a session behaves? Edit
the image's dotfiles, not the source.

## 6. A workspace is a normal host directory
Loaded workspaces are plain directories in tmpfs and plain git repos. Any host tool —
VS Code, Zed, JetBrains, ripgrep — works on them directly. NeonRoot doesn't trap you
in its own world; it puts a shell where the work is and gets out of the way. It also
doesn't impose a multiplexer: bring your own tmux, or let an image's dotfiles start one.

## 7. Hot/cold is the model
RAM is the **hot** working copy; the vault (drive, or any write-controlled location) is
**cold** storage. This split — and controlling *when* cold storage is written — is
NeonRoot's core idea, not an implementation detail. It's what serves both use cases from
one mechanism: **portability** (unplug and go) and **write-batching** (e.g. a
`$HOME/.claude` vault synced in bursts to spare a flash card).

## 8. Complex simplicity
Small binary, clear commands, predictable behavior, minimal magic. Prefer native Linux
facilities (XDG, tmpfs, flock, mountinfo, git, Podman) over reinventing them. Every
adapter is invoked via a `platform.Runner` seam so it's testable without spawning the
real tool, and every external dependency (git/tmux/podman) degrades gracefully when
absent.
