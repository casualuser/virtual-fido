package client

import (
	"bytes"
	"testing"

	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/state"
	"github.com/bulwarkid/virtual-fido/fido_client"
	"github.com/bulwarkid/virtual-fido/webauthn"
)

type DummyApprover struct{}

func (d *DummyApprover) ApproveClientAction(action fido_client.ClientAction, params fido_client.ClientActionRequestParams) (bool, int) {
	return true, 0
}

func TestResidentKeys(t *testing.T) {
	// Setup
	seed := []byte("test-seed")
	// Use in-memory counter store. NewTimeBasedCounterStore(base, nowFunc)
	counters := state.NewTimeBasedCounterStore(0, nil)
	approver := &DummyApprover{}
	client := New(seed, counters, approver)

	// Test Data
	rp := &webauthn.PublicKeyCredentialRPEntity{ID: "example.com", Name: "Example"}
	user := &webauthn.PublicKeyCrendentialUserEntity{ID: []byte("user-123"), Name: "User 123"}

	// 1. Registration (MakeCredential)
	// This should store the credential in the vault
	credSource := client.NewCredentialSource(
		[]webauthn.PublicKeyCredentialParams{{Type: "public-key", Algorithm: -7}}, // ES256
		nil,
		rp,
		user,
	)
	if credSource == nil {
		t.Fatal("NewCredentialSource returned nil")
	}

	// Verify persistence in counters
	creds := counters.GetCredentials("example.com")
	if len(creds) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(creds))
	}
	if !bytes.Equal(creds[0].UserID, user.ID) {
		t.Errorf("Stored UserID mismatch")
	}
	if !bytes.Equal(creds[0].CredID, credSource.ID) {
		t.Errorf("Stored CredID mismatch")
	}

	// 2. Authentication (GetAssertion) - Explicit AllowList (Stateless-ish)
	// Should work by finding the ID in store
	assertion := client.GetAssertionSource("example.com", []webauthn.PublicKeyCredentialDescriptor{
		{Type: "public-key", ID: credSource.ID},
	})
	if assertion == nil {
		t.Fatal("GetAssertionSource (AllowList) returned nil")
	}
	if !bytes.Equal(assertion.ID, credSource.ID) {
		t.Error("Assertion ID mismatch")
	}

	// 3. Authentication (GetAssertion) - Resident Key (Empty AllowList)
	// Should find via RPID
	rkAssertion := client.GetAssertionSource("example.com", nil)
	if rkAssertion == nil {
		t.Fatal("GetAssertionSource (Resident Key) returned nil")
	}
	if !bytes.Equal(rkAssertion.ID, credSource.ID) {
		t.Error("RK Assertion ID mismatch")
	}
	if !bytes.Equal(rkAssertion.User.ID, user.ID) {
		t.Errorf("RK Assertion UserID mismatch. Got %s, want %s", rkAssertion.User.ID, user.ID)
	}
}
