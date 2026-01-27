//go:build darwin

package transport

import (
	"context"
	"os"
	"os/signal"

	virtual_fido "github.com/bulwarkid/virtual-fido"
	"github.com/bulwarkid/virtual-fido/ctap"
	"github.com/bulwarkid/virtual-fido/ctap_hid"
	"github.com/bulwarkid/virtual-fido/mac"
	"github.com/bulwarkid/virtual-fido/u2f"
)

func runDarwinServer(client virtual_fido.FIDOClient, deviceName string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		cancel()
	}()

	ctapServer := ctap.NewCTAPServer(client)
	u2fServer := u2f.NewU2FServer(client)
	ctapHIDServer := ctap_hid.NewCTAPHIDServer(ctapServer, u2fServer)

	// Note: deviceName is currently ignored by the mac driver, but we could pass it if supported later
	go mac.Start(ctapHIDServer)

	<-ctx.Done()
	mac.Stop()
}
