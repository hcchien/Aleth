package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leith/api/internal/api"
	"github.com/leith/api/internal/store"
)

func TestOAuthPostAndListFlow(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// 1) OAuth login/bootstrap
	oauthReq := api.OAuthRequest{
		Provider: "google",
		Token:    "integration-token-1",
	}
	var authResp api.AuthSuccess
	doJSON(t, http.MethodPost, srv.URL+"/auth/oauth", oauthReq, nil, http.StatusOK, &authResp)
	if authResp.Did == "" {
		t.Fatalf("expected non-empty did")
	}

	// 2) Create post as that OAuth user
	createReq := api.CreatePostRequest{
		Body:        "integration hello",
		MediaHashes: []string{"hash-a"},
		Timestamp:   int(time.Now().Unix()),
		AuthorDid:   authResp.Did,
		Signature:   "00", // OAuth/L0 bypass for MVP
	}
	headers := map[string]string{
		"X-Mock-OAuth-Token": oauthReq.Token,
	}
	var created api.Post
	doJSON(t, http.MethodPost, srv.URL+"/posts", createReq, headers, http.StatusCreated, &created)
	if created.Id == "" {
		t.Fatalf("expected created post id")
	}
	if created.AuthorDid != authResp.Did {
		t.Fatalf("created author mismatch: %s", created.AuthorDid)
	}

	// 3) Read public feed and verify the post appears
	var feed []api.Post
	doJSON(t, http.MethodGet, srv.URL+"/posts?limit=20&offset=0", nil, headers, http.StatusOK, &feed)
	if len(feed) == 0 {
		t.Fatalf("expected at least one post in feed")
	}
	found := false
	for _, p := range feed {
		if p.Id == created.Id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created post id=%s not found in feed", created.Id)
	}
}

func TestPasskeyOptionsEndpoints(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	var registerOptions map[string]any
	doJSON(t, http.MethodGet, srv.URL+"/auth/register/options", nil, nil, http.StatusOK, &registerOptions)
	if registerOptions["sessionId"] == "" {
		t.Fatalf("expected registration session id")
	}

	doJSON(t, http.MethodGet, srv.URL+"/auth/login/options?did=did:vflow:missing", nil, nil, http.StatusNotFound, nil)
}

func TestV2ContentAndDiscussionFlow(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	headers := map[string]string{
		"X-Mock-DID": "did:vflow:integration-user-1",
	}

	createReq := map[string]any{
		"mode":                "discussion",
		"title":               "Should identity shape discourse?",
		"body":                "Certified humans should have higher discourse weight.",
		"visibility":          "public",
		"participationPolicy": "debate",
		"discussionShape":     "thread",
	}

	var created api.ContentItem
	doJSON(t, http.MethodPost, srv.URL+"/v2/content-items", createReq, headers, http.StatusCreated, &created)
	if created.Id == "" {
		t.Fatalf("expected created content item id")
	}
	if created.Mode != "discussion" {
		t.Fatalf("expected discussion mode, got %s", created.Mode)
	}

	var listed []api.ContentItem
	doJSON(t, http.MethodGet, srv.URL+"/v2/content-items?mode=discussion", nil, headers, http.StatusOK, &listed)
	if len(listed) == 0 {
		t.Fatalf("expected at least one discussion content item")
	}

	nodeReq := map[string]any{
		"nodeType": "rebuttal",
		"stance":   "oppose",
		"body":     "Identity should influence accountability, not truth itself.",
	}

	var createdNode api.DiscussionNode
	doJSON(t, http.MethodPost, srv.URL+"/v2/discussions/"+created.Id+"/nodes", nodeReq, headers, http.StatusCreated, &createdNode)
	if createdNode.Id == "" {
		t.Fatalf("expected discussion node id")
	}

	var nodes struct {
		DiscussionId string               `json:"discussionId"`
		Nodes        []api.DiscussionNode `json:"nodes"`
	}
	doJSON(t, http.MethodGet, srv.URL+"/v2/discussions/"+created.Id+"/nodes", nil, headers, http.StatusOK, &nodes)
	if len(nodes.Nodes) != 1 {
		t.Fatalf("expected exactly one node, got %d", len(nodes.Nodes))
	}
}

