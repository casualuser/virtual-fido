package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/airgap"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/client"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/seed"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/state"
	"github.com/bulwarkid/virtual-fido/cose"
	"github.com/bulwarkid/virtual-fido/ctap"
	"github.com/bulwarkid/virtual-fido/fido_client"
	"github.com/bulwarkid/virtual-fido/transport"
	"github.com/bulwarkid/virtual-fido/util"
	"github.com/bulwarkid/virtual-fido/webauthn"
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
		ok := transport.Prompt(fmt.Sprintf("Approve registration for %q (Y/n)?", params.RelyingParty))
		if ok {
			fmt.Printf("Approved registration for %q\n", params.RelyingParty)
		}
		return ok
	case fido_client.ClientActionFIDOGetAssertion:
		ok := transport.Prompt(fmt.Sprintf("Approve login for %q user %q (Y/n)?", params.RelyingParty, params.UserName))
		if ok {
			fmt.Printf("Approved login for %q user %q\n", params.RelyingParty, params.UserName)
		}
		return ok
	case fido_client.ClientActionU2FRegister:
		ok := transport.Prompt("Approve U2F registration (Y/n)?")
		if ok {
			fmt.Println("Approved U2F registration")
		}
		return ok
	case fido_client.ClientActionU2FAuthenticate:
		ok := transport.Prompt("Approve U2F authentication (Y/n)?")
		if ok {
			fmt.Println("Approved U2F authentication")
		}
		return ok
	default:
		ok := transport.Prompt(fmt.Sprintf("Approve action %d (Y/n)?", action))
		if ok {
			fmt.Printf("Approved action %d\n", action)
		}
		return ok
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
			fmt.Println("Paste request hex from authenticator (or stdin):")
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			req, err := airgap.DecodeRequest(line)
			if err != nil {
				return fmt.Errorf("decode request: %w", err)
			}
			if err := req.Validate(); err != nil {
				return fmt.Errorf("invalid request: %w", err)
			}
			fmt.Printf("RP: %s op=%d allowList=%d\n", req.RPID, req.Op, len(req.AllowList))
			hexReq, _ := airgap.EncodeRequest(req)
			fmt.Println("Send this hex to vault:")
			fmt.Println(hexReq)
			fmt.Println("Paste vault response hex:")
			respHex, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			resp, err := airgap.DecodeResponse(respHex)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			outHex, _ := airgap.EncodeResponse(resp)
			fmt.Println("Return this response to authenticator:")
			fmt.Println(outHex)
			return nil
		},
	}
}

func vaultCmd() *cobra.Command {
	var seedFile string
	var vaultPath string
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Process watch-only requests using seed and vault",
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
			fmt.Println("Paste request hex from watch-only:")
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			req, err := airgap.DecodeRequest(line)
			if err != nil {
				return fmt.Errorf("decode request: %w", err)
			}
			if err := req.Validate(); err != nil {
				return fmt.Errorf("invalid request: %w", err)
			}
			fmt.Printf("RP: %s op=%d allowList=%d\n", req.RPID, req.Op, len(req.AllowList))
			if !airgap.PromptYesNo("Approve? (Y/n)") {
				return fmt.Errorf("denied")
			}
			approver := promptApprover{}
			cl := client.New(seedBytes, counters, approver)
			resp, err := buildResponse(req, cl)
			if err != nil {
				return err
			}
			hexResp, _ := airgap.EncodeResponse(resp)
			fmt.Println("Response hex:")
			fmt.Println(hexResp)
			return nil
		},
	}
	cmd.Flags().StringVar(&seedFile, "seed-file", "", "path to hex seed (if empty, stdin)")
	cmd.Flags().StringVar(&vaultPath, "vault", "", "path to encrypted counter vault (required)")
	return cmd
}

func buildResponse(req *airgap.Request, cl *client.SeedClient) (*airgap.Response, error) {
	switch req.Op {
	case airgap.OpMakeCredential:
		params := []webauthn.PublicKeyCredentialParams{{Type: "public-key", Algorithm: cose.COSE_ALGORITHM_ID_ES256}}
		rp := &webauthn.PublicKeyCredentialRPEntity{ID: req.RPID, Name: req.RPID}
		user := &webauthn.PublicKeyCrendentialUserEntity{ID: req.UserHandle, Name: "user", DisplayName: "user"}
		cs := cl.NewCredentialSource(params, nil, rp, user)
		if cs == nil {
			return nil, fmt.Errorf("failed to create credential")
		}
		flags := byte(0x05) // UP + UV
		resp := ctap.BuildMakeCredentialResponse(cs, req.ClientDataHash, flags)
		return &airgap.Response{
			Op:                req.Op,
			CredentialID:      cs.ID,
			AuthenticatorData: resp.AuthData,
			AttestationObject: util.MarshalCBOR(resp),
		}, nil
	case airgap.OpGetAssertion:
		if len(req.AllowList) == 0 {
			return nil, fmt.Errorf("allowList required")
		}
		allow := []webauthn.PublicKeyCredentialDescriptor{{Type: "public-key", ID: req.AllowList[0]}}
		cs := cl.GetAssertionSource(req.RPID, allow)
		if cs == nil {
			return nil, fmt.Errorf("no credential")
		}
		flags := byte(0x05) // UP + UV
		ar := ctap.BuildGetAssertionResponse(cs, req.RPID, req.ClientDataHash, flags)
		return &airgap.Response{
			Op:                req.Op,
			CredentialID:      cs.ID,
			AuthenticatorData: ar.AuthenticatorData,
			Signature:         ar.Signature,
			UserHandle:        req.UserHandle,
			SignCount:         uint32(cs.SignatureCounter),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported op %d", req.Op)
	}
}
