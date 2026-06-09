package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/vesper/lobeam/internal/model"
)

// ---- Auth Handlers ----

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username, email, and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	role := "member"
	// First user is admin
	users, _ := s.db.ListUsers(1, 0)
	if len(users) == 0 {
		role = "admin"
	}

	u, err := s.userSvc.Register(req.Username, req.Email, req.Password, role)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	access, refresh, err := s.userSvc.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login after register failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":          u,
		"access_token":  access,
		"refresh_token": refresh,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	access, refresh, err := s.userSvc.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	access, err := s.userSvc.RefreshToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"access_token": access})
}

// ---- Upload Handlers ----

func (s *Server) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		FileCount     int    `json:"file_count"`
		Encrypted     bool   `json:"encrypted"`
		Password      string `json:"password"`
		MaxDownloads  int    `json:"max_downloads"`
		ExpiryHours   int    `json:"expiry_hours"`
		Note          string `json:"note"`
		SenderEmail   string `json:"sender_email"`
		ReceiverEmail string `json:"receiver_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		req.Name = "Untitled Transfer"
	}
	if req.MaxDownloads <= 0 {
		req.MaxDownloads = s.cfg.MaxDownloads
	}
	if req.ExpiryHours <= 0 {
		req.ExpiryHours = int(s.cfg.TransferExpiry.Hours())
	}

	// Storage quota check for authenticated users
	userID := getUserID(r)
	if userID > 0 {
		u, err := s.userSvc.GetUser(userID)
		if err == nil && u.StorageLimit > 0 {
			if u.StorageUsed >= u.StorageLimit {
				writeError(w, http.StatusForbidden, "storage quota exceeded")
				return
			}
		}
	}

	t := &model.Transfer{
		ID:            uuid.New().String()[:12],
		UserID:        int64Ptr(getUserID(r)),
		Name:          req.Name,
		Mode:          "link",
		Status:        "pending",
		Encrypted:     req.Encrypted,
		MaxDownloads:  req.MaxDownloads,
		ExpiresAt:     time.Now().Add(time.Duration(req.ExpiryHours) * time.Hour),
		Note:          req.Note,
		SenderEmail:   req.SenderEmail,
		ReceiverEmail: req.ReceiverEmail,
	}

	if req.Password != "" {
		t.PasswordHash = hashPassword(req.Password)
	}

	if err := s.db.CreateTransfer(t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create transfer: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"transfer_id":  t.ID,
		"expires_at":   t.ExpiresAt,
		"download_url": fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, t.ID),
	})

	// Audit log
	if userID > 0 {
		s.db.CreateAuditLog(&userID, "upload", "transfer", fmt.Sprintf("Created transfer %s", t.ID), r.RemoteAddr)
	}
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	transferID := r.Header.Get("X-Transfer-ID")
	fileID := r.Header.Get("X-File-ID")
	chunkIndexStr := r.Header.Get("X-Chunk-Index")
	chunkSHA256 := r.Header.Get("X-Chunk-Hash")
	totalChunksStr := r.Header.Get("X-Total-Chunks")
	fileName := r.Header.Get("X-File-Name")
	fileSizeStr := r.Header.Get("X-File-Size")
	mimeType := r.Header.Get("X-Mime-Type")

	if transferID == "" || chunkIndexStr == "" {
		writeError(w, http.StatusBadRequest, "missing required headers")
		return
	}

	chunkIndex, _ := strconv.Atoi(chunkIndexStr)
	totalChunks, _ := strconv.Atoi(totalChunksStr)
	fileSize, _ := strconv.ParseInt(fileSizeStr, 10, 64)

	// Create file record if first chunk
	if fileID == "" || fileID == "null" {
		fileID = uuid.New().String()[:16]
		f := &model.File{
			ID:         fileID,
			TransferID: transferID,
			Name:       fileName,
			Size:       fileSize,
			MimeType:   mimeType,
			ChunkCount: totalChunks,
		}
		if err := s.db.CreateFile(f); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create file record")
			return
		}
	}

	s.processChunk(w, r, transferID, fileID, chunkIndex, chunkSHA256, s.cfg.MaxChunkSize+1)
}

func (s *Server) handleUploadComplete(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "id")

	// Update transfer counts
	if err := s.db.UpdateTransferCounts(transferID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update transfer")
		return
	}

	if err := s.db.CompleteTransfer(transferID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete transfer")
		return
	}

	t, _ := s.db.GetTransfer(transferID)

	// Update storage used if owned by a user
	if t != nil && t.UserID != nil {
		s.db.UpdateUserStorageUsed(*t.UserID, t.TotalSize)
	}

	// Auto-notify receiver if email was provided
	if t != nil && t.ReceiverEmail != "" && s.notify != nil {
		go s.notify.SendTransferReady(t.ReceiverEmail, transferID, t.Name)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ready",
		"transfer":     t,
		"download_url": fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, transferID),
	})
}

// ---- Download Handlers ----

func (s *Server) handleGetTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := s.db.GetTransfer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}
	if t.Status == "expired" || t.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "transfer has expired")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleGetTransferFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	files, err := s.db.GetFilesByTransfer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	transferID := chi.URLParam(r, "id")
	fileID := chi.URLParam(r, "fileID")

	t, err := s.db.GetTransfer(transferID)
	if err != nil {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}

	if t.Status != "ready" {
		writeError(w, http.StatusBadRequest, "transfer not ready")
		return
	}

	if t.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "transfer has expired")
		return
	}

	if t.MaxDownloads > 0 && t.DownloadCount >= t.MaxDownloads {
		writeError(w, http.StatusGone, "download limit reached")
		return
	}

	f, err := s.db.GetFile(fileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	chunks, err := s.db.GetChunksByFile(fileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chunks")
		return
	}

	// Set headers - use inline for previewable types
	disposition := "attachment"
	if isPreviewableMIME(f.MimeType) || isPreviewableExt(f.Name) {
		disposition = "inline"
	}
	safeName := strings.ReplaceAll(f.Name, "\"", "_")
	safeName = strings.ReplaceAll(safeName, "\n", "_")
	safeName = strings.ReplaceAll(safeName, "\r", "_")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, safeName))
	w.Header().Set("Content-Type", f.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	w.Header().Set("X-File-Name", f.Name)
	w.Header().Set("X-File-Size", strconv.FormatInt(f.Size, 10))
	w.Header().Set("X-Encrypted", strconv.FormatBool(t.Encrypted))
	w.Header().Set("Accept-Ranges", "bytes")

	// Increment download count before streaming to prevent TOCTOU race
	s.db.IncrementDownload(transferID)
	s.db.CreateAuditLog(t.UserID, "download", "transfer", fmt.Sprintf("Downloaded file %s from transfer %s", f.Name, transferID), r.RemoteAddr)
	s.hub.BroadcastToRoom(transferID, "download", map[string]interface{}{
		"file":     f.Name,
		"file_id":  f.ID,
		"download": t.DownloadCount + 1,
		"max":      t.MaxDownloads,
	})
	if t.SenderEmail != "" && s.notify != nil {
		go s.notify.SendDownloadNotification(t.SenderEmail, transferID, f.Name)
	}

	// Handle HTTP Range for resumeable downloads
	if rangeHdr := r.Header.Get("Range"); rangeHdr != "" && !t.Encrypted {
		s.serveRange(w, r, f, chunks, rangeHdr)
		return
	}

	// Stream chunks
	for _, chunk := range chunks {
		reader, err := s.store.Get(r.Context(), chunk.StorageKey)
		if err != nil {
			return
		}
		io.Copy(w, reader)
		reader.Close()
	}
}

// ---- Clipboard Handlers ----

func (s *Server) handleCreateClipboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content   string `json:"content"`
		Language  string `json:"language"`
		Encrypted bool   `json:"encrypted"`
		Hours     int    `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if req.Hours <= 0 {
		req.Hours = 24
	}

	entry := &model.ClipboardEntry{
		ID:        uuid.New().String()[:12],
		UserID:    int64Ptr(getUserID(r)),
		Content:   req.Content,
		Language:  req.Language,
		Encrypted: req.Encrypted,
		ExpiresAt: time.Now().Add(time.Duration(req.Hours) * time.Hour),
	}

	if err := s.db.CreateClipboardEntry(entry); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create clipboard entry")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":  entry.ID,
		"url": fmt.Sprintf("%s/clipboard/%s", s.cfg.PublicURL, entry.ID),
	})
}

