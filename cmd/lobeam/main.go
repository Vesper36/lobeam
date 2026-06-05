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

	store, err := storage.NewLocalStore(cfg.DataDir + "/transfers")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
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