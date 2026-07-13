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
	"github.com/JGabrielGruber/neonroot/internal/template"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

var (
	createVaultFlag    string
	createFromFlag     string
	createImageFlag    string
	createMountFlag    string
	createTemplateFlag string
)

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

		idx, err := vault.ReadIndex(target.Path)
		if errors.Is(err, fs.ErrNotExist) {
			idx = vault.NewIndex()
		} else if err != nil {
			return err
		}
		if _, exists := vault.Workspace(idx, name); exists {
			return fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceExists, name, target.Name)
		}

		// Build the initial content in a throwaway tmpfs dir.
		content, err := os.MkdirTemp(app.Paths.Cache, "create-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(content)

		image := createImageFlag
		if createFromFlag != "" {
			srcImage, err := seedFrom(cmd.Context(), g, createFromFlag, target.Name, content)
			if err != nil {
				return err
			}
			if image == "" {
				image = srcImage // inherit the source workspace's image
			}
		} else if err := template.Write(createTemplateFlag, app.Paths.TemplatesDir(), content, name); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("no template %q — see 'neonroot template ls'", createTemplateFlag)
			}
			return err
		}

		root := filepath.Join("workspaces", name+".git")
		bare := filepath.Join(target.Path, root)
		if err := g.SeedContent(cmd.Context(), bare, content); err != nil {
			return err
		}

		entry := domain.IndexWorkspace{Name: name, Root: root, Mount: createMountFlag}
		if image != "" {
			entry.Images = []string{image}
		}
		idx.Workspaces = append(idx.Workspaces, entry)
		vault.Bump(idx)
		if err := vault.WriteIndex(target.Path, idx); err != nil {
			return err
		}

		msg := fmt.Sprintf("created workspace %q in vault %q (revision %d)", name, target.Name, idx.Revision)
		if image != "" {
			msg += fmt.Sprintf(", image %q", image)
		}
		app.UI.Success(msg)
		return nil
	},
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
	sidx, err := vault.ReadIndex(src.Path)
	if err != nil {
		return "", err
	}
	entry, ok := vault.Workspace(sidx, wsName)
	if !ok {
		return "", fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceNotFound, wsName, src.Name)
	}
	// Clone the source bare repo into content, then drop its history so the new
	// workspace starts fresh from the source's current files.
	if err := g.Clone(ctx, filepath.Join(src.Path, entry.Root), content); err != nil {
		return "", err
	}
	return entry.PrimaryImage(), os.RemoveAll(filepath.Join(content, ".git"))
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
	createCmd.Flags().StringVar(&createImageFlag, "image", "", "vault image the workspace runs inside (default: host-only)")
	createCmd.Flags().StringVar(&createMountFlag, "mount", "", "where the workspace mounts inside the container (default: /workspace)")
	createCmd.Flags().StringVar(&createTemplateFlag, "template", "default", "starter template (see 'neonroot template ls')")
	rootCmd.AddCommand(createCmd)
}