func (s *Server) handleGetClipboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := s.db.GetClipboardEntry(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "clipboard entry not found")
		return
	}
	if entry.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "clipboard entry has expired")
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// ---- P2P Handlers ----

func (s *Server) handleCreateP2P(w http.ResponseWriter, r *http.Request) {
	code := generateCode(6)
	session := &model.P2PSession{
		ID:        uuid.New().String()[:12],
		Code:      code,
		CreatorID: int64Ptr(getUserID(r)),
		Status:    "waiting",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := s.db.CreateP2PSession(session); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create p2p session")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"code": code,
		"url":  fmt.Sprintf("%s/p2p/%s", s.cfg.PublicURL, code),
	})
}

func (s *Server) handleGetP2P(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, err := s.db.GetP2PSessionByCode(code)
	if err != nil {
		writeError(w, http.StatusNotFound, "p2p session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// ---- Transfer Management ----

func (s *Server) handleListTransfers(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	transfers, err := s.db.ListTransfers(userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list transfers")
		return
	}
	writeJSON(w, http.StatusOK, transfers)
}

func (s *Server) handleDeleteTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Delete physical files from storage
	if files, err := s.db.GetFilesByTransfer(id); err == nil {
		for _, f := range files {
			if chunks, err := s.db.GetChunksByFile(f.ID); err == nil {
				for _, c := range chunks {
					s.store.Delete(r.Context(), c.StorageKey)
				}
			}
			// Also try storage_path for direct-stored files
			if f.StoragePath != "" {
				s.store.Delete(r.Context(), f.StoragePath)
			}
		}
	}

	s.db.UpdateTransferStatus(id, "expired")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleUpdateMagnet stores the WebTorrent magnet URI for a transfer
func (s *Server) handleUpdateMagnet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		MagnetURI string `json:"magnet_uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MagnetURI == "" {
		writeError(w, http.StatusBadRequest, "magnet_uri is required")
		return
	}
	if err := s.db.UpdateMagnetURI(id, req.MagnetURI); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update magnet URI")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- User Profile ----

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	u, err := s.userSvc.GetUser(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- Admin Handlers ----

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	users, err := s.db.ListUsers(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 100)
	offset := queryInt(r, "offset", 0)
	logs, err := s.db.GetAuditLogs(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get audit logs")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	if err := s.db.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var req struct {
		Role         string `json:"role"`
		StorageLimit int64  `json:"storage_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != "" {
		if err := s.db.UpdateUserRole(id, req.Role); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update role")
			return
		}
	}
	if req.StorageLimit > 0 {
		if err := s.db.UpdateUserStorageLimit(id, req.StorageLimit); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update storage limit")
			return
		}
	}

	u, err := s.userSvc.GetUser(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- Helpers ----

// processChunk is the shared chunk upload logic used by both handleUploadChunk
// and handleScopedUploadChunk. It handles resume check, body reading, hash
// verification, storage, and DB record creation.
func (s *Server) processChunk(w http.ResponseWriter, r *http.Request, transferID, fileID string, chunkIndex int, chunkSHA256 string, bodyLimit int64) {
	// Resume support
	if existingChunk, err := s.db.GetChunk(fileID, chunkIndex); err == nil && existingChunk != nil && existingChunk.Uploaded {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"chunk_index": chunkIndex,
			"file_id":     fileID,
			"size":        existingChunk.Size,
			"resumed":     true,
		})
		return
	}

	// Read chunk data with size limit
	var body []byte
	var err error
	if bodyLimit > 0 {
		body, err = io.ReadAll(io.LimitReader(r.Body, bodyLimit))
	} else {
		body, err = io.ReadAll(r.Body)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read chunk")
		return
	}
	defer r.Body.Close()
	if bodyLimit > 0 && int64(len(body)) >= bodyLimit {
		writeError(w, http.StatusRequestEntityTooLarge, "chunk exceeds maximum size")
		return
	}

	// Verify hash if provided
	if chunkSHA256 != "" {
		h := sha256.Sum256(body)
		actualHash := hex.EncodeToString(h[:])
		if actualHash != chunkSHA256 {
			writeError(w, http.StatusBadRequest, "chunk hash mismatch")
			return
		}
	}

	// Store chunk
	storageKey := fmt.Sprintf("%s/%s/chunk_%06d", transferID, fileID, chunkIndex)
	if err := s.store.Put(r.Context(), storageKey, bytes.NewReader(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store chunk")
		return
	}

	// Create chunk record
	chunk := &model.Chunk{
		ID:         uuid.New().String()[:16],
		FileID:     fileID,
		Index:      chunkIndex,
		Size:       int64(len(body)),
		SHA256:     chunkSHA256,
		Uploaded:   true,
		StorageKey: storageKey,
	}
	if err := s.db.CreateChunk(chunk); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record chunk")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"chunk_index": chunkIndex,
		"file_id":     fileID,
		"size":        len(body),
	})
}

