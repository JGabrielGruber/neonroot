package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/git"
	"github.com/JGabrielGruber/neonroot/internal/remote"
	"github.com/JGabrielGruber/neonroot/internal/template"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

var (
	createVaultFlag    string
	createFromFlag     string
	createSeedFlag     string
	createImageFlag    string
	createMountFlag    string
	createTemplateFlag string
	createShellFlag    string
	createWithFlag     string
	createPortFlag     string
	createUpFlag       string
	createSecretsFlag  bool
	createSandboxFlag  bool
	createIsolatedFlag bool
)

// splitList parses a comma-separated flag into trimmed, non-empty items.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// shellCommand turns a user-supplied shell string into a container command
// (run via `sh -c`), or nil to use the default (tmux if present, else bash).
func shellCommand(s string) []string {
	if s == "" {
		return nil
	}
	return []string{"sh", "-c", s}
}

var createCmd = &cobra.Command{
	Use:   "create <workspace>",
	Short: "Create a new workspace in a vault",
	Long: `Creates a workspace as a bare git repo in the vault, seeded from the
shipped default template or, with --from, from an existing workspace's files.
Optionally binds it to a container image with --image (workspaces without an
image run host-only).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		target, err := app.resolveVault(createVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(target); err != nil {
			return err
		}
		g := &git.Git{Runner: app.Runner}
		if !g.Available() {
			return fmt.Errorf("git is required to create workspaces but was not found on PATH")
		}
		lock, err := app.lock("vault-" + target.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		spec := wsSpec{
			Image: createImageFlag, With: splitList(createWithFlag),
			Mount: createMountFlag, Shell: shellCommand(createShellFlag),
			Ports: splitList(createPortFlag), Up: shellCommand(createUpFlag),
			Secrets: createSecretsFlag, Isolation: isolationProfile(createSandboxFlag, createIsolatedFlag),
			Seed: createSeedFlag, From: createFromFlag, Template: createTemplateFlag,
		}
		entry, rev, err := createWorkspace(cmd.Context(), g, target, name, spec)
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("created workspace %q in vault %q (revision %d)", name, target.Name, rev)
		if img := entry.PrimaryImage(); img != "" {
			msg += fmt.Sprintf(", image %q", img)
		}
		app.UI.Success(msg)
		return nil
	},
}

// wsSpec describes a workspace to create — the fields create/spawn set.
type wsSpec struct {
	Image     string   // primary image ("" = host-only)
	With      []string // sidecar images
	Mount     string
	Shell     []string
	Ports     []string
	Up        []string
	Secrets   bool
	Isolation string
	// Content source: Seed (a host dir) XOR From (an existing workspace); neither
	// falls back to Template (default "default").
	Seed     string
	From     string
	Template string
}

// createWorkspace seeds a workspace's bare repo (local or remote) and registers
// it in the vault's catalog. The caller holds the vault lock and has checked
// availability. Returns the stored entry (with any inherited image) and the new
// catalog revision.
func createWorkspace(ctx context.Context, g *git.Git, target domain.Vault, name string, spec wsSpec) (domain.IndexWorkspace, int64, error) {
	var zero domain.IndexWorkspace
	if spec.Seed != "" && spec.From != "" {
		return zero, 0, fmt.Errorf("--from and --seed are mutually exclusive")
	}
	if spec.Isolation != "" && spec.Secrets {
		return zero, 0, fmt.Errorf("--secrets and --sandbox/--isolated are mutually exclusive (a sandbox must not carry your identity)")
	}

	cat := app.catalog()
	idx, err := cat.Read(ctx, target)
	if errors.Is(err, fs.ErrNotExist) {
		idx = vault.NewIndex()
	} else if err != nil {
		return zero, 0, err
	}
	if _, exists := vault.Workspace(idx, name); exists {
		return zero, 0, fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceExists, name, target.Name)
	}

	// Build the initial content in a throwaway tmpfs dir.
	content, err := os.MkdirTemp(app.Paths.Cache, "create-*")
	if err != nil {
		return zero, 0, err
	}
	defer os.RemoveAll(content)

	image := spec.Image
	switch {
	case spec.Seed != "":
		if err := copyHostDir(spec.Seed, content); err != nil {
			return zero, 0, fmt.Errorf("seeding from %q: %w", spec.Seed, err)
		}
	case spec.From != "":
		srcImage, err := seedFrom(ctx, g, spec.From, target.Name, content)
		if err != nil {
			return zero, 0, err
		}
		if image == "" {
			image = srcImage // inherit the source workspace's image
		}
	default:
		tpl := spec.Template
		if tpl == "" {
			tpl = "default"
		}
		if err := template.Write(tpl, app.Paths.TemplatesDir(), content, name); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return zero, 0, fmt.Errorf("no template %q — see 'neonroot template ls'", tpl)
			}
			return zero, 0, err
		}
	}

	// Seed the bare repo — on the drive, or over ssh for a remote vault.
	root := filepath.Join("workspaces", name+".git")
	if target.IsRemote() {
		addr, err := remote.Parse(target.Remote)
		if err != nil {
			return zero, 0, err
		}
		t := remote.Transport{Runner: app.Runner, Addr: addr}
		if err := t.InitBare(ctx, root); err != nil {
			return zero, 0, err
		}
		if err := g.SeedPush(ctx, addr.SSHURL(root), content); err != nil {
			return zero, 0, err
		}
	} else if err := g.SeedContent(ctx, filepath.Join(target.Path, root), content); err != nil {
		return zero, 0, err
	}

	entry := domain.IndexWorkspace{
		Name: name, Root: root, Mount: spec.Mount, Shell: spec.Shell,
		Ports: spec.Ports, Up: spec.Up, Secrets: spec.Secrets, Isolation: spec.Isolation,
	}
	if image != "" {
		entry.Images = append([]string{image}, spec.With...)
	} else if len(spec.With) > 0 {
		return zero, 0, fmt.Errorf("--with (sidecars) requires --image (the primary container)")
	}
	idx.Workspaces = append(idx.Workspaces, entry)
	vault.Bump(idx)
	if err := cat.Write(ctx, target, idx); err != nil {
		return zero, 0, err
	}
	return entry, idx.Revision, nil
}

// seedFrom fills content with an existing workspace's files (its git working
// tree, without history) and returns that workspace's image (if any). ref is
// "<vault>/<workspace>" or "<workspace>" (resolved against defaultVault).
func seedFrom(ctx context.Context, g *git.Git, ref, defaultVault, content string) (string, error) {
	vaultName, wsName := parseWorkspaceRef(ref, defaultVault)
	src, err := app.resolveVault(vaultName)
	if err != nil {
		return "", err
	}
	if err := app.requireAvailable(src); err != nil {
		return "", err
	}
	sidx, err := app.catalog().Read(ctx, src)
	if err != nil {
		return "", err
	}
	entry, ok := vault.Workspace(sidx, wsName)
	if !ok {
		return "", fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceNotFound, wsName, src.Name)
	}
	// Clone the source bare repo (local path or ssh URL) into content, then drop
	// its history so the new workspace starts fresh from the source's files.
	origin := filepath.Join(src.Path, entry.Root)
	if src.IsRemote() {
		addr, err := remote.Parse(src.Remote)
		if err != nil {
			return "", err
		}
		origin = addr.SSHURL(entry.Root)
	}
	if err := g.Clone(ctx, origin, content); err != nil {
		return "", err
	}
	return entry.PrimaryImage(), os.RemoveAll(filepath.Join(content, ".git"))
}

// copyHostDir copies src's files into dst, skipping .git (the seeded workspace
// starts with fresh history). Symlinks are dereferenced to regular files.
func copyHostDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil // skip sockets/devices; symlinks fall through via ReadFile below
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if fi, err := os.Stat(path); err == nil && fi.Mode()&0o111 != 0 {
			mode = 0o755 // preserve the executable bit
		}
		return os.WriteFile(target, data, mode)
	})
}

// parseWorkspaceRef splits "vault/workspace" or "workspace" into its parts.
func parseWorkspaceRef(ref, defaultVault string) (vaultName, wsName string) {
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return defaultVault, ref
}

func init() {
	createCmd.Flags().StringVar(&createVaultFlag, "vault", "", "target vault (default: configured default vault)")
	createCmd.Flags().StringVar(&createFromFlag, "from", "", "seed from an existing workspace (<vault>/<workspace> or <workspace>)")
	createCmd.Flags().StringVar(&createSeedFlag, "seed", "", "seed from an existing host directory (its files become the initial commit; .git is skipped)")
	createCmd.Flags().StringVar(&createImageFlag, "image", "", "vault image the workspace runs inside (default: host-only)")
	createCmd.Flags().StringVar(&createMountFlag, "mount", "", "where the workspace mounts inside the container (default: /workspace)")
	createCmd.Flags().StringVar(&createTemplateFlag, "template", "default", "starter template (see 'neonroot template ls')")
	createCmd.Flags().StringVar(&createShellFlag, "shell", "", "command to run on attach into the container (default: a login shell)")
	createCmd.Flags().StringVar(&createWithFlag, "with", "", "sidecar images to run alongside (comma-separated, e.g. postgres,redis)")
	createCmd.Flags().StringVar(&createPortFlag, "port", "", "ports to publish to the host (comma-separated, 'host:container' or 'port')")
	createCmd.Flags().StringVar(&createUpFlag, "up", "", "dev command 'neonroot up' runs in the container (e.g. 'npm run dev')")
	createCmd.Flags().BoolVar(&createSecretsFlag, "secrets", false, "inject identity on load (bananenv env + ssh agent + gitconfig; opt-in, ephemeral)")
	createCmd.Flags().BoolVar(&createSandboxFlag, "sandbox", false, "lock the container down for agent/untrusted use (drop caps, no-new-privs, limits; network on)")
	createCmd.Flags().BoolVar(&createIsolatedFlag, "isolated", false, "sandbox + no network (for untrusted code)")
	rootCmd.AddCommand(createCmd)
}
