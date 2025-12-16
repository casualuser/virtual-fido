package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/client"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/seed"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/state"
	"github.com/bulwarkid/virtual-fido/fido_client"
	"github.com/bulwarkid/virtual-fido/transport"
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
	var seedFile string
	var vaultPath string
	var transportFlag string
	var deviceName string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run virtual authenticator (seed-based)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if vaultPath == "" {
				return fmt.Errorf("--vault is required")
			}
			seedBytes, err := seed.Load(seedFile)
			if err != nil {
				return fmt.Errorf("load seed: %w", err)
			}
			store := state.NewStore(vaultPath, seedBytes)
			counters, err := state.NewCounterStore(store)
			if err != nil {
				return fmt.Errorf("load vault: %w", err)
			}
			approver := promptApprover{}
			cl := client.New(seedBytes, counters, approver)
			mode := transport.Mode(transportFlag)
			if mode == "" {
				mode = defaultTransport()
			}
			if deviceName == "" {
				deviceName = "Virtual FIDO"
			}
			transport.Start(mode, cl, deviceName)
			return nil
		},
	}
	cmd.Flags().StringVar(&seedFile, "seed-file", "", "path to hex-encoded seed (if empty, read from stdin)")
	cmd.Flags().StringVar(&vaultPath, "vault", "", "path to encrypted counter vault (required)")
	defaultTransport := "usbip"
	if runtime.GOOS == "linux" {
		defaultTransport = "uhid"
	}
	cmd.Flags().StringVar(&transportFlag, "transport", defaultTransport, "transport: uhid (Linux) or usbip (default usbip on non-Linux, uhid on Linux)")
	cmd.Flags().StringVar(&deviceName, "device-name", "Virtual FIDO", "UHID/USB device name")
	return cmd
}

type promptApprover struct{}

func (promptApprover) ApproveClientAction(action fido_client.ClientAction, params fido_client.ClientActionRequestParams) bool {
	switch action {
	case fido_client.ClientActionFIDOMakeCredential:
		return transport.Prompt(fmt.Sprintf("Approve registration for %q (Y/n)?", params.RelyingParty))
	case fido_client.ClientActionFIDOGetAssertion:
		return transport.Prompt(fmt.Sprintf("Approve login for %q user %q (Y/n)?", params.RelyingParty, params.UserName))
	case fido_client.ClientActionU2FRegister:
		return transport.Prompt("Approve U2F registration (Y/n)?")
	case fido_client.ClientActionU2FAuthenticate:
		return transport.Prompt("Approve U2F authentication (Y/n)?")
	default:
		return transport.Prompt(fmt.Sprintf("Approve action %d (Y/n)?", action))
	}
}

func defaultTransport() transport.Mode {
	if runtime.GOOS == "linux" {
		return transport.ModeUHID
	}
	return transport.ModeUSBIP
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
