package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/template"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage workspace starter templates",
	Long: `Templates seed new workspaces (create --template <name>). They come from
the shipped defaults plus your own under $XDG_CONFIG_HOME/neonroot/templates/.
This is where dev-environment ergonomics live — editor configs, a .tmux.conf,
scaffolding — so you (or the community) can build and share rich setups.`,
}

var templateLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List available templates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-16s %s\n", "TEMPLATE", "SOURCE")
		for _, t := range template.List(app.Paths.TemplatesDir()) {
			fmt.Fprintf(out, "%-16s %s\n", t.Name, t.Source)
		}
		return nil
	},
}

var templateNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold a new user template to customize",
	Long: `Creates a new template directory under your config (seeded from the default
skeleton) that you can fill with editor configs, dotfiles, a .tmux.conf, etc.
Use {{workspace}} anywhere in file contents; it is replaced with the workspace
name on create.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		path, err := template.Scaffold(app.Paths.TemplatesDir(), name)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("template %q already exists", name)
			}
			return err
		}
		app.UI.Success(fmt.Sprintf("created template %q", name))
		app.UI.Info(fmt.Sprintf("edit it at %s, then 'neonroot create <ws> --template %s'", path, name))
		return nil
	},
}

var templatePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the user templates directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), app.Paths.TemplatesDir())
		return nil
	},
}

func init() {
	templateCmd.AddCommand(templateLsCmd, templateNewCmd, templatePathCmd)
	rootCmd.AddCommand(templateCmd)
}
