// Package envelope provides AES-256-GCM envelope encryption.
//
// Envelope encryption uses a two-layer key scheme:
//   - Master Key (static, from environment variable) encrypts per-record Data Encryption Keys (DEKs).
//   - DEK (random, unique per record) encrypts the actual data.
//
// This approach limits the blast radius of a compromised DEK to a single record,
// and allows Master Key rotation without re-encrypting all stored data.
package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	// keySize is the required AES-256 key length in bytes (256 bits).
	keySize = 32

	// nonceSize is the standard GCM nonce length in bytes (96 bits).
	nonceSize = 12
)

// EncryptedEnvelope holds the encrypted DEK and data produced by Encrypt.
//
// All fields are raw bytes suitable for BSON storage.
// The caller is responsible for persisting every field — omitting any
// field makes decryption impossible.
type EncryptedEnvelope struct {
	// EncryptedDEK is the Data Encryption Key encrypted by the Master Key.
	EncryptedDEK []byte

	// DEKNonce is the GCM nonce used when encrypting the DEK.
	DEKNonce []byte

	// EncryptedData is the plaintext data encrypted by the DEK.
	EncryptedData []byte

	// DataNonce is the GCM nonce used when encrypting the data.
	DataNonce []byte
}

// EnvelopeService performs envelope encryption using a static Master Key.
//
// Create one instance at application startup (via New) and share it
// across goroutines — all methods are safe for concurrent use.
type EnvelopeService struct {
	masterKey []byte // 32 bytes (AES-256)
}

// New creates an EnvelopeService from a hex-encoded Master Key.
//
// Parameters:
//   - masterKeyHex: 64-character hex string representing a 32-byte AES-256 key.
//
// Returns:
//   - *EnvelopeService ready to encrypt/decrypt
//   - error if the key is not valid hex or is not exactly 32 bytes
//
// Example:
//
//	svc, err := envelope.New("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(masterKeyHex string) (*EnvelopeService, error) {
	key, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("envelope: invalid hex master key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("envelope: master key must be %d bytes, got %d", keySize, len(key))
	}
	return &EnvelopeService{masterKey: key}, nil
}

// Encrypt generates a random DEK, encrypts plaintext with the DEK,
// then encrypts the DEK with the Master Key.
//
// Parameters:
//   - plaintext: raw bytes to encrypt (e.g., JSON-marshalled credential data).
//
// Returns:
//   - *EncryptedEnvelope containing the four fields to persist
//   - error if encryption fails
//
// Each call generates a unique DEK and unique nonces, so encrypting the
// same plaintext twice produces different ciphertexts.
//
// Example:
//
//	data, _ := json.Marshal(map[string]string{"botToken": "123:ABC"})
//	env, err := svc.Encrypt(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Persist env.EncryptedDEK, env.DEKNonce, env.EncryptedData, env.DataNonce
func (e *EnvelopeService) Encrypt(plaintext []byte) (*EncryptedEnvelope, error) {
	// 1. Generate random DEK (32 bytes)
	dek := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("envelope: failed to generate DEK: %w", err)
	}

	// 2. Encrypt plaintext with DEK
	encryptedData, dataNonce, err := aesGCMEncrypt(dek, plaintext)
	if err != nil {
		return nil, fmt.Errorf("envelope: failed to encrypt data: %w", err)
	}

	// 3. Encrypt DEK with Master Key
	encryptedDEK, dekNonce, err := aesGCMEncrypt(e.masterKey, dek)
	if err != nil {
		return nil, fmt.Errorf("envelope: failed to encrypt DEK: %w", err)
	}

	return &EncryptedEnvelope{
		EncryptedDEK:  encryptedDEK,
		DEKNonce:      dekNonce,
		EncryptedData: encryptedData,
		DataNonce:     dataNonce,
	}, nil
}

// Decrypt reverses the envelope: decrypts the DEK with the Master Key,
// then decrypts the data with the DEK.
//
// Parameters:
//   - env: the EncryptedEnvelope previously returned by Encrypt.
//
// Returns:
//   - plaintext bytes (the original data passed to Encrypt)
//   - error if any field is nil/empty or decryption fails (wrong key, tampered data)
//
// Example:
//
//	plaintext, err := svc.Decrypt(env)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	var creds map[string]string
//	json.Unmarshal(plaintext, &creds)
func (e *EnvelopeService) Decrypt(env *EncryptedEnvelope) ([]byte, error) {
	if env == nil {
		return nil, fmt.Errorf("envelope: nil envelope")
	}

	// 1. Decrypt DEK with Master Key
	dek, err := aesGCMDecrypt(e.masterKey, env.EncryptedDEK, env.DEKNonce)
	if err != nil {
		return nil, fmt.Errorf("envelope: failed to decrypt DEK: %w", err)
	}

	// 2. Decrypt data with DEK
	plaintext, err := aesGCMDecrypt(dek, env.EncryptedData, env.DataNonce)
	if err != nil {
		return nil, fmt.Errorf("envelope: failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// aesGCMEncrypt encrypts plaintext using AES-256-GCM with a random nonce.
func aesGCMEncrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// aesGCMDecrypt decrypts ciphertext using AES-256-GCM with the provided nonce.
func aesGCMDecrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
