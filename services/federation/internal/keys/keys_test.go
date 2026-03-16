package keys

import (
	"testing"
)

func TestGenerateAndDecrypt(t *testing.T) {
	secret := make([]byte, 32) // all-zero secret for test

	pubPEM, privEnc, err := GenerateActorKeyPair(secret)
	if err != nil {
		t.Fatalf("GenerateActorKeyPair error: %v", err)
	}
	if pubPEM == "" {
		t.Fatal("expected non-empty pubPEM")
	}
	if len(privEnc) == 0 {
		t.Fatal("expected non-empty privEnc")
	}

	priv, err := DecryptPrivateKey(privEnc, secret)
	if err != nil {
		t.Fatalf("DecryptPrivateKey error: %v", err)
	}
	if priv == nil {
		t.Fatal("expected non-nil private key")
	}
	if priv.N.BitLen() < 2048 {
		t.Fatalf("expected >= 2048-bit key, got %d", priv.N.BitLen())
	}
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	secret := make([]byte, 32)

	_, privEnc, err := GenerateActorKeyPair(secret)
	if err != nil {
		t.Fatalf("GenerateActorKeyPair error: %v", err)
	}

	// Corrupt the ciphertext to trigger auth failure
	privEnc[len(privEnc)-1] ^= 0xFF
	_, err = DecryptPrivateKey(privEnc, secret)
	if err == nil {
		t.Fatal("expected decryption error with corrupted ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	secret := make([]byte, 32)
	wrongSecret := make([]byte, 32)
	wrongSecret[0] = 0xFF // different key

	_, privEnc, err := GenerateActorKeyPair(secret)
	if err != nil {
		t.Fatalf("GenerateActorKeyPair error: %v", err)
	}

	_, err = DecryptPrivateKey(privEnc, wrongSecret)
	if err == nil {
		t.Fatal("expected decryption error with wrong key")
	}
}

func TestDecryptTooShort(t *testing.T) {
	secret := make([]byte, 32)
	_, err := DecryptPrivateKey([]byte("short"), secret)
	if err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}
