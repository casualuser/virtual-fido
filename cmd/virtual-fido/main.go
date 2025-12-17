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
	var countersPath string
	var transportFlag string
	var deviceName string
	var alwaysApprove bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run virtual authenticator (seed-based)",
		RunE: func(cmd *cobra.Command, args []string) error {
			seedBytes, err := seed.Load(seedFile)
			if err != nil {
				return fmt.Errorf("load seed: %w", err)
			}
			var counters client.CounterState
			if countersPath == "" {
				fmt.Println("Using in-memory time-based counters")
				counters = state.NewTimeBasedCounterStore(0, nil)
			} else {
				store := state.NewStore(countersPath, seedBytes)
				cs, err := state.NewCounterStore(store)
				if err != nil {
					return fmt.Errorf("load counters: %w", err)
				}
				fmt.Println("Loaded counters:")
				fmt.Print(cs.String())
				counters = cs
			}
			approver := promptApprover{alwaysApprove: alwaysApprove}
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
	cmd.Flags().StringVar(&countersPath, "counters", "", "path to encrypted counter store (optional; defaults to time-based)")
	defaultTransport := "usbip"
	if runtime.GOOS == "linux" {
		defaultTransport = "uhid"
	}
	cmd.Flags().StringVar(&transportFlag, "transport", defaultTransport, "transport: uhid (Linux) or usbip (default usbip on non-Linux, uhid on Linux)")
	cmd.Flags().StringVar(&deviceName, "device-name", "Virtual FIDO", "UHID/USB device name")
	cmd.Flags().BoolVar(&alwaysApprove, "auto-approve", false, "auto-approve all prompts without asking")
	return cmd
}

type promptApprover struct {
	alwaysApprove bool
}

func (p promptApprover) ApproveClientAction(action fido_client.ClientAction, params fido_client.ClientActionRequestParams) bool {
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
		ok := p.alwaysApprove || transport.Prompt(fmt.Sprintf("Approve registration for %q (Y/n)?", rp))
		if ok {
			if p.alwaysApprove {
				fmt.Printf("Auto-approved registration for %q\n", rp)
			} else {
				fmt.Printf("Approved registration for %q\n", rp)
			}
		} else {
			fmt.Printf("Denied registration for %q\n", rp)
		}
		return ok
	case fido_client.ClientActionFIDOGetAssertion:
		ok := p.alwaysApprove || transport.Prompt(fmt.Sprintf("Approve login for %q user %q (Y/n)?", rp, user))
		if ok {
			if p.alwaysApprove {
				fmt.Printf("Auto-approved login for %q user %q\n", rp, user)
			} else {
				fmt.Printf("Approved login for %q user %q\n", rp, user)
			}
		} else {
			fmt.Printf("Denied login for %q user %q\n", rp, user)
		}
		return ok
	case fido_client.ClientActionU2FRegister:
		ok := p.alwaysApprove || transport.Prompt("Approve U2F registration (Y/n)?")
		if ok {
			if p.alwaysApprove {
				fmt.Println("Auto-approved U2F registration")
			} else {
				fmt.Println("Approved U2F registration")
			}
		} else {
			fmt.Println("Denied U2F registration")
		}
		return ok
	case fido_client.ClientActionU2FAuthenticate:
		ok := p.alwaysApprove || transport.Prompt("Approve U2F authentication (Y/n)?")
		if ok {
			if p.alwaysApprove {
				fmt.Println("Auto-approved U2F authentication")
			} else {
				fmt.Println("Approved U2F authentication")
			}
		} else {
			fmt.Println("Denied U2F authentication")
		}
		return ok
	default:
		ok := p.alwaysApprove || transport.Prompt(fmt.Sprintf("Approve action %d (Y/n)?", action))
		if ok {
			if p.alwaysApprove {
				fmt.Printf("Auto-approved action %d\n", action)
			} else {
				fmt.Printf("Approved action %d\n", action)
			}
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
	var countersPath string
	cmd := &cobra.Command{
		Use:   "offline-only",
		Short: "Process online-only requests using seed and vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOfflineVault(seedFile, countersPath)
		},
	}
	cmd.Flags().StringVar(&seedFile, "seed-file", "", "path to hex seed (if empty, stdin)")
	cmd.Flags().StringVar(&countersPath, "counters", "", "path to encrypted counter store (optional; defaults to time-based)")
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

type offlineProxy struct {
	kind   airgap.HIDKind
	reader *bufio.Reader
}

func (p *offlineProxy) HandleMessage(data []byte) []byte {
	if p.kind == airgap.HIDKindCTAP && len(data) > 0 && data[0] == 0x04 {
		return buildStaticGetInfo()
	}
	meta := summarizeRequest(p.kind, data)
	req := &airgap.HIDRequest{
		Kind:     p.kind,
		Command:  data[0],
		Payload:  data,
		RPID:     meta.rpID,
		User:     meta.user,
		Op:       meta.op,
		AllowLen: meta.allowLen,
	}
	hexReq, _ := airgap.EncodeHIDRequest(req)
	fmt.Println("Request hex (send to offline-only):")
	fmt.Println(hexReq)
	fmt.Println("Paste response hex from offline-only (or press Enter to deny/skip):")
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		// Deny/skip
		if p.kind == airgap.HIDKindCTAP {
			return []byte{0x27} // ctap2ErrOperationDenied
		}
		// U2F conditions not satisfied (0x6985)
		return util.ToBE(uint16(0x6985))
	}
	resp, err := airgap.DecodeHIDResponse(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode response: %v\n", err)
		if p.kind == airgap.HIDKindCTAP {
			return []byte{0x27}
		}
		return util.ToBE(uint16(0x6985))
	}
	if resp.Error != "" {
		if p.kind == airgap.HIDKindCTAP {
			return []byte{0x27}
		}
		return util.ToBE(uint16(0x6985))
	}
	return resp.Payload
}

