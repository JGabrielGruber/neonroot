package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "status a workspace",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("status command - placeholder for v0.0.2\n")
    },
}

func init() {
    rootCmd.AddCommand(statusCmd)
}
