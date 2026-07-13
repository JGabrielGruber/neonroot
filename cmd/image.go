package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/runtime"
	"github.com/JGabrielGruber/neonroot/internal/template"
	"github.com/JGabrielGruber/neonroot/internal/vault"
	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var imageVaultFlag string

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Manage container images stored in a vault",
	Long: `Images live in a vault as a Containerfile (the definition) plus image.tar
(the built data). Building is an online step; loading a workspace then runs the
image straight from the vault's data with no network.`,
}

var imageCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Scaffold a new image definition (Containerfile) in a vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		v, err := app.resolveVault(imageVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		path := vault.ContainerfilePath(v.Path, name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("image %q already exists in vault %q", name, v.Name)
		}
		if err := template.WriteImageContainerfile(path, name); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("created image %q in vault %q", name, v.Name))
		app.UI.Info(fmt.Sprintf("edit %s, then 'neonroot image build %s'", path, name))
		return nil
	},
}

var imageBuildCmd = &cobra.Command{
	Use:   "build <name>",
	Short: "Build an image from its Containerfile and save its data into the vault",
	Long: `Builds the image (online — pulls its base and runs its build steps) and
saves the result as image.tar in the vault, so a later load can run it offline.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		v, err := app.resolveVault(imageVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		cfile := vault.ContainerfilePath(v.Path, name)
		if _, err := os.Stat(cfile); err != nil {
			return fmt.Errorf("no Containerfile for image %q — run 'neonroot image create %s' first", name, name)
		}

		pod, err := app.podman()
		if err != nil {
			return err
		}
		if !pod.Available() {
			return fmt.Errorf("podman is required to build images but was not found on PATH")
		}

		lock, err := app.lock("vault-" + v.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		ref := vault.ImageRef(name)
		app.UI.Step(fmt.Sprintf("building image %q", name))
		if err := pod.Build(cmd.Context(), ref, vault.ImageDir(v.Path, name)); err != nil {
			return err
		}
		app.UI.Step("saving image data into the vault")
		if err := pod.Save(cmd.Context(), ref, vault.ImageTarPath(v.Path, name)); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("built and stored image %q in vault %q", name, v.Name))
		return nil
	},
}

var imageLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List images stored in a vault",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := app.resolveVault(imageVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Join(v.Path, "images"))
		if err != nil {
			if os.IsNotExist(err) {
				app.UI.Info("no images in this vault")
				return nil
			}
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-16s %-8s %s\n", "IMAGE", "BUILT", "SIZE")
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			built, size := "no", "-"
			if info, err := os.Stat(vault.ImageTarPath(v.Path, e.Name())); err == nil {
				built, size = "yes", humanSize(info.Size())
			}
			fmt.Fprintf(out, "%-16s %-8s %s\n", e.Name(), built, size)
		}
		return nil
	},
}

var imageRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove an image (definition + data) from a vault",
	Long: `Deletes the image's Containerfile and image.tar from the vault, and its
loaded copy from the tmpfs store. This is always explicit — stopping a workspace
never removes a shared image.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		v, err := app.resolveVault(imageVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		dir := vault.ImageDir(v.Path, name)
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("no image %q in vault %q", name, v.Name)
		}

		// Warn about workspaces that still reference it.
		if idx, err := vault.ReadIndex(v.Path); err == nil {
			for _, w := range idx.Workspaces {
				for _, img := range w.Images {
					if img == name {
						app.UI.Warn(fmt.Sprintf("workspace %q still references image %q", w.Name, name))
					}
				}
			}
		}

		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		// Best-effort removal from the tmpfs store.
		if pod, err := app.podman(); err == nil && pod.Available() {
			_, _ = app.Runner.Run(cmd.Context(), "podman",
				append(podBaseArgs(pod), "rmi", "-f", vault.ImageRef(name))...)
		}
		app.UI.Success(fmt.Sprintf("removed image %q from vault %q", name, v.Name))
		return nil
	},
}

