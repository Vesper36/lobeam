package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vesper/lobeam/internal/integrations"
	"github.com/vesper/lobeam/internal/model"
)

// ---- Brand (public) ----

func (s *Server) handleGetBrand(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	brand, err := s.db.GetBrandByDomain(host)
	if err != nil {
		brand, _ = s.db.GetDefaultBrand()
	}
	if brand == nil {
		brand = &model.Brand{
			Name:               "LoBeam",
			PrimaryColor:       "#7c3aed",
			BackgroundColor:    "#09090b",
			AccentColor:        "#4f46e5",
			ShowPoweredBy:      true,
			DefaultExpiryHours: 24,
		}
	}
	writeJSON(w, http.StatusOK, brand)
}

func (s *Server) handleUpdateBrand(w http.ResponseWriter, r *http.Request) {
	var b model.Brand
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.db.UpsertBrand(&b); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update brand: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// ---- File Requests ----

func (s *Server) handleCreateFileRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title         string   `json:"title"`
		Description   string   `json:"description"`
		MaxFileSize   int64    `json:"max_file_size"`
		MaxFiles      int      `json:"max_files"`
		AllowedTypes  []string `json:"allowed_types"`
		CustomFields  []string `json:"custom_fields"`
		RequireFields []string `json:"require_fields"`
		ExpiryDays    int      `json:"expiry_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		req.Title = "File Request"
	}
	if req.ExpiryDays <= 0 {
		req.ExpiryDays = 30
	}

	allowedJSON, _ := json.Marshal(req.AllowedTypes)
	customJSON, _ := json.Marshal(req.CustomFields)
	requireJSON, _ := json.Marshal(req.RequireFields)

	fr := &model.FileRequest{
		ID:            uuid.New().String()[:8],
		UserID:        int64Ptr(getUserID(r)),
		Title:         req.Title,
		Description:   req.Description,
		MaxFileSize:   req.MaxFileSize,
		MaxFiles:      req.MaxFiles,
		AllowedTypes:  string(allowedJSON),
		CustomFields:  string(customJSON),
		RequireFields: string(requireJSON),
		Status:        "active",
		ExpiresAt:     time.Now().Add(time.Duration(req.ExpiryDays) * 24 * time.Hour),
	}

	if err := s.db.CreateFileRequest(fr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":  fr.ID,
		"url": fmt.Sprintf("%s/r/%s", s.cfg.PublicURL, fr.ID),
	})
}

func (s *Server) handleGetFileRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fr, err := s.db.GetFileRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if fr.Status != "active" || fr.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "request is no longer active")
		return
	}
	writeJSON(w, http.StatusOK, fr)
}

func (s *Server) handleListFileRequests(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	requests, err := s.db.ListFileRequests(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

// handleSubmitToFileRequest handles file uploads responding to a file request
func (s *Server) handleSubmitToFileRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fr, err := s.db.GetFileRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if fr.Status != "active" || fr.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "request is no longer active")
		return
	}

	fileName := r.Header.Get("X-File-Name")
	fileSizeStr := r.Header.Get("X-File-Size")
	mimeType := r.Header.Get("X-Mime-Type")
	uploaderName := r.Header.Get("X-Uploader-Name")
	uploaderEmail := r.Header.Get("X-Uploader-Email")
	uploaderMessage := r.Header.Get("X-Uploader-Message")

	if fileName == "" {
		writeError(w, http.StatusBadRequest, "missing file name")
		return
	}
	if fr.MaxFiles > 0 && fr.FileCount >= fr.MaxFiles {
		writeError(w, http.StatusForbidden, "request file limit reached")
		return
	}
	if allowedTypes := parseJSONStringList(fr.AllowedTypes); !fileTypeAllowed(fileName, mimeType, allowedTypes) {
		writeError(w, http.StatusForbidden, "file type is not allowed")
		return
	}

	fileSize, _ := parseInt64(fileSizeStr)
	if fr.MaxFileSize > 0 && fileSize > fr.MaxFileSize {
		writeError(w, http.StatusForbidden, "file exceeds size limit")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read upload")
		return
	}

	actualSize := int64(len(body))
	if fr.MaxFileSize > 0 && actualSize > fr.MaxFileSize {
		writeError(w, http.StatusForbidden, "file exceeds size limit")
		return
	}

	// Store file as part of a transfer associated with this request
	transferID := uuid.New().String()[:8]
	fileID := uuid.New().String()[:12]
	storagePath := fmt.Sprintf("requests/%s/%s_%s", id, fileID, fileName)

	if err := s.store.Put(storagePath, bytesReader(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store file")
		return
	}

	// Create transfer record
	t := &model.Transfer{
		ID:           transferID,
		UserID:       fr.UserID,
		Name:         fileName,
		Mode:         "link",
		Status:       "ready",
		FileCount:    1,
		TotalSize:    actualSize,
		MaxDownloads: 100,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	if err := s.db.CreateTransfer(t); err == nil {
		f := &model.File{
			ID:          fileID,
			TransferID:  transferID,
			Name:        fileName,
			Size:        actualSize,
			MimeType:    mimeType,
			ChunkCount:  1,
			ChunkSize:   actualSize,
			StoragePath: storagePath,
		}
		if err := s.db.CreateFile(f); err == nil {
			_ = s.db.CreateChunk(&model.Chunk{
				ID:         uuid.New().String()[:12],
				FileID:     fileID,
				Index:      0,
				Size:       actualSize,
				Uploaded:   true,
				StorageKey: storagePath,
			})
		}
	}
	_ = s.db.UpdateTransferCounts(transferID)
	_ = s.db.AddFileRequestUpload(id, actualSize)

	// Audit log
	if fr.UserID != nil {
		details := fmt.Sprintf("File %s submitted to request %s by %s", fileName, id, uploaderName)
		if uploaderMessage != "" {
			details = fmt.Sprintf("%s: %s", details, uploaderMessage)
		}
		s.db.CreateAuditLog(fr.UserID, "upload", "file_request",
			details, r.RemoteAddr)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"file_id":        fileID,
		"name":           fileName,
		"size":           actualSize,
		"request_id":     id,
		"uploader_name":  uploaderName,
		"uploader_email": uploaderEmail,
		"message":        uploaderMessage,
	})
}

func (s *Server) handleInitFileRequestUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fr, err := s.db.GetFileRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if fr.Status != "active" || fr.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "request is no longer active")
		return
	}

	var req struct {
		Name          string `json:"name"`
		Size          int64  `json:"size"`
		MimeType      string `json:"mime_type"`
		TotalChunks   int    `json:"total_chunks"`
		ChunkSize     int64  `json:"chunk_size"`
		UploaderName  string `json:"uploader_name"`
		UploaderEmail string `json:"uploader_email"`
		Message       string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing file name")
		return
	}
	if err := validateChunkedUploadMetadata(req.Size, req.TotalChunks, req.ChunkSize); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.MimeType == "" {
		req.MimeType = "application/octet-stream"
	}
	if fr.MaxFiles > 0 && fr.FileCount >= fr.MaxFiles {
		writeError(w, http.StatusForbidden, "request file limit reached")
		return
	}
	if fr.MaxFileSize > 0 && req.Size > fr.MaxFileSize {
		writeError(w, http.StatusForbidden, "file exceeds size limit")
		return
	}
	if allowedTypes := parseJSONStringList(fr.AllowedTypes); !fileTypeAllowed(req.Name, req.MimeType, allowedTypes) {
		writeError(w, http.StatusForbidden, "file type is not allowed")
		return
	}

	transferID := uuid.New().String()[:8]
	fileID := uuid.New().String()[:12]
	note := req.Message
	if req.UploaderName != "" || req.UploaderEmail != "" {
		note = strings.TrimSpace(fmt.Sprintf("Submitted by %s <%s>\n%s", req.UploaderName, req.UploaderEmail, req.Message))
	}

	t := &model.Transfer{
		ID:           transferID,
		UserID:       fr.UserID,
		Name:         req.Name,
		Mode:         "request:" + id,
		Status:       "pending",
		MaxDownloads: 100,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		Note:         note,
	}
	if err := s.db.CreateTransfer(t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create transfer")
		return
	}

	f := &model.File{
		ID:         fileID,
		TransferID: transferID,
		Name:       req.Name,
		Size:       req.Size,
		MimeType:   req.MimeType,
		ChunkCount: req.TotalChunks,
		ChunkSize:  req.ChunkSize,
	}
	if err := s.db.CreateFile(f); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file record")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"request_id":   id,
		"transfer_id":  transferID,
		"file_id":      fileID,
		"download_url": fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, transferID),
	})
}

func (s *Server) handleFileRequestUploadChunk(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fr, err := s.db.GetFileRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if fr.Status != "active" || fr.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "request is no longer active")
		return
	}
	s.handleScopedUploadChunk(w, r, "request:"+id)
}

func (s *Server) handleCompleteFileRequestUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	transferID := chi.URLParam(r, "transferID")
	fr, err := s.db.GetFileRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}

	t, err := s.getScopedTransfer(transferID, "request:"+id)
	if err != nil {
		writeError(w, http.StatusNotFound, "upload not found")
		return
	}
	if t.Status == "ready" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":       "ready",
			"transfer":     t,
			"download_url": fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, transferID),
		})
		return
	}
	if fr.Status != "active" || fr.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "request is no longer active")
		return
	}

	totalSize, err := s.ensureTransferChunksComplete(transferID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.UpdateTransferCounts(transferID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update transfer")
		return
	}
	if err := s.db.CompleteTransfer(transferID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete transfer")
		return
	}
	_ = s.db.AddFileRequestUpload(id, totalSize)
	if fr.UserID != nil {
		details := fmt.Sprintf("File submitted to request %s", id)
		if files, err := s.db.GetFilesByTransfer(transferID); err == nil && len(files) > 0 {
			details = fmt.Sprintf("File %s submitted to request %s", files[0].Name, id)
		}
		if t.Note != "" {
			details = fmt.Sprintf("%s: %s", details, t.Note)
		}
		s.db.CreateAuditLog(fr.UserID, "upload", "file_request", details, r.RemoteAddr)
	}

	updated, _ := s.db.GetTransfer(transferID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ready",
		"transfer":     updated,
		"download_url": fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, transferID),
	})
}

// ---- Web Folders ----

func (s *Server) handleCreateWebFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Mode        string `json:"mode"` // upload_only, download_only, both
		Password    string `json:"password"`
		MaxFileSize int64  `json:"max_file_size"`
		MaxFiles    int    `json:"max_files"`
		ExpiryDays  int    `json:"expiry_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		req.Name = "Untitled Folder"
	}
	if req.Mode == "" {
		req.Mode = "both"
	}
	if req.ExpiryDays <= 0 {
		req.ExpiryDays = 30
	}

	token := generateToken(16)

	folder := &model.WebFolder{
		ID:          uuid.New().String()[:8],
		Token:       token,
		UserID:      int64Ptr(getUserID(r)),
		Name:        req.Name,
		Description: req.Description,
		Mode:        req.Mode,
		MaxFileSize: req.MaxFileSize,
		MaxFiles:    req.MaxFiles,
		ExpiresAt:   time.Now().Add(time.Duration(req.ExpiryDays) * 24 * time.Hour),
	}

	if req.Password != "" {
		folder.PasswordHash = hashPassword(req.Password)
	}

	if err := s.db.CreateWebFolder(folder); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create folder")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":    folder.ID,
		"token": token,
		"url":   fmt.Sprintf("%s/f/%s", s.cfg.PublicURL, token),
	})
}

