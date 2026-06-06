package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	oidclib "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Provider represents a configured OIDC provider
type Provider struct {
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"-"` // never expose in JSON
	Scopes       []string `json:"scopes"`

	oidcProvider *oidclib.Provider
	oauth2Config *oauth2.Config
}

// OIDCUserInfo holds claims from the OIDC provider
type OIDCUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Manager manages multiple OIDC providers
type Manager struct {
	providers map[string]*Provider
	baseURL   string
}

// NewManager creates a new OIDC manager
func NewManager(baseURL string) *Manager {
	return &Manager{
		providers: make(map[string]*Provider),
		baseURL:   strings.TrimRight(baseURL, "/"),
	}
}

// AddProvider adds an OIDC provider from JSON config
func (m *Manager) AddProvider(cfg ProviderConfig) error {
	if cfg.Name == "" || cfg.Issuer == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return fmt.Errorf("oidc provider %q: name, issuer, client_id, and client_secret are required", cfg.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oidcProv, err := oidclib.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return fmt.Errorf("oidc provider %q: failed to discover issuer %s: %w", cfg.Name, cfg.Issuer, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidclib.ScopeOpenID, "profile", "email"}
	}

	callbackURL := fmt.Sprintf("%s/api/auth/oidc/%s/callback", m.baseURL, cfg.Name)

	p := &Provider{
		Name:         cfg.Name,
		DisplayName:  cfg.DisplayName,
		Issuer:       cfg.Issuer,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       scopes,
		oidcProvider: oidcProv,
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  callbackURL,
			Endpoint:     oidcProv.Endpoint(),
			Scopes:       scopes,
		},
	}

	if p.DisplayName == "" {
		p.DisplayName = cfg.Name
	}

	m.providers[cfg.Name] = p
	return nil
}

// ProviderConfig holds raw configuration for an OIDC provider
type ProviderConfig struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scopes       []string `json:"scopes"`
}

// GetAuthURL returns the OIDC authorization URL for a provider
func (m *Manager) GetAuthURL(providerName, state string) (string, error) {
	p, ok := m.providers[providerName]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerName)
	}
	return p.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

// ExchangeCode exchanges an authorization code for tokens and verifies the ID token
func (m *Manager) ExchangeCode(providerName, code string) (*OIDCUserInfo, error) {
	p, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Exchange code for tokens
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Extract and verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	verifier := p.oidcProvider.Verifier(&oidclib.Config{ClientID: p.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}

	// Extract claims
	var userInfo OIDCUserInfo
	if err := idToken.Claims(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Fallback: fetch userinfo endpoint if email is missing
	if userInfo.Email == "" {
		userInfo2, err := m.fetchUserInfo(p, token)
		if err == nil && userInfo2.Email != "" {
			userInfo.Email = userInfo2.Email
			if userInfo.Name == "" {
				userInfo.Name = userInfo2.Name
			}
		}
	}

	return &userInfo, nil
}

func (m *Manager) fetchUserInfo(p *Provider, token *oauth2.Token) (*OIDCUserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userInfo, err := p.oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}

	var info OIDCUserInfo
	if err := userInfo.Claims(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetProviders returns the list of configured providers (for frontend)
func (m *Manager) GetProviders() []map[string]string {
	var result []map[string]string
	for _, p := range m.providers {
		result = append(result, map[string]string{
			"name":         p.Name,
			"display_name": p.DisplayName,
		})
	}
	return result
}

// HasProviders returns true if any OIDC providers are configured
func (m *Manager) HasProviders() bool {
	return len(m.providers) > 0
}

// ProvidersJSON returns JSON-serialized provider list for the frontend
func (m *Manager) ProvidersJSON() string {
	data, _ := json.Marshal(m.GetProviders())
	return string(data)
}
