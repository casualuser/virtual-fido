package airgap

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/fxamacker/cbor/v2"
)

// Operation indicates whether the request is makeCredential or getAssertion.
type Operation uint8

const (
	OpMakeCredential Operation = 1
	OpGetAssertion   Operation = 2
)

// Request is a sanitized, serializable payload sent from watch-only to vault.
type Request struct {
	Op             Operation `cbor:"1,keyasint"`
	RPID           string    `cbor:"2,keyasint"`
	UserHandle     []byte    `cbor:"3,keyasint,omitempty"`
	ClientDataHash []byte    `cbor:"4,keyasint"`
	AllowList      [][]byte  `cbor:"5,keyasint,omitempty"`
	Label          string    `cbor:"6,keyasint,omitempty"`
}

// Response is sent from vault back to watch-only.
type Response struct {
	Op                Operation `cbor:"1,keyasint"`
	CredentialID      []byte    `cbor:"2,keyasint,omitempty"`
	AuthenticatorData []byte    `cbor:"3,keyasint,omitempty"`
	AttestationObject []byte    `cbor:"4,keyasint,omitempty"`
	Signature         []byte    `cbor:"5,keyasint,omitempty"`
	UserHandle        []byte    `cbor:"6,keyasint,omitempty"`
	SignCount         uint32    `cbor:"7,keyasint,omitempty"`
}

// EncodeRequest to hex for copy/paste.
func EncodeRequest(req *Request) (string, error) {
	b, err := cbor.Marshal(req)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// DecodeRequest parses hex input after sanitizing printable ASCII.
func DecodeRequest(s string) (*Request, error) {
	bin, err := sanitizeHex(s)
	if err != nil {
		return nil, err
	}
	var req Request
	if err := cbor.Unmarshal(bin, &req); err != nil {
		return nil, fmt.Errorf("decode request: %w", err)
	}
	return &req, nil
}

// EncodeResponse to hex for copy/paste.
func EncodeResponse(resp *Response) (string, error) {
	b, err := cbor.Marshal(resp)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// DecodeResponse parses hex input after sanitizing printable ASCII.
func DecodeResponse(s string) (*Response, error) {
	bin, err := sanitizeHex(s)
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := cbor.Unmarshal(bin, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}

// sanitizeHex keeps only printable ASCII, then hex-decodes.
func sanitizeHex(s string) ([]byte, error) {
	var sb strings.Builder
	for _, r := range s {
		if r > unicode.MaxASCII || r < 0x20 {
			continue
		}
		if strings.ContainsRune(" \t\r\n", r) {
			continue
		}
		sb.WriteRune(r)
	}
	clean := sb.String()
	if clean == "" {
		return nil, errors.New("empty input")
	}
	b, err := hex.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	return b, nil
}