func TestV2ProjectionFlow(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	headers := map[string]string{
		"X-Mock-DID": "did:vflow:projection-user-1",
	}

	ideaReq := map[string]any{
		"mode":       "idea",
		"title":      "My structured viewpoint",
		"body":       "Longer authored argument that remains mine until projected.",
		"visibility": "private",
	}

	var createdIdea api.ContentItem
	doJSON(t, http.MethodPost, srv.URL+"/v2/content-items", ideaReq, headers, http.StatusCreated, &createdIdea)
	if createdIdea.Mode != "idea" {
		t.Fatalf("expected idea mode, got %s", createdIdea.Mode)
	}

	projectionReq := map[string]any{
		"sourceIdeaId":                  createdIdea.Id,
		"projectedExcerpt":              "Public thesis extracted from the idea.",
		"participationPolicy":           "debate",
		"ownershipTransferAcknowledged": true,
		"discussionShape":               "thread",
	}

	var projection api.Projection
	doJSON(t, http.MethodPost, srv.URL+"/v2/projections", projectionReq, headers, http.StatusCreated, &projection)
	if projection.TargetDiscussionId == "" {
		t.Fatalf("expected target discussion id")
	}

	var createdDiscussion api.ContentItem
	doJSON(t, http.MethodGet, srv.URL+"/v2/content-items/"+projection.TargetDiscussionId, nil, headers, http.StatusOK, &createdDiscussion)
	if createdDiscussion.Mode != "discussion" {
		t.Fatalf("expected projected discussion mode, got %s", createdDiscussion.Mode)
	}
}

func TestV2TransformationFlow(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	headers := map[string]string{
		"X-Mock-DID": "did:vflow:transform-user-1",
	}

	murmurReq := map[string]any{
		"mode":       "murmur",
		"body":       "People need ways to evolve a raw intuition into a stronger public argument.",
		"visibility": "private",
	}

	var murmur api.ContentItem
	doJSON(t, http.MethodPost, srv.URL+"/v2/content-items", murmurReq, headers, http.StatusCreated, &murmur)
	if murmur.Id == "" {
		t.Fatalf("expected murmur id")
	}

	jobReq := map[string]any{
		"sourceContentIds": []string{murmur.Id},
		"targetMode":       "idea",
		"providerType":     "local_llm",
		"promptProfile":    "researcher",
	}

	var job api.TransformationJob
	doJSON(t, http.MethodPost, srv.URL+"/v2/transformation-jobs", jobReq, headers, http.StatusCreated, &job)
	if job.Status != "completed" {
		t.Fatalf("expected completed job, got %s", job.Status)
	}
	if job.OutputBody == nil || *job.OutputBody == "" {
		t.Fatalf("expected generated output body")
	}

	var loaded api.TransformationJob
	doJSON(t, http.MethodGet, srv.URL+"/v2/transformation-jobs/"+job.Id, nil, headers, http.StatusOK, &loaded)
	if loaded.Id != job.Id {
		t.Fatalf("expected job id %s, got %s", job.Id, loaded.Id)
	}

	var published api.ContentItem
	doJSON(t, http.MethodPost, srv.URL+"/v2/transformation-jobs/"+job.Id+"/publish", nil, headers, http.StatusCreated, &published)
	if published.Mode != "idea" {
		t.Fatalf("expected published idea, got %s", published.Mode)
	}
}

