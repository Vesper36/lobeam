package db

import (
	"database/sql"
	"errors"

	"github.com/vesper/lobeam/internal/model"
)

// ---- Brand ----

func (db *DB) GetBrandByDomain(domain string) (*model.Brand, error) {
	b := &model.Brand{}
	var showPoweredBy int
	var domainNullable sql.NullString
	err := db.conn.QueryRow(
		`SELECT id, domain, name, logo_url, primary_color, background_color, accent_color, email_from, email_footer, custom_css, custom_html, show_powered_by, max_file_size, default_expiry_hours, default_max_downloads, created_at, updated_at FROM brands WHERE domain = ? OR domain IS NULL ORDER BY domain DESC LIMIT 1`,
		domain,
	).Scan(&b.ID, &domainNullable, &b.Name, &b.LogoURL, &b.PrimaryColor, &b.BackgroundColor, &b.AccentColor, &b.EmailFrom, &b.EmailFooter, &b.CustomCSS, &b.CustomHTML, &showPoweredBy, &b.MaxFileSize, &b.DefaultExpiryHours, &b.DefaultMaxDownloads, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if domainNullable.Valid {
		b.Domain = domainNullable.String
	}
	b.ShowPoweredBy = showPoweredBy != 0
	return b, nil
}

func (db *DB) GetDefaultBrand() (*model.Brand, error) {
	b := &model.Brand{}
	var showPoweredBy int
	var domainNullable sql.NullString
	err := db.conn.QueryRow(
		`SELECT id, domain, name, logo_url, primary_color, background_color, accent_color, email_from, email_footer, custom_css, custom_html, show_powered_by, max_file_size, default_expiry_hours, default_max_downloads, created_at, updated_at FROM brands WHERE domain IS NULL OR domain = '' LIMIT 1`,
	).Scan(&b.ID, &domainNullable, &b.Name, &b.LogoURL, &b.PrimaryColor, &b.BackgroundColor, &b.AccentColor, &b.EmailFrom, &b.EmailFooter, &b.CustomCSS, &b.CustomHTML, &showPoweredBy, &b.MaxFileSize, &b.DefaultExpiryHours, &b.DefaultMaxDownloads, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create default brand
			_, err = db.conn.Exec(`INSERT INTO settings DEFAULT VALUES`)
			if err != nil {
				_, _ = db.conn.Exec(`INSERT INTO brands (domain, name) VALUES (NULL, 'LoBeam')`)
			}
			return &model.Brand{
				Name:               "LoBeam",
				PrimaryColor:       "#7c3aed",
				BackgroundColor:    "#09090b",
				AccentColor:        "#4f46e5",
				ShowPoweredBy:      true,
				DefaultExpiryHours: 24,
				DefaultMaxDownloads: 100,
			}, nil
		}
		return nil, err
	}
	if domainNullable.Valid {
		b.Domain = domainNullable.String
	}
	b.ShowPoweredBy = showPoweredBy != 0
	return b, nil
}

func (db *DB) UpsertBrand(b *model.Brand) error {
	if b.Domain == "" {
		_, err := db.conn.Exec(
			`UPDATE brands SET name=?, logo_url=?, primary_color=?, background_color=?, accent_color=?, email_from=?, email_footer=?, custom_css=?, custom_html=?, show_powered_by=?, max_file_size=?, default_expiry_hours=?, default_max_downloads=?, updated_at=CURRENT_TIMESTAMP WHERE domain IS NULL OR domain = ''`,
			b.Name, b.LogoURL, b.PrimaryColor, b.BackgroundColor, b.AccentColor, b.EmailFrom, b.EmailFooter, b.CustomCSS, b.CustomHTML, boolToInt(b.ShowPoweredBy), b.MaxFileSize, b.DefaultExpiryHours, b.DefaultMaxDownloads,
		)
		return err
	}
	_, err := db.conn.Exec(
		`INSERT INTO brands (domain, name, logo_url, primary_color, background_color, accent_color, email_from, email_footer, custom_css, custom_html, show_powered_by, max_file_size, default_expiry_hours, default_max_downloads)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(domain) DO UPDATE SET name=excluded.name, logo_url=excluded.logo_url, primary_color=excluded.primary_color, background_color=excluded.background_color, accent_color=excluded.accent_color, email_from=excluded.email_from, email_footer=excluded.email_footer, custom_css=excluded.custom_css, custom_html=excluded.custom_html, show_powered_by=excluded.show_powered_by, max_file_size=excluded.max_file_size, default_expiry_hours=excluded.default_expiry_hours, default_max_downloads=excluded.default_max_downloads, updated_at=CURRENT_TIMESTAMP`,
		b.Domain, b.Name, b.LogoURL, b.PrimaryColor, b.BackgroundColor, b.AccentColor, b.EmailFrom, b.EmailFooter, b.CustomCSS, b.CustomHTML, boolToInt(b.ShowPoweredBy), b.MaxFileSize, b.DefaultExpiryHours, b.DefaultMaxDownloads,
	)
	return err
}

