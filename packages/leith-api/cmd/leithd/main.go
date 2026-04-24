package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	middleware_chi "github.com/go-chi/chi/v5/middleware"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/leith/api/internal/api"
	"github.com/leith/api/internal/middleware"
	vfauth "github.com/leith/api/internal/service/auth"
	"github.com/leith/api/internal/service/content"
	"github.com/leith/api/internal/service/reputation"
	"github.com/leith/api/internal/store"
)

// Server implements the generated api.ServerInterface
type Server struct {
	api.Unimplemented
	store      store.Store
	contentSvc *content.Service
	repSvc     *reputation.Service
	passkeySvc *vfauth.Service

	sessionMu            sync.Mutex
	registrationSessions map[string]passkeyRegistrationSession
	loginSessions        map[string]passkeyLoginSession
}

type passkeyRegistrationSession struct {
	User    *vfauth.User
	Session webauthn.SessionData
}

type passkeyLoginSession struct {
	DID     string
	Session webauthn.SessionData
}

// Handle L0 Guest Sign In via OAuth
func (s *Server) PostAuthOauth(w http.ResponseWriter, r *http.Request) {
	var req api.OAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Provider == "" || req.Token == "" {
		http.Error(w, "provider and token are required", http.StatusBadRequest)
		return
	}

	// Mock DID derivation from provider/token.
	did := fmt.Sprintf("oauth:%s:%s", req.Provider, req.Token)
	did = strings.ReplaceAll(did, " ", "")

	if err := s.store.CreateUser(&store.User{
		DID:       did,
		OAuthID:   req.Token,
		TrustTier: store.L0_GUEST,
	}); err != nil {
		http.Error(w, "failed to provision oauth user", http.StatusInternalServerError)
		return
	}

	// Parse the OAuthRequest (e.g. from Google/Apple)
	// Verify the JWT signature using the provider's JWKS

	// Mock: Assign an L0 Guest Session Token
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.AuthSuccess{
		Did:       did,
		TrustTier: int(store.L0_GUEST),
	})
}

func (s *Server) GetAuthRegisterOptions(w http.ResponseWriter, r *http.Request) {
	if s.passkeySvc == nil {
		http.Error(w, "passkey service unavailable", http.StatusInternalServerError)
		return
	}

	userID := randomBytes(32)
	if len(userID) == 0 {
		http.Error(w, "failed to generate passkey session", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC().UnixNano()
	user := &vfauth.User{
		ID:          userID,
		Name:        fmt.Sprintf("passkey-%d", now),
		DisplayName: "Passkey User",
	}
	options, session, err := s.passkeySvc.BeginRegistration(user)
	if err != nil {
		http.Error(w, "failed to start passkey registration", http.StatusInternalServerError)
		return
	}

	sessionID := generateID("reg")
	s.sessionMu.Lock()
	s.registrationSessions[sessionID] = passkeyRegistrationSession{
		User:    user,
		Session: *session,
	}
	s.sessionMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sessionID,
		"publicKey": options.Response,
	})
}

func (s *Server) PostAuthRegister(w http.ResponseWriter, r *http.Request) {
	if s.passkeySvc == nil {
		http.Error(w, "passkey service unavailable", http.StatusInternalServerError)
		return
	}

	sessionID, credentialBody, err := extractCeremonyPayload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.sessionMu.Lock()
	pending, ok := s.registrationSessions[sessionID]
	if ok {
		delete(s.registrationSessions, sessionID)
	}
	s.sessionMu.Unlock()
	if !ok {
		http.Error(w, "registration session not found", http.StatusBadRequest)
		return
	}

	credential, err := s.passkeySvc.FinishRegistration(pending.User, pending.Session, requestWithJSONBody(r, credentialBody))
	if err != nil {
		http.Error(w, fmt.Sprintf("passkey registration failed: %v", err), http.StatusBadRequest)
		return
	}

	did := derivePasskeyDID(credential.ID)
	credentialsJSON, err := json.Marshal([]webauthn.Credential{*credential})
	if err != nil {
		http.Error(w, "failed to persist passkey credential", http.StatusInternalServerError)
		return
	}

	user := &store.User{
		DID:                did,
		PublicKey:          append([]byte(nil), credential.PublicKey...),
		AuthnUserID:        append([]byte(nil), pending.User.ID...),
		PasskeyCredentials: credentialsJSON,
		TrustTier:          store.L1_DEVICE,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.store.CreateUser(user); err != nil {
		http.Error(w, "failed to save passkey user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, api.AuthSuccess{
		Did:       did,
		TrustTier: int(store.L1_DEVICE),
	})
}

func (s *Server) GetAuthLoginOptions(w http.ResponseWriter, r *http.Request) {
	if s.passkeySvc == nil {
		http.Error(w, "passkey service unavailable", http.StatusInternalServerError)
		return
	}

	did := strings.TrimSpace(r.URL.Query().Get("did"))
	if did == "" {
		http.Error(w, "did query parameter is required", http.StatusBadRequest)
		return
	}

	storeUser, err := s.store.GetUserByDID(did)
	if err != nil {
		http.Error(w, "failed to load passkey user", http.StatusInternalServerError)
		return
	}
	if storeUser == nil || len(storeUser.PasskeyCredentials) == 0 || len(storeUser.AuthnUserID) == 0 {
		http.Error(w, "passkey user not found", http.StatusNotFound)
		return
	}

	passkeyUser, err := webauthnUserFromStore(*storeUser)
	if err != nil {
		http.Error(w, "failed to load passkey credentials", http.StatusInternalServerError)
		return
	}

	options, session, err := s.passkeySvc.BeginLogin(passkeyUser)
	if err != nil {
		http.Error(w, "failed to start passkey login", http.StatusInternalServerError)
		return
	}

	sessionID := generateID("log")
	s.sessionMu.Lock()
	s.loginSessions[sessionID] = passkeyLoginSession{
		DID:     did,
		Session: *session,
	}
	s.sessionMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sessionID,
		"did":       did,
		"publicKey": options.Response,
	})
}

func (s *Server) PostAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.passkeySvc == nil {
		http.Error(w, "passkey service unavailable", http.StatusInternalServerError)
		return
	}

	sessionID, credentialBody, err := extractCeremonyPayload(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.sessionMu.Lock()
	pending, ok := s.loginSessions[sessionID]
	if ok {
		delete(s.loginSessions, sessionID)
	}
	s.sessionMu.Unlock()
	if !ok {
		http.Error(w, "login session not found", http.StatusBadRequest)
		return
	}

	storeUser, err := s.store.GetUserByDID(pending.DID)
	if err != nil {
		http.Error(w, "failed to load passkey user", http.StatusInternalServerError)
		return
	}
	if storeUser == nil {
		http.Error(w, "passkey user not found", http.StatusNotFound)
		return
	}

	passkeyUser, err := webauthnUserFromStore(*storeUser)
	if err != nil {
		http.Error(w, "failed to load passkey credentials", http.StatusInternalServerError)
		return
	}

	validatedCredential, err := s.passkeySvc.FinishLogin(passkeyUser, pending.Session, requestWithJSONBody(r, credentialBody))
	if err != nil {
		http.Error(w, fmt.Sprintf("passkey login failed: %v", err), http.StatusBadRequest)
		return
	}

	if err := updateStoredCredential(storeUser, validatedCredential); err != nil {
		http.Error(w, "failed to update passkey credential state", http.StatusInternalServerError)
		return
	}
	if err := s.store.CreateUser(storeUser); err != nil {
		http.Error(w, "failed to persist passkey user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, api.AuthSuccess{
		Did:       storeUser.DID,
		TrustTier: int(store.L1_DEVICE),
	})
}

func (s *Server) PostPosts(w http.ResponseWriter, r *http.Request) {
	var req api.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}
	if did != req.AuthorDid {
		http.Error(w, "authorDid must match authenticated DID", http.StatusForbidden)
		return
	}

	post, err := s.contentSvc.CreatePost(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPIPost(*post))
}

func (s *Server) GetV2Me(w http.ResponseWriter, r *http.Request) {
	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		writeJSON(w, http.StatusOK, api.MeResponse{
			Identity: api.Identity{
				Did:         "anonymous",
				DisplayName: "訪客",
				TrustTier:   int(store.L0_GUEST),
			},
			Capabilities: capabilitiesForTier(store.L0_GUEST),
		})
		return
	}

	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	authMethod := "passkey"
	if user.TrustTier == store.L0_GUEST {
		authMethod = "oauth"
	}

	capabilities := capabilitiesForTier(user.TrustTier)
	capabilities.CanReviewVerificationCases = s.isActiveVerifier(user.DID)

	writeJSON(w, http.StatusOK, api.MeResponse{
		Identity: api.Identity{
			Did:         user.DID,
			DisplayName: displayNameForDID(user.DID),
			TrustTier:   int(user.TrustTier),
			AuthMethod:  &authMethod,
		},
		Capabilities: capabilities,
	})
}

