package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Host         string
	Port         int
	PublicURL    string
	DataDir      string

	// Database
	DBPath string

	// Storage
	StorageType string // "local" or "s3"
	MaxFileSize int64   // 0 = unlimited

	// Auth
	JWTSecret        string
	JWTExpiry        time.Duration
	RefreshExpiry    time.Duration

	// Encryption
	DefaultEncryption bool

	// SMTP
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// WebRTC
	STUNServers []string
	TURNServer  string
	TURNUser    string
	TURNPass    string

	// Limits
	MaxChunkSize     int64
	TransferExpiry   time.Duration
	MaxDownloads     int
	AllowAnonymous   bool
}

func Load() *Config {
	dataDir := envStr("LOBEAM_DATA_DIR", "./data")
	dbPath := envStr("LOBEAM_DB_PATH", dataDir+"/lobeam.db")

	_ = os.MkdirAll(dataDir, 0755)

	return &Config{
		Host:         envStr("LOBEAM_HOST", "0.0.0.0"),
		Port:         envInt("LOBEAM_PORT", 8080),
		PublicURL:    envStr("LOBEAM_PUBLIC_URL", "http://localhost:8080"),
		DataDir:      dataDir,

		DBPath: dbPath,

		StorageType: envStr("LOBEAM_STORAGE_TYPE", "local"),
		MaxFileSize: int64(envInt("LOBEAM_MAX_FILE_SIZE", 0)),

		JWTSecret:     envStr("LOBEAM_JWT_SECRET", "change-me-in-production"),
		JWTExpiry:     time.Duration(envInt("LOBEAM_JWT_EXPIRY_HOURS", 24)) * time.Hour,
		RefreshExpiry: time.Duration(envInt("LOBEAM_REFRESH_EXPIRY_DAYS", 30)) * 24 * time.Hour,

		DefaultEncryption: envBool("LOBEAM_DEFAULT_ENCRYPTION", true),

		SMTPHost:     envStr("LOBEAM_SMTP_HOST", ""),
		SMTPPort:     envInt("LOBEAM_SMTP_PORT", 587),
		SMTPUsername: envStr("LOBEAM_SMTP_USERNAME", ""),
		SMTPPassword: envStr("LOBEAM_SMTP_PASSWORD", ""),
		SMTPFrom:     envStr("LOBEAM_SMTP_FROM", ""),

		STUNServers: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
		},
		TURNServer: envStr("LOBEAM_TURN_SERVER", ""),
		TURNUser:   envStr("LOBEAM_TURN_USER", ""),
		TURNPass:   envStr("LOBEAM_TURN_PASS", ""),

		MaxChunkSize:   int64(envInt("LOBEAM_MAX_CHUNK_MB", 5)) * 1024 * 1024,
		TransferExpiry: time.Duration(envInt("LOBEAM_TRANSFER_EXPIRY_HOURS", 24)) * time.Hour,
		MaxDownloads:   envInt("LOBEAM_MAX_DOWNLOADS", 100),
		AllowAnonymous: envBool("LOBEAM_ALLOW_ANONYMOUS", true),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
