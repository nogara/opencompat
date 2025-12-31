// Package auth provides OAuth authentication for the Codex API.
package auth

import "time"

// TokenData represents the OAuth token response from OpenAI.
type TokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
}

// AuthBundle contains the tokens and metadata stored on disk.
type AuthBundle struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id,omitempty"`
	Email        string    `json:"email,omitempty"`
}

// IsExpired returns true if the access token has expired.
func (a *AuthBundle) IsExpired() bool {
	// Consider expired 60 seconds before actual expiry for safety margin
	return time.Now().Add(60 * time.Second).After(a.ExpiresAt)
}

// IsValid returns true if the auth bundle has required tokens.
func (a *AuthBundle) IsValid() bool {
	return a.AccessToken != "" && a.RefreshToken != ""
}

// PKCEData holds the PKCE verifier and challenge for OAuth.
type PKCEData struct {
	Verifier  string
	Challenge string
}

// OAuthError represents an error response from the OAuth server.
type OAuthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}