type requestMeta struct {
	rpID     string
	user     string
	op       string
	allowLen int
}

func summarizeRequest(kind airgap.HIDKind, data []byte) requestMeta {
	if kind == airgap.HIDKindCTAP && len(data) > 0 {
		cmd := data[0]
		switch cmd {
		case 0x01: // makeCredential
			args, err := ctap.DecodeMakeCredentialArgs(data[1:])
			if err == nil && args.RP != nil {
				userName := ""
				if args.User != nil {
					userName = args.User.Name
				}
				return requestMeta{rpID: args.RP.ID, user: userName, op: "makeCredential", allowLen: len(args.ExcludeList)}
			}
		case 0x02: // getAssertion
			args, err := ctap.DecodeGetAssertionArgs(data[1:])
			if err == nil {
				return requestMeta{rpID: args.RPID, user: "", op: "getAssertion", allowLen: len(args.AllowList)}
			}
		}
	}
	return requestMeta{}
}

func handleLocal(report []byte) (bool, []byte) {
	if len(report) < 7 {
		return false, nil
	}
	cmd := report[4]
	switch cmd {
	case 0x81: // PING
		return true, report
	case 0xBB: // KEEPALIVE
		return true, nil
	default:
		return false, nil
	}
}

func buildStaticGetInfo() []byte {
	resp := map[int]interface{}{
		1: []string{"FIDO_2_0", "U2F_V2"},
		3: ctap.DefaultAAGUID,
		4: map[string]bool{
			"plat": false,
			"rk":   false,
			"up":   true,
		},
	}
	payload := util.MarshalCBOR(resp)
	return append([]byte{0x00}, payload...)
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
	go func() {
		<-ctx.Done()
		_ = dev.Close()
	}()

	fmt.Printf("Online-only UHID relay started as %q. Copy request hex to offline-only and paste responses back.\n", name)
	reader := bufio.NewReader(os.Stdin)

	ctapProxy := &offlineProxy{kind: airgap.HIDKindCTAP, reader: reader}
	u2fProxy := &offlineProxy{kind: airgap.HIDKindU2F, reader: reader}
	hidServer := ctap_hid.NewCTAPHIDServer(ctapProxy, u2fProxy)
	hidServer.SetResponseHandler(func(resp []byte) {
		if ctx.Err() != nil {
			return
		}
		_ = dev.WriteReport(ctx, resp)
	})

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		report, err := dev.ReadReport(ctx)
		if err != nil {
			return err
		}
		if handled, resp := handleLocal(report); handled {
			if resp != nil {
				_ = dev.WriteReport(ctx, resp)
			}
			continue
		}
		hidServer.HandleMessage(report)
	}
}

func runOfflineVault(seedFile, vaultPath string) error {
	seedBytes, err := seed.Load(seedFile)
	if err != nil {
		return fmt.Errorf("load seed: %w", err)
	}
	var counters client.CounterState
	if vaultPath == "" {
		fmt.Println("Using in-memory time-based counters")
		counters = state.NewTimeBasedCounterStore(0, nil)
	} else {
		store := state.NewStore(vaultPath, seedBytes)
		cs, err := state.NewCounterStore(store)
		if err != nil {
			return fmt.Errorf("load vault: %w", err)
		}
		fmt.Println("Loaded vault:")
		fmt.Print(cs.String())
		counters = cs
	}
	approver := promptApprover{}
	cl := client.New(seedBytes, counters, approver)

	ctapServer := ctap.NewCTAPServer(cl)
	u2fServer := u2f.NewU2FServer(cl)

	fmt.Println("Paste request hex blobs from online-only; Ctrl+D to exit.")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		req, err := airgap.DecodeHIDRequest(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decode request: %v\n", err)
			continue
		}
		if req.Op != "" || req.RPID != "" {
			fmt.Printf("Request: kind=%d op=%s rp=%s user=%s allow=%d\n", req.Kind, req.Op, req.RPID, req.User, req.AllowLen)
		}
		if !airgap.PromptYesNo("Approve? (Y/n)") {
			respHex, _ := airgap.EncodeHIDResponse(&airgap.HIDResponse{Error: "denied"})
			fmt.Println(respHex)
			continue
		}
		var payload []byte
		switch req.Kind {
		case airgap.HIDKindCTAP:
			payload = ctapServer.HandleMessage(req.Payload)
		case airgap.HIDKindU2F:
			payload = u2fServer.HandleMessage(req.Payload)
		default:
			fmt.Fprintf(os.Stderr, "unknown request kind %d\n", req.Kind)
			continue
		}
		respHex, err := airgap.EncodeHIDResponse(&airgap.HIDResponse{Payload: payload})
		if err != nil {
			fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
			continue
		}
		fmt.Println(respHex)
	}
	return scanner.Err()
}
