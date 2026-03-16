// Package ap constructs ActivityPub and WebFinger JSON responses.
package ap

// WebFingerResponse is the JRD (JSON Resource Descriptor) returned by
// GET /.well-known/webfinger?resource=acct:{username}@{domain}.
type WebFingerResponse struct {
	Subject string          `json:"subject"`
	Links   []WebFingerLink `json:"links"`
}

// WebFingerLink is one entry in a JRD links array.
type WebFingerLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}

// BuildWebFinger returns the JRD for the given username on domain.
func BuildWebFinger(domain, username string) WebFingerResponse {
	return WebFingerResponse{
		Subject: "acct:" + username + "@" + domain,
		Links: []WebFingerLink{
			{
				Rel:  "self",
				Type: "application/activity+json",
				Href: "https://" + domain + "/@" + username,
			},
		},
	}
}
