package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/edgard/opencompat/internal/config"
)

// Store manages token persistence and refresh.
type Store struct {
	bundle   *AuthBundle
	clientID string
}

// NewStore creates a new token store.
func NewStore() *Store {
	return &Store{
		clientID: config.DefaultOAuthClientID,
	}
}

// SetClientID sets the OAuth client ID for token refresh.
func (s *Store) SetClientID(clientID string) {
	s.clientID = clientID
}

// Load reads the auth bundle from disk.
func (s *Store) Load() error {
	path := config.AuthFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("not logged in - run 'opencompat login' first")
		}
		return fmt.Errorf("failed to read auth file: %w", err)
	}

	var bundle AuthBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("failed to parse auth file: %w", err)
	}

	s.bundle = &bundle
	return nil
}

// Save writes the auth bundle to disk.
func (s *Store) Save() error {
	if s.bundle == nil {
		return errors.New("no auth bundle to save")
	}

	if err := config.EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	data, err := json.MarshalIndent(s.bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth bundle: %w", err)
	}

	path := config.AuthFilePath()
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}

	return nil
}

// GetBundle returns the current auth bundle, refreshing if needed.
func (s *Store) GetBundle() (*AuthBundle, error) {
	if s.bundle == nil {
		if err := s.Load(); err != nil {
			return nil, err
		}
	}

	if s.bundle.IsExpired() {
		if err := s.Refresh(); err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
	}

	return s.bundle, nil
}

// SetBundle updates the auth bundle and extracts metadata from tokens.
func (s *Store) SetBundle(tokens *TokenData) error {
	expiresAt := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	bundle := &AuthBundle{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IDToken:      tokens.IDToken,
		ExpiresAt:    expiresAt,
	}

	// Extract account ID from ID token first (preferred), then access token
	if tokens.IDToken != "" {
		if accountID, err := ExtractAccountID(tokens.IDToken); err == nil {
			bundle.AccountID = accountID
		}
	}
	if bundle.AccountID == "" {
		if accountID, err := ExtractAccountID(tokens.AccessToken); err == nil {
			bundle.AccountID = accountID
		}
	}

	// Extract email from ID token if available, otherwise from access token
	if tokens.IDToken != "" {
		if email, err := ExtractEmail(tokens.IDToken); err == nil {
			bundle.Email = email
		}
	}
	if bundle.Email == "" {
		if email, err := ExtractEmail(tokens.AccessToken); err == nil {
			bundle.Email = email
		}
	}

	s.bundle = bundle
	return s.Save()
}

// Refresh exchanges the refresh token for a new access token.
func (s *Store) Refresh() error {
	if s.bundle == nil || s.bundle.RefreshToken == "" {
		return errors.New("no refresh token available")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {s.bundle.RefreshToken},
		"client_id":     {s.clientID},
	}

	req, err := http.NewRequest("POST", config.OAuthTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthError
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Error != "" {
			return fmt.Errorf("token refresh failed: %s - %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	var tokens TokenData
	if err := json.Unmarshal(body, &tokens); err != nil {
		return err
	}

	// Keep the old refresh token if a new one wasn't provided
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = s.bundle.RefreshToken
	}

	return s.SetBundle(&tokens)
}

// Clear removes the stored auth bundle.
func (s *Store) Clear() error {
	s.bundle = nil
	path := config.AuthFilePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsLoggedIn returns true if valid credentials exist.
func (s *Store) IsLoggedIn() bool {
	if s.bundle == nil {
		if err := s.Load(); err != nil {
			return false
		}
	}
	return s.bundle.IsValid()
}