var imageSnapshotCmd = &cobra.Command{
	Use:   "snapshot <workspace>",
	Short: "Capture a loaded workspace's running container as its image data",
	Long: `Commits the workspace's running container (capturing changes you made
inside it, e.g. installed packages) and saves it back as the image's data in the
vault. This is how inside-container changes become durable and reproducible.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsName := args[0]
		ws, err := workspace.ReadState(app.Paths, wsName)
		if err != nil {
			return err
		}
		if ws.ContainerID == "" || len(ws.Images) == 0 {
			return fmt.Errorf("workspace %q has no running container to snapshot", wsName)
		}
		v, err := app.resolveVault(ws.SourceVault)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		pod, err := app.podman()
		if err != nil {
			return err
		}
		if !pod.Available() {
			return fmt.Errorf("podman is required but was not found on PATH")
		}

		image := ws.Images[0] // primary
		ref := vault.ImageRef(image)
		app.UI.Step(fmt.Sprintf("committing container of %q", wsName))
		if err := pod.Commit(cmd.Context(), ws.ContainerID, ref); err != nil {
			return err
		}
		app.UI.Step(fmt.Sprintf("saving image %q into the vault", image))
		if err := pod.Save(cmd.Context(), ref, vault.ImageTarPath(v.Path, image)); err != nil {
			return err
		}
		app.UI.Success(fmt.Sprintf("snapshotted %q into image %q in vault %q", wsName, image, v.Name))
		return nil
	},
}

var imageRenameFlag string

var imageSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Edit an image (rename)",
	Long: `Renames an image: moves its definition + data in the vault, re-tags the
stored image data, and updates every workspace that references it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if !cmd.Flags().Changed("rename") {
			return fmt.Errorf("nothing to set — use --rename <newname>")
		}
		newName := imageRenameFlag

		v, err := app.resolveVault(imageVaultFlag)
		if err != nil {
			return err
		}
		if err := app.requireAvailable(v); err != nil {
			return err
		}
		if _, err := os.Stat(vault.ImageDir(v.Path, name)); err != nil {
			return fmt.Errorf("no image %q in vault %q", name, v.Name)
		}
		if _, err := os.Stat(vault.ImageDir(v.Path, newName)); err == nil {
			return fmt.Errorf("image %q already exists in vault %q", newName, v.Name)
		}

		lock, err := app.lock("vault-" + v.Name)
		if err != nil {
			return err
		}
		defer lock.Unlock()

		// Move the definition + data directory.
		if err := os.Rename(vault.ImageDir(v.Path, name), vault.ImageDir(v.Path, newName)); err != nil {
			return err
		}

		// Re-tag the stored image data so its internal ref matches the new name.
		if _, err := os.Stat(vault.ImageTarPath(v.Path, newName)); err == nil {
			pod, perr := app.podman()
			if perr == nil && pod.Available() {
				oldRef, newRef := vault.ImageRef(name), vault.ImageRef(newName)
				tar := vault.ImageTarPath(v.Path, newName)
				if err := pod.LoadImage(cmd.Context(), tar); err == nil {
					_ = pod.Tag(cmd.Context(), oldRef, newRef)
					_ = pod.Save(cmd.Context(), newRef, tar)
				}
			} else {
				app.UI.Warn(fmt.Sprintf("rebuild to refresh its data: 'neonroot image build %s'", newName))
			}
		}

		// Update workspaces that reference the old image name.
		if idx, err := vault.ReadIndex(v.Path); err == nil {
			changed := false
			for i := range idx.Workspaces {
				for j, img := range idx.Workspaces[i].Images {
					if img == name {
						idx.Workspaces[i].Images[j] = newName
						changed = true
					}
				}
			}
			if changed {
				vault.Bump(idx)
				if err := vault.WriteIndex(v.Path, idx); err != nil {
					return err
				}
			}
		}

		app.UI.Success(fmt.Sprintf("renamed image %q → %q in vault %q", name, newName, v.Name))
		return nil
	},
}

// podBaseArgs exposes the storage-pinning args for ad-hoc podman calls.
func podBaseArgs(p *runtime.Podman) []string {
	return []string{"--root", p.GraphRoot, "--runroot", p.RunRoot}
}

func init() {
	for _, c := range []*cobra.Command{imageCreateCmd, imageBuildCmd, imageLsCmd, imageRmCmd, imageSetCmd} {
		c.Flags().StringVar(&imageVaultFlag, "vault", "", "vault holding the image (default: configured default vault)")
	}
	imageSetCmd.Flags().StringVar(&imageRenameFlag, "rename", "", "rename the image")
	imageCmd.AddCommand(imageCreateCmd, imageBuildCmd, imageLsCmd, imageRmCmd, imageSnapshotCmd, imageSetCmd)
	rootCmd.AddCommand(imageCmd)
}
