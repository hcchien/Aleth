// Package keys handles RSA key pair generation and AES-256-GCM encryption
// of actor private keys at rest. Each Aleth user's ActivityPub signing key
// is distinct from their passkey and is held entirely by the platform.
package keys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
)

// GenerateActorKeyPair creates a fresh RSA-2048 key pair for an actor.
// It returns:
//   - pubPEM: PKIX public key in PEM format ("BEGIN PUBLIC KEY")
//   - privEnc: AES-256-GCM ciphertext of the PKCS#8 private key PEM
//   - err
func GenerateActorKeyPair(platformSecret []byte) (pubPEM string, privEnc []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", nil, fmt.Errorf("generate rsa key: %w", err)
	}

	// Marshal private key → PKCS#8 DER → PEM
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", nil, fmt.Errorf("marshal private key: %w", err)
	}
	privPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	// Marshal public key → PKIX DER → PEM
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", nil, fmt.Errorf("marshal public key: %w", err)
	}
	pubPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	// Encrypt private key PEM with AES-256-GCM
	enc, err := aesgcmEncrypt(platformSecret, privPEMBytes)
	if err != nil {
		return "", nil, fmt.Errorf("encrypt private key: %w", err)
	}

	return string(pubPEMBytes), enc, nil
}

// DecryptPrivateKey decrypts an AES-256-GCM ciphertext produced by
// GenerateActorKeyPair and returns the RSA private key.
func DecryptPrivateKey(privEnc []byte, platformSecret []byte) (*rsa.PrivateKey, error) {
	privPEMBytes, err := aesgcmDecrypt(platformSecret, privEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}
	block, _ := pem.Decode(privPEMBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return rsaKey, nil
}

// ─── AES-256-GCM helpers ──────────────────────────────────────────────────────

// aesgcmEncrypt encrypts plaintext with AES-256-GCM using a random nonce.
// Output layout: [12-byte nonce][ciphertext+tag].
func aesgcmEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, len(nonce)+len(ct))
	copy(out, nonce)
	copy(out[len(nonce):], ct)
	return out, nil
}

// aesgcmDecrypt reverses aesgcmEncrypt.
func aesgcmDecrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := data[:ns], data[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}
