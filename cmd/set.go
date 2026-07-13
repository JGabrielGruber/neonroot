package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/domain"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var (
	setVaultFlag       string
	setRenameFlag      string
	setImageFlag       string
	setAddImageFlag    string
	setRemoveImageFlag string
	setMountFlag       string
	setShellFlag       string
	setNoImageFlag     bool
)

var setCmd = &cobra.Command{
	Use:   "set <workspace>",
	Short: "Edit a workspace's attributes (image, mount, name)",
	Long: `Edits a workspace in its vault: rename it, change or add/remove its
image(s), or set its mount point. Changes take effect on the next load; rename
requires the workspace to be stopped first.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		f := cmd.Flags()

		v, err := app.resolveVault(setVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		lock, err := app.lock("vault-" + v.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		idx, err := vault.ReadIndex(v.Path)
		if err != nil {
			return err
		}
		pos := -1
		for i := range idx.Workspaces {
			if idx.Workspaces[i].Name == name {
				pos = i
				break
			}
		}
		if pos < 0 {
			return fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceNotFound, name, v.Name)
		}
		entry := idx.Workspaces[pos]

		// Rename: move the bare repo and re-key the entry. Requires it stopped.
		if f.Changed("rename") {
			if workspace.IsLoaded(app.Paths, name) {
				return fmt.Errorf("workspace %q is loaded — run 'neonroot stop %s' before renaming", name, name)
			}
			if _, exists := vault.Workspace(idx, setRenameFlag); exists {
				return fmt.Errorf("%w: %q in vault %q", domain.ErrWorkspaceExists, setRenameFlag, v.Name)
			}
			newRoot := filepath.Join("workspaces", setRenameFlag+".git")
			if err := os.Rename(filepath.Join(v.Path, entry.Root), filepath.Join(v.Path, newRoot)); err != nil {
				return err
			}
			entry.Name = setRenameFlag
			entry.Root = newRoot
		}

		// Image / mount edits.
		switch {
		case setNoImageFlag:
			entry.Images = nil
		case f.Changed("image"):
			entry.Images = []string{setImageFlag}
		}
		if f.Changed("add-image") {
			entry.Images = append(entry.Images, setAddImageFlag)
		}
		if f.Changed("remove-image") {
			kept := entry.Images[:0:0]
			for _, img := range entry.Images {
				if img != setRemoveImageFlag {
					kept = append(kept, img)
				}
			}
			entry.Images = kept
		}
		if f.Changed("mount") {
			entry.Mount = setMountFlag
		}
		if f.Changed("shell") {
			entry.Shell = shellCommand(setShellFlag)
		}

		idx.Workspaces[pos] = entry
		vault.Bump(idx)
		if err := vault.WriteIndex(v.Path, idx); err != nil {
			return err
		}

		app.UI.Success(fmt.Sprintf("updated workspace %q in vault %q (revision %d)", entry.Name, v.Name, idx.Revision))
		if workspace.IsLoaded(app.Paths, entry.Name) {
			app.UI.Info("reload the workspace for image/mount changes to take effect")
		}
		return nil
	},
}

func init() {
	f := setCmd.Flags()
	f.StringVar(&setVaultFlag, "vault", "", "vault holding the workspace (default: configured default vault)")
	f.StringVar(&setRenameFlag, "rename", "", "rename the workspace")
	f.StringVar(&setImageFlag, "image", "", "set the image (replaces the image list)")
	f.StringVar(&setAddImageFlag, "add-image", "", "add a sidecar image")
	f.StringVar(&setRemoveImageFlag, "remove-image", "", "remove an image from the list")
	f.StringVar(&setMountFlag, "mount", "", "set the container mount target")
	f.StringVar(&setShellFlag, "shell", "", "command to run on attach (empty resets to default: a login shell)")
	f.BoolVar(&setNoImageFlag, "no-image", false, "make the workspace host-only (clear its images)")
	rootCmd.AddCommand(setCmd)
}
