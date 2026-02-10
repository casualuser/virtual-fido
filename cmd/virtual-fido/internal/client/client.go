package client

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"math/big"

	"github.com/bulwarkid/virtual-fido/cmd/virtual-fido/internal/state"
	"github.com/bulwarkid/virtual-fido/cose"
	"github.com/bulwarkid/virtual-fido/crypto"
	"github.com/bulwarkid/virtual-fido/ctap"
	"github.com/bulwarkid/virtual-fido/fido_client"
	"github.com/bulwarkid/virtual-fido/identities"
	"github.com/bulwarkid/virtual-fido/u2f"
	"github.com/bulwarkid/virtual-fido/webauthn"
	"golang.org/x/crypto/hkdf"
)

// State persists counters and credentials.
type State interface {
	IncrementCred(id []byte) uint32
	EnsureCred(id []byte)
	CredValue(id []byte) (uint32, bool)
	SetCred(id []byte, v uint32)
	IncrementGlobal() uint32
	AddCredential(rpID string, userID, credID []byte)
	GetCredentials(rpID string) []state.CredentialEntry
	GetCredential(credID []byte) (state.CredentialEntry, bool)
}

// Approver proxies user prompts.
type Approver interface {
	ApproveClientAction(action fido_client.ClientAction, params fido_client.ClientActionRequestParams) (bool, int)
}

// SeedClient implements CTAP/U2F clients using deterministic credentials derived from a seed.
type SeedClient struct {
	seed     []byte
	counters State
	approver Approver
}

func New(seed []byte, counters State, approver Approver) *SeedClient {
	return &SeedClient{
		seed:     append([]byte(nil), seed...),
		counters: counters,
		approver: approver,
	}
}

// --- CTAP client methods ---

// SupportsResidentKey reports support for Discoverable Credentials (Resident Keys).
func (c *SeedClient) SupportsResidentKey() bool { return true }
func (c *SeedClient) SupportsPIN() bool         { return false }

func (c *SeedClient) NewCredentialSource(params []webauthn.PublicKeyCredentialParams, excludeList []webauthn.PublicKeyCredentialDescriptor, rp *webauthn.PublicKeyCredentialRPEntity, user *webauthn.PublicKeyCrendentialUserEntity) *identities.CredentialSource {
	if !supportsES256(params) {
		return nil
	}
	if rp.Name == "" {
		rp = &webauthn.PublicKeyCredentialRPEntity{ID: rp.ID, Name: rp.ID}
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Name
	}
	credID := deriveCredID(c.seed, rp.ID, user.ID)
	for _, ex := range excludeList {
		if string(ex.ID) == string(credID) {
			return nil
		}
	}
	priv := deriveKey(c.seed, credID)
	c.counters.EnsureCred(credID)
	c.counters.AddCredential(rp.ID, user.ID, credID)
	cs := identities.CredentialSource{
		Type:             "public-key",
		ID:               credID,
		PrivateKey:       &cose.SupportedCOSEPrivateKey{ECDSA: priv},
		RelyingParty:     rp,
		User:             user,
		SignatureCounter: 0,
	}
	return &cs
}

func (c *SeedClient) GetAssertionSource(rpID string, allowList []webauthn.PublicKeyCredentialDescriptor) *identities.CredentialSource {
	storedCreds := c.counters.GetCredentials(rpID)

	var user *webauthn.PublicKeyCrendentialUserEntity
	var credID []byte

	// Strategy: Find a matching credential
	found := false

	// 1. Try to find a match in stored credentials
	for _, stored := range storedCreds {
		// If allowList is present, filter by it
		if len(allowList) > 0 {
			matchesAllowList := false
			for _, allowed := range allowList {
				if bytes.Equal(allowed.ID, stored.CredID) {
					matchesAllowList = true
					break
				}
			}
			if !matchesAllowList {
				continue
			}
		}

		// Found a candidate (first one for now)
		credID = stored.CredID
		user = &webauthn.PublicKeyCrendentialUserEntity{
			ID:          stored.UserID,
			Name:        "Stored User", // We don't store names yet, could add to CredentialEntry or ignore
			DisplayName: "Stored User",
		}
		found = true
		break
	}

	// 2. Fallback for legacy/stateless credentials (only if allowList is provided)
	if !found && len(allowList) > 0 {
		// Try legacy derivation with "default-user"
		// This supports existing tests and stateless flows
		legacyUserID := []byte("default-user")
		legacyCredID := deriveCredID(c.seed, rpID, legacyUserID)

		for _, allowed := range allowList {
			if bytes.Equal(allowed.ID, legacyCredID) {
				credID = legacyCredID
				user = &webauthn.PublicKeyCrendentialUserEntity{
					ID:          legacyUserID,
					Name:        "Default User",
					DisplayName: "Default User",
				}
				found = true
				break
			}
		}
	}

	if !found {
		return nil
	}

	priv := deriveKey(c.seed, credID)
	current, _ := c.counters.CredValue(credID)
	cs := identities.CredentialSource{
		Type:       "public-key",
		ID:         credID,
		PrivateKey: &cose.SupportedCOSEPrivateKey{ECDSA: priv},
		RelyingParty: &webauthn.PublicKeyCredentialRPEntity{
			ID:   rpID,
			Name: rpID,
		},
		User:             user,
		SignatureCounter: int32(current),
	}
	return &cs
}

func (c *SeedClient) BumpSignatureCounter(cs *identities.CredentialSource) int32 {
	count := c.counters.IncrementCred(cs.ID)
	cs.SignatureCounter = int32(count)
	return cs.SignatureCounter
}