func TestV2GovernanceFlow(t *testing.T) {
	handler := newHandler(store.NewMemoryStore())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	authorHeaders := map[string]string{
		"X-Mock-DID":        "did:vflow:gov-author",
		"X-Mock-Trust-Tier": "1",
	}

	var discussion api.ContentItem
	doJSON(t, http.MethodPost, srv.URL+"/v2/content-items", map[string]any{
		"mode":                "discussion",
		"title":               "Governance target",
		"body":                "This is a public discussion that can be forked or flagged.",
		"visibility":          "public",
		"participationPolicy": "debate",
		"discussionShape":     "thread",
	}, authorHeaders, http.StatusCreated, &discussion)

	l1Headers := map[string]string{
		"X-Mock-DID":        "did:vflow:l1-user",
		"X-Mock-Trust-Tier": "1",
	}
	doJSON(t, http.MethodPost, srv.URL+"/v2/discussions/"+discussion.Id+"/forks", map[string]any{
		"reason": "I want to fork this but should be blocked at L1",
	}, l1Headers, http.StatusForbidden, nil)

	l2Headers := map[string]string{
		"X-Mock-DID":        "did:vflow:l2-user",
		"X-Mock-Trust-Tier": "2",
	}
	var fork api.DiscussionFork
	doJSON(t, http.MethodPost, srv.URL+"/v2/discussions/"+discussion.Id+"/forks", map[string]any{
		"reason": "I want to branch the argument into a parallel debate.",
	}, l2Headers, http.StatusCreated, &fork)
	if fork.ForkDiscussionId == "" {
		t.Fatalf("expected fork discussion id")
	}

	var flag api.ModerationAction
	doJSON(t, http.MethodPost, srv.URL+"/v2/moderation-actions", map[string]any{
		"targetContentId": discussion.Id,
		"actionType":      "flag",
		"reason":          "Needs community review",
	}, l2Headers, http.StatusCreated, &flag)
	if flag.ActionType != "flag" {
		t.Fatalf("expected flag action")
	}

	doJSON(t, http.MethodPost, srv.URL+"/v2/moderation-actions", map[string]any{
		"targetContentId": discussion.Id,
		"actionType":      "slash",
		"reason":          "Should fail for L2",
	}, l2Headers, http.StatusForbidden, nil)

	l4Headers := map[string]string{
		"X-Mock-DID":        "did:vflow:l4-user",
		"X-Mock-Trust-Tier": "4",
	}
	var slash api.ModerationAction
	doJSON(t, http.MethodPost, srv.URL+"/v2/moderation-actions", map[string]any{
		"targetContentId": discussion.Id,
		"actionType":      "slash",
		"reason":          "Severe governance action",
	}, l4Headers, http.StatusCreated, &slash)
	if slash.ActionType != "slash" {
		t.Fatalf("expected slash action")
	}
}

