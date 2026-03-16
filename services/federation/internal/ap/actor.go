package ap

// BuildActor returns the ActivityPub Person JSON-LD object for the given user.
// The DID is surfaced via alsoKnownAs to bridge the identity layer.
func BuildActor(domain, username, displayName, did, pubKeyPEM string) map[string]any {
	actorURL := "https://" + domain + "/@" + username
	name := displayName
	if name == "" {
		name = username
	}
	return map[string]any{
		"@context": []any{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1",
		},
		"id":                actorURL,
		"type":              "Person",
		"preferredUsername": username,
		"name":              name,
		"inbox":             actorURL + "/inbox",
		"outbox":            actorURL + "/outbox",
		"followers":         actorURL + "/followers",
		"following":         actorURL + "/following",
		"publicKey": map[string]any{
			"id":           actorURL + "#main-key",
			"owner":        actorURL,
			"publicKeyPem": pubKeyPEM,
		},
		"alsoKnownAs": []string{did},
	}
}
