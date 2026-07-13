package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
    Use:   "commit",
    Short: "commit a workspace",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("commit command - placeholder for v0.0.2\n")
    },
}

func init() {
    rootCmd.AddCommand(commitCmd)
}