func (s *Server) handleGetWebFolder(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "folder has expired")
		return
	}
	writeJSON(w, http.StatusOK, folder)
}

func (s *Server) handleGetWebFolderFiles(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	files, err := s.db.GetWebFolderFiles(folder.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) handleUploadToWebFolder(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Mode == "download_only" {
		writeError(w, http.StatusForbidden, "uploads not allowed in this folder")
		return
	}
	if folder.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "folder has expired")
		return
	}
	if folder.MaxFiles > 0 && folder.FileCount >= folder.MaxFiles {
		writeError(w, http.StatusForbidden, "folder is full")
		return
	}

	// Parse upload
	transferID := r.Header.Get("X-Transfer-ID")
	fileName := r.Header.Get("X-File-Name")
	fileSizeStr := r.Header.Get("X-File-Size")
	mimeType := r.Header.Get("X-Mime-Type")
	uploaderName := r.Header.Get("X-Uploader-Name")
	uploaderEmail := r.Header.Get("X-Uploader-Email")

	if fileName == "" {
		writeError(w, http.StatusBadRequest, "missing file name")
		return
	}

	if folder.MaxFileSize > 0 {
		// Verify file size
		fileSize, _ := parseInt64(fileSizeStr)
		if fileSize > folder.MaxFileSize {
			writeError(w, http.StatusForbidden, "file exceeds size limit")
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read upload")
		return
	}

	if folder.MaxFileSize > 0 && int64(len(body)) > folder.MaxFileSize {
		writeError(w, http.StatusForbidden, "file exceeds size limit")
		return
	}

	fileID := uuid.New().String()[:12]
	storagePath := fmt.Sprintf("folders/%s/%s_%s", folder.ID, fileID, fileName)

	if err := s.store.Put(storagePath, io.NopCloser(bytesReader(body))); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store file")
		return
	}

	fileSize, _ := parseInt64(fileSizeStr)
	if fileSize == 0 {
		fileSize = int64(len(body))
	}

	folderFile := &model.WebFolderFile{
		ID:            fileID,
		FolderID:      folder.ID,
		Name:          fileName,
		Size:          fileSize,
		MimeType:      mimeType,
		StoragePath:   storagePath,
		UploaderName:  uploaderName,
		UploaderEmail: uploaderEmail,
	}

	if err := s.db.AddWebFolderFile(folderFile); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record file")
		return
	}

	s.db.UpdateWebFolderCounts(folder.ID)

	// Send notification if upload notification is enabled
	if folder.UserID != nil && s.notify != nil {
		_ = s.notify.SendDownloadNotification("", transferID, fileName)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"file_id":   fileID,
		"name":      fileName,
		"size":      fileSize,
		"folder_id": folder.ID,
	})
}

