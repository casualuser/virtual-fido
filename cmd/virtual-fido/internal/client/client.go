package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"math/big"

	"github.com/bulwarkid/virtual-fido/cose"
	"github.com/bulwarkid/virtual-fido/crypto"
	"github.com/bulwarkid/virtual-fido/ctap"
	"github.com/bulwarkid/virtual-fido/fido_client"
	"github.com/bulwarkid/virtual-fido/identities"
	"github.com/bulwarkid/virtual-fido/u2f"
	"github.com/bulwarkid/virtual-fido/webauthn"
	"golang.org/x/crypto/hkdf"
)

// CounterState persists counters; implemented via the state.CounterStore wrapper.
type CounterState interface {
	IncrementCred(id []byte) uint32
	EnsureCred(id []byte)
	CredValue(id []byte) (uint32, bool)
	SetCred(id []byte, v uint32)
	IncrementGlobal() uint32
}

// Approver proxies user prompts.
type Approver interface {
	ApproveClientAction(action fido_client.ClientAction, params fido_client.ClientActionRequestParams) bool
}

// SeedClient implements CTAP/U2F clients using deterministic credentials derived from a seed.
type SeedClient struct {
	seed     []byte
	counters CounterState
	approver Approver
}

func New(seed []byte, counters CounterState, approver Approver) *SeedClient {
	return &SeedClient{
		seed:     append([]byte(nil), seed...),
		counters: counters,
		approver: approver,
	}
}

// --- CTAP client methods ---

func (c *SeedClient) SupportsResidentKey() bool { return false }
func (c *SeedClient) SupportsPIN() bool         { return false }

func (c *SeedClient) NewCredentialSource(params []webauthn.PublicKeyCredentialParams, excludeList []webauthn.PublicKeyCredentialDescriptor, rp *webauthn.PublicKeyCredentialRPEntity, user *webauthn.PublicKeyCrendentialUserEntity) *identities.CredentialSource {
	if !supportsES256(params) {
		return nil
	}
	if rp.Name == "" {
		rp = &webauthn.PublicKeyCredentialRPEntity{ID: rp.ID, Name: rp.ID}
	}
	if user.Name == "" {
		user.Name = string(user.ID)
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
	if len(allowList) == 0 {
		return nil
	}
	credID := allowList[0].ID
	priv := deriveKey(c.seed, credID)
	count := c.counters.IncrementCred(credID)
	userName := hex.EncodeToString(credID)
	cs := identities.CredentialSource{
		Type:       "public-key",
		ID:         credID,
		PrivateKey: &cose.SupportedCOSEPrivateKey{ECDSA: priv},
		RelyingParty: &webauthn.PublicKeyCredentialRPEntity{
			ID:   rpID,
			Name: rpID,
		},
		User: &webauthn.PublicKeyCrendentialUserEntity{
			ID:          []byte{},
			Name:        userName,
			DisplayName: userName,
		},
		SignatureCounter: int32(count),
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
	return c.approver.ApproveClientAction(fido_client.ClientActionFIDOMakeCredential, fido_client.ClientActionRequestParams{RelyingParty: rpName})
}

func (c *SeedClient) ApproveAccountLogin(cs *identities.CredentialSource) bool {
	return c.approver.ApproveClientAction(fido_client.ClientActionFIDOGetAssertion, fido_client.ClientActionRequestParams{
		RelyingParty: cs.RelyingParty.Name,
		UserName:     cs.User.Name,
	})
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
	return c.approver.ApproveClientAction(fido_client.ClientActionU2FRegister, fido_client.ClientActionRequestParams{})
}

func (c *SeedClient) ApproveU2FAuthentication(*webauthn.KeyHandle) bool {
	return c.approver.ApproveClientAction(fido_client.ClientActionU2FAuthenticate, fido_client.ClientActionRequestParams{})
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
