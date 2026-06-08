package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/vesper/lobeam/internal/config"
	"github.com/vesper/lobeam/internal/db"
	"github.com/vesper/lobeam/internal/integrations"
	"github.com/vesper/lobeam/internal/notify"
	"github.com/vesper/lobeam/internal/oidc"
	"github.com/vesper/lobeam/internal/storage"
	"github.com/vesper/lobeam/internal/user"
)

type Server struct {
	cfg        *config.Config
	db         *db.DB
	store      storage.Store
	userSvc    *user.Service
	notify     *notify.Service
	intSvc     *integrations.Service
	hub        *WSHub
	staticFS   fs.FS
	oidcMgr    *oidc.Manager
	oidcStates sync.Map // state -> oidcStateEntry
}

func New(cfg *config.Config, database *db.DB, store storage.Store, userSvc *user.Service, notif *notify.Service, staticFS fs.FS) *Server {
	s := &Server{
		cfg:      cfg,
		db:       database,
		store:    store,
		userSvc:  userSvc,
		notify:   notif,
		intSvc:   integrations.NewService(cfg.SlackWebhookURL, cfg.ZoomWebhookURL, cfg.GoogleWebhookURL, cfg.PublicURL),
		hub:      NewWSHub(),
		staticFS: staticFS,
	}

	// Initialize OIDC manager if providers configured
	if len(cfg.OIDCProviders) > 0 {
		mgr := oidc.NewManager(cfg.PublicURL)
		for _, p := range cfg.OIDCProviders {
			if err := mgr.AddProvider(oidc.ProviderConfig{
				Name:         p.Name,
				DisplayName:  p.DisplayName,
				Issuer:       p.Issuer,
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				Scopes:       p.Scopes,
			}); err != nil {
				fmt.Printf("Warning: failed to add OIDC provider %s: %v\n", p.Name, err)
			}
		}
		s.oidcMgr = mgr
	}

	go s.hub.Run()

	// Cleanup expired OIDC states every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			s.oidcStates.Range(func(key, value any) bool {
				if entry, ok := value.(oidcStateEntry); ok && entry.expires.Before(now) {
					s.oidcStates.Delete(key)
				}
				return true
			})
		}
	}()

	return s
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(0))
	r.Use(chiMiddleware.Compress(5, "text/html", "application/json", "text/css", "application/javascript"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Encryption-Key", "X-Transfer-ID", "X-File-ID", "X-Chunk-Index", "X-Total-Chunks", "X-File-Name", "X-File-Size", "X-Mime-Type", "X-Chunk-Hash", "X-Uploader-Name", "X-Uploader-Email", "X-Uploader-Message", "X-Password"},
		ExposedHeaders:   []string{"X-Upload-Offset", "X-Total-Size", "X-File-Name", "X-File-Size", "X-Encrypted", "Accept-Ranges", "Content-Range"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Brand (public, loaded for UI customization)
		r.Get("/brand", s.handleGetBrand)

		r.Post("/auth/register", s.rateLimitMiddleware(5, s.handleRegister))
		r.Post("/auth/login", s.rateLimitMiddleware(10, s.handleLogin))
		r.Post("/auth/refresh", s.handleRefresh)

		// OIDC / SSO
		r.Get("/auth/oidc/providers", s.handleOIDCProviders)
		r.Get("/auth/oidc/{provider}", s.handleOIDCRedirect)
		r.Get("/auth/oidc/{provider}/callback", s.handleOIDCCallback)

		r.Get("/t/{id}", s.handleGetTransfer)
		r.Get("/t/{id}/files", s.handleGetTransferFiles)
		r.Get("/t/{id}/download/{fileID}", s.handleDownloadFile)
		r.Post("/t/{id}/download/{fileID}", s.handleDownloadFile)
		r.Post("/t/{id}/email", s.handleEmailTransfer)
		r.Post("/t/{id}/share/{platform}", s.handleShareTransfer)
		r.Put("/t/{id}/magnet", s.handleUpdateMagnet)
		r.Get("/clipboard/{id}", s.handleGetClipboard)
		r.Post("/clipboard", s.handleCreateClipboard)

		r.Post("/upload/init", s.handleUploadInit)
		r.Post("/upload/chunk", s.handleUploadChunk)
		r.Post("/upload/complete/{id}", s.handleUploadComplete)

		r.Post("/p2p/create", s.handleCreateP2P)
		r.Get("/p2p/{code}", s.handleGetP2P)
		r.Get("/p2p/ws/{code}", s.handleP2PWebSocket)

		// File requests (public view, auth to create)
		r.Get("/r/{id}", s.handleGetFileRequest)
		r.Post("/r/{id}/submit", s.handleSubmitToFileRequest)
		r.Post("/r/{id}/uploads/init", s.handleInitFileRequestUpload)
		r.Post("/r/{id}/uploads/chunk", s.handleFileRequestUploadChunk)
		r.Post("/r/{id}/uploads/{transferID}/complete", s.handleCompleteFileRequestUpload)

		// Web folders (public access via token)
		r.Get("/f/{token}", s.handleGetWebFolder)
		r.Get("/f/{token}/files", s.handleGetWebFolderFiles)
		r.Post("/f/{token}/upload", s.handleUploadToWebFolder)
		r.Post("/f/{token}/uploads/init", s.handleInitWebFolderUpload)
		r.Post("/f/{token}/uploads/chunk", s.handleWebFolderUploadChunk)
		r.Post("/f/{token}/uploads/{transferID}/complete", s.handleCompleteWebFolderUpload)
		r.Get("/f/{token}/download/{fileID}", s.handleDownloadFromWebFolder)
		r.Post("/f/{token}/download/{fileID}", s.handleDownloadFromWebFolder)

		// Settings (public read, auth to write)
		r.Get("/settings", s.handleGetSettings)

		// ICE config for WebRTC (TURN/STUN)
		r.Get("/ice-config", s.handleGetICEConfig)

		// Web folder password verification
		r.Post("/f/{token}/verify", s.handleVerifyFolderPassword)

		r.Get("/ws", s.handleWebSocket)

		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			r.Get("/transfers", s.handleListTransfers)
			r.Delete("/transfers/{id}", s.handleDeleteTransfer)

			r.Get("/me", s.handleGetProfile)

			// File requests (auth required)
			r.Post("/file-requests", s.handleCreateFileRequest)
			r.Get("/file-requests", s.handleListFileRequests)

			// Web folders (auth required to create)
			r.Post("/folders", s.handleCreateWebFolder)
			r.Get("/folders", s.handleListWebFolders)

			// Settings (admin)
			r.Post("/settings", s.handleUpdateSettings)
			r.Post("/brand", s.handleUpdateBrand)

			r.Group(func(r chi.Router) {
				r.Use(s.adminMiddleware)
				r.Get("/admin/users", s.handleListUsers)
				r.Get("/admin/logs", s.handleGetAuditLogs)
				r.Delete("/admin/users/{id}", s.handleDeleteUser)
				r.Put("/admin/users/{id}", s.handleUpdateUser)
			})
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		dbOK := true
		if err := s.db.Ping(); err != nil {
			dbOK = false
		}
		status := "ok"
		code := http.StatusOK
		if !dbOK {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, map[string]interface{}{
			"status":   status,
			"database": dbOK,
			"version":  "1.0.0",
		})
	})

	// SPA frontend - serve embedded static files
	r.Get("/*", s.serveSPA())

	return r
}

