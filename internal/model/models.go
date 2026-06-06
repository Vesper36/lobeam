package model

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // admin, member, viewer
	StorageUsed  int64     `json:"storage_used"`
	StorageLimit int64     `json:"storage_limit"`
	OIDCProvider string    `json:"oidc_provider,omitempty"`
	OIDCSub      string    `json:"oidc_sub,omitempty"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Transfer struct {
	ID            string    `json:"id"`
	UserID        *int64    `json:"user_id,omitempty"`
	Name          string    `json:"name"`
	Mode          string    `json:"mode"` // link, p2p, upload
	Status        string    `json:"status"` // pending, uploading, ready, downloading, expired
	FileCount     int       `json:"file_count"`
	TotalSize     int64     `json:"total_size"`
	Encrypted     bool      `json:"encrypted"`
	PasswordHash  string    `json:"-"`
	MaxDownloads  int       `json:"max_downloads"`
	DownloadCount int       `json:"download_count"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Note          string    `json:"note"`
	SenderEmail   string    `json:"sender_email"`
	ReceiverEmail string    `json:"receiver_email"`
	MagnetURI     string    `json:"magnet_uri,omitempty"`
}

type File struct {
	ID          string    `json:"id"`
	TransferID  string    `json:"transfer_id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	ChunkCount  int       `json:"chunk_count"`
	ChunkSize   int64     `json:"chunk_size"`
	SHA256      string    `json:"sha256"`
	StoragePath string    `json:"storage_path"`
	CreatedAt   time.Time `json:"created_at"`
}

type Chunk struct {
	ID         string    `json:"id"`
	FileID     string    `json:"file_id"`
	Index      int       `json:"index"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
	Uploaded   bool      `json:"uploaded"`
	StorageKey string    `json:"storage_key"`
}

type ClipboardEntry struct {
	ID        string    `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Encrypted bool      `json:"encrypted"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type P2PSession struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	CreatorID *int64    `json:"creator_id,omitempty"`
	Status    string    `json:"status"` // waiting, connected, transferring, completed, expired
	Files     []string  `json:"files"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}
