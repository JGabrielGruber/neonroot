package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "neonroot",
    Short: "NeonRoot — Portable Workspace Manager",
    Long: `NeonRoot is a lightweight CLI for managing ephemeral development workspaces.
It hydrates pods from external cold storage into /tmp and manages tmux + Podman sessions.`,
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("NeonRoot v0.0.2")
        fmt.Println("Use 'neonroot --help' for available commands.")
    },
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    rootCmd.Version = "0.0.2"
    rootCmd.SetVersionTemplate(`NeonRoot {{.Version}}
`)
}
