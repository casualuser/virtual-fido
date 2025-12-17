//go:build windows

package usbipwin2

import (
	"errors"
	"fmt"
	"net"

	virtual_fido "github.com/bulwarkid/virtual-fido"
)

// Client is a thin placeholder for a Windows usbip client transport.
// It assumes a usbip client driver (e.g., usbip-win2) is installed to
// materialize a virtual USB/HID device locally.
type Client struct {
	Address string // host:port of usbip server
}

// New creates a Windows usbip client pointing at the usbip server.
func New(address string) *Client {
	return &Client{Address: address}
}

// Start connects to the usbip server and would attach the virtual device via
// the installed usbip client driver. Currently not implemented.
func (c *Client) Start(fido virtual_fido.FIDOClient) error {
	if c.Address == "" {
		return errors.New("usbipwin2: address required")
	}
	if _, err := net.ResolveTCPAddr("tcp", c.Address); err != nil {
		return fmt.Errorf("usbipwin2: resolve address: %w", err)
	}
	return errors.New("usbipwin2: not implemented (requires Windows usbip client driver integration)")
}
