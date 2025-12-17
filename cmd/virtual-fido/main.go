package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"

	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/airgap"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/client"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/seed"
	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/state"
	"github.com/bulwarkid/virtual-fido/cose"
	"github.com/bulwarkid/virtual-fido/ctap"
	"github.com/bulwarkid/virtual-fido/ctap_hid"
	"github.com/bulwarkid/virtual-fido/fido_client"
	"github.com/bulwarkid/virtual-fido/transport"
	"github.com/bulwarkid/virtual-fido/u2f"
	"github.com/bulwarkid/virtual-fido/uhid"
	"github.com/bulwarkid/virtual-fido/util"
	"github.com/bulwarkid/virtual-fido/webauthn"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "virtual-fido",
		Short: "Seed-based virtual FIDO toolchain",
	}

	root.AddCommand(genSeedCmd(), runCmd(), onlineOnlyCmd(), offlineOnlyCmd())

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
	rp := params.RelyingParty
	if rp == "" {
		rp = "<unknown-rp>"
	}
	user := params.UserName
	if user == "" {
		user = "<unknown-user>"
	}
	switch action {
	case fido_client.ClientActionFIDOMakeCredential:
		ok := transport.Prompt(fmt.Sprintf("Approve registration for %q (Y/n)?", rp))
		if ok {
			fmt.Printf("Approved registration for %q\n", rp)
		} else {
			fmt.Printf("Denied registration for %q\n", rp)
		}
		return ok
	case fido_client.ClientActionFIDOGetAssertion:
		ok := transport.Prompt(fmt.Sprintf("Approve login for %q user %q (Y/n)?", rp, user))
		if ok {
			fmt.Printf("Approved login for %q user %q\n", rp, user)
		} else {
			fmt.Printf("Denied login for %q user %q\n", rp, user)
		}
		return ok
	case fido_client.ClientActionU2FRegister:
		ok := transport.Prompt("Approve U2F registration (Y/n)?")
		if ok {
			fmt.Println("Approved U2F registration")
		} else {
			fmt.Println("Denied U2F registration")
		}
		return ok
	case fido_client.ClientActionU2FAuthenticate:
		ok := transport.Prompt("Approve U2F authentication (Y/n)?")
		if ok {
			fmt.Println("Approved U2F authentication")
		} else {
			fmt.Println("Denied U2F authentication")
		}
		return ok
	default:
		ok := transport.Prompt(fmt.Sprintf("Approve action %d (Y/n)?", action))
		if ok {
			fmt.Printf("Approved action %d\n", action)
		} else {
			fmt.Printf("Denied action %d\n", action)
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

func onlineOnlyCmd() *cobra.Command {
	var transportFlag string
	var deviceName string
	cmd := &cobra.Command{
		Use:   "online-only",
		Short: "Run online relay mode (air-gapped flow)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := transport.Mode(transportFlag)
			if mode == "" {
				mode = defaultTransport()
			}
			if mode != transport.ModeUHID {
				return fmt.Errorf("online-only currently supports --transport uhid")
			}
			if deviceName == "" {
				deviceName = "Virtual FIDO (relay)"
			}
			return runOnlineUHID(deviceName)
		},
	}
	defaultTransport := "uhid"
	cmd.Flags().StringVar(&transportFlag, "transport", defaultTransport, "transport (only uhid is supported for online-only)")
	cmd.Flags().StringVar(&deviceName, "device-name", "Virtual FIDO (relay)", "UHID device name")
	return cmd
}

func offlineOnlyCmd() *cobra.Command {
	var seedFile string
	var vaultPath string
	cmd := &cobra.Command{
		Use:   "offline-only",
		Short: "Process online-only requests using seed and vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			if vaultPath == "" {
				return fmt.Errorf("--vault is required")
			}
			return runOfflineVault(seedFile, vaultPath)
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

func runOnlineUHID(name string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		cancel()
	}()

	dev, err := uhid.Open(name, uhid.ReportDescriptorFIDO)
	if err != nil {
		return err
	}
	defer dev.Close()
	dev.WaitReady()

	fmt.Printf("Online-only UHID relay started as %q. Copy request hex to offline-only and paste responses back.\n", name)
	reader := bufio.NewReader(os.Stdin)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		report, err := dev.ReadReport(ctx)
		if err != nil {
			return err
		}
		batch := airgap.PacketBatch{Reports: [][]byte{report}}
		hexReq, _ := airgap.EncodePackets(batch)
		fmt.Println("Request hex:")
		fmt.Println(hexReq)
		fmt.Println("Paste response hex from offline-only:")
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			fmt.Println("empty response, skipping")
			continue
		}
		respBatch, err := airgap.DecodePackets(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decode response: %v\n", err)
			continue
		}
		for _, resp := range respBatch.Reports {
			if err := dev.WriteReport(ctx, resp); err != nil {
				return err
			}
		}
	}
}

func runOfflineVault(seedFile, vaultPath string) error {
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

	ctapServer := ctap.NewCTAPServer(cl)
	u2fServer := u2f.NewU2FServer(cl)
	hidServer := ctap_hid.NewCTAPHIDServer(ctapServer, u2fServer)

	var responses [][]byte
	hidServer.SetResponseHandler(func(resp []byte) {
		responses = append(responses, resp)
	})

	fmt.Println("Paste request hex batches from online-only; Ctrl+D to exit.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		batch, err := airgap.DecodePackets(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decode request: %v\n", err)
			continue
		}
		responses = responses[:0]
		for _, rpt := range batch.Reports {
			hidServer.HandleMessage(rpt)
		}
		outHex, err := airgap.EncodePackets(airgap.PacketBatch{Reports: responses})
		if err != nil {
			fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
			continue
		}
		fmt.Println(outHex)
	}
	return scanner.Err()
}
