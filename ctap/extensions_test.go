package ctap

import (
	"testing"

	"github.com/bulwarkid/virtual-fido/cose"
	"github.com/bulwarkid/virtual-fido/test"
	"github.com/bulwarkid/virtual-fido/util"
	"github.com/bulwarkid/virtual-fido/webauthn"
	"github.com/fxamacker/cbor/v2"
)

func TestMakeCredential_WithCredProps(t *testing.T) {
	client := &dummyCTAPClient{}
	ctap := NewCTAPServer(client)

	args := MakeCredentialArgs{
		ClientDataHash: []byte{},
		RP: &webauthn.PublicKeyCredentialRPEntity{
			ID:   "example.com",
			Name: "Example",
		},
		User: &webauthn.PublicKeyCrendentialUserEntity{
			ID:          []byte{0, 1, 2, 3, 4},
			DisplayName: "Alice",
			Name:        "Alice",
		},
		PubKeyCredParams: []webauthn.PublicKeyCredentialParams{
			{
				Type:      "public-key",
				Algorithm: cose.COSE_ALGORITHM_ID_ES256,
			},
		},
		Extensions: map[string]interface{}{
			"credProps": true,
		},
	}
	argBytes, err := cbor.Marshal(&args)
	util.CheckErr(err, "Cant create makeCredentialArgs")
	message := util.Concat([]byte{byte(ctapCommandMakeCredential)}, argBytes)

	responseBytes := ctap.HandleMessage(message)
	test.AssertNotNil(t, responseBytes, "Response is nil")
	test.AssertEqual(t, ctapStatusCode(responseBytes[0]), ctap1ErrSuccess, "Response code is not success")

	var response MakeCredentialResponse
	err = cbor.Unmarshal(responseBytes[1:], &response)
	util.CheckErr(err, "Invalid response")

	// Verify authData has ExtensionDataIncluded flag (0x80)
	// Response.AuthData:
	// 32 byte RPID Hash
	// 1 byte flags (index 32)
	// 4 byte counter

	test.Assert(t, len(response.AuthData) > 37, "AuthData too short")
	flags := response.AuthData[32]
	test.Assert(t, flags&0x80 != 0, "Extension Data Included flag not set")
}