// ---- File Request ----

func (db *DB) CreateFileRequest(r *model.FileRequest) error {
	var userID interface{}
	if r.UserID != nil {
		userID = *r.UserID
	}
	_, err := db.conn.Exec(
		`INSERT INTO file_requests (id, user_id, title, description, max_file_size, max_files, allowed_types, custom_fields, require_fields, status, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, userID, r.Title, r.Description, r.MaxFileSize, r.MaxFiles, r.AllowedTypes, r.CustomFields, r.RequireFields, r.Status, r.ExpiresAt,
	)
	return err
}

func (db *DB) GetFileRequest(id string) (*model.FileRequest, error) {
	r := &model.FileRequest{}
	var userID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT id, user_id, title, description, max_file_size, max_files, allowed_types, custom_fields, require_fields, status, file_count, total_size, expires_at, created_at FROM file_requests WHERE id = ?`, id,
	).Scan(&r.ID, &userID, &r.Title, &r.Description, &r.MaxFileSize, &r.MaxFiles, &r.AllowedTypes, &r.CustomFields, &r.RequireFields, &r.Status, &r.FileCount, &r.TotalSize, &r.ExpiresAt, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		v := userID.Int64
		r.UserID = &v
	}
	return r, nil
}

func (db *DB) UpdateFileRequestCounts(id string) error {
	_, err := db.conn.Exec(`
		UPDATE file_requests SET
			file_count = (SELECT COUNT(*) FROM transfers WHERE receiver_email != '' AND mode = 'request' AND sender_email = ?),
			total_size = (SELECT COALESCE(SUM(total_size), 0) FROM transfers WHERE mode = 'request' AND sender_email = ?)
		WHERE id = ?`, id, id, id)
	return err
}

func (db *DB) ListFileRequests(userID int64) ([]*model.FileRequest, error) {
	rows, err := db.conn.Query(
		`SELECT id, title, description, max_file_size, max_files, status, file_count, total_size, expires_at, created_at FROM file_requests WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*model.FileRequest
	for rows.Next() {
		r := &model.FileRequest{}
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.MaxFileSize, &r.MaxFiles, &r.Status, &r.FileCount, &r.TotalSize, &r.ExpiresAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		requests = append(requests, r)
	}
	return requests, nil
}

// ---- Web Folder ----

func (db *DB) CreateWebFolder(f *model.WebFolder) error {
	var userID interface{}
	if f.UserID != nil {
		userID = *f.UserID
	}
	_, err := db.conn.Exec(
		`INSERT INTO web_folders (id, token, user_id, name, description, mode, password_hash, max_file_size, max_files, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Token, userID, f.Name, f.Description, f.Mode, f.PasswordHash, f.MaxFileSize, f.MaxFiles, f.ExpiresAt,
	)
	return err
}

func (db *DB) GetWebFolderByToken(token string) (*model.WebFolder, error) {
	f := &model.WebFolder{}
	var userID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT id, token, user_id, name, description, mode, password_hash, file_count, total_size, max_file_size, max_files, expires_at, created_at FROM web_folders WHERE token = ? AND expires_at > CURRENT_TIMESTAMP`, token,
	).Scan(&f.ID, &f.Token, &userID, &f.Name, &f.Description, &f.Mode, &f.PasswordHash, &f.FileCount, &f.TotalSize, &f.MaxFileSize, &f.MaxFiles, &f.ExpiresAt, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		v := userID.Int64
		f.UserID = &v
	}
	return f, nil
}

func (db *DB) AddWebFolderFile(f *model.WebFolderFile) error {
	_, err := db.conn.Exec(
		`INSERT INTO web_folder_files (id, folder_id, name, size, mime_type, storage_path, uploader_name, uploader_email)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.FolderID, f.Name, f.Size, f.MimeType, f.StoragePath, f.UploaderName, f.UploaderEmail,
	)
	return err
}

