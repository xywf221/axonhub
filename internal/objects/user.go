package objects

// UserInfo contains the authenticated user's profile.
type UserInfo struct {
	ID             GUID               `json:"id"`
	Email          string             `json:"email"`
	FirstName      string             `json:"firstName"`
	LastName       string             `json:"lastName"`
	IsOwner        bool               `json:"isOwner"`
	PreferLanguage string             `json:"preferLanguage"`
	Avatar         *string            `json:"avatar,omitempty"`
	OIDCIdentities []OIDCIdentityInfo `json:"oidcIdentities"`
	HasPassword    bool               `json:"hasPassword"`
}

// OIDCIdentityInfo contains an identity provider association.
type OIDCIdentityInfo struct {
	ID      GUID   `json:"id"`
	IdpName string `json:"idpName"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
	Email   string `json:"email"`
}
