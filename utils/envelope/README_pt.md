# envelope — Envelope encryption AES-256-GCM

Encriptação em duas camadas: uma **Master Key** estática encripta uma **Data Encryption Key (DEK)** fresca por registro, e a DEK encripta os dados em si. Isso limita o raio de impacto de uma DEK comprometida a um único registro, e permite rotacionar a Master Key sem re-encriptar todos os dados armazenados.

> Nome do pacote: `envelope` (diretório: `envelope/`).

## Superfície

```go
type EncryptedEnvelope struct {
    EncryptedDEK  []byte // DEK encriptada com a Master Key
    DEKNonce      []byte // nonce GCM usado para encriptar a DEK
    EncryptedData []byte // dados encriptados com a DEK
    DataNonce     []byte // nonce GCM usado para encriptar os dados
}

type EnvelopeService struct{ /* masterKey []byte unexported */ }

func New(masterKeyHex string) (*EnvelopeService, error)
func (e *EnvelopeService) Encrypt(plaintext []byte) (*EncryptedEnvelope, error)
func (e *EnvelopeService) Decrypt(env *EncryptedEnvelope) ([]byte, error)
```

### Internos

| Constante | Valor |
|---|---|
| `keySize`   | `32` bytes (AES-256) |
| `nonceSize` | `12` bytes (GCM padrão) |

### `New`

Decodifica uma string hex de 64 caracteres em uma chave de 32 bytes. Erros:

- `envelope: invalid hex master key` — input não-hex.
- `envelope: master key must be 32 bytes, got N` — comprimento errado.

### `Encrypt`

1. Gera uma DEK aleatória de 32 bytes via `crypto/rand`.
2. AES-256-GCM-encripta `plaintext` com a DEK (nonce aleatório de 12 bytes).
3. AES-256-GCM-encripta a DEK com a Master Key (nonce aleatório de 12 bytes separado).
4. Retorna os 4 byte slices para persistir juntos.

Cada chamada gera DEK e nonces únicos, então encriptar o mesmo plaintext duas vezes produz ciphertexts diferentes.

### `Decrypt`

Inverte a ordem: decifra DEK com Master Key → decifra dados com DEK. Retorna erros descritivos em cada passo (`failed to decrypt DEK`, `failed to decrypt data`); envelope `nil` é rejeitado com `nil envelope`.

## Concorrência

`*EnvelopeService` é seguro para uso concorrente — `Encrypt` e `Decrypt` apenas leem o campo `masterKey`, e `crypto/cipher` GCM é thread-safe por chamada.

## Uso

```go
svc, err := envelope.New(os.Getenv("MASTER_KEY_HEX")) // 64 hex chars
if err != nil { return err }

env, err := svc.Encrypt([]byte(`{"botToken":"123:ABC"}`))
if err != nil { return err }

// Persista os 4 campos:
//   env.EncryptedDEK, env.DEKNonce, env.EncryptedData, env.DataNonce

// Depois:
plaintext, err := svc.Decrypt(env)
if err != nil { return err }
```

## Notas

- Os 4 campos de `EncryptedEnvelope` são obrigatórios para decifrar; perder qualquer um quebra a decifração permanentemente. Persista a struct como uma unidade.
- Rotação de Master Key: re-encripte apenas `EncryptedDEK` (decifre com a chave antiga, re-encripte com a nova) — o ciphertext dos dados não muda. Não há helper embutido para isso; escreva como migração one-off quando necessário.
- A Master Key precisa vir de fora do binário (env var, segredo via KMS, etc.). Nunca compile dentro.
