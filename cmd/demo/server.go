package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	virtual_fido "github.com/bulwarkid/virtual-fido"
	"github.com/bulwarkid/virtual-fido/fido_client"
)

func prompt(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(prompt)
	fmt.Print("--> ")
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Could not read user input: %s - %s\n", response, err)
		panic(err)
	}
	response = strings.ToLower(strings.TrimSpace(response))
	if response == "" || response == "y" || response == "yes" {
		return true
	}
	return false
}

type ClientSupport struct {
	vaultFilename   string
	vaultPassphrase string
}

func (support *ClientSupport) ApproveClientAction(action fido_client.ClientAction, params fido_client.ClientActionRequestParams) bool {
	switch action {
	case fido_client.ClientActionFIDOGetAssertion:
		return prompt(fmt.Sprintf("Approve login for \"%s\" with identity \"%s\" (Y/n)?", params.RelyingParty, params.UserName))
	case fido_client.ClientActionFIDOMakeCredential:
		return prompt(fmt.Sprintf("Approve account creation for \"%s\" (Y/n)?", params.RelyingParty))
	case fido_client.ClientActionU2FAuthenticate:
		return prompt("Approve registration of U2F device (Y/n)?")
	case fido_client.ClientActionU2FRegister:
		return prompt("Approve use of U2F device (Y/n)?")
	}
	fmt.Printf("Unknown client action for approval: %d\n", action)
	return false
}

func (support *ClientSupport) SaveData(data []byte) {
	f, err := os.OpenFile(support.vaultFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	checkErr(err, "Could not open vault file")
	_, err = f.Write(data)
	checkErr(err, "Could not write vault data")
}

func (support *ClientSupport) RetrieveData() []byte {
	f, err := os.Open(support.vaultFilename)
	if os.IsNotExist(err) {
		return nil
	}
	checkErr(err, "Could not open vault")
	data, err := io.ReadAll(f)
	checkErr(err, "Could not read vault data")
	return data
}

func (support *ClientSupport) Passphrase() string {
	return support.vaultPassphrase
}

func runUsbipServer(client virtual_fido.FIDOClient) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		virtual_fido.Start(client)
		wg.Done()
	}()
	go func() {
		time.Sleep(500 * time.Millisecond)
		prog := platformUSBIPExec()
		if prog != nil {
			prog.Stdin = os.Stdin
			prog.Stdout = os.Stdout
			prog.Stderr = os.Stderr
			err := prog.Run()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			}
		}
		wg.Done()
	}()
	wg.Wait()
}

func runUhidServer(client virtual_fido.FIDOClient) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// Let Ctrl+C stop the process; StartUHID exits on ctx cancel.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		cancel()
	}()

	err := virtual_fido.StartUHID(ctx, client, "Virtual FIDO")
	if err != nil {
		fmt.Printf("UHID error: %v\n", err)
	}
}

func runServer(client virtual_fido.FIDOClient) {
	switch strings.ToLower(transport) {
	case "usbip":
		runUsbipServer(client)
	case "uhid":
		runUhidServer(client)
	default:
		fmt.Printf("Unknown transport %q; expected usbip or uhid\n",
			transport)
	}
}
