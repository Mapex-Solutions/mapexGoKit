# envelope — AES-256-GCM envelope encryption

Two-layer key encryption: a static **Master Key** encrypts a fresh per-record **Data Encryption Key (DEK)**, and the DEK encrypts the actual data. This limits the blast radius of a compromised DEK to a single record, and lets you rotate the Master Key without re-encrypting all stored data.

> Package name: `envelope` (directory: `envelope/`).

## Surface

```go
type EncryptedEnvelope struct {
    EncryptedDEK  []byte // DEK encrypted with the Master Key
    DEKNonce      []byte // GCM nonce used to encrypt the DEK
    EncryptedData []byte // Data encrypted with the DEK
    DataNonce     []byte // GCM nonce used to encrypt the data
}

type EnvelopeService struct{ /* unexported masterKey []byte */ }

func New(masterKeyHex string) (*EnvelopeService, error)
func (e *EnvelopeService) Encrypt(plaintext []byte) (*EncryptedEnvelope, error)
func (e *EnvelopeService) Decrypt(env *EncryptedEnvelope) ([]byte, error)
```

### Internals

| Constant | Value |
|---|---|
| `keySize`   | `32` bytes (AES-256) |
| `nonceSize` | `12` bytes (standard GCM) |

### `New`

Decodes a 64-character hex string into a 32-byte key. Errors:

- `envelope: invalid hex master key` — non-hex input.
- `envelope: master key must be 32 bytes, got N` — wrong length.

### `Encrypt`

1. Generates a 32-byte random DEK via `crypto/rand`.
2. AES-256-GCM-encrypts `plaintext` with the DEK (random 12-byte nonce).
3. AES-256-GCM-encrypts the DEK with the Master Key (separate random 12-byte nonce).
4. Returns the four byte slices to persist together.

Each call yields a unique DEK and unique nonces, so encrypting the same plaintext twice produces different ciphertexts.

### `Decrypt`

Reverses the order: decrypt DEK with Master Key → decrypt data with DEK. Returns descriptive errors on each step (`failed to decrypt DEK`, `failed to decrypt data`); `nil` envelope is rejected with `nil envelope`.

## Concurrency

`*EnvelopeService` is safe for concurrent use — `Encrypt` and `Decrypt` only read the `masterKey` field, and `crypto/cipher` GCM is thread-safe per call.

## Usage

```go
svc, err := envelope.New(os.Getenv("MASTER_KEY_HEX")) // 64 hex chars
if err != nil { return err }

env, err := svc.Encrypt([]byte(`{"botToken":"123:ABC"}`))
if err != nil { return err }

// Persist all four fields:
//   env.EncryptedDEK, env.DEKNonce, env.EncryptedData, env.DataNonce

// Later:
plaintext, err := svc.Decrypt(env)
if err != nil { return err }
```

## Notes

- All four `EncryptedEnvelope` fields are required to decrypt; missing any one breaks decryption permanently. Persist the struct as a unit.
- Master Key rotation: re-encrypt only `EncryptedDEK` (decrypt with old key, re-encrypt with new key) — the data ciphertext does not change. There is no built-in helper for this; write it as a one-off migration when needed.
- The Master Key must come from outside the binary (env var, KMS-fetched secret, etc.). Never compile it in.