func (s *Server) serveSPA() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API and WebSocket paths should not be handled by SPA
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws") {
			http.NotFound(w, r)
			return
		}

		// Try to serve static file
		if s.staticFS != nil {
			// Remove leading slash
			filePath := strings.TrimPrefix(path, "/")
			if filePath == "" {
				filePath = "index.html"
			}

			f, err := s.staticFS.Open(filePath)
			if err == nil {
				f.Close()
				// Set correct content type
				contentType := "text/html"
				if strings.HasSuffix(filePath, ".js") {
					contentType = "application/javascript"
				} else if strings.HasSuffix(filePath, ".css") {
					contentType = "text/css"
				} else if strings.HasSuffix(filePath, ".svg") {
					contentType = "image/svg+xml"
				} else if strings.HasSuffix(filePath, ".png") {
					contentType = "image/png"
				} else if strings.HasSuffix(filePath, ".ico") {
					contentType = "image/x-icon"
				} else if strings.HasSuffix(filePath, ".json") {
					contentType = "application/json"
				} else if strings.HasSuffix(filePath, ".woff2") {
					contentType = "font/woff2"
				}
				w.Header().Set("Content-Type", contentType)
				w.Header().Set("Cache-Control", "public, max-age=3600")
				http.FileServer(http.FS(s.staticFS)).ServeHTTP(w, r)
				return
			}
		}

		// Fallback to index.html for SPA routing
		if s.staticFS != nil {
			f, err := s.staticFS.Open("index.html")
			if err == nil {
				f.Close()
				w.Header().Set("Content-Type", "text/html")
				http.ServeFileFS(w, r, s.staticFS, "index.html")
				return
			}
		}

		http.NotFound(w, r)
	}
}

func EmbedStaticFS(staticFS embed.FS) fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil
	}
	return sub
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.db.DeleteExpiredTransfers()
			s.db.CleanupExpired()
		}
	}()

	fmt.Printf(`
 __    __   _______  __       __
|  |  |  | |   ____||  |     |  |
|  |_|  | |  |__   |  |     |  |
|       | |   __|  |  |     |  |
|       | |  |____ |  `+"`"+`----.|  `+"`"+`----.
|___|___| |_______||_______||_______|

  LoBeam - Large Object Beam
  Server starting on %s
  Public URL: %s
`, addr, s.cfg.PublicURL)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Router(),
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
