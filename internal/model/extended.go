package model

import "time"

type Brand struct {
	ID                  int64     `json:"id"`
	Domain              string    `json:"domain"`
	Name                string    `json:"name"`
	LogoURL             string    `json:"logo_url"`
	PrimaryColor        string    `json:"primary_color"`
	BackgroundColor     string    `json:"background_color"`
	AccentColor         string    `json:"accent_color"`
	EmailFrom           string    `json:"email_from"`
	EmailFooter         string    `json:"email_footer"`
	CustomCSS           string    `json:"custom_css"`
	CustomHTML          string    `json:"custom_html"`
	ShowPoweredBy       bool      `json:"show_powered_by"`
	MaxFileSize         int64     `json:"max_file_size"`
	DefaultExpiryHours  int       `json:"default_expiry_hours"`
	DefaultMaxDownloads int       `json:"default_max_downloads"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type FileRequest struct {
	ID            string    `json:"id"`
	UserID        *int64    `json:"user_id,omitempty"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	MaxFileSize   int64     `json:"max_file_size"`
	MaxFiles      int       `json:"max_files"`
	AllowedTypes  string    `json:"allowed_types"`
	CustomFields  string    `json:"custom_fields"`
	RequireFields string    `json:"require_fields"`
	Status        string    `json:"status"` // active, closed
	FileCount     int       `json:"file_count"`
	TotalSize     int64     `json:"total_size"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type WebFolder struct {
	ID            string    `json:"id"`
	Token         string    `json:"token"`
	UserID        *int64    `json:"user_id,omitempty"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Mode          string    `json:"mode"` // upload_only, download_only, both
	PasswordHash  string    `json:"-"`
	FileCount     int       `json:"file_count"`
	TotalSize     int64     `json:"total_size"`
	MaxFileSize   int64     `json:"max_file_size"`
	MaxFiles      int       `json:"max_files"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type WebFolderFile struct {
	ID            string    `json:"id"`
	FolderID      string    `json:"folder_id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	MimeType      string    `json:"mime_type"`
	StoragePath   string    `json:"storage_path"`
	UploaderName  string    `json:"uploader_name"`
	UploaderEmail string    `json:"uploader_email"`
	CreatedAt     time.Time `json:"created_at"`
}

type Settings struct {
	ID              int64     `json:"id"`
	SiteName        string    `json:"site_name"`
	SiteDescription string    `json:"site_description"`
	LogoURL         string    `json:"logo_url"`
	FaviconURL      string    `json:"favicon_url"`
	PrimaryColor    string    `json:"primary_color"`
	AccentColor     string    `json:"accent_color"`
	EmailFrom       string    `json:"email_from"`
	EmailFooter     string    `json:"email_footer"`
	AllowRegister   bool      `json:"allow_register"`
	AllowAnonymous  bool      `json:"allow_anonymous"`
	MaxUploadSize   int64     `json:"max_upload_size"`
	UpdatedAt       time.Time `json:"updated_at"`
}
