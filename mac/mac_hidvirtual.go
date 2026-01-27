//go:build darwin && hidvirtual

package mac

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/bulwarkid/virtual-fido/ctap_hid"
	"github.com/bulwarkid/virtual-fido/util"
)

/*
#cgo CFLAGS: -DHID_VIRTUAL
#cgo LDFLAGS: -framework CoreHID -framework Foundation -L${SRCDIR}/output -lHIDVirtualDevice
#include <stdlib.h>
#include <stdint.h>
#include "HIDVirtualDevice/HIDVirtualDeviceBridge.h"

extern void bridgeCallbackWrapper(const uint8_t* data, size_t length);
*/
import "C"

var hidLogger = util.NewLogger("[HID] ", util.LogLevelTrace)

var ctapHIDServer *ctap_hid.CTAPHIDServer
var virtualDevice C.hid_virtual_device_t

func handleResponse(response []byte) {
	if len(response) > 0 {
		fmt.Printf("DEBUG: Sending %d bytes response to HID device: %x\n", len(response), response)
		os.Stdout.Sync()
		C.hid_virtual_device_send_report(
			virtualDevice,
			(*C.uint8_t)(unsafe.Pointer(&response[0])),
			C.size_t(len(response)),
		)
	}
}

//export HIDReceiveReport
func HIDReceiveReport(dataPointer *C.uint8_t, length C.size_t) {
	data := C.GoBytes(unsafe.Pointer(dataPointer), C.int(length))
	fmt.Printf("DEBUG: Received %d bytes report: %x\n", int(length), data)
	os.Stdout.Sync()
	hidLogger.Printf("Received %d bytes report: %x", int(length), data)
	ctapHIDServer.HandleMessage(data)
}

var stopChannel = make(chan bool)

func Stop() {
	stopChannel <- true
}

func Start(server *ctap_hid.CTAPHIDServer) {
	server.SetResponseHandler(handleResponse)
	ctapHIDServer = server

	deviceName := C.CString("Virtual FIDO")
	defer C.free(unsafe.Pointer(deviceName))

	virtualDevice = C.hid_virtual_device_create(deviceName)
	if virtualDevice == nil {
		hidLogger.Println("Failed to create HIDVirtualDevice")
		return
	}

	C.hid_virtual_device_set_callback(virtualDevice, C.hid_report_callback_t(C.bridgeCallbackWrapper))

	hidLogger.Println("HIDVirtualDevice started successfully")

	// Keep running (the device will handle events via callbacks) until stopped
	<-stopChannel
	hidLogger.Println("Stopping HIDVirtualDevice...")
}
