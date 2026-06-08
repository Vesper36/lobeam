package main

import (
	"embed"
	"log"
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
	cfg := config.Load()

	if cfg.JWTSecret == "change-me-in-production" {
		log.Println("WARNING: Using default JWT secret. Set LOBEAM_JWT_SECRET for production use.")
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
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
			log.Fatalf("Failed to initialize S3 storage: %v", err)
		}
		store = s3Store
		log.Printf("Using S3 storage: bucket=%s endpoint=%s", cfg.S3Bucket, cfg.S3Endpoint)
	default:
		localStore, err := storage.NewLocalStore(cfg.DataDir + "/transfers")
		if err != nil {
			log.Fatalf("Failed to initialize storage: %v", err)
		}
		store = localStore
	}

	userSvc := user.NewService(database, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshExpiry)
	notifSvc := notify.NewService(cfg)

	staticFS := server.EmbedStaticFS(staticEmbed)

	srv := server.New(cfg, database, store, userSvc, notifSvc, staticFS)

	log.Println("Starting LoBeam server...")
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}