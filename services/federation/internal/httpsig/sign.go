// Package httpsig implements HTTP Signature signing (draft-cavage-http-signatures-12)
// for outbound ActivityPub delivery requests. Phase 2 wires the signer;
// Phase 3 will exercise it when posting to remote inboxes.
package httpsig

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SignRequest signs an outbound HTTP request with the actor's RSA private key
// using the cavage-12 HTTP Signatures scheme.
//
// Signed headers: (request-target), host, date.
// For POST requests a Digest header (SHA-256 of the body) is expected to
// already be set on the request and is included in the signature.
//
// keyID should be the full key URI, e.g.
//
//	"https://aleth.social/@alice#main-key"
func SignRequest(req *http.Request, keyID string, privKey *rsa.PrivateKey) error {
	// Ensure Date is set.
	if req.Header.Get("Date") == "" {
		req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}

	// Build the list of headers to sign.
	headerNames := []string{"(request-target)", "host", "date"}
	if req.Header.Get("Digest") != "" {
		headerNames = append(headerNames, "digest")
	}

	// Build the signing string.
	signingParts := make([]string, 0, len(headerNames))
	for _, h := range headerNames {
		var val string
		switch h {
		case "(request-target)":
			path := req.URL.RequestURI()
			val = fmt.Sprintf("(request-target): %s %s", strings.ToLower(req.Method), path)
		case "host":
			host := req.Host
			if host == "" {
				host = req.URL.Host
			}
			val = "host: " + host
		default:
			val = strings.ToLower(h) + ": " + req.Header.Get(h)
		}
		signingParts = append(signingParts, val)
	}
	signingString := strings.Join(signingParts, "\n")

	// Hash and sign.
	h := sha256.New()
	h.Write([]byte(signingString))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, digest)
	if err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	sigB64 := base64.StdEncoding.EncodeToString(sig)
	headerList := strings.Join(headerNames, " ")

	sigHeader := fmt.Sprintf(
		`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
		keyID, headerList, sigB64,
	)
	req.Header.Set("Signature", sigHeader)
	return nil
}
