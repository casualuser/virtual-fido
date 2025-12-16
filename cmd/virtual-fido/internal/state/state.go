package state

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/fxamacker/cbor/v2"
	"golang.org/x/crypto/hkdf"
)

// Vault holds persisted counters keyed by credential ID.
// AuthenticationCounter is the global U2F-style counter.
type Vault struct {
	Counters              map[string]uint32 `cbor:"1,keyasint"`
	AuthenticationCounter uint32            `cbor:"2,keyasint"`
}

// Store persists Vault as encrypted CBOR using a key derived from seed.
type Store struct {
	path string
	key  []byte
}

// NewStore creates or loads a store at path using seed-derived key.
func NewStore(path string, seed []byte) *Store {
	return &Store{
		path: path,
		key:  deriveKey(seed),
	}
}

// Load returns the stored vault (or an empty one if file missing).
func (s *Store) Load() (*Vault, error) {
	if s.path == "" {
		return nil, errors.New("state path required")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Vault{Counters: map[string]uint32{}}, nil
		}
		return nil, err
	}
	if len(data) < 1+12 {
		return nil, errors.New("state file too small")
	}
	if data[0] != 1 {
		return nil, errors.New("unsupported state version")
	}
	nonce := data[1 : 1+12]
	ct := data[1+12:]
	plain, err := decrypt(s.key, nonce, ct)
	if err != nil {
		return nil, err
	}
	var v Vault
	if err := cbor.Unmarshal(plain, &v); err != nil {
		return nil, err
	}
	if v.Counters == nil {
		v.Counters = map[string]uint32{}
	}
	return &v, nil
}

// Save writes the vault atomically.
func (s *Store) Save(v *Vault) error {
	if v == nil {
		return errors.New("vault nil")
	}
	if v.Counters == nil {
		v.Counters = map[string]uint32{}
	}
	plain, err := cbor.Marshal(v)
	if err != nil {
		return err
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ct, err := encrypt(s.key, nonce, plain)
	if err != nil {
		return err
	}
	buf := append([]byte{1}, append(nonce, ct...)...)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".vault-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func deriveKey(seed []byte) []byte {
	if len(seed) == 0 {
		panic("seed required")
	}
	h := hkdf.New(sha256.New, seed, nil, []byte("virtual-fido:vault"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(h, key); err != nil {
		panic(err)
	}
	return key
}

func encrypt(key, nonce, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, data, nil), nil
}

func decrypt(key, nonce, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, data, nil)
}

// HashSeed returns a short fingerprint of the seed for logging.
func HashSeed(seed []byte) string {
	m := hmac.New(sha256.New, []byte("virtual-fido:fingerprint"))
	m.Write(seed)
	sum := m.Sum(nil)
	return hex.EncodeToString(sum[:4])
}
