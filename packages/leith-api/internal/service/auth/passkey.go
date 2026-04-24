package auth

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Config holds the webauthn configuration settings
type Config struct {
	RPDisplayName string
	RPID          string
	RPOrigins     []string
}

// Service handles Passkey operations and DID derivations
type Service struct {
	wa *webauthn.WebAuthn
}

// NewService initializes the passkey service
func NewService(cfg Config) (*Service, error) {
	wconfig := &webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
	}

	wa, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init webauthn: %w", err)
	}

	return &Service{wa: wa}, nil
}

func (s *Service) BeginRegistration(user *User) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	return s.wa.BeginRegistration(user)
}

func (s *Service) FinishRegistration(user *User, session webauthn.SessionData, req *http.Request) (*webauthn.Credential, error) {
	return s.wa.FinishRegistration(user, session, req)
}

func (s *Service) BeginLogin(user *User) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	return s.wa.BeginLogin(user)
}

func (s *Service) FinishLogin(user *User, session webauthn.SessionData, req *http.Request) (*webauthn.Credential, error) {
	return s.wa.FinishLogin(user, session, req)
}

// GenerateDID derived the unique DID for a Public Key (Ed25519)
// For the MVP, we assume the pubkey is either provided as raw bytes or we parse it
// from the webauthn response.
// Format: did:vflow:{hex(pubkey)}
func GenerateDID(pubKey ed25519.PublicKey) string {
	hexKey := hex.EncodeToString(pubKey)
	return fmt.Sprintf("did:vflow:%s", hexKey)
}

// ParseDID extracts the hex public key from a DID string
func ParseDID(did string) (string, error) {
	const prefix = "did:vflow:"
	if len(did) <= len(prefix) || did[:len(prefix)] != prefix {
		return "", fmt.Errorf("invalid DID format")
	}
	return did[len(prefix):], nil
}

// User is a stub for the webauthn.User interface
type User struct {
	ID          []byte
	DisplayName string
	Name        string
	Credentials []webauthn.Credential
}

// Implementation of webauthn.User
func (u *User) WebAuthnID() []byte {
	return u.ID
}

func (u *User) WebAuthnName() string {
	return u.Name
}

func (u *User) WebAuthnDisplayName() string {
	return u.DisplayName
}

func (u *User) WebAuthnIcon() string {
	return ""
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}
