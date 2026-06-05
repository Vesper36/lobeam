package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vesper/mimo/internal/model"
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
		Name         string `json:"name"`
		FileCount    int    `json:"file_count"`
		Encrypted    bool   `json:"encrypted"`
		Password     string `json:"password"`
		MaxDownloads int    `json:"max_downloads"`
		ExpiryHours  int    `json:"expiry_hours"`
		Note         string `json:"note"`
		SenderEmail  string `json:"sender_email"`
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

	t := &model.Transfer{
		ID:            uuid.New().String()[:8],
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
		writeError(w, http.StatusInternalServerError, "failed to create transfer")
		return
	}

	// Create transfer directory
	transferDir := filepath.Join(s.cfg.DataDir, "transfers", t.ID)
	os.MkdirAll(transferDir, 0755)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"transfer_id": t.ID,
		"expires_at":  t.ExpiresAt,
	})
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	// Parse multipart or raw body
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
		fileID = uuid.New().String()[:12]
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

	// Read chunk data
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read chunk")
		return
	}
	defer r.Body.Close()

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
	if err := s.store.Put(storageKey, bytes.NewReader(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store chunk")
		return
	}

	// Create chunk record
	chunk := &model.Chunk{
		ID:         uuid.New().String()[:12],
		FileID:     fileID,
		Index:      chunkIndex,
		Size:       int64(len(body)),
		SHA256:     chunkSHA256,
		Uploaded:   true,
		StorageKey: storageKey,
	}
	s.db.CreateChunk(chunk)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"chunk_index": chunkIndex,
		"file_id":     fileID,
		"size":        len(body),
	})
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ready",
		"transfer":   t,
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

	// Set headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", f.Name))
	w.Header().Set("Content-Type", f.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	w.Header().Set("X-File-Name", f.Name)
	w.Header().Set("X-File-Size", strconv.FormatInt(f.Size, 10))
	w.Header().Set("X-Encrypted", strconv.FormatBool(t.Encrypted))

	// Stream chunks
	for _, chunk := range chunks {
		reader, err := s.store.Get(chunk.StorageKey)
		if err != nil {
			return
		}
		io.Copy(w, reader)
		reader.Close()
	}

	s.db.IncrementDownload(transferID)
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
		ID:        uuid.New().String()[:8],
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
		ID:        uuid.New().String()[:8],
		Code:      code,
		CreatorID: int64Ptr(getUserID(r)),
		Status:    "waiting",
		ExpiresAt: time.Now().Add(10 * time.Minute),
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
	// TODO: delete files from storage
	s.db.UpdateTransferStatus(id, "expired")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

// ---- Helpers ----

func int64Ptr(v int64) *int64 {
	return &v
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func generateCode(length int) string {
	u := strings.ReplaceAll(uuid.New().String(), "-", "")
	return strings.ToUpper(u[:length])
}
