// Package secrets assembles the ephemeral, opt-in identity a loaded workspace's
// container receives: environment variables (from bananenv and/or a user-supplied
// dotenv file) written to a tmpfs env-file, plus bind-mounts for the SSH agent
// socket and ~/.gitconfig. Nothing durable is written — the env-file lives in
// tmpfs and is removed with the workspace; the SSH agent is a socket, so no key
// material ever enters the container.
package secrets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/platform"
)

// Options controls what to assemble.
type Options struct {
	// Identity includes the "full secrets" set: bananenv env vars plus the SSH
	// agent socket and ~/.gitconfig bind-mounts.
	Identity bool
	// ExtraEnvFile is an optional dotenv file merged on top (later wins).
	ExtraEnvFile string
	// EnvDir is the tmpfs directory the generated env-file is written into.
	EnvDir string
	// Getenv/Home are injectable for testing; nil/"" fall back to the process.
	Getenv func(string) string
	Home   string
}

// envFileName is the generated podman --env-file inside a workspace's state dir.
const envFileName = "secrets.env"

// Build assembles the SessionOpts for a load. Missing bananenv, a missing SSH
// agent, or a missing gitconfig are non-fatal (returned as warnings); only a bad
// explicit ExtraEnvFile or a write failure is an error. A zero result (no env,
// no mounts) is returned when nothing is requested.
func Build(ctx context.Context, runner platform.Runner, o Options) (domain.SessionOpts, []string, error) {
	if !o.Identity && o.ExtraEnvFile == "" {
		return domain.SessionOpts{}, nil, nil
	}
	getenv := o.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	home := o.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	env := map[string]string{}
	var mounts []domain.Mount
	var warnings []string

	if o.Identity {
		be, err := bananenvEnv(ctx, runner)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("bananenv: %v — skipping its vars", err))
		}
		merge(env, be)
		m, idEnv, w := identity(getenv, home)
		mounts = m
		merge(env, idEnv)
		warnings = append(warnings, w...)
	}

	if o.ExtraEnvFile != "" {
		fe, err := parseDotenv(o.ExtraEnvFile)
		if err != nil {
			return domain.SessionOpts{}, warnings, fmt.Errorf("reading --env-file %q: %w", o.ExtraEnvFile, err)
		}
		merge(env, fe)
	}

	var envFile string
	if len(env) > 0 {
		p, err := writeEnvFile(o.EnvDir, env)
		if err != nil {
			return domain.SessionOpts{}, warnings, err
		}
		envFile = p
	}
	return domain.SessionOpts{EnvFile: envFile, Mounts: mounts}, warnings, nil
}

// bananenvEnv returns the vars from `bananenv list`, or nil if bananenv is not on
// PATH (an optional dependency). bananenv emits shell `export K="V"` lines.
func bananenvEnv(ctx context.Context, runner platform.Runner) (map[string]string, error) {
	if _, err := runner.LookPath("bananenv"); err != nil {
		return nil, nil
	}
	out, err := runner.Run(ctx, "bananenv", "list")
	if err != nil {
		return nil, err
	}
	return parseExports(string(out)), nil
}

// parseExports reads bananenv's `export KEY="VALUE"` lines (mirroring bananenv's
// own reader), dropping its internal BANANENV_* bookkeeping.
func parseExports(s string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "export ") {
			continue
		}
		kv := strings.SplitN(line[len("export "):], "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		if key == "" || strings.HasPrefix(key, "BANANENV_") {
			continue
		}
		env[key] = strings.Trim(kv[1], `"`)
	}
	return env
}

// parseDotenv reads KEY=VALUE lines (blank lines and # comments ignored; an
// optional leading "export " and surrounding quotes are stripped).
func parseDotenv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		if key == "" {
			continue
		}
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, `"'`)
		env[key] = val
	}
	return env, nil
}

// identity builds the SSH-agent + gitconfig passthrough. A missing agent or
// gitconfig each downgrade to a warning rather than failing the load.
func identity(getenv func(string) string, home string) ([]domain.Mount, map[string]string, []string) {
	var mounts []domain.Mount
	env := map[string]string{}
	var warnings []string

	if sock := getenv("SSH_AUTH_SOCK"); sock != "" {
		mounts = append(mounts, domain.Mount{Source: sock, Target: "/ssh-agent"})
		env["SSH_AUTH_SOCK"] = "/ssh-agent"
	} else {
		warnings = append(warnings, "SSH_AUTH_SOCK not set — no ssh agent forwarded (in-container git over ssh won't authenticate)")
	}

	gitconfig := filepath.Join(home, ".gitconfig")
	if fi, err := os.Stat(gitconfig); err == nil && !fi.IsDir() {
		// Rootless podman maps the container root to the host uid, so /root is home.
		mounts = append(mounts, domain.Mount{Source: gitconfig, Target: "/root/.gitconfig", ReadOnly: true})
	} else {
		warnings = append(warnings, "~/.gitconfig not found — git identity not passed into the container")
	}
	return mounts, env, warnings
}

// writeEnvFile writes a podman --env-file (KEY=VALUE lines) into dir at mode 0600
// and returns its path.
func writeEnvFile(dir string, env map[string]string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	var b strings.Builder
	for k, v := range env {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	path := filepath.Join(dir, envFileName)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func merge(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}
