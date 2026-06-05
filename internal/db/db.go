package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/vesper/lobeam/internal/model"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(1)
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member',
		storage_used INTEGER NOT NULL DEFAULT 0,
		storage_limit INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS transfers (
		id TEXT PRIMARY KEY,
		user_id INTEGER,
		name TEXT NOT NULL,
		mode TEXT NOT NULL DEFAULT 'link',
		status TEXT NOT NULL DEFAULT 'pending',
		file_count INTEGER NOT NULL DEFAULT 0,
		total_size INTEGER NOT NULL DEFAULT 0,
		encrypted INTEGER NOT NULL DEFAULT 0,
		password_hash TEXT NOT NULL DEFAULT '',
		max_downloads INTEGER NOT NULL DEFAULT 100,
		download_count INTEGER NOT NULL DEFAULT 0,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		note TEXT NOT NULL DEFAULT '',
		sender_email TEXT NOT NULL DEFAULT '',
		receiver_email TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		transfer_id TEXT NOT NULL REFERENCES transfers(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		size INTEGER NOT NULL,
		mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
		chunk_count INTEGER NOT NULL DEFAULT 0,
		chunk_size INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT '',
		storage_path TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id TEXT PRIMARY KEY,
		file_id TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
		idx INTEGER NOT NULL,
		size INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT '',
		uploaded INTEGER NOT NULL DEFAULT 0,
		storage_key TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS clipboard (
		id TEXT PRIMARY KEY,
		user_id INTEGER,
		content TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT '',
		encrypted INTEGER NOT NULL DEFAULT 0,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS p2p_sessions (
		id TEXT PRIMARY KEY,
		code TEXT UNIQUE NOT NULL,
		creator_id INTEGER,
		status TEXT NOT NULL DEFAULT 'waiting',
		files TEXT NOT NULL DEFAULT '[]',
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		action TEXT NOT NULL,
		resource TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT '',
		ip TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS brands (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT UNIQUE,
		name TEXT NOT NULL DEFAULT 'LoBeam',
		logo_url TEXT NOT NULL DEFAULT '',
		primary_color TEXT NOT NULL DEFAULT '#7c3aed',
		background_color TEXT NOT NULL DEFAULT '#09090b',
		accent_color TEXT NOT NULL DEFAULT '#4f46e5',
		email_from TEXT NOT NULL DEFAULT '',
		email_footer TEXT NOT NULL DEFAULT '',
		custom_css TEXT NOT NULL DEFAULT '',
		custom_html TEXT NOT NULL DEFAULT '',
		show_powered_by INTEGER NOT NULL DEFAULT 1,
		max_file_size INTEGER NOT NULL DEFAULT 0,
		default_expiry_hours INTEGER NOT NULL DEFAULT 24,
		default_max_downloads INTEGER NOT NULL DEFAULT 100,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS file_requests (
		id TEXT PRIMARY KEY,
		user_id INTEGER,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		max_file_size INTEGER NOT NULL DEFAULT 0,
		max_files INTEGER NOT NULL DEFAULT 0,
		allowed_types TEXT NOT NULL DEFAULT '',
		custom_fields TEXT NOT NULL DEFAULT '[]',
		require_fields TEXT NOT NULL DEFAULT '[]',
		status TEXT NOT NULL DEFAULT 'active',
		file_count INTEGER NOT NULL DEFAULT 0,
		total_size INTEGER NOT NULL DEFAULT 0,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS web_folders (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		user_id INTEGER,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		mode TEXT NOT NULL DEFAULT 'both',
		password_hash TEXT NOT NULL DEFAULT '',
		file_count INTEGER NOT NULL DEFAULT 0,
		total_size INTEGER NOT NULL DEFAULT 0,
		max_file_size INTEGER NOT NULL DEFAULT 0,
		max_files INTEGER NOT NULL DEFAULT 0,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS web_folder_files (
		id TEXT PRIMARY KEY,
		folder_id TEXT NOT NULL REFERENCES web_folders(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		size INTEGER NOT NULL,
		mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
		storage_path TEXT NOT NULL DEFAULT '',
		uploader_name TEXT NOT NULL DEFAULT '',
		uploader_email TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		site_name TEXT NOT NULL DEFAULT 'LoBeam',
		site_description TEXT NOT NULL DEFAULT 'Self-hosted file transfer',
		logo_url TEXT NOT NULL DEFAULT '',
		favicon_url TEXT NOT NULL DEFAULT '',
		primary_color TEXT NOT NULL DEFAULT '#7c3aed',
		accent_color TEXT NOT NULL DEFAULT '#4f46e5',
		email_from TEXT NOT NULL DEFAULT '',
		email_footer TEXT NOT NULL DEFAULT '',
		allow_register INTEGER NOT NULL DEFAULT 1,
		allow_anonymous INTEGER NOT NULL DEFAULT 1,
		max_upload_size INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_transfers_user ON transfers(user_id);
	CREATE INDEX IF NOT EXISTS idx_transfers_status ON transfers(status);
	CREATE INDEX IF NOT EXISTS idx_transfers_expires ON transfers(expires_at);
	CREATE INDEX IF NOT EXISTS idx_files_transfer ON files(transfer_id);
	CREATE INDEX IF NOT EXISTS idx_chunks_file ON chunks(file_id);
	CREATE INDEX IF NOT EXISTS idx_clipboard_expires ON clipboard(expires_at);
	CREATE INDEX IF NOT EXISTS idx_p2p_code ON p2p_sessions(code);
	CREATE INDEX IF NOT EXISTS idx_folder_token ON web_folders(token);
	CREATE INDEX IF NOT EXISTS idx_request_status ON file_requests(status);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// ---- Transfer ----

func (db *DB) CreateTransfer(t *model.Transfer) error {
	var userID interface{}
	if t.UserID != nil {
		userID = *t.UserID
	}
	_, err := db.conn.Exec(
		`INSERT INTO transfers (id, user_id, name, mode, status, encrypted, password_hash, max_downloads, expires_at, note, sender_email, receiver_email)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, userID, t.Name, t.Mode, t.Status, boolToInt(t.Encrypted), t.PasswordHash, t.MaxDownloads, t.ExpiresAt, t.Note, t.SenderEmail, t.ReceiverEmail,
	)
	return err
}

func (db *DB) GetTransfer(id string) (*model.Transfer, error) {
	t := &model.Transfer{}
	var enc int
	var completed sql.NullTime
	err := db.conn.QueryRow(
		`SELECT id, user_id, name, mode, status, file_count, total_size, encrypted, password_hash, max_downloads, download_count, expires_at, created_at, completed_at, note, sender_email, receiver_email
		 FROM transfers WHERE id = ?`, id,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Mode, &t.Status, &t.FileCount, &t.TotalSize, &enc, &t.PasswordHash, &t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.CreatedAt, &completed, &t.Note, &t.SenderEmail, &t.ReceiverEmail)
	if err != nil {
		return nil, err
	}
	t.Encrypted = enc != 0
	if completed.Valid {
		t.CompletedAt = &completed.Time
	}
	return t, nil
}

func (db *DB) UpdateTransferStatus(id, status string) error {
	_, err := db.conn.Exec(`UPDATE transfers SET status = ? WHERE id = ?`, status, id)
	return err
}

func (db *DB) CompleteTransfer(id string) error {
	_, err := db.conn.Exec(`UPDATE transfers SET status = 'ready', completed_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (db *DB) IncrementDownload(id string) error {
	_, err := db.conn.Exec(`UPDATE transfers SET download_count = download_count + 1 WHERE id = ?`, id)
	return err
}

func (db *DB) UpdateTransferCounts(id string) error {
	_, err := db.conn.Exec(`
		UPDATE transfers SET
			file_count = (SELECT COUNT(*) FROM files WHERE transfer_id = ?),
			total_size = (SELECT COALESCE(SUM(size), 0) FROM files WHERE transfer_id = ?)
		WHERE id = ?`, id, id, id)
	return err
}

func (db *DB) ListTransfers(userID int64, limit, offset int) ([]*model.Transfer, error) {
	rows, err := db.conn.Query(
		`SELECT id, user_id, name, mode, status, file_count, total_size, encrypted, max_downloads, download_count, expires_at, created_at, note
		 FROM transfers WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []*model.Transfer
	for rows.Next() {
		t := &model.Transfer{}
		var enc int
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Mode, &t.Status, &t.FileCount, &t.TotalSize, &enc, &t.MaxDownloads, &t.DownloadCount, &t.ExpiresAt, &t.CreatedAt, &t.Note); err != nil {
			return nil, err
		}
		t.Encrypted = enc != 0
		transfers = append(transfers, t)
	}
	return transfers, nil
}

func (db *DB) DeleteExpiredTransfers() (int64, error) {
	res, err := db.conn.Exec(`DELETE FROM transfers WHERE expires_at < CURRENT_TIMESTAMP AND status != 'expired'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- File ----

func (db *DB) CreateFile(f *model.File) error {
	_, err := db.conn.Exec(
		`INSERT INTO files (id, transfer_id, name, size, mime_type, chunk_count, chunk_size, sha256, storage_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.TransferID, f.Name, f.Size, f.MimeType, f.ChunkCount, f.ChunkSize, f.SHA256, f.StoragePath,
	)
	return err
}

func (db *DB) GetFilesByTransfer(transferID string) ([]*model.File, error) {
	rows, err := db.conn.Query(
		`SELECT id, transfer_id, name, size, mime_type, chunk_count, chunk_size, sha256, storage_path, created_at
		 FROM files WHERE transfer_id = ? ORDER BY created_at`, transferID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*model.File
	for rows.Next() {
		f := &model.File{}
		if err := rows.Scan(&f.ID, &f.TransferID, &f.Name, &f.Size, &f.MimeType, &f.ChunkCount, &f.ChunkSize, &f.SHA256, &f.StoragePath, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (db *DB) GetFile(id string) (*model.File, error) {
	f := &model.File{}
	err := db.conn.QueryRow(
		`SELECT id, transfer_id, name, size, mime_type, chunk_count, chunk_size, sha256, storage_path, created_at
		 FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.TransferID, &f.Name, &f.Size, &f.MimeType, &f.ChunkCount, &f.ChunkSize, &f.SHA256, &f.StoragePath, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// ---- Chunk ----

func (db *DB) CreateChunk(c *model.Chunk) error {
	_, err := db.conn.Exec(
		`INSERT INTO chunks (id, file_id, idx, size, sha256, uploaded, storage_key)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.FileID, c.Index, c.Size, c.SHA256, boolToInt(c.Uploaded), c.StorageKey,
	)
	return err
}

func (db *DB) MarkChunkUploaded(id string) error {
	_, err := db.conn.Exec(`UPDATE chunks SET uploaded = 1 WHERE id = ?`, id)
	return err
}

// GetChunk returns a single chunk by file_id and chunk index
func (db *DB) GetChunk(fileID string, index int) (*model.Chunk, error) {
	c := &model.Chunk{}
	var up int
	err := db.conn.QueryRow(
		`SELECT id, file_id, idx, size, sha256, uploaded, storage_key
		 FROM chunks WHERE file_id = ? AND idx = ?`, fileID, index,
	).Scan(&c.ID, &c.FileID, &c.Index, &c.Size, &c.SHA256, &up, &c.StorageKey)
	if err != nil {
		return nil, err
	}
	c.Uploaded = up != 0
	return c, nil
}

func (db *DB) GetChunksByFile(fileID string) ([]*model.Chunk, error) {
	rows, err := db.conn.Query(
		`SELECT id, file_id, idx, size, sha256, uploaded, storage_key
		 FROM chunks WHERE file_id = ? ORDER BY idx`, fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*model.Chunk
	for rows.Next() {
		c := &model.Chunk{}
		var up int
		if err := rows.Scan(&c.ID, &c.FileID, &c.Index, &c.Size, &c.SHA256, &up, &c.StorageKey); err != nil {
			return nil, err
		}
		c.Uploaded = up != 0
		chunks = append(chunks, c)
	}
	return chunks, nil
}

func (db *DB) GetUploadedChunkIndexes(fileID string) ([]int, error) {
	rows, err := db.conn.Query(`SELECT idx FROM chunks WHERE file_id = ? AND uploaded = 1 ORDER BY idx`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []int
	for rows.Next() {
		var idx int
		if err := rows.Scan(&idx); err != nil {
			return nil, err
		}
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

// ---- Clipboard ----

func (db *DB) CreateClipboardEntry(e *model.ClipboardEntry) error {
	var userID interface{}
	if e.UserID != nil {
		userID = *e.UserID
	}
	_, err := db.conn.Exec(
		`INSERT INTO clipboard (id, user_id, content, language, encrypted, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, userID, e.Content, e.Language, boolToInt(e.Encrypted), e.ExpiresAt,
	)
	return err
}

func (db *DB) GetClipboardEntry(id string) (*model.ClipboardEntry, error) {
	e := &model.ClipboardEntry{}
	var userID sql.NullInt64
	var enc int
	err := db.conn.QueryRow(
		`SELECT id, user_id, content, language, encrypted, expires_at, created_at FROM clipboard WHERE id = ?`, id,
	).Scan(&e.ID, &userID, &e.Content, &e.Language, &enc, &e.ExpiresAt, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		v := userID.Int64
		e.UserID = &v
	}
	e.Encrypted = enc != 0
	return e, nil
}

// ---- P2P Session ----

func (db *DB) CreateP2PSession(s *model.P2PSession) error {
	var creatorID interface{}
	if s.CreatorID != nil {
		creatorID = *s.CreatorID
	}
	_, err := db.conn.Exec(
		`INSERT INTO p2p_sessions (id, code, creator_id, status, files, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, s.Code, creatorID, s.Status, "[]", s.ExpiresAt,
	)
	return err
}

func (db *DB) GetP2PSessionByCode(code string) (*model.P2PSession, error) {
	s := &model.P2PSession{}
	var creatorID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT id, code, creator_id, status, files, expires_at, created_at FROM p2p_sessions WHERE code = ? AND expires_at > CURRENT_TIMESTAMP`, code,
	).Scan(&s.ID, &s.Code, &creatorID, &s.Status, &s.Files, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	if creatorID.Valid {
		v := creatorID.Int64
		s.CreatorID = &v
	}
	return s, nil
}

func (db *DB) UpdateP2PSessionStatus(id, status string) error {
	_, err := db.conn.Exec(`UPDATE p2p_sessions SET status = ? WHERE id = ?`, status, id)
	return err
}

// ---- Audit Log ----

func (db *DB) CreateAuditLog(userID *int64, action, resource, detail, ip string) error {
	_, err := db.conn.Exec(
		`INSERT INTO audit_logs (user_id, action, resource, detail, ip) VALUES (?, ?, ?, ?, ?)`,
		userID, action, resource, detail, ip,
	)
	return err
}

func (db *DB) GetAuditLogs(limit, offset int) ([]*model.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT id, user_id, action, resource, detail, ip, created_at FROM audit_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*model.AuditLog
	for rows.Next() {
		l := &model.AuditLog{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.Action, &l.Resource, &l.Detail, &l.IP, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// ---- User ----

func (db *DB) CreateUser(u *model.User) error {
	res, err := db.conn.Exec(
		`INSERT INTO users (username, email, password_hash, role, storage_limit)
		 VALUES (?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.PasswordHash, u.Role, u.StorageLimit,
	)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) GetUserByID(id int64) (*model.User, error) {
	u := &model.User{}
	err := db.conn.QueryRow(
		`SELECT id, username, email, password_hash, role, storage_used, storage_limit, created_at, updated_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.StorageUsed, &u.StorageLimit, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (db *DB) GetUserByUsername(username string) (*model.User, error) {
	u := &model.User{}
	err := db.conn.QueryRow(
		`SELECT id, username, email, password_hash, role, storage_used, storage_limit, created_at, updated_at FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.StorageUsed, &u.StorageLimit, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (db *DB) GetUserByEmail(email string) (*model.User, error) {
	u := &model.User{}
	err := db.conn.QueryRow(
		`SELECT id, username, email, password_hash, role, storage_used, storage_limit, created_at, updated_at FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.StorageUsed, &u.StorageLimit, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (db *DB) ListUsers(limit, offset int) ([]*model.User, error) {
	rows, err := db.conn.Query(
		`SELECT id, username, email, password_hash, role, storage_used, storage_limit, created_at, updated_at FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.StorageUsed, &u.StorageLimit, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) UpdateUserStorageUsed(id int64, delta int64) error {
	_, err := db.conn.Exec(`UPDATE users SET storage_used = storage_used + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, delta, id)
	return err
}

func (db *DB) DeleteUser(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (db *DB) CleanupExpired() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete expired clipboard
	tx.Exec(`DELETE FROM clipboard WHERE expires_at < CURRENT_TIMESTAMP`)
	// Delete expired p2p sessions
	tx.Exec(`DELETE FROM p2p_sessions WHERE expires_at < CURRENT_TIMESTAMP`)

	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
