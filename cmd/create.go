package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
    Use:   "create",
    Short: "create a workspace",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("create command - placeholder for v0.0.2\n")
    },
}

func init() {
    rootCmd.AddCommand(createCmd)
}
