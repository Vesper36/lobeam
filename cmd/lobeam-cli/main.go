package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultServer = "http://localhost:8080"
	chunkSize     = 5 * 1024 * 1024 // 5MB
)

var (
	server   = flag.String("server", os.Getenv("LOBEAM_SERVER"), "LoBeam server URL")
	username = flag.String("user", os.Getenv("LOBEAM_USER"), "Username")
	password = flag.String("pass", os.Getenv("LOBEAM_PASS"), "Password")
)

type authResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
}

type initResp struct {
	TransferID string `json:"transfer_id"`
	ExpiresAt  string `json:"expires_at"`
}

type chunkResp struct {
	FileID string `json:"file_id"`
	Size   int    `json:"size"`
}

type completeResp struct {
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "upload":
		uploadCmd(os.Args[2:])
	case "login":
		loginCmd(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("lobeam-cli 1.0.0")
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`lobeam-cli - Command-line file transfer client

Usage:
  lobeam-cli <command> [options]

Commands:
  login                  Login to LoBeam server
  upload <file>...       Upload files to LoBeam server
  version                Show version

Options:
  -server URL            LoBeam server URL (default: $LOBEAM_SERVER or http://localhost:8080)
  -user NAME             Username
  -pass PASSWORD         Password

Environment:
  LOBEAM_SERVER            Default server URL
  LOBEAM_USER              Default username
  LOBEAM_PASS              Default password
  LOBEAM_TOKEN             Access token (set after login)

Examples:
  lobeam-cli login -server https://lobeam.example.com -user alice -pass secret
  lobeam-cli upload ./file.zip
  lobeam-cli upload -encrypted ./secret.pdf
  lobeam-cli upload -expiry 168 -downloads 5 ./docs/`)
}

func loginCmd(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	fs.StringVar(server, "server", getServer(), "LoBeam server URL")
	fs.StringVar(username, "user", os.Getenv("LOBEAM_USER"), "Username")
	fs.StringVar(password, "pass", os.Getenv("LOBEAM_PASS"), "Password")
	fs.Parse(args)

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Error: -user and -pass are required")
		os.Exit(1)
	}

	resp, err := login(*server, *username, *password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}

	// Save token to ~/.lobeam-token
	home, _ := os.UserHomeDir()
	tokenPath := filepath.Join(home, ".lobeam-token")
	os.WriteFile(tokenPath, []byte(resp.AccessToken), 0600)
	os.Setenv("LOBEAM_TOKEN", resp.AccessToken)

	fmt.Printf("Logged in as %s (role: %s)\n", resp.User.Username, resp.User.Role)
	fmt.Printf("Token saved to %s\n", tokenPath)
}

func uploadCmd(args []string) {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	fs.StringVar(server, "server", getServer(), "LoBeam server URL")
	encrypted := fs.Bool("encrypted", true, "Encrypt files")
	password := fs.String("password", "", "Password for extra protection")
	expiryHours := fs.Int("expiry", 24, "Hours until expiration")
	maxDownloads := fs.Int("downloads", 100, "Max downloads")
	note := fs.String("note", "", "Note for recipient")
	fs.Parse(args)

	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "Error: at least one file required")
		os.Exit(1)
	}

	token := os.Getenv("LOBEAM_TOKEN")
	if token == "" {
		home, _ := os.UserHomeDir()
		if data, err := os.ReadFile(filepath.Join(home, ".lobeam-token")); err == nil {
			token = string(data)
		}
	}

	// Verify all files exist
	var totalSize int64
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			// Add all files in directory
			filepath.Walk(f, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				totalSize += info.Size()
				return nil
			})
		} else {
			totalSize += info.Size()
		}
	}

	name := filepath.Base(files[0])
	if len(files) > 1 {
		name = fmt.Sprintf("%d files", len(files))
	}

	fmt.Printf("Initializing transfer: %s (%d bytes)...\n", name, totalSize)
	t, err := initUpload(*server, token, name, *encrypted, *password, *expiryHours, *maxDownloads, *note)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Init failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Transfer ID: %s\n", t.TransferID)

	// Upload each file
	for _, filePath := range files {
		info, _ := os.Stat(filePath)
		if info != nil && info.IsDir() {
			filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				return uploadFile(*server, token, t.TransferID, path, info.Size())
			})
		} else {
			uploadFile(*server, token, t.TransferID, filePath, info.Size())
		}
	}

	fmt.Println("Completing transfer...")
	c, err := completeUpload(*server, token, t.TransferID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Complete failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nTransfer complete!\n")
	fmt.Printf("Download URL: %s\n", c.DownloadURL)
	fmt.Printf("Expires: %s\n", t.ExpiresAt)
}

