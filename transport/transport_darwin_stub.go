//go:build !darwin

package transport

import virtual_fido "github.com/bulwarkid/virtual-fido"

func runDarwinServer(client virtual_fido.FIDOClient, deviceName string) {}
