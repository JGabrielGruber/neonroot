package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "list a workspace",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("list command - placeholder for v0.0.2\n")
    },
}

func init() {
    rootCmd.AddCommand(listCmd)
}