func (c *SeedClient) GetIdentity(id []byte) *identities.CredentialSource {
	entry, ok := c.counters.GetCredential(id)
	if !ok {
		return nil
	}
	priv := deriveKey(c.seed, id)
	cs := identities.CredentialSource{
		Type:       "public-key",
		ID:         id,
		PrivateKey: &cose.SupportedCOSEPrivateKey{ECDSA: priv},
		RelyingParty: &webauthn.PublicKeyCredentialRPEntity{
			ID:   entry.RPID,
			Name: entry.RPID,
		},
		User: &webauthn.PublicKeyCrendentialUserEntity{
			ID:          entry.UserID,
			Name:        "Stored User",
			DisplayName: "Stored User",
		},
	}
	val, ok := c.counters.CredValue(id)
	if ok {
		cs.SignatureCounter = int32(val)
	}
	return &cs
}

func (c *SeedClient) CreateAttestationCertificiate(priv *cose.SupportedCOSEPrivateKey) []byte {
	caCert, caKey := c.attestationCA()
	cert, err := identities.CreateSelfSignedAttestationCertificate(caCert, caKey, priv)
	if err != nil || cert == nil {
		return nil
	}
	return cert.Raw
}

func (c *SeedClient) ApproveAccountCreation(rpName, rpID string) bool {
	if rpName == "" {
		rpName = rpID
	}
	ok, _ := c.approver.ApproveClientAction(fido_client.ClientActionFIDOMakeCredential, fido_client.ClientActionRequestParams{RelyingParty: rpName})
	return ok
}

func (c *SeedClient) ApproveAccountLogin(cs *identities.CredentialSource) bool {
	ok, _ := c.approver.ApproveClientAction(fido_client.ClientActionFIDOGetAssertion, fido_client.ClientActionRequestParams{
		RelyingParty: cs.RelyingParty.Name,
		UserName:     cs.User.Name,
	})
	return ok
}

// --- PIN stubs ---
func (c *SeedClient) PINHash() []byte                  { return nil }
func (c *SeedClient) SetPINHash([]byte)                {}
func (c *SeedClient) PINRetries() int32                { return 8 }
func (c *SeedClient) SetPINRetries(int32)              {}
func (c *SeedClient) PINKeyAgreement() *crypto.ECDHKey { return crypto.GenerateECDHKey() }
func (c *SeedClient) PINToken() []byte                 { return nil }

// --- U2F client methods ---

func (c *SeedClient) SealingEncryptionKey() []byte { return hkdfBytes(c.seed, "u2f-seal", 32) }

func (c *SeedClient) NewPrivateKey() *ecdsa.PrivateKey {
	return crypto.GenerateECDSAKey()
}

func (c *SeedClient) NewAuthenticationCounterId() uint32 {
	return c.counters.IncrementGlobal()
}

func (c *SeedClient) ApproveU2FRegistration(*webauthn.KeyHandle) bool {
	ok, _ := c.approver.ApproveClientAction(fido_client.ClientActionU2FRegister, fido_client.ClientActionRequestParams{})
	return ok
}

func (c *SeedClient) ApproveU2FAuthentication(*webauthn.KeyHandle) bool {
	ok, _ := c.approver.ApproveClientAction(fido_client.ClientActionU2FAuthenticate, fido_client.ClientActionRequestParams{})
	return ok
}

// --- helpers ---

func supportsES256(params []webauthn.PublicKeyCredentialParams) bool {
	for _, p := range params {
		if p.Algorithm == cose.COSE_ALGORITHM_ID_ES256 && p.Type == "public-key" {
			return true
		}
	}
	return false
}

func deriveCredID(seed []byte, rpID string, userID []byte) []byte {
	mac := hmac.New(sha256.New, seed)
	mac.Write([]byte("credid:"))
	mac.Write([]byte(rpID))
	mac.Write(userID)
	return mac.Sum(nil)
}

func deriveKey(seed []byte, credID []byte) *ecdsa.PrivateKey {
	okm := hkdfBytes(seed, "key:"+string(credID), 32)
	d := new(big.Int).SetBytes(okm)
	curve := elliptic.P256()
	n := curve.Params().N
	d.Mod(d, new(big.Int).Sub(n, big.NewInt(1)))
	d.Add(d, big.NewInt(1))
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = curve
	priv.D = d
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	return priv
}

func hkdfBytes(seed []byte, info string, size int) []byte {
	r := hkdf.New(sha256.New, seed, nil, []byte(info))
	out := make([]byte, size)
	if _, err := r.Read(out); err != nil {
		panic(err)
	}
	return out
}

// Derive deterministic attestation CA from seed.
func (c *SeedClient) attestationCA() (*x509.Certificate, *cose.SupportedCOSEPrivateKey) {
	seedKey := hkdfBytes(c.seed, "attca", 32)
	d := new(big.Int).SetBytes(seedKey)
	curve := elliptic.P256()
	n := curve.Params().N
	d.Mod(d, new(big.Int).Sub(n, big.NewInt(1)))
	d.Add(d, big.NewInt(1))
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	caPriv := &cose.SupportedCOSEPrivateKey{ECDSA: priv}
	caCert, err := identities.CreateSelfSignedCA(caPriv)
	if err != nil {
		return nil, nil
	}
	return caCert, caPriv
}

// Ensure interface compliance.
var _ ctap.CTAPClient = (*SeedClient)(nil)
var _ u2f.U2FClient = (*SeedClient)(nil)