func uploadFile(server, token, transferID, filePath string, fileSize int64) error {
	fmt.Printf("Uploading %s (%s)...\n", filepath.Base(filePath), formatBytes(fileSize))

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	var fileID string

	for i := 0; i < totalChunks; i++ {
		buf := make([]byte, chunkSize)
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		chunk := buf[:n]

		// Hash
		h := sha256.Sum256(chunk)
		hash := hex.EncodeToString(h[:])

		resp, err := uploadChunk(server, token, transferID, fileID, i, totalChunks, filepath.Base(filePath), fileSize, mimeType, chunk, hash)
		if err != nil {
			return err
		}
		fileID = resp.FileID

		pct := float64(i+1) / float64(totalChunks) * 100
		fmt.Printf("  Chunk %d/%d (%.1f%%)\n", i+1, totalChunks, pct)
	}
	return nil
}

// ---- HTTP API calls ----

func getServer() string {
	if *server != "" {
		return *server
	}
	return defaultServer
}

func login(server, username, password string) (*authResp, error) {
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := http.Post(server+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	var r authResp
	json.NewDecoder(resp.Body).Decode(&r)
	return &r, nil
}

func initUpload(server, token, name string, encrypted bool, password string, expiryHours, maxDownloads int, note string) (*initResp, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"name":          name,
		"file_count":    1,
		"encrypted":     encrypted,
		"password":      password,
		"expiry_hours":  expiryHours,
		"max_downloads": maxDownloads,
		"note":          note,
	})
	req, _ := http.NewRequest("POST", server+"/api/upload/init", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	var r initResp
	json.NewDecoder(resp.Body).Decode(&r)
	return &r, nil
}

func uploadChunk(server, token, transferID, fileID string, chunkIndex, totalChunks int, fileName string, fileSize int64, mimeType string, data []byte, hash string) (*chunkResp, error) {
	req, _ := http.NewRequest("POST", server+"/api/upload/chunk", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Transfer-ID", transferID)
	if fileID != "" {
		req.Header.Set("X-File-ID", fileID)
	}
	req.Header.Set("X-Chunk-Index", fmt.Sprintf("%d", chunkIndex))
	req.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req.Header.Set("X-File-Name", fileName)
	req.Header.Set("X-File-Size", fmt.Sprintf("%d", fileSize))
	req.Header.Set("X-Mime-Type", mimeType)
	req.Header.Set("X-Chunk-Hash", hash)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	var r chunkResp
	json.NewDecoder(resp.Body).Decode(&r)
	return &r, nil
}

func completeUpload(server, token, transferID string) (*completeResp, error) {
	req, _ := http.NewRequest("POST", server+"/api/upload/complete/"+transferID, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	var r completeResp
	json.NewDecoder(resp.Body).Decode(&r)
	return &r, nil
}

func formatBytes(b int64) string {
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	f := float64(b)
	for f >= k && i < len(sizes)-1 {
		f /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", f, sizes[i])
}

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, strings.TrimSpace(`
lobeam-cli - Command-line file transfer

Usage:
  lobeam-cli <command> [options]

Run 'lobeam-cli' for help.`))
	}
	time.Sleep(0) // placeholder
}
