package envelope

import (
	"encoding/hex"
	"testing"
)

// validHexKey is a 64-char hex string (32 bytes) for testing.
const validHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestNew_ValidKey(t *testing.T) {
	svc, err := New(validHexKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNew_InvalidHex(t *testing.T) {
	_, err := New("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestNew_WrongKeyLength(t *testing.T) {
	shortKey := hex.EncodeToString([]byte("tooshort"))
	_, err := New(shortKey)
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	svc, err := New(validHexKey)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	plaintext := []byte(`{"botToken":"123:ABC","chatId":"-100123"}`)

	env, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// All envelope fields must be non-empty
	if len(env.EncryptedDEK) == 0 {
		t.Error("EncryptedDEK is empty")
	}
	if len(env.DEKNonce) == 0 {
		t.Error("DEKNonce is empty")
	}
	if len(env.EncryptedData) == 0 {
		t.Error("EncryptedData is empty")
	}
	if len(env.DataNonce) == 0 {
		t.Error("DataNonce is empty")
	}

	// Decrypt must recover original plaintext
	decrypted, err := svc.Decrypt(env)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_UniqueCiphertexts(t *testing.T) {
	svc, _ := New(validHexKey)
	plaintext := []byte("same data")

	env1, _ := svc.Encrypt(plaintext)
	env2, _ := svc.Encrypt(plaintext)

	// Different DEK + nonces each time → different ciphertexts
	if hex.EncodeToString(env1.EncryptedData) == hex.EncodeToString(env2.EncryptedData) {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts")
	}
}

func TestDecrypt_NilEnvelope(t *testing.T) {
	svc, _ := New(validHexKey)
	_, err := svc.Decrypt(nil)
	if err == nil {
		t.Fatal("expected error for nil envelope")
	}
}

func TestDecrypt_WrongMasterKey(t *testing.T) {
	svc1, _ := New(validHexKey)
	svc2, _ := New("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")

	env, _ := svc1.Encrypt([]byte("secret"))

	_, err := svc2.Decrypt(env)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong master key")
	}
}

func TestDecrypt_TamperedData(t *testing.T) {
	svc, _ := New(validHexKey)
	env, _ := svc.Encrypt([]byte("secret"))

	// Flip a byte in encrypted data
	env.EncryptedData[0] ^= 0xFF

	_, err := svc.Decrypt(env)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}