func TestV2TrustVerifierFlow(t *testing.T) {
	db := store.NewMemoryStore()
	verifierDID := "did:vflow:l4-verifier-test"
	if err := db.CreateUser(&store.User{
		DID:       verifierDID,
		TrustTier: store.L4_AUTHORITY,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier user: %v", err)
	}
	if err := db.CreateVerifier(&store.Verifier{
		ID:             "vrf_test",
		VerifierDID:    verifierDID,
		VerifierType:   "test",
		Scope:          "global",
		AuthorityLevel: store.L4_AUTHORITY,
		Status:         store.VerifierActive,
		AppointedByDID: "system",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	handler := newHandler(db)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	subjectHeaders := map[string]string{
		"X-Mock-DID": "did:vflow:l2-subject-test",
	}
	caseReq := map[string]any{
		"requestedTier":  2,
		"credentialType": "social_vouch",
		"evidenceJson":   `{"vouches":["did:vflow:friend-a"],"note":"integration test"}`,
	}
	var createdCase api.VerificationCase
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/verification-cases", caseReq, subjectHeaders, http.StatusCreated, &createdCase)
	if createdCase.Id == "" || createdCase.AssignedVerifierDid == nil || *createdCase.AssignedVerifierDid != verifierDID {
		t.Fatalf("expected case assigned to bootstrap verifier, got %+v", createdCase)
	}

	verifierHeaders := map[string]string{
		"X-Mock-DID": verifierDID,
	}
	var queue []api.VerificationCase
	doJSON(t, http.MethodGet, srv.URL+"/v2/trust/verifier/cases", nil, verifierHeaders, http.StatusOK, &queue)
	if len(queue) != 1 {
		t.Fatalf("expected one verifier case, got %d", len(queue))
	}

	decisionReq := map[string]any{
		"decision":                 "approve",
		"reason":                   "evidence satisfies prototype L2 social vouching policy",
		"credentialIssuanceSource": "internal_verifier_issued",
	}
	var decision api.VerifierDecision
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/verifier/cases/"+createdCase.Id+"/decision", decisionReq, verifierHeaders, http.StatusCreated, &decision)
	if decision.IssuedAssessmentId == nil {
		t.Fatalf("expected issued assessment id")
	}

	var profile api.TrustProfile
	doJSON(t, http.MethodGet, srv.URL+"/v2/trust/me", nil, subjectHeaders, http.StatusOK, &profile)
	if profile.Identity.TrustTier != int(store.L2_SOCIAL) {
		t.Fatalf("expected subject upgraded to L2, got L%d", profile.Identity.TrustTier)
	}
	if len(profile.TrustAssessments) != 1 {
		t.Fatalf("expected one trust assessment, got %d", len(profile.TrustAssessments))
	}
}

func TestTrustedIssuerExternalCredentialFlow(t *testing.T) {
	db := store.NewMemoryStore()
	verifierDID := "did:vflow:l4-verifier-external"
	if err := db.CreateUser(&store.User{
		DID:       verifierDID,
		TrustTier: store.L4_AUTHORITY,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier user: %v", err)
	}
	if err := db.CreateVerifier(&store.Verifier{
		ID:             "vrf_external",
		VerifierDID:    verifierDID,
		VerifierType:   "test",
		Scope:          "global",
		AuthorityLevel: store.L4_AUTHORITY,
		Status:         store.VerifierActive,
		AppointedByDID: "system",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	handler := newHandler(db)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	verifierHeaders := map[string]string{"X-Mock-DID": verifierDID}
	createIssuerReq := map[string]any{
		"issuerDid":          "did:web:issuer.example",
		"issuerName":         "Issuer Example",
		"scopes":             []string{"professional"},
		"credentialTypes":    []string{"contribution_history"},
		"maxTrustTierIssued": 3,
		"status":             "active",
	}
	var issuerResp map[string]any
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/issuers", createIssuerReq, verifierHeaders, http.StatusCreated, &issuerResp)

	subjectHeaders := map[string]string{"X-Mock-DID": "did:vflow:l3-subject-external"}
	caseReq := map[string]any{
		"requestedTier":  3,
		"credentialType": "contribution_history",
		"evidenceJson":   `{"years":4,"portfolio":"https://example.com"}`,
	}
	var createdCase api.VerificationCase
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/verification-cases", caseReq, subjectHeaders, http.StatusCreated, &createdCase)

	decisionReq := map[string]any{
		"decision":                 "approve",
		"reason":                   "trusted external issuer evidence is accepted",
		"credentialIssuanceSource": "external_issuer_verified",
		"externalIssuerDid":        "did:web:issuer.example",
	}
	var decision api.VerifierDecision
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/verifier/cases/"+createdCase.Id+"/decision", decisionReq, verifierHeaders, http.StatusCreated, &decision)

	var profile api.TrustProfile
	doJSON(t, http.MethodGet, srv.URL+"/v2/trust/me", nil, subjectHeaders, http.StatusOK, &profile)
	if profile.Identity.TrustTier != int(store.L3_TEMPORAL) {
		t.Fatalf("expected subject upgraded to L3, got L%d", profile.Identity.TrustTier)
	}
	if len(profile.Credentials) != 1 {
		t.Fatalf("expected one credential, got %d", len(profile.Credentials))
	}
	if profile.Credentials[0].IssuanceSource != "external_issuer_verified" {
		t.Fatalf("expected external credential issuance source, got %s", profile.Credentials[0].IssuanceSource)
	}
}

func TestWalletPresentationVerificationFlow(t *testing.T) {
	db := store.NewMemoryStore()
	verifierDID := "did:vflow:l4-wallet-verifier"
	if err := db.CreateUser(&store.User{
		DID:       verifierDID,
		TrustTier: store.L4_AUTHORITY,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier user: %v", err)
	}
	if err := db.CreateVerifier(&store.Verifier{
		ID:             "vrf_wallet",
		VerifierDID:    verifierDID,
		VerifierType:   "test",
		Scope:          "global",
		AuthorityLevel: store.L4_AUTHORITY,
		Status:         store.VerifierActive,
		AppointedByDID: "system",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	handler := newHandler(db)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	verifierHeaders := map[string]string{"X-Mock-DID": verifierDID}
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/issuers", map[string]any{
		"issuerDid":          "did:web:wallet-issuer.example",
		"issuerName":         "Wallet Issuer",
		"scopes":             []string{"professional"},
		"credentialTypes":    []string{"organization_membership"},
		"maxTrustTierIssued": 3,
		"status":             "active",
	}, verifierHeaders, http.StatusCreated, &map[string]any{})

	subjectHeaders := map[string]string{"X-Mock-DID": "did:vflow:wallet-subject"}
	var request api.WalletPresentationRequest
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/presentation-requests", map[string]any{
		"requestedTier":     3,
		"credentialType":    "organization_membership",
		"purpose":           "Wallet verification integration test",
		"allowedIssuerDids": []string{"did:web:wallet-issuer.example"},
	}, subjectHeaders, http.StatusCreated, &request)
	if request.Id == "" || request.Challenge == "" || request.RequestUri == "" {
		t.Fatalf("expected wallet request challenge and request uri, got %+v", request)
	}

	var verification api.WalletPresentationVerification
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/presentation-requests/"+request.Id+"/complete", map[string]any{
		"issuerDid":          "did:web:wallet-issuer.example",
		"credentialType":     "organization_membership",
		"presentationFormat": "mock_wallet",
		"claimsJson":         `{"memberId":"org-42","role":"researcher"}`,
		"proof":              "wallet-signature-placeholder",
		"audience":           request.RequestUri,
		"nonce":              request.Challenge,
	}, subjectHeaders, http.StatusCreated, &verification)
	if verification.Status != api.Verified {
		t.Fatalf("expected verified wallet presentation, got %+v", verification)
	}
	if verification.IssuedAssessmentId == nil || verification.IssuedCredentialId == nil {
		t.Fatalf("expected issued assessment and credential ids, got %+v", verification)
	}

	var profile api.TrustProfile
	doJSON(t, http.MethodGet, srv.URL+"/v2/trust/me", nil, subjectHeaders, http.StatusOK, &profile)
	if profile.Identity.TrustTier != int(store.L3_TEMPORAL) {
		t.Fatalf("expected subject upgraded to L3, got L%d", profile.Identity.TrustTier)
	}
}

func TestWalletPresentationRejectsNonceMismatch(t *testing.T) {
	db := store.NewMemoryStore()
	verifierDID := "did:vflow:l4-wallet-verifier-reject"
	if err := db.CreateUser(&store.User{
		DID:       verifierDID,
		TrustTier: store.L4_AUTHORITY,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier user: %v", err)
	}
	if err := db.CreateVerifier(&store.Verifier{
		ID:             "vrf_wallet_reject",
		VerifierDID:    verifierDID,
		VerifierType:   "test",
		Scope:          "global",
		AuthorityLevel: store.L4_AUTHORITY,
		Status:         store.VerifierActive,
		AppointedByDID: "system",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	handler := newHandler(db)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	verifierHeaders := map[string]string{"X-Mock-DID": verifierDID}
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/issuers", map[string]any{
		"issuerDid":          "did:web:wallet-issuer-reject.example",
		"issuerName":         "Wallet Issuer Reject",
		"scopes":             []string{"professional"},
		"credentialTypes":    []string{"organization_membership"},
		"maxTrustTierIssued": 3,
		"status":             "active",
	}, verifierHeaders, http.StatusCreated, &map[string]any{})

	subjectHeaders := map[string]string{"X-Mock-DID": "did:vflow:wallet-subject-reject"}
	var request api.WalletPresentationRequest
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/presentation-requests", map[string]any{
		"requestedTier":  3,
		"credentialType": "organization_membership",
	}, subjectHeaders, http.StatusCreated, &request)

	var verification api.WalletPresentationVerification
	doJSON(t, http.MethodPost, srv.URL+"/v2/trust/presentation-requests/"+request.Id+"/complete", map[string]any{
		"issuerDid":          "did:web:wallet-issuer-reject.example",
		"credentialType":     "organization_membership",
		"presentationFormat": "mock_wallet",
		"claimsJson":         `{"memberId":"org-9"}`,
		"proof":              "wallet-signature-placeholder",
		"audience":           request.RequestUri,
		"nonce":              "wrong-nonce",
	}, subjectHeaders, http.StatusCreated, &verification)
	if verification.Status != api.Rejected {
		t.Fatalf("expected rejected wallet presentation, got %+v", verification)
	}

	var profile api.TrustProfile
	doJSON(t, http.MethodGet, srv.URL+"/v2/trust/me", nil, subjectHeaders, http.StatusOK, &profile)
	if profile.Identity.TrustTier != int(store.L1_DEVICE) {
		t.Fatalf("expected subject to remain L1, got L%d", profile.Identity.TrustTier)
	}
}

func doJSON(t *testing.T, method, url string, payload any, headers map[string]string, wantStatus int, out any) {
	t.Helper()

	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s expected status %d, got %d", method, url, wantStatus, resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}
