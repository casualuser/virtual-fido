//go:build darwin && hidvirtual

package vhid

import (
	"fmt"
	"os"

	"github.com/bulwarkid/virtual-fido/ctap_hid"
	"github.com/bulwarkid/virtual-fido/util"
)

var hidLogger = util.NewLogger("[HID] ", util.LogLevelTrace)

var ctapHIDServer *ctap_hid.CTAPHIDServer
var stopChannel = make(chan bool)

func handleResponse(response []byte) {
	if len(response) > 0 {
		fmt.Printf("DEBUG: Sending %d bytes response to HID device: %x\n", len(response), response)
		os.Stdout.Sync()
		PureGoSend(response)
	}
}

func Stop() {
	PureGoStop()
	stopChannel <- true
}

func Start(server *ctap_hid.CTAPHIDServer) {
	ctapHIDServer = server
	server.SetResponseHandler(handleResponse)

	PureGoStart(server)

	// Keep running until stopped
	<-stopChannel
	hidLogger.Println("Stopping HIDVirtualDevice...")
}
