package state

import "encoding/hex"

// CounterStore wraps a Store/Vault with simple increment helpers.
type CounterStore struct {
	store *Store
	vault *Vault
}

// NewCounterStore loads vault via store.
func NewCounterStore(store *Store) (*CounterStore, error) {
	v, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &CounterStore{store: store, vault: v}, nil
}

// IncrementCred increments and returns the per-credential counter.
func (c *CounterStore) IncrementCred(id []byte) uint32 {
	key := encodeKey(id)
	val := c.vault.Counters[key]
	val++
	c.vault.Counters[key] = val
	_ = c.store.Save(c.vault)
	return val
}

// EnsureCred initializes a credential counter to zero if absent.
func (c *CounterStore) EnsureCred(id []byte) {
	key := encodeKey(id)
	if _, ok := c.vault.Counters[key]; !ok {
		c.vault.Counters[key] = 0
		_ = c.store.Save(c.vault)
	}
}

// CredValue returns the stored counter and whether it exists.
func (c *CounterStore) CredValue(id []byte) (uint32, bool) {
	val, ok := c.vault.Counters[encodeKey(id)]
	return val, ok
}

// SetCred sets the counter to the given value.
func (c *CounterStore) SetCred(id []byte, v uint32) {
	c.vault.Counters[encodeKey(id)] = v
	_ = c.store.Save(c.vault)
}

// IncrementGlobal increments the global authentication counter (U2F style).
func (c *CounterStore) IncrementGlobal() uint32 {
	c.vault.AuthenticationCounter++
	_ = c.store.Save(c.vault)
	return c.vault.AuthenticationCounter
}

func encodeKey(id []byte) string {
	return hex.EncodeToString(id)
}