func (s *Server) handleInitWebFolderUpload(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Mode == "download_only" {
		writeError(w, http.StatusForbidden, "uploads not allowed in this folder")
		return
	}
	if folder.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "folder has expired")
		return
	}
	if folder.MaxFiles > 0 && folder.FileCount >= folder.MaxFiles {
		writeError(w, http.StatusForbidden, "folder is full")
		return
	}

	var req struct {
		Name          string `json:"name"`
		Size          int64  `json:"size"`
		MimeType      string `json:"mime_type"`
		TotalChunks   int    `json:"total_chunks"`
		ChunkSize     int64  `json:"chunk_size"`
		UploaderName  string `json:"uploader_name"`
		UploaderEmail string `json:"uploader_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing file name")
		return
	}
	if err := validateChunkedUploadMetadata(req.Size, req.TotalChunks, req.ChunkSize); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.MimeType == "" {
		req.MimeType = "application/octet-stream"
	}
	if folder.MaxFileSize > 0 && req.Size > folder.MaxFileSize {
		writeError(w, http.StatusForbidden, "file exceeds size limit")
		return
	}

	transferID := uuid.New().String()[:8]
	fileID := uuid.New().String()[:12]

	t := &model.Transfer{
		ID:           transferID,
		Name:         req.Name,
		Mode:         "folder:" + folder.ID,
		Status:       "pending",
		MaxDownloads: 1,
		ExpiresAt:    folder.ExpiresAt,
		Note:         req.UploaderName,
		SenderEmail:  req.UploaderEmail,
	}
	if err := s.db.CreateTransfer(t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create folder upload")
		return
	}

	f := &model.File{
		ID:         fileID,
		TransferID: transferID,
		Name:       req.Name,
		Size:       req.Size,
		MimeType:   req.MimeType,
		ChunkCount: req.TotalChunks,
		ChunkSize:  req.ChunkSize,
	}
	if err := s.db.CreateFile(f); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file record")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"folder_id":   folder.ID,
		"transfer_id": transferID,
		"file_id":     fileID,
	})
}

func (s *Server) handleWebFolderUploadChunk(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Mode == "download_only" {
		writeError(w, http.StatusForbidden, "uploads not allowed in this folder")
		return
	}
	if folder.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "folder has expired")
		return
	}
	s.handleScopedUploadChunk(w, r, "folder:"+folder.ID)
}

func (s *Server) handleCompleteWebFolderUpload(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	transferID := chi.URLParam(r, "transferID")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Mode == "download_only" {
		writeError(w, http.StatusForbidden, "uploads not allowed in this folder")
		return
	}
	if folder.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusGone, "folder has expired")
		return
	}

	t, err := s.getScopedTransfer(transferID, "folder:"+folder.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "upload not found")
		return
	}
	files, err := s.db.GetFilesByTransfer(t.ID)
	if err != nil || len(files) != 1 {
		writeError(w, http.StatusBadRequest, "invalid folder upload")
		return
	}
	file := files[0]
	if t.Status == "ready" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"file_id":   file.ID,
			"name":      file.Name,
			"size":      file.Size,
			"folder_id": folder.ID,
		})
		return
	}
	if folder.MaxFiles > 0 && folder.FileCount >= folder.MaxFiles {
		writeError(w, http.StatusForbidden, "folder is full")
		return
	}
	if folder.MaxFileSize > 0 && file.Size > folder.MaxFileSize {
		writeError(w, http.StatusForbidden, "file exceeds size limit")
		return
	}
	if _, err := s.ensureTransferChunksComplete(transferID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	storagePath := fmt.Sprintf("folders/%s/%s_%s", folder.ID, file.ID, file.Name)
	if err := s.storeWebFolderFileFromChunks(file, storagePath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assemble folder file")
		return
	}

	folderFile := &model.WebFolderFile{
		ID:            file.ID,
		FolderID:      folder.ID,
		Name:          file.Name,
		Size:          file.Size,
		MimeType:      file.MimeType,
		StoragePath:   storagePath,
		UploaderName:  t.Note,
		UploaderEmail: t.SenderEmail,
	}
	if existing, err := s.db.GetWebFolderFile(file.ID); err == nil && existing.FolderID == folder.ID {
		_ = s.db.UpdateWebFolderCounts(folder.ID)
		_ = s.db.UpdateTransferCounts(transferID)
		_ = s.db.CompleteTransfer(transferID)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"file_id":   existing.ID,
			"name":      existing.Name,
			"size":      existing.Size,
			"folder_id": existing.FolderID,
		})
		return
	}
	if err := s.db.AddWebFolderFile(folderFile); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record file")
		return
	}
	_ = s.db.UpdateWebFolderCounts(folder.ID)
	_ = s.db.UpdateTransferCounts(transferID)
	_ = s.db.CompleteTransfer(transferID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"file_id":   file.ID,
		"name":      file.Name,
		"size":      file.Size,
		"folder_id": folder.ID,
	})
}

func (s *Server) handleDownloadFromWebFolder(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	fileID := chi.URLParam(r, "fileID")

	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.Mode == "upload_only" {
		writeError(w, http.StatusForbidden, "downloads not allowed in this folder")
		return
	}

	file, err := s.db.GetWebFolderFile(fileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if file.FolderID != folder.ID {
		writeError(w, http.StatusForbidden, "file not in this folder")
		return
	}

	reader, err := s.store.Get(file.StoragePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file data not found")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.Name))
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
	io.Copy(w, reader)
}

func (s *Server) handleListWebFolders(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	folders, err := s.db.ListWebFolders(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list folders")
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

// ---- Settings ----

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var s2 model.Settings
	if err := json.NewDecoder(r.Body).Decode(&s2); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.db.UpdateSettings(&s2); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}
	writeJSON(w, http.StatusOK, s2)
}

// ---- ICE Config (TURN/STUN for WebRTC) ----

func (s *Server) handleGetICEConfig(w http.ResponseWriter, r *http.Request) {
	iceServers := []map[string]interface{}{
		{"urls": s.cfg.STUNServers},
	}
	if s.cfg.TURNServer != "" {
		server := s.cfg.TURNServer
		if !strings.HasPrefix(server, "turn:") && !strings.HasPrefix(server, "turns:") {
			server = "turn:" + server
		}
		entry := map[string]interface{}{
			"urls":       []string{server},
			"username":   s.cfg.TURNUser,
			"credential": s.cfg.TURNPass,
		}
		iceServers = append(iceServers, entry)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ice_servers": iceServers,
	})
}

// ---- Web Folder Password Verification ----

func (s *Server) handleVerifyFolderPassword(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	folder, err := s.db.GetWebFolderByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if folder.PasswordHash == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !verifyPassword(req.Password, folder.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

// ---- Integration Share ----

func (s *Server) handleShareTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	platform := chi.URLParam(r, "platform")

	transfer, err := s.db.GetTransfer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}

	files, _ := s.db.GetFilesByTransfer(id)
	fileName := transfer.Name
	var fileSize int64
	if len(files) > 0 {
		fileName = files[0].Name
		fileSize = files[0].Size
	}

	var req struct {
		Message    string `json:"message"`
		SenderName string `json:"sender_name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.SenderName == "" {
		req.SenderName = "Someone"
	}

	payload := integrations.SharePayload{
		TransferID:  id,
		FileName:    fileName,
		FileSize:    fileSize,
		SenderName:  req.SenderName,
		DownloadURL: fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, id),
		Message:     req.Message,
	}

	var shareErr error
	switch platform {
	case "slack":
		shareErr = s.intSvc.SendToSlack(payload)
	case "zoom":
		shareErr = s.intSvc.SendToZoom(payload)
	case "google":
		shareErr = s.intSvc.SendToGoogleWorkspace(payload)
	default:
		writeError(w, http.StatusBadRequest, "unsupported platform: "+platform)
		return
	}

	if shareErr != nil {
		writeError(w, http.StatusBadGateway, "failed to share: "+shareErr.Error())
		return
	}

	s.db.CreateAuditLog(int64Ptr(getUserID(r)), "share", "transfer",
		fmt.Sprintf("Shared transfer %s to %s", id, platform), r.RemoteAddr)

	writeJSON(w, http.StatusOK, map[string]string{"status": "shared", "platform": platform})
}

