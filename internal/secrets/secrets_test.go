package secrets

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/JGabrielGruber/neonroot/internal/platform/runnertest"
)

// readEnvFile parses the generated KEY=VALUE env-file back into a map.
func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		m[kv[0]] = kv[1]
	}
	return m
}

func TestBuild_BananenvPlusEnvFileAndIdentity(t *testing.T) {
	rec := runnertest.New()
	// bananenv list emits shell exports (incl. its own BANANENV_FILE bookkeeping).
	rec.Stdout["bananenv"] = "export FOO=\"bar\"\nexport TOKEN=\"s3cret\"\nexport BANANENV_FILE=\"/tmp/x\"\n"

	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// An extra dotenv that overrides FOO and adds BAZ.
	extra := filepath.Join(t.TempDir(), "extra.env")
	if err := os.WriteFile(extra, []byte("# comment\nFOO=override\nBAZ=qux\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	envDir := t.TempDir()
	opts, warns, err := Build(context.Background(), rec, Options{
		Identity:     true,
		ExtraEnvFile: extra,
		EnvDir:       envDir,
		Getenv:       func(k string) string { return map[string]string{"SSH_AUTH_SOCK": "/run/agent.sock"}[k] },
		Home:         home,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}

	env := readEnvFile(t, opts.EnvFile)
	if env["FOO"] != "override" { // --env-file wins over bananenv
		t.Errorf("FOO = %q, want override", env["FOO"])
	}
	if env["TOKEN"] != "s3cret" || env["BAZ"] != "qux" {
		t.Errorf("missing merged vars: %v", env)
	}
	if _, ok := env["BANANENV_FILE"]; ok {
		t.Errorf("BANANENV_ bookkeeping should be filtered: %v", env)
	}
	if env["SSH_AUTH_SOCK"] != "/ssh-agent" {
		t.Errorf("SSH_AUTH_SOCK in-container = %q, want /ssh-agent", env["SSH_AUTH_SOCK"])
	}

	// Mode 0600 — the env-file holds secrets.
	fi, _ := os.Stat(opts.EnvFile)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("env-file mode = %v, want 0600", fi.Mode().Perm())
	}

	// Mounts: the agent socket (rw) and gitconfig (ro).
	got := map[string]string{}
	for _, m := range opts.Mounts {
		tag := m.Target
		if m.ReadOnly {
			tag += ":ro"
		}
		got[m.Source] = tag
	}
	if got["/run/agent.sock"] != "/ssh-agent" {
		t.Errorf("agent mount: %v", opts.Mounts)
	}
	if got[filepath.Join(home, ".gitconfig")] != "/root/.gitconfig:ro" {
		t.Errorf("gitconfig mount: %v", opts.Mounts)
	}
}

func TestBuild_MissingAgentAndGitconfigWarn(t *testing.T) {
	rec := runnertest.New()
	rec.Missing["bananenv"] = true // bananenv absent → no vars, no error

	opts, warns, err := Build(context.Background(), rec, Options{
		Identity: true,
		EnvDir:   t.TempDir(),
		Getenv:   func(string) string { return "" }, // no SSH_AUTH_SOCK
		Home:     t.TempDir(),                       // no .gitconfig
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(warns)
	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings (agent, gitconfig), got %v", warns)
	}
	if opts.EnvFile != "" {
		t.Errorf("no env expected, but got env-file %q", opts.EnvFile)
	}
	if len(opts.Mounts) != 0 {
		t.Errorf("no mounts expected, got %v", opts.Mounts)
	}
}

func TestBuild_NothingRequested(t *testing.T) {
	rec := runnertest.New()
	opts, warns, err := Build(context.Background(), rec, Options{EnvDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if opts.EnvFile != "" || len(opts.Mounts) != 0 || len(warns) != 0 {
		t.Errorf("expected a zero result, got %+v warns=%v", opts, warns)
	}
	if len(rec.Calls) != 0 {
		t.Errorf("nothing requested should not shell out: %v", rec.Lines())
	}
}
