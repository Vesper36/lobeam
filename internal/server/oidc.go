package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vesper/lobeam/internal/model"
)

// handleOIDCProviders returns the list of configured OIDC providers for the frontend
func (s *Server) handleOIDCProviders(w http.ResponseWriter, r *http.Request) {
	if s.oidcMgr == nil || !s.oidcMgr.HasProviders() {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, s.oidcMgr.GetProviders())
}

// handleOIDCRedirect initiates the OIDC login flow by redirecting to the provider
func (s *Server) handleOIDCRedirect(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	if s.oidcMgr == nil {
		writeError(w, http.StatusNotFound, "no OIDC providers configured")
		return
	}

	// Generate state token to prevent CSRF
	state := generateOIDCState()

	// Store state with expiry (5 min) in a simple in-memory map
	s.oidcStates.Store(state, oidcStateEntry{
		provider: providerName,
		expires:  time.Now().Add(5 * time.Minute),
	})

	authURL, err := s.oidcMgr.GetAuthURL(providerName, state)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback handles the OIDC callback after the user authenticates
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	// Validate state
	entry, ok := s.oidcStates.LoadAndDelete(state)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}
	stateEntry := entry.(oidcStateEntry)
	if stateEntry.expires.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "state expired")
		return
	}

	if s.oidcMgr == nil {
		writeError(w, http.StatusInternalServerError, "OIDC not configured")
		return
	}

	// Exchange code for user info
	userInfo, err := s.oidcMgr.ExchangeCode(providerName, code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "OIDC authentication failed: "+err.Error())
		return
	}

	if userInfo.Email == "" {
		writeError(w, http.StatusBadRequest, "OIDC provider did not return an email address")
		return
	}

	// Find or create user
	user, err := s.db.GetUserByOIDC(providerName, userInfo.Sub)
	if err != nil {
		// No OIDC-linked user. Try to find by email.
		user, err = s.db.GetUserByEmail(userInfo.Email)
		if err != nil {
			// New user - create account
			username := generateUsernameFromEmail(userInfo.Email)
			// Ensure unique username
			baseUsername := username
			for i := 1; ; i++ {
				if _, err := s.db.GetUserByUsername(username); err != nil {
					break // username is available
				}
				username = fmt.Sprintf("%s%d", baseUsername, i)
			}

			role := "member"
			users, _ := s.db.ListUsers(1, 0)
			if len(users) == 0 {
				role = "admin"
			}

			user = &model.User{
				Username:     username,
				Email:        userInfo.Email,
				PasswordHash: "", // no password for OIDC users
				Role:         role,
				OIDCProvider: providerName,
				OIDCSub:      userInfo.Sub,
				AvatarURL:    userInfo.Picture,
			}
			if err := s.db.CreateUser(user); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
				return
			}
		} else {
			// Link existing user to OIDC provider
			user.OIDCProvider = providerName
			user.OIDCSub = userInfo.Sub
			if userInfo.Picture != "" {
				user.AvatarURL = userInfo.Picture
			}
			s.db.LinkUserOIDC(user.ID, providerName, userInfo.Sub, userInfo.Picture)
		}
	}

	// Generate JWT tokens
	accessToken, err := s.userSvc.GenerateToken(user, s.cfg.JWTExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	refreshToken, err := s.userSvc.GenerateToken(user, s.cfg.RefreshExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	// Audit log
	s.db.CreateAuditLog(&user.ID, "login", "oidc",
		fmt.Sprintf("OIDC login via %s", providerName), r.RemoteAddr)

	// Redirect to frontend with tokens in URL fragment (not query string, for security)
	redirectURL := fmt.Sprintf("/auth/callback#access_token=%s&refresh_token=%s", accessToken, refreshToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// oidcStateEntry stores OIDC state for CSRF protection
type oidcStateEntry struct {
	provider string
	expires  time.Time
}

func generateOIDCState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateUsernameFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	username := parts[0]
	// Clean up username
	username = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, strings.ToLower(username))
	if len(username) > 20 {
		username = username[:20]
	}
	if username == "" {
		username = "user"
	}
	return username
}
