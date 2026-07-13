package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/hydration"
	"github.com/JGabrielGruber/neonroot/internal/template"
	"github.com/JGabrielGruber/neonroot/internal/vault"
)

var (
	createVaultFlag string
	createFromFlag  string
	createImageFlag string
)

var createCmd = &cobra.Command{
	Use:   "create <workspace>",
	Short: "Create a new workspace in a vault",
	Long: `Creates a workspace, seeded from the shipped default template or, with
--from, by copying an existing workspace's files. Optionally binds it to a
container image with --image (workspaces without an image run host-only).`,
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

		root := filepath.Join("workspaces", name)
		dstDir := filepath.Join(target.Path, root)
		image := createImageFlag

		if createFromFlag != "" {
			srcImage, err := seedFrom(createFromFlag, target.Name, name, dstDir)
			if err != nil {
				return err
			}
			if image == "" {
				image = srcImage // inherit the source workspace's image
			}
		} else {
			if err := template.WriteDefault(dstDir, name); err != nil {
				return err
			}
		}

		idx.Workspaces = append(idx.Workspaces, domain.IndexWorkspace{Name: name, Root: root, Image: image})
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

// seedFrom copies an existing workspace's files into dstDir and returns that
// workspace's image (if any). ref is "<vault>/<workspace>" or "<workspace>"
// (resolved against defaultVault).
func seedFrom(ref, defaultVault, name, dstDir string) (string, error) {
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
	srcDir := filepath.Join(src.Path, entry.Root)
	// Reuse hydration's copy (progress + free-space pre-flight on the drive).
	if _, err := hydration.Hydrate(name, srcDir, dstDir, app.UI); err != nil {
		return "", err
	}
	return entry.Image, nil
}

// parseWorkspaceRef splits "vault/workspace" or "workspace" into its parts.
func parseWorkspaceRef(ref, defaultVault string) (vaultName, wsName string) {
	if i := strings.IndexByte(ref, '/'); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return defaultVault, ref
}

func init() {
	createCmd.Flags().StringVarP(&createVaultFlag, "vault", "", "", "target vault (default: configured default vault)")
	createCmd.Flags().StringVar(&createFromFlag, "from", "", "seed from an existing workspace (<vault>/<workspace> or <workspace>)")
	createCmd.Flags().StringVar(&createImageFlag, "image", "", "container image the workspace runs inside (default: host-only)")
	rootCmd.AddCommand(createCmd)
}