// ---- Email (for transfer email) ----

func (s *Server) handleEmailTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	transfer, err := s.db.GetTransfer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}

	var req struct {
		Email   string `json:"email"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if s.notify == nil {
		writeError(w, http.StatusServiceUnavailable, "email service is not configured")
		return
	}

	downloadURL := fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, transfer.ID)
	if err := s.notify.SendTransferEmail(req.Email, transfer.SenderEmail, req.Subject, req.Message, downloadURL, transfer.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send email: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// ---- Helpers ----

func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseJSONStringList(raw string) []string {
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	clean := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			clean = append(clean, item)
		}
	}
	return clean
}

func fileTypeAllowed(fileName, mimeType string, allowedTypes []string) bool {
	if len(allowedTypes) == 0 {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	mime := strings.ToLower(mimeType)
	for _, allowed := range allowedTypes {
		normalized := strings.TrimSpace(strings.ToLower(allowed))
		if normalized == "" {
			continue
		}
		if strings.HasPrefix(normalized, ".") {
			normalized = strings.TrimPrefix(normalized, ".")
		}
		if strings.HasSuffix(normalized, "/*") && strings.HasPrefix(mime, strings.TrimSuffix(normalized, "*")) {
			return true
		}
		if strings.Contains(normalized, "/") && normalized == mime {
			return true
		}
		if normalized == ext {
			return true
		}
	}
	return false
}

func validateChunkedUploadMetadata(size int64, totalChunks int, chunkSize int64) error {
	if size < 0 {
		return fmt.Errorf("file size must not be negative")
	}
	if totalChunks <= 0 {
		return fmt.Errorf("total chunks must be positive")
	}
	if chunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}

	expectedChunks := int64(1)
	if size > 0 {
		expectedChunks = ((size - 1) / chunkSize) + 1
	}
	if int64(totalChunks) != expectedChunks {
		return fmt.Errorf("total chunks does not match file size")
	}
	return nil
}

func expectedChunkSize(file *model.File, chunkIndex int) (int64, error) {
	if file.Size < 0 {
		return 0, fmt.Errorf("file size must not be negative")
	}
	if file.ChunkCount <= 0 {
		return 0, fmt.Errorf("invalid chunk count")
	}
	if file.ChunkSize <= 0 {
		return 0, fmt.Errorf("invalid chunk size")
	}
	if chunkIndex < 0 || chunkIndex >= file.ChunkCount {
		return 0, fmt.Errorf("invalid chunk index")
	}
	if chunkIndex == file.ChunkCount-1 {
		size := file.Size - file.ChunkSize*int64(file.ChunkCount-1)
		if size < 0 {
			return 0, fmt.Errorf("invalid chunk metadata")
		}
		return size, nil
	}
	return file.ChunkSize, nil
}

func (s *Server) getScopedTransfer(transferID, expectedMode string) (*model.Transfer, error) {
	t, err := s.db.GetTransfer(transferID)
	if err != nil {
		return nil, err
	}
	if t.Mode != expectedMode {
		return nil, fmt.Errorf("upload scope mismatch")
	}
	return t, nil
}

func (s *Server) handleScopedUploadChunk(w http.ResponseWriter, r *http.Request, expectedMode string) {
	transferID := r.Header.Get("X-Transfer-ID")
	fileID := r.Header.Get("X-File-ID")
	chunkIndexStr := r.Header.Get("X-Chunk-Index")
	chunkSHA256 := r.Header.Get("X-Chunk-Hash")

	if transferID == "" || fileID == "" || chunkIndexStr == "" {
		writeError(w, http.StatusBadRequest, "missing required headers")
		return
	}
	t, err := s.getScopedTransfer(transferID, expectedMode)
	if err != nil {
		writeError(w, http.StatusNotFound, "upload not found")
		return
	}
	if t.Status == "ready" {
		writeError(w, http.StatusConflict, "upload already completed")
		return
	}

	file, err := s.db.GetFile(fileID)
	if err != nil || file.TransferID != transferID {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 || chunkIndex >= file.ChunkCount {
		writeError(w, http.StatusBadRequest, "invalid chunk index")
		return
	}
	expectedSize, err := expectedChunkSize(file, chunkIndex)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if existingChunk, err := s.db.GetChunk(fileID, chunkIndex); err == nil && existingChunk != nil && existingChunk.Uploaded {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"chunk_index": chunkIndex,
			"file_id":     fileID,
			"size":        existingChunk.Size,
			"resumed":     true,
		})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, expectedSize+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read chunk")
		return
	}
	defer r.Body.Close()
	if int64(len(body)) != expectedSize {
		writeError(w, http.StatusBadRequest, "chunk size mismatch")
		return
	}

	if chunkSHA256 != "" {
		h := sha256.Sum256(body)
		actualHash := hex.EncodeToString(h[:])
		if actualHash != chunkSHA256 {
			writeError(w, http.StatusBadRequest, "chunk hash mismatch")
			return
		}
	}

	storageKey := fmt.Sprintf("%s/%s/chunk_%06d", transferID, fileID, chunkIndex)
	if err := s.store.Put(storageKey, bytes.NewReader(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store chunk")
		return
	}

	chunk := &model.Chunk{
		ID:         uuid.New().String()[:12],
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

func (s *Server) ensureTransferChunksComplete(transferID string) (int64, error) {
	files, err := s.db.GetFilesByTransfer(transferID)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("upload has no files")
	}

	var totalSize int64
	for _, file := range files {
		chunks, err := s.db.GetChunksByFile(file.ID)
		if err != nil {
			return 0, err
		}
		if len(chunks) != file.ChunkCount {
			return 0, fmt.Errorf("upload is missing chunks")
		}
		var fileSize int64
		seen := make(map[int]bool, file.ChunkCount)
		for _, chunk := range chunks {
			if !chunk.Uploaded {
				return 0, fmt.Errorf("upload is missing chunks")
			}
			if chunk.Index < 0 || chunk.Index >= file.ChunkCount || seen[chunk.Index] {
				return 0, fmt.Errorf("upload has invalid chunks")
			}
			seen[chunk.Index] = true
			fileSize += chunk.Size
		}
		if file.Size != fileSize {
			return 0, fmt.Errorf("uploaded size does not match file size")
		}
		totalSize += fileSize
	}
	return totalSize, nil
}

func (s *Server) storeWebFolderFileFromChunks(file *model.File, storagePath string) error {
	chunks, err := s.db.GetChunksByFile(file.ID)
	if err != nil {
		return err
	}
	readers := make([]io.ReadCloser, 0, len(chunks))
	for _, chunk := range chunks {
		reader, err := s.store.Get(chunk.StorageKey)
		if err != nil {
			for _, r := range readers {
				r.Close()
			}
			return err
		}
		readers = append(readers, reader)
	}
	defer func() {
		for _, reader := range readers {
			reader.Close()
		}
	}()

	streams := make([]io.Reader, len(readers))
	for i, reader := range readers {
		streams[i] = reader
	}
	return s.store.Put(storagePath, io.MultiReader(streams...))
}

func bytesReader(b []byte) io.Reader {
	return &byteReader{b: b}
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
