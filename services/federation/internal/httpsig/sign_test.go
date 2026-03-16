package httpsig

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"strings"
	"testing"
)

func TestSignRequest(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key gen: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://remote.example/inbox", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Digest", "SHA-256=abc=")

	keyID := "https://example.com/@alice#main-key"
	if err := SignRequest(req, keyID, privKey); err != nil {
		t.Fatalf("SignRequest error: %v", err)
	}

	sig := req.Header.Get("Signature")
	if sig == "" {
		t.Fatal("expected Signature header")
	}
	if !strings.Contains(sig, `keyId="`+keyID+`"`) {
		t.Fatalf("signature missing keyId: %v", sig)
	}
	if req.Header.Get("Date") == "" {
		t.Fatal("expected Date header to be set")
	}
}
