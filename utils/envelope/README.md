# envelope

AES-256-GCM envelope encryption for sensitive data (credentials, tokens, secrets).

## How It Works

Envelope encryption uses two layers of keys:

1. **Master Key** — A static 32-byte key loaded from an environment variable. Encrypts/decrypts the DEK.
2. **DEK (Data Encryption Key)** — A random 32-byte key generated per record. Encrypts/decrypts the actual data.

This limits the blast radius: a compromised DEK only exposes one record. Master Key rotation does not require re-encrypting all stored data.

## Installation

This package is part of the `utils` module. Import it as:

```go
import "github.com/Mapex-Solutions/MapexOS/utils/envelope"
```

No external dependencies — uses only Go standard library (`crypto/aes`, `crypto/cipher`, `crypto/rand`).

## API

### `New(masterKeyHex string) (*EnvelopeService, error)`

Creates a new `EnvelopeService` from a 64-character hex-encoded Master Key (32 bytes).

```go
svc, err := envelope.New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
if err != nil {
    log.Fatal(err)
}
```

**Errors:**
- Invalid hex string
- Key is not exactly 32 bytes

---

### `Encrypt(plaintext []byte) (*EncryptedEnvelope, error)`

Generates a random DEK, encrypts `plaintext` with the DEK (AES-256-GCM), then encrypts the DEK with the Master Key (AES-256-GCM).

```go
data, _ := json.Marshal(map[string]string{"botToken": "123:ABC"})
env, err := svc.Encrypt(data)
// Persist: env.EncryptedDEK, env.DEKNonce, env.EncryptedData, env.DataNonce
```

Each call produces unique ciphertexts (new DEK + new nonces), even for the same plaintext.

---

### `Decrypt(env *EncryptedEnvelope) ([]byte, error)`

Reverses the envelope: decrypts the DEK with the Master Key, then decrypts the data with the DEK.

```go
plaintext, err := svc.Decrypt(env)
var creds map[string]string
json.Unmarshal(plaintext, &creds)
fmt.Println(creds["botToken"]) // "123:ABC"
```

**Errors:**
- Nil envelope
- Wrong Master Key
- Tampered ciphertext (GCM authentication failure)

---

### `EncryptedEnvelope` (struct)

All four fields must be persisted together. Omitting any field makes decryption impossible.

| Field | Description |
|-------|-------------|
| `EncryptedDEK` | DEK encrypted by the Master Key |
| `DEKNonce` | GCM nonce used for DEK encryption |
| `EncryptedData` | Data encrypted by the DEK |
| `DataNonce` | GCM nonce used for data encryption |

## Usage with MongoDB (BSON)

```go
type Credential struct {
    EncryptedDEK  []byte `bson:"encryptedDEK"`
    DEKNonce      []byte `bson:"dekNonce"`
    EncryptedData []byte `bson:"encryptedData"`
    DataNonce     []byte `bson:"dataNonce"`
}
```

## Testing

```bash
cd workspace_go/packages/utils
go test ./envelope/...
```