func int64Ptr(v int64) *int64 {
	return &v
}

func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		// Fallback should never happen, but log and return empty to surface the error
		return ""
	}
	return string(hash)
}

func verifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateCode(length int) string {
	u := strings.ReplaceAll(uuid.New().String(), "-", "")
	return strings.ToUpper(u[:length])
}

// MIME types that browsers can preview inline
var previewMIMETypes = map[string]bool{
	"image/jpeg":                 true,
	"image/png":                  true,
	"image/gif":                  true,
	"image/webp":                 true,
	"image/svg+xml":              true,
	"video/mp4":                  true,
	"video/webm":                 true,
	"video/ogg":                  true,
	"audio/mpeg":                 true,
	"audio/ogg":                  true,
	"audio/wav":                  true,
	"audio/webm":                 true,
	"application/pdf":            true,
	"text/plain":                 true,
	"text/html":                  true,
	"text/css":                   true,
	"text/javascript":            true,
	"text/csv":                   true,
	"application/json":           true,
	"application/xml":            true,
	"application/vnd.ms-excel":   true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"application/msword":         true,
	"application/vnd.ms-powerpoint": true,
	"text/markdown":              true,
	"text/x-python":              true,
	"text/x-go":                  true,
	"text/x-rust":                true,
	"application/x-yaml":         true,
	"text/yaml":                  true,
	"text/x-c":                   true,
	"text/x-c++":                 true,
	"text/x-java":                true,
	"text/x-sql":                 true,
}

var previewExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".svg": true, ".mp4": true, ".webm": true, ".ogg": true, ".mp3": true,
	".wav": true, ".pdf": true, ".txt": true, ".md": true, ".csv": true,
	".json": true, ".xml": true, ".html": true, ".css": true, ".js": true,
	".ico": true, ".bmp": true,
	".xls": true, ".xlsx": true, ".ods": true,
	".doc": true, ".docx": true, ".rtf": true, ".odt": true,
	".ppt": true, ".pptx": true, ".odp": true,
	".psd": true, ".ai": true, ".sketch": true, ".fig": true,
	".py": true, ".go": true, ".rs": true, ".java": true, ".ts": true,
	".jsx": true, ".tsx": true, ".yaml": true, ".yml": true, ".toml": true,
	".sql": true, ".sh": true, ".bat": true, ".ini": true, ".cfg": true,
	".conf": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
	".rb": true, ".php": true, ".swift": true, ".kt": true, ".dart": true,
	".lua": true, ".r": true, ".log": true, ".ics": true, ".vcf": true,
	".scss": true, ".less": true,
}

func isPreviewableMIME(mime string) bool {
	return previewMIMETypes[mime]
}

func isPreviewableExt(name string) bool {
	ext := strings.ToLower(name)
	for i := len(ext) - 1; i >= 0; i-- {
		if ext[i] == '.' {
			return previewExtensions[ext[i:]]
		}
	}
	return false
}

// serveRange handles HTTP Range requests for resumeable downloads
func (s *Server) serveRange(w http.ResponseWriter, r *http.Request, file *model.File, chunks []*model.Chunk, rangeHdr string) {
	var start, end int64 = 0, file.Size - 1
	if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-%d", &start, &end); err != nil {
		fmt.Sscanf(rangeHdr, "bytes=%d-", &start)
	}
	if start < 0 {
		start = 0
	}
	if end >= file.Size {
		end = file.Size - 1
	}
	if start > end {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", file.Size))
		return
	}
	length := end - start + 1
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Size))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusPartialContent)

	var written int64
	flusher, canFlush := w.(http.Flusher)
	for _, c := range chunks {
		if written >= length {
			break
		}
		rd, err := s.store.Get(r.Context(), c.StorageKey)
		if err != nil {
			return
		}
		// Skip chunks before the start offset
		if start > 0 {
			if start < c.Size {
				skipped, _ := io.CopyN(io.Discard, rd, start)
				start -= skipped
			} else {
				start -= c.Size
				rd.Close()
				continue
			}
		}
		// Write the relevant portion of this chunk
		toWrite := c.Size - start
		if toWrite > length-written {
			toWrite = length - written
		}
		n, err := io.CopyN(w, rd, toWrite)
		written += n
		rd.Close()
		if err != nil {
			return
		}
		if canFlush {
			flusher.Flush()
		}
		start = 0
	}
}
