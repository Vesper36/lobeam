package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/vesper/lobeam/internal/config"
	"github.com/vesper/lobeam/internal/db"
	"github.com/vesper/lobeam/internal/notify"
	"github.com/vesper/lobeam/internal/server"
	"github.com/vesper/lobeam/internal/storage"
	"github.com/vesper/lobeam/internal/user"
)

//go:embed static/*
var staticEmbed embed.FS

func main() {
	// Configure structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	if cfg.JWTSecret == "change-me-in-production" {
		slog.Warn("using default JWT secret -- set LOBEAM_JWT_SECRET for production")
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	database, err := db.New(cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Ensure default settings and brand exist
	database.GetSettings()
	database.GetDefaultBrand()

	var store storage.Store
	switch cfg.StorageType {
	case "s3":
		s3Store, err := storage.NewS3Store(storage.S3Config{
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			AccessKeyID:     cfg.S3AccessKey,
			SecretAccessKey: cfg.S3SecretKey,
			Bucket:          cfg.S3Bucket,
			Prefix:          cfg.S3Prefix,
			UseSSL:          cfg.S3UseSSL,
		})
		if err != nil {
			slog.Error("failed to initialize S3 storage", "error", err)
			os.Exit(1)
		}
		store = s3Store
		slog.Info("using S3 storage", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)
	default:
		localStore, err := storage.NewLocalStore(cfg.DataDir + "/transfers")
		if err != nil {
			slog.Error("failed to initialize storage", "error", err)
			os.Exit(1)
		}
		store = localStore
		slog.Info("using local storage", "path", cfg.DataDir+"/transfers")
	}

	userSvc := user.NewService(database, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshExpiry)
	notifSvc := notify.NewService(cfg)

	staticFS := server.EmbedStaticFS(staticEmbed)

	srv := server.New(cfg, database, store, userSvc, notifSvc, staticFS)

	if err := srv.Start(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
