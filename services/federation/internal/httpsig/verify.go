package httpsig

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// VerifyRequest verifies the HTTP Signature on an incoming request using the
// cavage-12 scheme (same scheme used for signing outbound requests).
//
// fetchKey is called with the keyId from the Signature header and must return
// the corresponding RSA public key. It is the caller's responsibility to cache
// the result to avoid repeated network fetches.
func VerifyRequest(r *http.Request, fetchKey func(keyID string) (*rsa.PublicKey, error)) error {
	sigHeader := r.Header.Get("Signature")
	if sigHeader == "" {
		return fmt.Errorf("missing Signature header")
	}

	params := parseSignatureHeader(sigHeader)
	keyID := params["keyId"]
	headersParam := params["headers"]
	sigB64 := params["signature"]

	if keyID == "" || sigB64 == "" {
		return fmt.Errorf("invalid Signature header: missing keyId or signature")
	}

	pubKey, err := fetchKey(keyID)
	if err != nil {
		return fmt.Errorf("fetch public key %q: %w", keyID, err)
	}

	// Build the list of header names to verify. Fall back to just "date" when
	// the Signature header omits the headers parameter (non-standard but seen
	// in some implementations).
	headerNames := []string{"date"}
	if headersParam != "" {
		headerNames = strings.Fields(headersParam)
	}

	// Reconstruct the signing string exactly as the sender built it.
	signingParts := make([]string, 0, len(headerNames))
	for _, h := range headerNames {
		var val string
		switch h {
		case "(request-target)":
			val = fmt.Sprintf("(request-target): %s %s",
				strings.ToLower(r.Method), r.URL.RequestURI())
		case "host":
			host := r.Host
			if host == "" {
				host = r.URL.Host
			}
			val = "host: " + host
		default:
			val = strings.ToLower(h) + ": " + r.Header.Get(h)
		}
		signingParts = append(signingParts, val)
	}
	signingString := strings.Join(signingParts, "\n")

	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(signingString))
	digest := h.Sum(nil)

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest, sig); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}

// parseSignatureHeader parses the cavage HTTP Signature header into a
// key→value map.  Values are unquoted.
//
// Example input:
//
//	keyId="https://example.com/users/alice#main-key",algorithm="rsa-sha256",headers="(request-target) host date digest",signature="base64..."
func parseSignatureHeader(header string) map[string]string {
	out := make(map[string]string)
	// We can't simply split on "," because the signature value itself is
	// base64 and may technically contain "+" but not "," — however we split
	// carefully: only split on commas that are immediately followed by a word
	// character (the next key name). A simpler approach that works in practice:
	// find each key= token and extract until the next key= or end of string.
	rest := header
	for rest != "" {
		// Find next key name (letters only before '=').
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(rest[:eq])
		rest = rest[eq+1:]

		var val string
		if strings.HasPrefix(rest, `"`) {
			// Quoted value — find closing quote (not escaped).
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				break
			}
			val = rest[1 : end+1]
			rest = rest[end+2:] // skip closing quote
			// Skip optional comma separator.
			if strings.HasPrefix(rest, ",") {
				rest = rest[1:]
			}
		} else {
			// Unquoted — read until comma.
			comma := strings.IndexByte(rest, ',')
			if comma < 0 {
				val = rest
				rest = ""
			} else {
				val = rest[:comma]
				rest = rest[comma+1:]
			}
		}
		out[key] = val
	}
	return out
}
