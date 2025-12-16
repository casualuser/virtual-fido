package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "virtual-fido",
		Short: "Seed-based virtual FIDO toolchain",
	}

	root.AddCommand(genSeedCmd(), runCmd(), watchOnlyCmd(), vaultCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func genSeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen-seed",
		Short: "Generate a new random seed (32 bytes, hex)",
		RunE: func(cmd *cobra.Command, args []string) error {
			buf := make([]byte, 32)
			if _, err := rand.Read(buf); err != nil {
				return fmt.Errorf("generate seed: %w", err)
			}
			fmt.Println(hex.EncodeToString(buf))
			return nil
		},
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run virtual authenticator (seed-based)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("run: not implemented yet")
		},
	}
}

func watchOnlyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch-only",
		Short: "Run in watch-only mode (air-gapped flow)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("watch-only: not implemented yet")
		},
	}
}

func vaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vault",
		Short: "Process watch-only requests using seed and vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("vault: not implemented yet")
		},
	}
}
