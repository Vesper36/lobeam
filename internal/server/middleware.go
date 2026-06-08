package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUsername contextKey = "username"
	ctxRole     contextKey = "role"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		if tokenStr == auth {
			writeError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		claims, err := s.userSvc.ValidateToken(tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxUsername, claims.Username)
		ctx = context.WithValue(ctx, ctxRole, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ctxRole).(string)
		if role != "admin" {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getUserID(r *http.Request) int64 {
	id, _ := r.Context().Value(ctxUserID).(int64)
	return id
}

func getRole(r *http.Request) string {
	role, _ := r.Context().Value(ctxRole).(string)
	return role
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("writeJSON error: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Simple per-IP rate limiter using token bucket
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int // requests per minute
}

type visitor struct {
	tokens   int
	lastSeen time.Time
}

var rateLimiters = struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiter
}{limiters: make(map[string]*rateLimiter)}

func getRateLimiter(rate int) *rateLimiter {
	rateLimiters.mu.Lock()
	defer rateLimiters.mu.Unlock()
	key := fmt.Sprintf("%d", rate)
	if rl, ok := rateLimiters.limiters[key]; ok {
		return rl
	}
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
	}
	rateLimiters.limiters[key] = rl
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists || time.Since(v.lastSeen) > time.Minute {
		rl.visitors[ip] = &visitor{tokens: rl.rate - 1, lastSeen: time.Now()}
		return true
	}

	if v.tokens <= 0 {
		return false
	}
	v.tokens--
	v.lastSeen = time.Now()
	return true
}

func (s *Server) rateLimitMiddleware(requestsPerMin int, next http.HandlerFunc) http.HandlerFunc {
	rl := getRateLimiter(requestsPerMin)
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = strings.Split(forwarded, ",")[0]
		}
		if !rl.allow(ip) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next(w, r)
	}
}