func (db *DB) GetWebFolderFiles(folderID string) ([]*model.WebFolderFile, error) {
	rows, err := db.conn.Query(
		`SELECT id, folder_id, name, size, mime_type, storage_path, uploader_name, uploader_email, created_at FROM web_folder_files WHERE folder_id = ? ORDER BY created_at DESC`, folderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*model.WebFolderFile
	for rows.Next() {
		f := &model.WebFolderFile{}
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.StoragePath, &f.UploaderName, &f.UploaderEmail, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (db *DB) GetWebFolderFile(id string) (*model.WebFolderFile, error) {
	f := &model.WebFolderFile{}
	err := db.conn.QueryRow(
		`SELECT id, folder_id, name, size, mime_type, storage_path, uploader_name, uploader_email, created_at FROM web_folder_files WHERE id = ?`, id,
	).Scan(&f.ID, &f.FolderID, &f.Name, &f.Size, &f.MimeType, &f.StoragePath, &f.UploaderName, &f.UploaderEmail, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (db *DB) UpdateWebFolderCounts(folderID string) error {
	_, err := db.conn.Exec(`
		UPDATE web_folders SET
			file_count = (SELECT COUNT(*) FROM web_folder_files WHERE folder_id = ?),
			total_size = (SELECT COALESCE(SUM(size), 0) FROM web_folder_files WHERE folder_id = ?)
		WHERE id = ?`, folderID, folderID, folderID)
	return err
}

func (db *DB) ListWebFolders(userID int64) ([]*model.WebFolder, error) {
	rows, err := db.conn.Query(
		`SELECT id, token, name, description, mode, file_count, total_size, max_file_size, max_files, expires_at, created_at FROM web_folders WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []*model.WebFolder
	for rows.Next() {
		f := &model.WebFolder{}
		if err := rows.Scan(&f.ID, &f.Token, &f.Name, &f.Description, &f.Mode, &f.FileCount, &f.TotalSize, &f.MaxFileSize, &f.MaxFiles, &f.ExpiresAt, &f.CreatedAt); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, nil
}

// ---- Settings ----

func (db *DB) GetSettings() (*model.Settings, error) {
	s := &model.Settings{}
	var allowReg, allowAnon int
	err := db.conn.QueryRow(
		`SELECT id, site_name, site_description, logo_url, favicon_url, primary_color, accent_color, email_from, email_footer, allow_register, allow_anonymous, max_upload_size, updated_at FROM settings WHERE id = 1`,
	).Scan(&s.ID, &s.SiteName, &s.SiteDescription, &s.LogoURL, &s.FaviconURL, &s.PrimaryColor, &s.AccentColor, &s.EmailFrom, &s.EmailFooter, &allowReg, &allowAnon, &s.MaxUploadSize, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, _ = db.conn.Exec(`INSERT OR IGNORE INTO settings (id) VALUES (1)`)
			return db.GetSettings()
		}
		return nil, err
	}
	s.AllowRegister = allowReg != 0
	s.AllowAnonymous = allowAnon != 0
	return s, nil
}

func (db *DB) UpdateSettings(s *model.Settings) error {
	_, err := db.conn.Exec(
		`UPDATE settings SET site_name=?, site_description=?, logo_url=?, favicon_url=?, primary_color=?, accent_color=?, email_from=?, email_footer=?, allow_register=?, allow_anonymous=?, max_upload_size=?, updated_at=CURRENT_TIMESTAMP WHERE id=1`,
		s.SiteName, s.SiteDescription, s.LogoURL, s.FaviconURL, s.PrimaryColor, s.AccentColor, s.EmailFrom, s.EmailFooter, boolToInt(s.AllowRegister), boolToInt(s.AllowAnonymous), s.MaxUploadSize,
	)
	return err
}
