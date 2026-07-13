package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
    Use:   "load",
    Short: "load a workspace",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("load command - placeholder for v0.0.2\n")
    },
}

func init() {
    rootCmd.AddCommand(loadCmd)
}
