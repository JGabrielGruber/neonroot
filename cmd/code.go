package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/JGabrielGruber/neonroot/internal/workspace"
)

var codeCmd = &cobra.Command{
	Use:   "code <workspace>",
	Short: "Open a loaded workspace in your editor",
	Long: `Opens a loaded workspace's directory in $VISUAL, then $EDITOR, else 'code'.
The workspace is a normal host directory, so your editor runs on the host with
full speed while the container (if any) provides the toolchain.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := workspace.ReadState(app.Paths, args[0])
		if err != nil {
			return err
		}
		editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "code")
		bin, err := exec.LookPath(editor)
		if err != nil {
			return fmt.Errorf("editor %q not found on PATH: %w", editor, err)
		}
		// Hand over to the editor by replacing this process.
		return syscall.Exec(bin, []string{editor, ws.Root}, os.Environ())
	},
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(codeCmd)
}