func (s *Server) GetV2ContentItems(w http.ResponseWriter, r *http.Request, params api.GetV2ContentItemsParams) {
	limit := 20
	offset := 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}

	var mode *store.ContentMode
	if params.Mode != nil && *params.Mode != "" {
		m := store.ContentMode(*params.Mode)
		mode = &m
	}
	var visibility *store.ContentVisibility
	if params.Visibility != nil && *params.Visibility != "" {
		v := store.ContentVisibility(*params.Visibility)
		visibility = &v
	}

	items, err := s.store.GetContentItems(mode, visibility, params.AuthorDid, limit, offset)
	if err != nil {
		http.Error(w, "failed to load content items", http.StatusInternalServerError)
		return
	}

	resp := make([]api.ContentItem, 0, len(items))
	for _, item := range items {
		resp = append(resp, toAPIContentItem(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) PostV2ContentItems(w http.ResponseWriter, r *http.Request) {
	var req api.CreateContentItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}

	user, err := s.store.GetUserByDID(did)
	if err != nil || user == nil {
		user, err = s.currentUserFromRequest(r)
		if err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}
	}

	mode := store.ContentMode(req.Mode)
	if mode != store.ModeMurmur && mode != store.ModeIdea && mode != store.ModeDiscussion {
		http.Error(w, "invalid content mode", http.StatusBadRequest)
		return
	}

	visibility := defaultVisibilityForMode(mode)
	if req.Visibility != nil && *req.Visibility != "" {
		visibility = store.ContentVisibility(*req.Visibility)
	}
	status := store.StatusDraft
	if mode == store.ModeDiscussion {
		status = store.StatusActive
	}

	item := &store.ContentItem{
		ID:         generateID("cnt"),
		AuthorDID:  user.DID,
		Body:       req.Body,
		Mode:       mode,
		Status:     status,
		Visibility: visibility,
		TrustTier:  user.TrustTier,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if req.Title != nil {
		item.Title = *req.Title
	}
	if req.ParticipationPolicy != nil {
		item.ParticipationPolicy = store.ParticipationPolicy(*req.ParticipationPolicy)
	}
	if req.DiscussionShape != nil {
		item.DiscussionShape = store.DiscussionShape(*req.DiscussionShape)
	}
	if req.SourceContentIds != nil {
		item.SourceContentIDs = append(item.SourceContentIDs, (*req.SourceContentIds)...)
	}
	if item.Mode == store.ModeDiscussion {
		if item.ParticipationPolicy == "" {
			item.ParticipationPolicy = store.ParticipationComment
		}
		if item.DiscussionShape == "" {
			item.DiscussionShape = store.DiscussionThread
		}
		now := time.Now().UTC()
		item.PublishedAt = &now
	}

	if err := s.store.CreateContentItem(item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, toAPIContentItem(*item))
}

func (s *Server) GetV2ContentItemsContentItemId(w http.ResponseWriter, r *http.Request, contentItemId string) {
	item, err := s.store.GetContentItemByID(contentItemId)
	if err != nil {
		http.Error(w, "failed to load content item", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "content item not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, toAPIContentItem(*item))
}

func (s *Server) GetV2DiscussionsDiscussionIdNodes(w http.ResponseWriter, r *http.Request, discussionId string) {
	item, err := s.store.GetContentItemByID(discussionId)
	if err != nil {
		http.Error(w, "failed to load discussion", http.StatusInternalServerError)
		return
	}
	if item == nil || item.Mode != store.ModeDiscussion {
		http.Error(w, "discussion not found", http.StatusNotFound)
		return
	}

	nodes, err := s.store.GetDiscussionNodes(discussionId)
	if err != nil {
		http.Error(w, "failed to load discussion nodes", http.StatusInternalServerError)
		return
	}

	resp := make([]api.DiscussionNode, 0, len(nodes))
	for _, node := range nodes {
		resp = append(resp, toAPIDiscussionNode(node))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"discussionId": discussionId,
		"nodes":        resp,
	})
}

func (s *Server) PostV2DiscussionsDiscussionIdNodes(w http.ResponseWriter, r *http.Request, discussionId string) {
	var req api.CreateDiscussionNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}

	item, err := s.store.GetContentItemByID(discussionId)
	if err != nil {
		http.Error(w, "failed to load discussion", http.StatusInternalServerError)
		return
	}
	if item == nil || item.Mode != store.ModeDiscussion {
		http.Error(w, "discussion not found", http.StatusNotFound)
		return
	}

	node := &store.DiscussionNode{
		ID:           generateID("node"),
		DiscussionID: discussionId,
		AuthorDID:    did,
		NodeType:     store.DiscussionNodeType(req.NodeType),
		Stance:       store.DiscussionStance(req.Stance),
		Body:         req.Body,
		CreatedAt:    time.Now().UTC(),
	}
	if req.ParentNodeId != nil && *req.ParentNodeId != "" {
		node.ParentNodeID = req.ParentNodeId
	}

	if err := s.store.CreateDiscussionNode(node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIDiscussionNode(*node))
}

func (s *Server) PostV2Projections(w http.ResponseWriter, r *http.Request) {
	var req api.CreateProjectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if !req.OwnershipTransferAcknowledged {
		http.Error(w, "ownership transfer acknowledgement is required", http.StatusBadRequest)
		return
	}

	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}

	sourceIdea, err := s.store.GetContentItemByID(req.SourceIdeaId)
	if err != nil {
		http.Error(w, "failed to load idea", http.StatusInternalServerError)
		return
	}
	if sourceIdea == nil || sourceIdea.Mode != store.ModeIdea {
		http.Error(w, "idea not found", http.StatusNotFound)
		return
	}
	if sourceIdea.AuthorDID != did {
		http.Error(w, "only the idea owner may project it", http.StatusForbidden)
		return
	}

	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	discussionShape := store.DiscussionThread
	if req.DiscussionShape != nil && *req.DiscussionShape != "" {
		discussionShape = store.DiscussionShape(*req.DiscussionShape)
	}
	participationPolicy := store.ParticipationPolicy(req.ParticipationPolicy)
	if participationPolicy == "" {
		participationPolicy = store.ParticipationDebate
	}

	now := time.Now().UTC()
	discussion := &store.ContentItem{
		ID:                  generateID("cnt"),
		AuthorDID:           did,
		Title:               sourceIdea.Title,
		Body:                req.ProjectedExcerpt,
		Mode:                store.ModeDiscussion,
		Status:              store.StatusActive,
		Visibility:          store.VisibilityPublic,
		TrustTier:           user.TrustTier,
		CreatedAt:           now,
		UpdatedAt:           now,
		PublishedAt:         &now,
		ParticipationPolicy: participationPolicy,
		DiscussionShape:     discussionShape,
		SourceContentIDs:    []string{sourceIdea.ID},
	}
	if err := s.store.CreateContentItem(discussion); err != nil {
		http.Error(w, "failed to create discussion", http.StatusBadRequest)
		return
	}

	projection := &store.Projection{
		ID:                            generateID("prj"),
		SourceIdeaID:                  sourceIdea.ID,
		TargetDiscussionID:            discussion.ID,
		ProjectedExcerpt:              req.ProjectedExcerpt,
		ParticipationPolicy:           participationPolicy,
		OwnershipTransferAcknowledged: req.OwnershipTransferAcknowledged,
		CreatedByDID:                  did,
		CreatedAt:                     now,
	}
	if err := s.store.CreateProjection(projection); err != nil {
		http.Error(w, "failed to create projection", http.StatusBadRequest)
		return
	}

	relation := &store.ContentRelation{
		ID:            generateID("rel"),
		FromContentID: sourceIdea.ID,
		ToContentID:   discussion.ID,
		RelationType:  store.RelationProjectedFrom,
		CreatedAt:     now,
	}
	if err := s.store.CreateContentRelation(relation); err != nil {
		http.Error(w, "failed to create content relation", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, toAPIProjection(*projection))
}

func (s *Server) PostV2TransformationJobs(w http.ResponseWriter, r *http.Request) {
	var req api.CreateTransformationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}
	if len(req.SourceContentIds) == 0 {
		http.Error(w, "at least one source content id is required", http.StatusBadRequest)
		return
	}
	targetMode := store.ContentMode(req.TargetMode)
	providerType := store.TransformationProviderType(req.ProviderType)
	if targetMode != store.ModeIdea && targetMode != store.ModeDiscussion && targetMode != store.ModeMurmur {
		http.Error(w, "invalid target mode", http.StatusBadRequest)
		return
	}

	sources := make([]store.ContentItem, 0, len(req.SourceContentIds))
	for _, sourceID := range req.SourceContentIds {
		item, err := s.store.GetContentItemByID(sourceID)
		if err != nil {
			http.Error(w, "failed to load source content", http.StatusInternalServerError)
			return
		}
		if item == nil {
			http.Error(w, "source content not found", http.StatusNotFound)
			return
		}
		if item.AuthorDID != did {
			http.Error(w, "source content must belong to the requesting user", http.StatusForbidden)
			return
		}
		sources = append(sources, *item)
	}

	outputTitle, outputBody := generateTransformationDraft(sources, targetMode, providerType, optionalString(req.PromptProfile))
	now := time.Now().UTC()
	job := &store.TransformationJob{
		ID:               generateID("trf"),
		RequestedByDID:   did,
		SourceContentIDs: append([]string(nil), req.SourceContentIds...),
		TargetMode:       targetMode,
		ProviderType:     providerType,
		PromptProfile:    optionalString(req.PromptProfile),
		Status:           store.TransformationCompleted,
		OutputTitle:      outputTitle,
		OutputBody:       outputBody,
		CreatedAt:        now,
		CompletedAt:      &now,
	}
	if err := s.store.CreateTransformationJob(job); err != nil {
		http.Error(w, "failed to create transformation job", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, toAPITransformationJob(*job))
}

func (s *Server) GetV2TransformationJobsJobId(w http.ResponseWriter, r *http.Request, jobId string) {
	job, err := s.store.GetTransformationJobByID(jobId)
	if err != nil {
		http.Error(w, "failed to load transformation job", http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "transformation job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, toAPITransformationJob(*job))
}

func (s *Server) PostV2TransformationJobsJobIdPublish(w http.ResponseWriter, r *http.Request, jobId string) {
	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}
	job, err := s.store.GetTransformationJobByID(jobId)
	if err != nil {
		http.Error(w, "failed to load transformation job", http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "transformation job not found", http.StatusNotFound)
		return
	}
	if job.RequestedByDID != did {
		http.Error(w, "only the requesting user may publish this job", http.StatusForbidden)
		return
	}
	if job.Status != store.TransformationCompleted {
		http.Error(w, "transformation job is not ready to publish", http.StatusBadRequest)
		return
	}
	user, err := s.ensureUserForDID(did)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	item := &store.ContentItem{
		ID:               generateID("cnt"),
		AuthorDID:        did,
		Title:            job.OutputTitle,
		Body:             job.OutputBody,
		Mode:             job.TargetMode,
		Status:           store.StatusDraft,
		Visibility:       defaultVisibilityForMode(job.TargetMode),
		TrustTier:        user.TrustTier,
		CreatedAt:        now,
		UpdatedAt:        now,
		SourceContentIDs: append([]string(nil), job.SourceContentIDs...),
	}
	if item.Mode == store.ModeDiscussion {
		item.Status = store.StatusActive
		item.Visibility = store.VisibilityPublic
		item.ParticipationPolicy = store.ParticipationDebate
		item.DiscussionShape = store.DiscussionThread
		item.PublishedAt = &now
	}
	if err := s.store.CreateContentItem(item); err != nil {
		http.Error(w, "failed to publish transformed content", http.StatusBadRequest)
		return
	}
	for _, sourceID := range job.SourceContentIDs {
		relation := &store.ContentRelation{
			ID:            generateID("rel"),
			FromContentID: sourceID,
			ToContentID:   item.ID,
			RelationType:  store.RelationExpandedFrom,
			CreatedAt:     now,
		}
		if err := s.store.CreateContentRelation(relation); err != nil {
			http.Error(w, "failed to create content relation", http.StatusBadRequest)
			return
		}
	}
	job.Status = store.TransformationPublished
	job.PublishedContentID = &item.ID
	if err := s.store.UpdateTransformationJob(job); err != nil {
		http.Error(w, "failed to update transformation job", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIContentItem(*item))
}

func (s *Server) PostV2DiscussionsDiscussionIdForks(w http.ResponseWriter, r *http.Request, discussionId string) {
	var req api.CreateDiscussionForkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	if user.TrustTier < store.L2_SOCIAL {
		http.Error(w, "L2 required to fork discussions", http.StatusForbidden)
		return
	}
	source, err := s.store.GetContentItemByID(discussionId)
	if err != nil {
		http.Error(w, "failed to load discussion", http.StatusInternalServerError)
		return
	}
	if source == nil || source.Mode != store.ModeDiscussion {
		http.Error(w, "discussion not found", http.StatusNotFound)
		return
	}
	now := time.Now().UTC()
	forkedDiscussion := &store.ContentItem{
		ID:                  generateID("cnt"),
		AuthorDID:           did,
		Title:               "Fork: " + source.Title,
		Body:                req.Reason + "\n\n---\n\n" + source.Body,
		Mode:                store.ModeDiscussion,
		Status:              store.StatusActive,
		Visibility:          store.VisibilityPublic,
		TrustTier:           user.TrustTier,
		CreatedAt:           now,
		UpdatedAt:           now,
		PublishedAt:         &now,
		ParticipationPolicy: store.ParticipationDebate,
		DiscussionShape:     source.DiscussionShape,
		SourceContentIDs:    []string{source.ID},
	}
	if forkedDiscussion.Title == "Fork: " {
		forkedDiscussion.Title = "Forked Discussion"
	}
	if forkedDiscussion.DiscussionShape == "" {
		forkedDiscussion.DiscussionShape = store.DiscussionThread
	}
	if err := s.store.CreateContentItem(forkedDiscussion); err != nil {
		http.Error(w, "failed to create fork discussion", http.StatusBadRequest)
		return
	}
	fork := &store.DiscussionFork{
		ID:                 generateID("frk"),
		SourceDiscussionID: source.ID,
		ForkDiscussionID:   forkedDiscussion.ID,
		CreatedByDID:       did,
		Reason:             req.Reason,
		CreatedAt:          now,
	}
	if err := s.store.CreateDiscussionFork(fork); err != nil {
		http.Error(w, "failed to create discussion fork", http.StatusBadRequest)
		return
	}
	relation := &store.ContentRelation{
		ID:            generateID("rel"),
		FromContentID: source.ID,
		ToContentID:   forkedDiscussion.ID,
		RelationType:  store.RelationForkedFrom,
		CreatedAt:     now,
	}
	if err := s.store.CreateContentRelation(relation); err != nil {
		http.Error(w, "failed to create content relation", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIDiscussionFork(*fork))
}

func (s *Server) PostV2ModerationActions(w http.ResponseWriter, r *http.Request) {
	var req api.CreateModerationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	requiredTier := store.L2_SOCIAL
	actionType := store.ModerationActionType(req.ActionType)
	switch actionType {
	case store.ModerationFlag:
		requiredTier = store.L2_SOCIAL
	case store.ModerationHide, store.ModerationLock, store.ModerationSlash:
		requiredTier = store.L4_AUTHORITY
	default:
		http.Error(w, "invalid moderation action type", http.StatusBadRequest)
		return
	}
	if user.TrustTier < requiredTier {
		http.Error(w, fmt.Sprintf("L%d required for this moderation action", requiredTier), http.StatusForbidden)
		return
	}
	target, err := s.store.GetContentItemByID(req.TargetContentId)
	if err != nil {
		http.Error(w, "failed to load target content", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "target content not found", http.StatusNotFound)
		return
	}
	action := &store.ModerationAction{
		ID:              generateID("mod"),
		TargetContentID: req.TargetContentId,
		ActionType:      actionType,
		Reason:          req.Reason,
		InitiatedByDID:  did,
		RequiredTier:    requiredTier,
		Status:          store.ModerationOpen,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.store.CreateModerationAction(action); err != nil {
		http.Error(w, "failed to create moderation action", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIModerationAction(*action))
}

func (s *Server) GetV2TrustMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	credentials, err := s.store.GetCredentialsBySubjectDID(user.DID)
	if err != nil {
		http.Error(w, "failed to load credentials", http.StatusInternalServerError)
		return
	}
	assessments, err := s.store.GetTrustAssessmentsBySubjectDID(user.DID)
	if err != nil {
		http.Error(w, "failed to load trust assessments", http.StatusInternalServerError)
		return
	}
	cases, err := s.store.GetVerificationCases(&user.DID, nil)
	if err != nil {
		http.Error(w, "failed to load verification cases", http.StatusInternalServerError)
		return
	}
	auditLogs, err := s.store.GetTrustAuditLogs(&user.DID, 20)
	if err != nil {
		http.Error(w, "failed to load trust audit logs", http.StatusInternalServerError)
		return
	}
	verifier, err := s.store.GetVerifierByDID(user.DID)
	if err != nil {
		http.Error(w, "failed to load verifier profile", http.StatusInternalServerError)
		return
	}

	authMethod := "passkey"
	if user.TrustTier == store.L0_GUEST {
		authMethod = "oauth"
	}
	capabilities := capabilitiesForTier(user.TrustTier)
	capabilities.CanReviewVerificationCases = verifier != nil && verifier.Status == store.VerifierActive
	profile := api.TrustProfile{
		Identity: api.Identity{
			Did:         user.DID,
			DisplayName: displayNameForDID(user.DID),
			TrustTier:   int(user.TrustTier),
			AuthMethod:  &authMethod,
		},
		Capabilities:      capabilities,
		Credentials:       make([]api.Credential, 0, len(credentials)),
		TrustAssessments:  make([]api.TrustAssessment, 0, len(assessments)),
		VerificationCases: make([]api.VerificationCase, 0, len(cases)),
		AuditLogs:         make([]api.TrustAuditLog, 0, len(auditLogs)),
	}
	if verifier != nil {
		v := toAPIVerifier(*verifier)
		profile.Verifier = &v
	}
	for _, credential := range credentials {
		profile.Credentials = append(profile.Credentials, toAPICredential(credential))
	}
	for _, assessment := range assessments {
		profile.TrustAssessments = append(profile.TrustAssessments, toAPITrustAssessment(assessment))
	}
	for _, verificationCase := range cases {
		profile.VerificationCases = append(profile.VerificationCases, toAPIVerificationCase(verificationCase))
	}
	for _, auditLog := range auditLogs {
		profile.AuditLogs = append(profile.AuditLogs, toAPITrustAuditLog(auditLog))
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) PostV2TrustVerificationCases(w http.ResponseWriter, r *http.Request) {
	var req api.CreateVerificationCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	if user.TrustTier < store.L1_DEVICE {
		http.Error(w, "L1 passkey identity required to request verification", http.StatusForbidden)
		return
	}
	if req.RequestedTier < int(store.L2_SOCIAL) || req.RequestedTier > int(store.L3_TEMPORAL) {
		http.Error(w, "verification requests currently support L2-L3", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.CredentialType) == "" || strings.TrimSpace(req.EvidenceJson) == "" {
		http.Error(w, "credentialType and evidenceJson are required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	verificationCase := &store.VerificationCase{
		ID:             generateID("case"),
		SubjectDID:     user.DID,
		RequestedTier:  store.TrustTier(req.RequestedTier),
		CredentialType: strings.TrimSpace(req.CredentialType),
		EvidenceJSON:   req.EvidenceJson,
		Status:         store.VerificationSubmitted,
		CreatedAt:      now,
	}
	if verifier := firstActiveVerifier(s.store); verifier != nil {
		verificationCase.AssignedVerifierDID = verifier.VerifierDID
		verificationCase.Status = store.VerificationAssigned
	}
	if err := s.store.CreateVerificationCase(verificationCase); err != nil {
		http.Error(w, "failed to create verification case", http.StatusBadRequest)
		return
	}
	_ = s.store.CreateTrustAuditLog(&store.TrustAuditLog{
		ID:               generateID("aud"),
		ActorDID:         user.DID,
		TargetDID:        user.DID,
		TargetResourceID: verificationCase.ID,
		ActionType:       "verification_case.created",
		MetadataJSON:     fmt.Sprintf(`{"requestedTier":%d,"credentialType":%q}`, req.RequestedTier, req.CredentialType),
		CreatedAt:        now,
	})
	writeJSON(w, http.StatusCreated, toAPIVerificationCase(*verificationCase))
}

func (s *Server) GetV2TrustVerificationCasesCaseId(w http.ResponseWriter, r *http.Request, caseId string) {
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	verificationCase, err := s.store.GetVerificationCaseByID(caseId)
	if err != nil {
		http.Error(w, "failed to load verification case", http.StatusInternalServerError)
		return
	}
	if verificationCase == nil {
		http.Error(w, "verification case not found", http.StatusNotFound)
		return
	}
	if verificationCase.SubjectDID != user.DID && !s.isActiveVerifier(user.DID) {
		http.Error(w, "only the subject or an active verifier may view this case", http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, toAPIVerificationCase(*verificationCase))
}

func (s *Server) GetV2TrustVerifierCases(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	if !s.isActiveVerifier(user.DID) {
		http.Error(w, "active L4 verifier authority required", http.StatusForbidden)
		return
	}
	cases, err := s.store.GetVerificationCases(nil, nil)
	if err != nil {
		http.Error(w, "failed to load verification cases", http.StatusInternalServerError)
		return
	}
	resp := make([]api.VerificationCase, 0, len(cases))
	for _, verificationCase := range cases {
		if verificationCase.AssignedVerifierDID == "" ||
			verificationCase.AssignedVerifierDID == user.DID ||
			verificationCase.Status == store.VerificationSubmitted {
			resp = append(resp, toAPIVerificationCase(verificationCase))
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) PostV2TrustVerifierCasesCaseIdDecision(w http.ResponseWriter, r *http.Request, caseId string) {
	var req api.CreateVerifierDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	if !s.isActiveVerifier(user.DID) {
		http.Error(w, "active L4 verifier authority required", http.StatusForbidden)
		return
	}
	verificationCase, err := s.store.GetVerificationCaseByID(caseId)
	if err != nil {
		http.Error(w, "failed to load verification case", http.StatusInternalServerError)
		return
	}
	if verificationCase == nil {
		http.Error(w, "verification case not found", http.StatusNotFound)
		return
	}
	if verificationCase.Status == store.VerificationApproved || verificationCase.Status == store.VerificationRejected {
		http.Error(w, "verification case already decided", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	decisionType := store.VerifierDecisionType(req.Decision)
	issuanceSource := store.CredentialIssuanceSource(req.CredentialIssuanceSource)
	if decisionType != store.VerifierDecisionApprove &&
		decisionType != store.VerifierDecisionReject &&
		decisionType != store.VerifierDecisionRequestMoreEvidence {
		http.Error(w, "invalid verifier decision", http.StatusBadRequest)
		return
	}
	if issuanceSource != store.CredentialInternalVerifierIssued && issuanceSource != store.CredentialExternalIssuerVerified {
		http.Error(w, "invalid credential issuance source", http.StatusBadRequest)
		return
	}
	externalIssuerDID := optionalString(req.ExternalIssuerDid)

	verificationCase.AssignedVerifierDID = user.DID
	verificationCase.Decision = string(decisionType)
	verificationCase.DecisionReason = req.Reason
	verificationCase.DecidedAt = &now

	var assessmentID *string
	var credentialID *string
	switch decisionType {
	case store.VerifierDecisionApprove:
		verificationCase.Status = store.VerificationApproved
		assessment := &store.TrustAssessment{
			ID:           generateID("tas"),
			SubjectDID:   verificationCase.SubjectDID,
			Tier:         verificationCase.RequestedTier,
			Source:       store.TrustSourceVerifier,
			Score:        1,
			EvidenceRefs: []string{verificationCase.ID},
			IssuedByDID:  user.DID,
			EffectiveAt:  now,
		}
		if err := s.store.CreateTrustAssessment(assessment); err != nil {
			http.Error(w, "failed to issue trust assessment", http.StatusInternalServerError)
			return
		}
		assessmentID = &assessment.ID

		if verificationCase.RequestedTier >= store.L3_TEMPORAL {
			issuerDID := user.DID
			if issuanceSource == store.CredentialExternalIssuerVerified {
				trustedIssuer, err := s.store.GetTrustedIssuerByDID(externalIssuerDID)
				if err != nil {
					http.Error(w, "failed to load trusted issuer", http.StatusInternalServerError)
					return
				}
				if trustedIssuer == nil || trustedIssuer.Status != store.TrustedIssuerActive {
					http.Error(w, "external issuer is not trusted", http.StatusBadRequest)
					return
				}
				if trustedIssuer.MaxTrustTierIssued < verificationCase.RequestedTier || !stringSliceContains(trustedIssuer.CredentialTypes, verificationCase.CredentialType) {
					http.Error(w, "external issuer is not authorized for this credential type or tier", http.StatusBadRequest)
					return
				}
				issuerDID = trustedIssuer.IssuerDID
			}
			credential := &store.Credential{
				ID:                generateID("cred"),
				SubjectDID:        verificationCase.SubjectDID,
				IssuerDID:         issuerDID,
				ExternalIssuerDID: externalIssuerDID,
				CredentialType:    verificationCase.CredentialType,
				ClaimsJSON:        verificationCase.EvidenceJSON,
				Status:            store.CredentialActive,
				IssuanceSource:    issuanceSource,
				IssuedAt:          now,
			}
			if err := s.store.CreateCredential(credential); err != nil {
				http.Error(w, "failed to issue credential", http.StatusInternalServerError)
				return
			}
			credentialID = &credential.ID
		}
	case store.VerifierDecisionReject:
		verificationCase.Status = store.VerificationRejected
	default:
		verificationCase.Status = store.VerificationAssigned
		verificationCase.DecidedAt = nil
	}

	if err := s.store.UpdateVerificationCase(verificationCase); err != nil {
		http.Error(w, "failed to update verification case", http.StatusInternalServerError)
		return
	}
	decision := &store.VerifierDecision{
		ID:                       generateID("dec"),
		CaseID:                   verificationCase.ID,
		VerifierDID:              user.DID,
		Decision:                 decisionType,
		Reason:                   req.Reason,
		CredentialIssuanceSource: issuanceSource,
		ExternalIssuerDID:        externalIssuerDID,
		IssuedAssessmentID:       assessmentID,
		IssuedCredentialID:       credentialID,
		CreatedAt:                now,
	}
	if err := s.store.CreateVerifierDecision(decision); err != nil {
		http.Error(w, "failed to create verifier decision", http.StatusInternalServerError)
		return
	}
	_ = s.store.CreateTrustAuditLog(&store.TrustAuditLog{
		ID:               generateID("aud"),
		ActorDID:         user.DID,
		TargetDID:        verificationCase.SubjectDID,
		TargetResourceID: verificationCase.ID,
		ActionType:       "verification_case.decided",
		MetadataJSON:     fmt.Sprintf(`{"decision":%q,"requestedTier":%d}`, decisionType, verificationCase.RequestedTier),
		CreatedAt:        now,
	})
	writeJSON(w, http.StatusCreated, toAPIVerifierDecision(*decision))
}

func (s *Server) GetV2TrustVerifiers(w http.ResponseWriter, r *http.Request) {
	verifiers, err := s.store.GetVerifiers()
	if err != nil {
		http.Error(w, "failed to load verifiers", http.StatusInternalServerError)
		return
	}
	resp := make([]api.Verifier, 0, len(verifiers))
	for _, verifier := range verifiers {
		resp = append(resp, toAPIVerifier(verifier))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) GetV2TrustIssuers(w http.ResponseWriter, r *http.Request) {
	issuers, err := s.store.GetTrustedIssuers()
	if err != nil {
		http.Error(w, "failed to load trusted issuers", http.StatusInternalServerError)
		return
	}
	resp := make([]api.TrustedIssuer, 0, len(issuers))
	for _, issuer := range issuers {
		resp = append(resp, toAPITrustedIssuer(issuer))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) PostV2TrustIssuers(w http.ResponseWriter, r *http.Request) {
	var req api.CreateTrustedIssuerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	if !s.isActiveVerifier(user.DID) {
		http.Error(w, "active L4 verifier authority required", http.StatusForbidden)
		return
	}
	existingIssuer, err := s.store.GetTrustedIssuerByDID(strings.TrimSpace(req.IssuerDid))
	if err != nil {
		http.Error(w, "failed to load existing trusted issuer", http.StatusInternalServerError)
		return
	}
	issuer := &store.TrustedIssuer{
		ID:                 generateID("iss"),
		IssuerDID:          strings.TrimSpace(req.IssuerDid),
		IssuerName:         strings.TrimSpace(req.IssuerName),
		Scopes:             append([]string(nil), req.Scopes...),
		CredentialTypes:    append([]string(nil), req.CredentialTypes...),
		MaxTrustTierIssued: store.TrustTier(req.MaxTrustTierIssued),
		AppointedByDID:     user.DID,
		CreatedAt:          time.Now().UTC(),
	}
	if existingIssuer != nil {
		issuer.ID = existingIssuer.ID
		issuer.CreatedAt = existingIssuer.CreatedAt
		issuer.ExpiresAt = existingIssuer.ExpiresAt
		issuer.RevokedAt = existingIssuer.RevokedAt
	}
	if req.Status != nil {
		issuer.Status = store.TrustedIssuerStatus(*req.Status)
	}
	if req.ExpiresAt != nil {
		expiresAt := *req.ExpiresAt
		issuer.ExpiresAt = &expiresAt
	}
	if issuer.IssuerDID == "" || issuer.IssuerName == "" || len(issuer.CredentialTypes) == 0 {
		http.Error(w, "issuerDid, issuerName, and credentialTypes are required", http.StatusBadRequest)
		return
	}
	if issuer.Status == "" {
		issuer.Status = store.TrustedIssuerActive
	}
	if issuer.MaxTrustTierIssued < store.L2_SOCIAL || issuer.MaxTrustTierIssued > store.L4_AUTHORITY {
		http.Error(w, "maxTrustTierIssued must be between L2 and L4", http.StatusBadRequest)
		return
	}
	if err := s.store.CreateTrustedIssuer(issuer); err != nil {
		http.Error(w, "failed to create trusted issuer", http.StatusInternalServerError)
		return
	}
	persistedIssuer, err := s.store.GetTrustedIssuerByDID(issuer.IssuerDID)
	if err != nil {
		http.Error(w, "failed to reload trusted issuer", http.StatusInternalServerError)
		return
	}
	if persistedIssuer != nil {
		issuer = persistedIssuer
	}
	_ = s.store.CreateTrustAuditLog(&store.TrustAuditLog{
		ID:               generateID("aud"),
		ActorDID:         user.DID,
		TargetDID:        issuer.IssuerDID,
		TargetResourceID: issuer.ID,
		ActionType:       "trusted_issuer.upserted",
		MetadataJSON:     fmt.Sprintf(`{"maxTrustTierIssued":%d,"credentialTypes":%q}`, issuer.MaxTrustTierIssued, issuer.CredentialTypes),
		CreatedAt:        issuer.CreatedAt,
	})
	writeJSON(w, http.StatusCreated, toAPITrustedIssuer(*issuer))
}

func (s *Server) GetV2TrustPresentationRequests(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	var subjectDID *string
	if !s.isActiveVerifier(user.DID) {
		subjectDID = &user.DID
	}
	requests, err := s.store.GetWalletPresentationRequests(subjectDID)
	if err != nil {
		http.Error(w, "failed to load wallet presentation requests", http.StatusInternalServerError)
		return
	}
	resp := make([]api.WalletPresentationRequest, 0, len(requests))
	for _, request := range requests {
		verification, err := s.store.GetLatestWalletPresentationVerification(request.ID)
		if err != nil {
			http.Error(w, "failed to load wallet presentation verification", http.StatusInternalServerError)
			return
		}
		resp = append(resp, toAPIWalletPresentationRequest(request, verification))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) PostV2TrustPresentationRequests(w http.ResponseWriter, r *http.Request) {
	var req api.CreateWalletPresentationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	if user.TrustTier < store.L1_DEVICE {
		http.Error(w, "L1 passkey identity required to create wallet presentation requests", http.StatusForbidden)
		return
	}
	if req.RequestedTier < int(store.L2_SOCIAL) || req.RequestedTier > int(store.L3_TEMPORAL) {
		http.Error(w, "wallet presentation requests currently support L2-L3", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.CredentialType) == "" {
		http.Error(w, "credentialType is required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	expiresInMinutes := 15
	if req.ExpiresInMinutes != nil && *req.ExpiresInMinutes > 0 && *req.ExpiresInMinutes <= 60 {
		expiresInMinutes = *req.ExpiresInMinutes
	}
	expiresAt := now.Add(time.Duration(expiresInMinutes) * time.Minute)
	challenge := fmt.Sprintf("%x", randomBytes(16))
	requestID := generateID("wpr")
	requestURI := buildWalletRequestURI(requestID, challenge)
	qrPayloadBytes, _ := json.Marshal(map[string]any{
		"type":       "aleth_wallet_verification",
		"requestId":  requestID,
		"requestUri": requestURI,
		"nonce":      challenge,
	})
	walletRequest := &store.WalletPresentationRequest{
		ID:                requestID,
		SubjectDID:        user.DID,
		VerifierDID:       s.walletVerifierDID(),
		RequestedTier:     store.TrustTier(req.RequestedTier),
		CredentialType:    strings.TrimSpace(req.CredentialType),
		Purpose:           strings.TrimSpace(optionalString(req.Purpose)),
		AllowedIssuerDIDs: normalizeStringSlice(req.AllowedIssuerDids),
		Challenge:         challenge,
		RequestURI:        requestURI,
		QRPayload:         string(qrPayloadBytes),
		Status:            store.WalletRequestPending,
		CreatedAt:         now,
		ExpiresAt:         &expiresAt,
	}
	if err := s.store.CreateWalletPresentationRequest(walletRequest); err != nil {
		http.Error(w, "failed to create wallet presentation request", http.StatusInternalServerError)
		return
	}
	_ = s.store.CreateTrustAuditLog(&store.TrustAuditLog{
		ID:               generateID("aud"),
		ActorDID:         user.DID,
		TargetDID:        user.DID,
		TargetResourceID: walletRequest.ID,
		ActionType:       "wallet_presentation_request.created",
		MetadataJSON:     fmt.Sprintf(`{"requestedTier":%d,"credentialType":%q}`, walletRequest.RequestedTier, walletRequest.CredentialType),
		CreatedAt:        now,
	})
	writeJSON(w, http.StatusCreated, toAPIWalletPresentationRequest(*walletRequest, nil))
}

func (s *Server) GetV2TrustPresentationRequestsRequestId(w http.ResponseWriter, r *http.Request, requestId string) {
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	walletRequest, err := s.store.GetWalletPresentationRequestByID(requestId)
	if err != nil {
		http.Error(w, "failed to load wallet presentation request", http.StatusInternalServerError)
		return
	}
	if walletRequest == nil {
		http.Error(w, "wallet presentation request not found", http.StatusNotFound)
		return
	}
	if walletRequest.SubjectDID != user.DID && !s.isActiveVerifier(user.DID) {
		http.Error(w, "only the subject or an active verifier may view this request", http.StatusForbidden)
		return
	}
	verification, err := s.store.GetLatestWalletPresentationVerification(walletRequest.ID)
	if err != nil {
		http.Error(w, "failed to load wallet presentation verification", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toAPIWalletPresentationRequest(*walletRequest, verification))
}

func (s *Server) PostV2TrustPresentationRequestsRequestIdComplete(w http.ResponseWriter, r *http.Request, requestId string) {
	var req api.CompleteWalletPresentationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	user, err := s.currentUserFromRequest(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	walletRequest, err := s.store.GetWalletPresentationRequestByID(requestId)
	if err != nil {
		http.Error(w, "failed to load wallet presentation request", http.StatusInternalServerError)
		return
	}
	if walletRequest == nil {
		http.Error(w, "wallet presentation request not found", http.StatusNotFound)
		return
	}
	if walletRequest.SubjectDID != user.DID && !s.isActiveVerifier(user.DID) {
		http.Error(w, "only the subject or an active verifier may complete this request", http.StatusForbidden)
		return
	}

	now := time.Now().UTC()
	verification := &store.WalletPresentationVerification{
		ID:                 generateID("wpv"),
		RequestID:          walletRequest.ID,
		SubjectDID:         walletRequest.SubjectDID,
		IssuerDID:          strings.TrimSpace(req.IssuerDid),
		CredentialType:     strings.TrimSpace(req.CredentialType),
		PresentationFormat: strings.TrimSpace(req.PresentationFormat),
		ClaimsJSON:         req.ClaimsJson,
		Proof:              strings.TrimSpace(req.Proof),
		Audience:           strings.TrimSpace(req.Audience),
		Nonce:              strings.TrimSpace(req.Nonce),
		Status:             store.WalletVerificationVerified,
		CreatedAt:          now,
		VerifiedAt:         &now,
	}
	requestStatus := store.WalletRequestVerified
	notes := []string{}
	requestExpired := false
	appendNote := func(format string, args ...any) {
		notes = append(notes, fmt.Sprintf(format, args...))
		verification.Status = store.WalletVerificationRejected
		requestStatus = store.WalletRequestRejected
	}

	if walletRequest.ExpiresAt != nil && walletRequest.ExpiresAt.Before(now) {
		requestExpired = true
		appendNote("request expired before submission")
	}
	if verification.CredentialType == "" || verification.CredentialType != walletRequest.CredentialType {
		appendNote("credential type does not match request")
	}
	if verification.PresentationFormat == "" {
		appendNote("presentation format is required")
	}
	if verification.Proof == "" {
		appendNote("proof payload is required")
	}
	if verification.ClaimsJSON == "" {
		appendNote("claims payload is required")
	}
	if verification.Nonce == "" || verification.Nonce != walletRequest.Challenge {
		appendNote("nonce does not match request challenge")
	}
	if verification.Audience == "" || verification.Audience != walletRequest.RequestURI {
		appendNote("audience does not match request URI")
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(now) {
		appendNote("submitted credential is expired")
	}
	if req.RevokedAt != nil {
		appendNote("submitted credential has been revoked")
	}

	var trustedIssuer *store.TrustedIssuer
	if verification.IssuerDID == "" {
		appendNote("issuer DID is required")
	} else {
		trustedIssuer, err = s.store.GetTrustedIssuerByDID(verification.IssuerDID)
		if err != nil {
			http.Error(w, "failed to load trusted issuer", http.StatusInternalServerError)
			return
		}
		if trustedIssuer == nil {
			appendNote("issuer is not in trusted issuer registry")
		} else {
			verification.TrustedIssuerDID = trustedIssuer.IssuerDID
			if trustedIssuer.Status != store.TrustedIssuerActive {
				appendNote("trusted issuer is not active")
			}
			if trustedIssuer.ExpiresAt != nil && trustedIssuer.ExpiresAt.Before(now) {
				appendNote("trusted issuer entry is expired")
			}
			if trustedIssuer.RevokedAt != nil {
				appendNote("trusted issuer entry is revoked")
			}
			if len(walletRequest.AllowedIssuerDIDs) > 0 && !stringSliceContains(walletRequest.AllowedIssuerDIDs, trustedIssuer.IssuerDID) {
				appendNote("issuer is not allowed by this request")
			}
			if trustedIssuer.MaxTrustTierIssued < walletRequest.RequestedTier {
				appendNote("issuer is not authorized for requested trust tier")
			}
			if !stringSliceContains(trustedIssuer.CredentialTypes, walletRequest.CredentialType) {
				appendNote("issuer is not authorized for this credential type")
			}
		}
	}

	if verification.Status == store.WalletVerificationVerified && trustedIssuer != nil {
		credential := &store.Credential{
			ID:                generateID("cred"),
			SubjectDID:        walletRequest.SubjectDID,
			IssuerDID:         trustedIssuer.IssuerDID,
			ExternalIssuerDID: trustedIssuer.IssuerDID,
			CredentialType:    walletRequest.CredentialType,
			ClaimsJSON:        verification.ClaimsJSON,
			Proof:             verification.Proof,
			Status:            store.CredentialActive,
			IssuanceSource:    store.CredentialExternalIssuerVerified,
			IssuedAt:          now,
			ExpiresAt:         req.ExpiresAt,
			RevokedAt:         req.RevokedAt,
		}
		if err := s.store.CreateCredential(credential); err != nil {
			http.Error(w, "failed to create credential", http.StatusInternalServerError)
			return
		}
		assessment := &store.TrustAssessment{
			ID:           generateID("tas"),
			SubjectDID:   walletRequest.SubjectDID,
			Tier:         walletRequest.RequestedTier,
			Score:        1,
			Source:       store.TrustSourceCredential,
			EvidenceRefs: []string{walletRequest.ID, verification.ID},
			IssuedByDID:  trustedIssuer.IssuerDID,
			EffectiveAt:  now,
			ExpiresAt:    req.ExpiresAt,
		}
		if err := s.store.CreateTrustAssessment(assessment); err != nil {
			http.Error(w, "failed to create trust assessment", http.StatusInternalServerError)
			return
		}
		verification.IssuedCredentialID = &credential.ID
		verification.IssuedAssessmentID = &assessment.ID
		notes = append(notes, "wallet presentation accepted by trusted issuer policy")
	} else if len(notes) == 0 {
		notes = append(notes, "wallet presentation rejected by policy")
	}

	verification.Notes = strings.Join(notes, "; ")
	if requestExpired {
		requestStatus = store.WalletRequestExpired
	}
	walletRequest.Status = requestStatus
	walletRequest.CompletedAt = &now
	if requestStatus == store.WalletRequestRejected {
		walletRequest.Status = store.WalletRequestRejected
	}
	if err := s.store.CreateWalletPresentationVerification(verification); err != nil {
		http.Error(w, "failed to create wallet presentation verification", http.StatusInternalServerError)
		return
	}
	if err := s.store.UpdateWalletPresentationRequest(walletRequest); err != nil {
		http.Error(w, "failed to update wallet presentation request", http.StatusInternalServerError)
		return
	}
	_ = s.store.CreateTrustAuditLog(&store.TrustAuditLog{
		ID:               generateID("aud"),
		ActorDID:         user.DID,
		TargetDID:        walletRequest.SubjectDID,
		TargetResourceID: walletRequest.ID,
		ActionType:       "wallet_presentation_request.completed",
		MetadataJSON:     fmt.Sprintf(`{"status":%q,"issuerDid":%q,"credentialType":%q}`, verification.Status, verification.IssuerDID, verification.CredentialType),
		CreatedAt:        now,
	})
	writeJSON(w, http.StatusCreated, toAPIWalletPresentationVerification(*verification))
}

func (s *Server) GetPosts(w http.ResponseWriter, r *http.Request, params api.GetPostsParams) {
	limit := 20
	offset := 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	posts, err := s.repSvc.GetPublicFeed(limit, offset)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		return
	}

	resp := make([]api.Post, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, toAPIPost(p))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func toAPIPost(p store.Post) api.Post {
	postID := strconv.FormatInt(p.ID, 10)
	var parentID *string
	if p.ParentID.Valid {
		v := strconv.FormatInt(p.ParentID.Int64, 10)
		parentID = &v
	}

	return api.Post{
		Id:              postID,
		Body:            p.Body,
		MediaHashes:     &p.MediaHashes,
		ParentId:        parentID,
		Timestamp:       int(p.Timestamp),
		AuthorDid:       p.AuthorDID,
		Signature:       p.Signature,
		VisibilityScore: ptrFloat32(float32(p.VisibilityScore)),
	}
}

func ptrFloat32(v float32) *float32 {
	return &v
}

func toAPIContentItem(item store.ContentItem) api.ContentItem {
	var title *string
	if item.Title != "" {
		title = &item.Title
	}
	var publishedAtTime *time.Time
	if item.PublishedAt != nil {
		value := *item.PublishedAt
		publishedAtTime = &value
	}
	var participationPolicy *api.ContentItemParticipationPolicy
	if item.ParticipationPolicy != "" {
		value := api.ContentItemParticipationPolicy(item.ParticipationPolicy)
		participationPolicy = &value
	}
	var discussionShape *api.ContentItemDiscussionShape
	if item.DiscussionShape != "" {
		value := api.ContentItemDiscussionShape(item.DiscussionShape)
		discussionShape = &value
	}
	var sourceContentIDs *[]string
	if len(item.SourceContentIDs) > 0 {
		copied := append([]string(nil), item.SourceContentIDs...)
		sourceContentIDs = &copied
	}
	trustTier := int(item.TrustTier)
	return api.ContentItem{
		Id:                  item.ID,
		AuthorDid:           item.AuthorDID,
		Title:               title,
		Body:                item.Body,
		Mode:                api.ContentItemMode(item.Mode),
		Status:              api.ContentItemStatus(item.Status),
		Visibility:          api.ContentItemVisibility(item.Visibility),
		TrustTier:           &trustTier,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
		PublishedAt:         publishedAtTime,
		ParticipationPolicy: participationPolicy,
		DiscussionShape:     discussionShape,
		SourceContentIds:    sourceContentIDs,
	}
}

func toAPIDiscussionNode(node store.DiscussionNode) api.DiscussionNode {
	return api.DiscussionNode{
		Id:           node.ID,
		DiscussionId: node.DiscussionID,
		ParentNodeId: node.ParentNodeID,
		AuthorDid:    node.AuthorDID,
		NodeType:     api.DiscussionNodeNodeType(node.NodeType),
		Stance:       api.DiscussionNodeStance(node.Stance),
		Body:         node.Body,
		CreatedAt:    node.CreatedAt,
	}
}

func toAPIProjection(projection store.Projection) api.Projection {
	return api.Projection{
		Id:                            projection.ID,
		SourceIdeaId:                  projection.SourceIdeaID,
		TargetDiscussionId:            projection.TargetDiscussionID,
		ProjectedExcerpt:              projection.ProjectedExcerpt,
		ParticipationPolicy:           api.ProjectionParticipationPolicy(projection.ParticipationPolicy),
		OwnershipTransferAcknowledged: projection.OwnershipTransferAcknowledged,
		CreatedByDid:                  projection.CreatedByDID,
		CreatedAt:                     projection.CreatedAt,
	}
}

func toAPITransformationJob(job store.TransformationJob) api.TransformationJob {
	var completedAtTime *time.Time
	if job.CompletedAt != nil {
		value := *job.CompletedAt
		completedAtTime = &value
	}
	var outputTitle *string
	if job.OutputTitle != "" {
		outputTitle = &job.OutputTitle
	}
	var outputBody *string
	if job.OutputBody != "" {
		outputBody = &job.OutputBody
	}
	return api.TransformationJob{
		Id:                 job.ID,
		RequestedByDid:     job.RequestedByDID,
		SourceContentIds:   append([]string(nil), job.SourceContentIDs...),
		TargetMode:         api.TransformationJobTargetMode(job.TargetMode),
		ProviderType:       api.TransformationJobProviderType(job.ProviderType),
		PromptProfile:      job.PromptProfile,
		Status:             api.TransformationJobStatus(job.Status),
		OutputTitle:        outputTitle,
		OutputBody:         outputBody,
		CreatedAt:          job.CreatedAt,
		CompletedAt:        completedAtTime,
		PublishedContentId: job.PublishedContentID,
	}
}

func toAPIDiscussionFork(fork store.DiscussionFork) api.DiscussionFork {
	return api.DiscussionFork{
		Id:                 fork.ID,
		SourceDiscussionId: fork.SourceDiscussionID,
		ForkDiscussionId:   fork.ForkDiscussionID,
		CreatedByDid:       fork.CreatedByDID,
		Reason:             fork.Reason,
		CreatedAt:          fork.CreatedAt,
	}
}

func toAPIModerationAction(action store.ModerationAction) api.ModerationAction {
	return api.ModerationAction{
		Id:              action.ID,
		TargetContentId: action.TargetContentID,
		ActionType:      api.ModerationActionActionType(action.ActionType),
		Reason:          action.Reason,
		InitiatedByDid:  action.InitiatedByDID,
		RequiredTier:    int(action.RequiredTier),
		Status:          api.ModerationActionStatus(action.Status),
		CreatedAt:       action.CreatedAt,
	}
}

func toAPICredential(credential store.Credential) api.Credential {
	var proof *string
	if credential.Proof != "" {
		proof = &credential.Proof
	}
	var externalIssuerDID *string
	if credential.ExternalIssuerDID != "" {
		externalIssuerDID = &credential.ExternalIssuerDID
	}
	return api.Credential{
		Id:                credential.ID,
		SubjectDid:        credential.SubjectDID,
		IssuerDid:         credential.IssuerDID,
		ExternalIssuerDid: externalIssuerDID,
		CredentialType:    credential.CredentialType,
		ClaimsJson:        credential.ClaimsJSON,
		Status:            api.CredentialStatus(credential.Status),
		IssuanceSource:    api.CredentialIssuanceSource(credential.IssuanceSource),
		Proof:             proof,
		IssuedAt:          credential.IssuedAt,
		ExpiresAt:         credential.ExpiresAt,
		RevokedAt:         credential.RevokedAt,
	}
}

func toAPITrustAssessment(assessment store.TrustAssessment) api.TrustAssessment {
	var evidenceRefs *[]string
	if len(assessment.EvidenceRefs) > 0 {
		copied := append([]string(nil), assessment.EvidenceRefs...)
		evidenceRefs = &copied
	}
	return api.TrustAssessment{
		Id:           assessment.ID,
		SubjectDid:   assessment.SubjectDID,
		Tier:         int(assessment.Tier),
		Source:       api.TrustAssessmentSource(assessment.Source),
		Score:        float32(assessment.Score),
		EvidenceRefs: evidenceRefs,
		IssuedByDid:  assessment.IssuedByDID,
		EffectiveAt:  assessment.EffectiveAt,
		ExpiresAt:    assessment.ExpiresAt,
		RevokedAt:    assessment.RevokedAt,
	}
}

func toAPIVerifier(verifier store.Verifier) api.Verifier {
	return api.Verifier{
		Id:             verifier.ID,
		VerifierDid:    verifier.VerifierDID,
		VerifierType:   verifier.VerifierType,
		Scope:          verifier.Scope,
		AuthorityLevel: int(verifier.AuthorityLevel),
		Status:         api.VerifierStatus(verifier.Status),
		AppointedByDid: verifier.AppointedByDID,
		CreatedAt:      verifier.CreatedAt,
		ExpiresAt:      verifier.ExpiresAt,
		RevokedAt:      verifier.RevokedAt,
	}
}

func toAPITrustedIssuer(issuer store.TrustedIssuer) api.TrustedIssuer {
	return api.TrustedIssuer{
		Id:                 issuer.ID,
		IssuerDid:          issuer.IssuerDID,
		IssuerName:         issuer.IssuerName,
		Status:             api.TrustedIssuerStatus(issuer.Status),
		Scopes:             append([]string(nil), issuer.Scopes...),
		CredentialTypes:    append([]string(nil), issuer.CredentialTypes...),
		MaxTrustTierIssued: int(issuer.MaxTrustTierIssued),
		AppointedByDid:     issuer.AppointedByDID,
		CreatedAt:          issuer.CreatedAt,
		ExpiresAt:          issuer.ExpiresAt,
		RevokedAt:          issuer.RevokedAt,
	}
}

func toAPIWalletPresentationRequest(request store.WalletPresentationRequest, verification *store.WalletPresentationVerification) api.WalletPresentationRequest {
	var purpose *string
	if request.Purpose != "" {
		purpose = &request.Purpose
	}
	var apiVerification *api.WalletPresentationVerification
	if verification != nil {
		v := toAPIWalletPresentationVerification(*verification)
		apiVerification = &v
	}
	return api.WalletPresentationRequest{
		Id:                request.ID,
		SubjectDid:        request.SubjectDID,
		VerifierDid:       request.VerifierDID,
		RequestedTier:     int(request.RequestedTier),
		CredentialType:    request.CredentialType,
		Purpose:           purpose,
		AllowedIssuerDids: append([]string(nil), request.AllowedIssuerDIDs...),
		Challenge:         request.Challenge,
		RequestUri:        request.RequestURI,
		QrPayload:         request.QRPayload,
		Status:            api.WalletPresentationRequestStatus(request.Status),
		CreatedAt:         request.CreatedAt,
		ExpiresAt:         request.ExpiresAt,
		CompletedAt:       request.CompletedAt,
		Verification:      apiVerification,
	}
}

func toAPIWalletPresentationVerification(verification store.WalletPresentationVerification) api.WalletPresentationVerification {
	var trustedIssuerDID *string
	if verification.TrustedIssuerDID != "" {
		trustedIssuerDID = &verification.TrustedIssuerDID
	}
	var notes *string
	if verification.Notes != "" {
		notes = &verification.Notes
	}
	return api.WalletPresentationVerification{
		Id:                 verification.ID,
		RequestId:          verification.RequestID,
		SubjectDid:         verification.SubjectDID,
		IssuerDid:          verification.IssuerDID,
		CredentialType:     verification.CredentialType,
		PresentationFormat: verification.PresentationFormat,
		ClaimsJson:         verification.ClaimsJSON,
		Proof:              verification.Proof,
		Audience:           verification.Audience,
		Nonce:              verification.Nonce,
		Status:             api.WalletPresentationVerificationStatus(verification.Status),
		TrustedIssuerDid:   trustedIssuerDID,
		Notes:              notes,
		IssuedCredentialId: verification.IssuedCredentialID,
		IssuedAssessmentId: verification.IssuedAssessmentID,
		CreatedAt:          verification.CreatedAt,
		VerifiedAt:         verification.VerifiedAt,
	}
}

func toAPIVerificationCase(verificationCase store.VerificationCase) api.VerificationCase {
	var assignedVerifierDID *string
	if verificationCase.AssignedVerifierDID != "" {
		assignedVerifierDID = &verificationCase.AssignedVerifierDID
	}
	var decision *string
	if verificationCase.Decision != "" {
		decision = &verificationCase.Decision
	}
	var decisionReason *string
	if verificationCase.DecisionReason != "" {
		decisionReason = &verificationCase.DecisionReason
	}
	return api.VerificationCase{
		Id:                  verificationCase.ID,
		SubjectDid:          verificationCase.SubjectDID,
		RequestedTier:       int(verificationCase.RequestedTier),
		CredentialType:      verificationCase.CredentialType,
		EvidenceJson:        verificationCase.EvidenceJSON,
		Status:              api.VerificationCaseStatus(verificationCase.Status),
		AssignedVerifierDid: assignedVerifierDID,
		Decision:            decision,
		DecisionReason:      decisionReason,
		CreatedAt:           verificationCase.CreatedAt,
		DecidedAt:           verificationCase.DecidedAt,
	}
}

func toAPIVerifierDecision(decision store.VerifierDecision) api.VerifierDecision {
	var externalIssuerDID *string
	if decision.ExternalIssuerDID != "" {
		externalIssuerDID = &decision.ExternalIssuerDID
	}
	return api.VerifierDecision{
		Id:                       decision.ID,
		CaseId:                   decision.CaseID,
		VerifierDid:              decision.VerifierDID,
		Decision:                 api.VerifierDecisionDecision(decision.Decision),
		Reason:                   decision.Reason,
		CredentialIssuanceSource: api.VerifierDecisionCredentialIssuanceSource(decision.CredentialIssuanceSource),
		ExternalIssuerDid:        externalIssuerDID,
		IssuedAssessmentId:       decision.IssuedAssessmentID,
		IssuedCredentialId:       decision.IssuedCredentialID,
		CreatedAt:                decision.CreatedAt,
	}
}

func toAPITrustAuditLog(auditLog store.TrustAuditLog) api.TrustAuditLog {
	var targetDID *string
	if auditLog.TargetDID != "" {
		targetDID = &auditLog.TargetDID
	}
	var targetResourceID *string
	if auditLog.TargetResourceID != "" {
		targetResourceID = &auditLog.TargetResourceID
	}
	var metadataJSON *string
	if auditLog.MetadataJSON != "" {
		metadataJSON = &auditLog.MetadataJSON
	}
	return api.TrustAuditLog{
		Id:               auditLog.ID,
		ActorDid:         auditLog.ActorDID,
		TargetDid:        targetDID,
		TargetResourceId: targetResourceID,
		ActionType:       auditLog.ActionType,
		MetadataJson:     metadataJSON,
		CreatedAt:        auditLog.CreatedAt,
	}
}

func capabilitiesForTier(tier store.TrustTier) api.CapabilitySnapshot {
	return api.CapabilitySnapshot{
		CanCreateMurmur:            true,
		CanCreateIdea:              tier >= store.L1_DEVICE,
		CanCreateDiscussion:        tier >= store.L1_DEVICE,
		CanReplyToDiscussion:       tier >= store.L0_GUEST,
		CanForkDiscussion:          tier >= store.L2_SOCIAL,
		CanFlagContent:             tier >= store.L2_SOCIAL,
		CanSlashContent:            tier >= store.L4_AUTHORITY,
		CanModerate:                tier >= store.L4_AUTHORITY,
		CanRequestVerification:     tier >= store.L1_DEVICE,
		CanReviewVerificationCases: tier >= store.L4_AUTHORITY,
	}
}

func displayNameForDID(did string) string {
	if strings.HasPrefix(did, "oauth:") {
		return "訪客"
	}
	if strings.HasPrefix(did, "did:vflow:") {
		raw := strings.TrimPrefix(did, "did:vflow:")
		if len(raw) > 8 {
			return "User_" + raw[:6]
		}
		return "User_" + raw
	}
	return did
}

func defaultVisibilityForMode(mode store.ContentMode) store.ContentVisibility {
	switch mode {
	case store.ModeDiscussion:
		return store.VisibilityPublic
	case store.ModeIdea:
		return store.VisibilityPrivate
	default:
		return store.VisibilityPrivate
	}
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
}

func buildWalletRequestURI(requestID, challenge string) string {
	return fmt.Sprintf("openid4vp://aleth/request/%s?nonce=%s", requestID, challenge)
}

func randomBytes(size int) []byte {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil
	}
	return buf
}

func derivePasskeyDID(credentialID []byte) string {
	sum := sha256.Sum256(credentialID)
	return fmt.Sprintf("did:vflow:%x", sum[:16])
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func normalizeStringSlice(values *[]string) []string {
	if values == nil {
		return nil
	}
	res := make([]string, 0, len(*values))
	for _, value := range *values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		res = append(res, value)
	}
	return res
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func generateTransformationDraft(sources []store.ContentItem, targetMode store.ContentMode, provider store.TransformationProviderType, profile string) (string, string) {
	sourceTitles := make([]string, 0, len(sources))
	sourceBodies := make([]string, 0, len(sources))
	for _, source := range sources {
		title := source.Title
		if title == "" {
			title = firstSentence(source.Body)
		}
		sourceTitles = append(sourceTitles, title)
		sourceBodies = append(sourceBodies, source.Body)
	}

	title := "AI Draft"
	if len(sourceTitles) > 0 {
		title = fmt.Sprintf("%s synthesis", sourceTitles[0])
	}
	if targetMode == store.ModeIdea {
		title = fmt.Sprintf("Idea Draft: %s", title)
	}

	body := fmt.Sprintf(
		"Provider: %s\nProfile: %s\nTarget Mode: %s\n\nCore Signals:\n- %s\n\nDraft:\n%s\n\nSuggested Expansion:\nThis draft combines the selected source content into a more structured viewpoint suitable for revision before publication.",
		provider,
		defaultProfile(profile),
		targetMode,
		strings.Join(sourceTitles, "\n- "),
		strings.Join(sourceBodies, "\n\n"),
	)

	return title, body
}

func defaultProfile(profile string) string {
	if profile == "" {
		return "general"
	}
	return profile
}

func firstSentence(value string) string {
	for _, separator := range []string{"\n", "。", ".", "!", "?"} {
		if idx := strings.Index(value, separator); idx > 0 {
			return strings.TrimSpace(value[:idx])
		}
	}
	return strings.TrimSpace(value)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func extractCeremonyPayload(r *http.Request) (sessionID string, credentialBody []byte, err error) {
	var payload struct {
		SessionID  string          `json:"sessionId"`
		Credential json.RawMessage `json:"credential"`
	}
	if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return "", nil, fmt.Errorf("invalid JSON body")
	}
	if strings.TrimSpace(payload.SessionID) == "" {
		return "", nil, fmt.Errorf("sessionId is required")
	}
	if len(payload.Credential) == 0 {
		return "", nil, fmt.Errorf("credential is required")
	}
	return strings.TrimSpace(payload.SessionID), payload.Credential, nil
}

func requestWithJSONBody(r *http.Request, body []byte) *http.Request {
	clone := r.Clone(r.Context())
	clone.Body = ioNopCloser{bytes.NewReader(body)}
	clone.ContentLength = int64(len(body))
	return clone
}

func webauthnUserFromStore(user store.User) (*vfauth.User, error) {
	credentials := []webauthn.Credential{}
	if len(user.PasskeyCredentials) > 0 {
		if err := json.Unmarshal(user.PasskeyCredentials, &credentials); err != nil {
			return nil, err
		}
	}
	return &vfauth.User{
		ID:          append([]byte(nil), user.AuthnUserID...),
		Name:        user.DID,
		DisplayName: displayNameForDID(user.DID),
		Credentials: credentials,
	}, nil
}

func updateStoredCredential(user *store.User, validated *webauthn.Credential) error {
	credentials := []webauthn.Credential{}
	if len(user.PasskeyCredentials) > 0 {
		if err := json.Unmarshal(user.PasskeyCredentials, &credentials); err != nil {
			return err
		}
	}

	updated := false
	for i := range credentials {
		if bytes.Equal(credentials[i].ID, validated.ID) {
			credentials[i] = *validated
			updated = true
			break
		}
	}
	if !updated {
		credentials = append(credentials, *validated)
	}

	encoded, err := json.Marshal(credentials)
	if err != nil {
		return err
	}
	user.PasskeyCredentials = encoded
	user.PublicKey = append([]byte(nil), validated.PublicKey...)
	if user.TrustTier < store.L1_DEVICE {
		user.TrustTier = store.L1_DEVICE
	}
	return nil
}

type ioNopCloser struct {
	*bytes.Reader
}

func (ioNopCloser) Close() error {
	return nil
}

func (s *Server) ensureUserForDID(did string) (*store.User, error) {
	user, err := s.store.GetUserByDID(did)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	tier := store.L0_GUEST
	switch {
	case strings.HasPrefix(did, "did:vflow:"):
		tier = store.L1_DEVICE
	case strings.HasPrefix(did, "oauth:"):
		tier = store.L0_GUEST
	}

	user = &store.User{
		DID:       did,
		TrustTier: tier,
		CreatedAt: time.Now().UTC(),
	}
	if strings.HasPrefix(did, "oauth:") {
		user.OAuthID = did
	}
	if err := s.store.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Server) currentUserFromRequest(r *http.Request) (*store.User, error) {
	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		return nil, fmt.Errorf("missing auth context DID")
	}
	user, err := s.ensureUserForDID(did)
	if err != nil {
		return nil, err
	}
	if mockTier, ok := r.Context().Value("mock_trust_tier").(store.TrustTier); ok && mockTier > user.TrustTier {
		copyUser := *user
		copyUser.TrustTier = mockTier
		return &copyUser, nil
	}
	return user, nil
}

func (s *Server) isActiveVerifier(did string) bool {
	verifier, err := s.store.GetVerifierByDID(did)
	if err != nil || verifier == nil {
		return false
	}
	return verifier.Status == store.VerifierActive && verifier.AuthorityLevel >= store.L4_AUTHORITY
}

func firstActiveVerifier(db store.Store) *store.Verifier {
	verifiers, err := db.GetVerifiers()
	if err != nil {
		return nil
	}
	for i := range verifiers {
		if verifiers[i].Status == store.VerifierActive && verifiers[i].AuthorityLevel >= store.L4_AUTHORITY {
			return &verifiers[i]
		}
	}
	return nil
}

func (s *Server) walletVerifierDID() string {
	if verifier := firstActiveVerifier(s.store); verifier != nil {
		return verifier.VerifierDID
	}
	return "did:web:aleth.local"
}

func bootstrapVerifiersFromEnv(db store.Store) error {
	raw := strings.TrimSpace(os.Getenv("LEITH_BOOTSTRAP_VERIFIER_DIDS"))
	if raw == "" {
		return nil
	}
	for _, token := range strings.Split(raw, ",") {
		did := strings.TrimSpace(token)
		if did == "" {
			continue
		}
		existing, err := db.GetVerifierByDID(did)
		if err != nil {
			return err
		}
		if existing != nil && existing.Status == store.VerifierActive {
			continue
		}
		user, err := db.GetUserByDID(did)
		if err != nil {
			return err
		}
		if user == nil {
			user = &store.User{
				DID:       did,
				TrustTier: store.L4_AUTHORITY,
				CreatedAt: time.Now().UTC(),
			}
		}
		if user.TrustTier < store.L4_AUTHORITY {
			user.TrustTier = store.L4_AUTHORITY
		}
		if err := db.CreateUser(user); err != nil {
			return err
		}

		now := time.Now().UTC()
		verifier := &store.Verifier{
			ID:             generateID("vrf"),
			VerifierDID:    did,
			VerifierType:   "platform_bootstrap",
			Scope:          "global",
			AuthorityLevel: store.L4_AUTHORITY,
			Status:         store.VerifierActive,
			AppointedByDID: "system",
			CreatedAt:      now,
		}
		if existing != nil {
			verifier.ID = existing.ID
			verifier.CreatedAt = existing.CreatedAt
		}
		if err := db.CreateVerifier(verifier); err != nil {
			return err
		}
		if err := db.CreateTrustAssessment(&store.TrustAssessment{
			ID:           generateID("tas"),
			SubjectDID:   did,
			Tier:         store.L4_AUTHORITY,
			Source:       store.TrustSourceSystem,
			Score:        1,
			EvidenceRefs: []string{"bootstrap_env"},
			IssuedByDID:  "system",
			EffectiveAt:  now,
		}); err != nil {
			return err
		}
		if err := db.CreateTrustAuditLog(&store.TrustAuditLog{
			ID:           generateID("aud"),
			ActorDID:     "system",
			TargetDID:    did,
			ActionType:   "verifier.bootstrapped",
			MetadataJSON: `{"source":"LEITH_BOOTSTRAP_VERIFIER_DIDS"}`,
			CreatedAt:    now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func initStoreFromEnv() (store.Store, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("LEITH_STORE_BACKEND")))
	if backend == "" {
		backend = "memory"
	}

	switch backend {
	case "memory":
		return store.NewMemoryStore(), nil
	case "sql":
		driver := strings.TrimSpace(os.Getenv("LEITH_DB_DRIVER"))
		if driver == "" {
			driver = "sqlite3"
		}
		dsn := strings.TrimSpace(os.Getenv("LEITH_DB_DSN"))
		if dsn == "" {
			dsn = "./leith.db"
		}
		dbStore, err := store.OpenSQLStore(driver, dsn)
		if err != nil {
			return nil, fmt.Errorf("sql backend init failed (driver=%s dsn=%s): %w", driver, dsn, err)
		}
		return dbStore, nil
	default:
		return nil, fmt.Errorf("unsupported LEITH_STORE_BACKEND=%q", backend)
	}
}

func configuredStoreBackend() string {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("LEITH_STORE_BACKEND")))
	if backend == "" {
		return "memory"
	}
	return backend
}

func devCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Mock-DID, X-Mock-OAuth-Token, X-Mock-Trust-Tier")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func newHandler(db store.Store) http.Handler {
	passkeySvc, err := vfauth.NewService(vfauth.Config{
		RPDisplayName: "Aleth",
		RPID:          "localhost",
		RPOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		},
	})
	if err != nil {
		log.Printf("passkey service init failed: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware_chi.Logger)
	r.Use(devCORS)
	r.Use(middleware.AuthContext)
	r.Use(middleware.RateLimiter(db))

	serverImpl := &Server{
		store:                db,
		contentSvc:           content.NewService(db),
		repSvc:               reputation.NewService(db),
		passkeySvc:           passkeySvc,
		registrationSessions: map[string]passkeyRegistrationSession{},
		loginSessions:        map[string]passkeyLoginSession{},
	}
	api.HandlerFromMux(serverImpl, r)
	return r
}

func main() {
	db, err := initStoreFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	if err := bootstrapVerifiersFromEnv(db); err != nil {
		log.Fatalf("Failed to bootstrap verifiers: %v", err)
	}
	handler := newHandler(db)

	port := 8080
	log.Printf("Store backend initialized: %s", configuredStoreBackend())
	fmt.Printf("Starting VeriFlow API Server on :%d...\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
