package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/vesper/mimo/internal/config"
	"github.com/vesper/mimo/internal/db"
	"github.com/vesper/mimo/internal/storage"
	"github.com/vesper/mimo/internal/user"
)

type Server struct {
	cfg       *config.Config
	db        *db.DB
	store     storage.Store
	userSvc   *user.Service
	hub       *WSHub
	staticFS  fs.FS
}

func New(cfg *config.Config, database *db.DB, store storage.Store, userSvc *user.Service, staticFS fs.FS) *Server {
	s := &Server{
		cfg:      cfg,
		db:       database,
		store:    store,
		userSvc:  userSvc,
		hub:      NewWSHub(),
		staticFS: staticFS,
	}
	go s.hub.Run()
	return s
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(0))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Encryption-Key", "X-Transfer-ID", "X-File-ID", "X-Chunk-Index", "X-Total-Chunks", "X-File-Name", "X-File-Size", "X-Mime-Type", "X-Chunk-Hash"},
		ExposedHeaders:   []string{"X-Upload-Offset", "X-Total-Size", "X-File-Name", "X-File-Size", "X-Encrypted"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/refresh", s.handleRefresh)

		r.Get("/t/{id}", s.handleGetTransfer)
		r.Get("/t/{id}/files", s.handleGetTransferFiles)
		r.Get("/t/{id}/download/{fileID}", s.handleDownloadFile)
		r.Post("/t/{id}/download/{fileID}", s.handleDownloadFile)
		r.Get("/clipboard/{id}", s.handleGetClipboard)
		r.Post("/clipboard", s.handleCreateClipboard)

		r.Post("/upload/init", s.handleUploadInit)
		r.Post("/upload/chunk", s.handleUploadChunk)
		r.Post("/upload/complete/{id}", s.handleUploadComplete)

		r.Post("/p2p/create", s.handleCreateP2P)
		r.Get("/p2p/{code}", s.handleGetP2P)
		r.Get("/p2p/ws/{code}", s.handleP2PWebSocket)

		r.Get("/ws", s.handleWebSocket)

		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			r.Get("/transfers", s.handleListTransfers)
			r.Delete("/transfers/{id}", s.handleDeleteTransfer)

			r.Get("/me", s.handleGetProfile)

			r.Group(func(r chi.Router) {
				r.Use(s.adminMiddleware)
				r.Get("/admin/users", s.handleListUsers)
				r.Get("/admin/logs", s.handleGetAuditLogs)
				r.Delete("/admin/users/{id}", s.handleDeleteUser)
			})
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
|       | |  |____ |  ` + "`" + `----.|  ` + "`" + `----.
|___|___| |_______||_______||_______|

  MIMO - Modern Intelligent Managed Operations
  Server starting on %s
  Public URL: %s
`, addr, s.cfg.PublicURL)

	return http.ListenAndServe(addr, s.Router())
}