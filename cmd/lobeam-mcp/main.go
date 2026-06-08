package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// LoBeam MCP Server — lets Claude, Cursor, and other AI tools
// upload files to a LoBeam server and get shareable download links.
//
// Implements the Model Context Protocol (MCP) stdio transport.
// Usage: lobeam-mcp -server https://your-lobeam.example.com

var serverURL string

func main() {
	serverURL = os.Getenv("LOBEAM_SERVER")
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}
	// CLI flag override
	for i, arg := range os.Args {
		if arg == "-server" && i+1 < len(os.Args) {
			serverURL = os.Args[i+1]
		}
	}
	serverURL = strings.TrimRight(serverURL, "/")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		handleMessage(scanner.Bytes(), writer)
		writer.Flush()
	}
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func handleMessage(data []byte, w *bufio.Writer) {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeError(w, nil, -32700, "Parse error")
		return
	}

	switch req.Method {
	case "initialize":
		writeResult(w, req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "lobeam-mcp",
				"version": "1.0.0",
			},
		})

	case "tools/list":
		writeResult(w, req.ID, map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "upload_file",
					"description": "Upload a file to LoBeam and return a shareable download link. Supports chunked upload for large files.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"file_path": map[string]interface{}{
								"type":        "string",
								"description": "Absolute path to the file to upload",
							},
							"encrypt": map[string]interface{}{
								"type":        "boolean",
								"description": "Enable E2E encryption (default: true)",
							},
							"expiry_hours": map[string]interface{}{
								"type":        "number",
								"description": "Hours until the transfer expires (default: 24)",
							},
							"max_downloads": map[string]interface{}{
								"type":        "number",
								"description": "Maximum number of downloads (default: 100)",
							},
							"note": map[string]interface{}{
								"type":        "string",
								"description": "Optional note/description for the transfer",
							},
							"password": map[string]interface{}{
								"type":        "string",
								"description": "Optional password to protect the download",
							},
						},
						"required": []string{"file_path"},
					},
				},
				{
					"name":        "create_clipboard",
					"description": "Create a network clipboard entry (text/code sharing) and return a shareable link.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Text content to share",
							},
							"language": map[string]interface{}{
								"type":        "string",
								"description": "Programming language for syntax highlighting (e.g., python, go, javascript)",
							},
							"hours": map[string]interface{}{
								"type":        "number",
								"description": "Hours until entry expires (default: 24)",
							},
						},
						"required": []string{"content"},
					},
				},
				{
					"name":        "create_web_folder",
					"description": "Create a shared web folder for bidirectional file sharing. Returns a token-based URL.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type":        "string",
								"description": "Folder name",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "Folder description",
							},
							"mode": map[string]interface{}{
								"type":        "string",
								"description": "Access mode: 'both' (upload+download), 'upload_only' (collect files), 'download_only' (distribute files)",
							},
							"password": map[string]interface{}{
								"type":        "string",
								"description": "Optional password to protect folder access",
							},
							"expiry_days": map[string]interface{}{
								"type":        "number",
								"description": "Days until folder expires (default: 30)",
							},
						},
						"required": []string{"name"},
					},
				},
				{
					"name":        "create_file_request",
					"description": "Create a file request link to collect files from others. Returns a shareable request URL.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"title": map[string]interface{}{
								"type":        "string",
								"description": "Title for the file request",
							},
							"description": map[string]interface{}{
								"type":        "string",
								"description": "Description of what files are needed",
							},
							"max_file_size_mb": map[string]interface{}{
								"type":        "number",
								"description": "Max file size in MB (0 = unlimited)",
							},
							"max_files": map[string]interface{}{
								"type":        "number",
								"description": "Max number of files (0 = unlimited)",
							},
							"expiry_days": map[string]interface{}{
								"type":        "number",
								"description": "Days until request expires (default: 30)",
							},
						},
						"required": []string{"title"},
					},
				},
			},
		})

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			writeError(w, req.ID, -32602, "Invalid params")
			return
		}

		switch params.Name {
		case "upload_file":
			result, err := handleToolUpload(params.Arguments)
			if err != nil {
				writeToolError(w, req.ID, err.Error())
				return
			}
			writeToolResult(w, req.ID, result)

		case "create_clipboard":
			result, err := handleToolClipboard(params.Arguments)
			if err != nil {
				writeToolError(w, req.ID, err.Error())
				return
			}
			writeToolResult(w, req.ID, result)

		case "create_web_folder":
			result, err := handleToolWebFolder(params.Arguments)
			if err != nil {
				writeToolError(w, req.ID, err.Error())
				return
			}
			writeToolResult(w, req.ID, result)

		case "create_file_request":
			result, err := handleToolFileRequest(params.Arguments)
			if err != nil {
				writeToolError(w, req.ID, err.Error())
				return
			}
			writeToolResult(w, req.ID, result)

		default:
			writeError(w, req.ID, -32601, fmt.Sprintf("Unknown tool: %s", params.Name))
		}

	case "ping":
		writeResult(w, req.ID, map[string]string{})

	default:
		writeError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func handleToolUpload(args map[string]interface{}) (string, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	fileName := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		fileName = filePath[idx+1:]
	}

	encrypt := true
	if v, ok := args["encrypt"].(bool); ok {
		encrypt = v
	}

	expiryHours := 24.0
	if v, ok := args["expiry_hours"].(float64); ok && v > 0 {
		expiryHours = v
	}

	maxDownloads := 100.0
	if v, ok := args["max_downloads"].(float64); ok && v > 0 {
		maxDownloads = v
	}

	note, _ := args["note"].(string)
	password, _ := args["password"].(string)

	// Step 1: Init upload
	initBody, _ := json.Marshal(map[string]interface{}{
		"name":          fileName,
		"file_count":    1,
		"encrypted":     encrypt,
		"password":      password,
		"max_downloads": int(maxDownloads),
		"expiry_hours":  int(expiryHours),
		"note":          note,
	})
	resp, err := http.Post(serverURL+"/api/upload/init", "application/json", bytes.NewReader(initBody))
	if err != nil {
		return "", fmt.Errorf("init upload failed: %w", err)
	}
	defer resp.Body.Close()

	var initRes struct {
		TransferID string `json:"transfer_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initRes); err != nil {
		return "", fmt.Errorf("parse init response: %w", err)
	}

	// Step 2: Upload chunk
	hash := sha256.Sum256(data)
	chunkHash := hex.EncodeToString(hash[:])

	req, _ := http.NewRequest("POST", serverURL+"/api/upload/chunk", bytes.NewReader(data))
	req.Header.Set("X-Transfer-ID", initRes.TransferID)
	req.Header.Set("X-File-ID", "null")
	req.Header.Set("X-Chunk-Index", "0")
	req.Header.Set("X-Total-Chunks", "1")
	req.Header.Set("X-File-Name", fileName)
	req.Header.Set("X-File-Size", fmt.Sprintf("%d", len(data)))
	req.Header.Set("X-Mime-Type", detectMimeType(fileName))
	req.Header.Set("X-Chunk-Hash", chunkHash)

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload chunk failed: %w", err)
	}
	resp2.Body.Close()

	// Step 3: Complete
	resp3, err := http.Post(serverURL+"/api/upload/complete/"+initRes.TransferID, "", nil)
	if err != nil {
		return "", fmt.Errorf("complete upload failed: %w", err)
	}
	defer resp3.Body.Close()

	var completeRes struct {
		DownloadURL string `json:"download_url"`
	}
	json.NewDecoder(resp3.Body).Decode(&completeRes)

	result := fmt.Sprintf("Uploaded %s (%d bytes)", fileName, len(data))
	if completeRes.DownloadURL != "" {
		result += fmt.Sprintf("\nDownload: %s", completeRes.DownloadURL)
	}

	return result, nil
}

func handleToolClipboard(args map[string]interface{}) (string, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	language, _ := args["language"].(string)
	hours := 24.0
	if v, ok := args["hours"].(float64); ok && v > 0 {
		hours = v
	}

	body, _ := json.Marshal(map[string]interface{}{
		"content":  content,
		"language": language,
		"hours":    int(hours),
	})
	resp, err := http.Post(serverURL+"/api/clipboard", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create clipboard failed: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&res)

	return fmt.Sprintf("Clipboard created.\nLink: %s\nToken: %s", res.URL, res.ID), nil
}

func handleToolWebFolder(args map[string]interface{}) (string, error) {
	name, _ := args["name"].(string)
	if name == "" {
		name = "Shared Folder"
	}
	desc, _ := args["description"].(string)
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "both"
	}
	password, _ := args["password"].(string)
	expiryDays := 30.0
	if v, ok := args["expiry_days"].(float64); ok && v > 0 {
		expiryDays = v
	}

	body, _ := json.Marshal(map[string]interface{}{
		"name":        name,
		"description": desc,
		"mode":        mode,
		"password":    password,
		"expiry_days": int(expiryDays),
	})
	resp, err := http.Post(serverURL+"/api/folders", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create folder failed: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&res)

	return fmt.Sprintf("Web folder created.\nURL: %s\nToken: %s (Mode: %s)", res.URL, res.Token, mode), nil
}

func handleToolFileRequest(args map[string]interface{}) (string, error) {
	title, _ := args["title"].(string)
	if title == "" {
		title = "File Request"
	}
	desc, _ := args["description"].(string)
	maxSizeMB := 0.0
	if v, ok := args["max_file_size_mb"].(float64); ok {
		maxSizeMB = v
	}
	maxFiles := 0.0
	if v, ok := args["max_files"].(float64); ok {
		maxFiles = v
	}
	expiryDays := 30.0
	if v, ok := args["expiry_days"].(float64); ok && v > 0 {
		expiryDays = v
	}

	body, _ := json.Marshal(map[string]interface{}{
		"title":         title,
		"description":   desc,
		"max_file_size": int64(maxSizeMB * 1024 * 1024),
		"max_files":     int(maxFiles),
		"expiry_days":   int(expiryDays),
	})
	resp, err := http.Post(serverURL+"/api/file-requests", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		URL string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&res)

	return fmt.Sprintf("File request created.\nShare this link with the sender: %s", res.URL), nil
}

func detectMimeType(name string) string {
	ext := strings.ToLower(name)
	for i := len(ext) - 1; i >= 0; i-- {
		if ext[i] == '.' {
			ext = ext[i:]
			break
		}
	}
	mimeTypes := map[string]string{
		".txt": "text/plain", ".md": "text/markdown",
		".html": "text/html", ".css": "text/css", ".js": "text/javascript",
		".json": "application/json", ".xml": "application/xml",
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".svg": "image/svg+xml", ".webp": "image/webp",
		".mp4": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg",
		".pdf": "application/pdf", ".zip": "application/zip",
		".go": "text/plain", ".py": "text/plain", ".rs": "text/plain",
		".yaml": "text/plain", ".yml": "text/plain", ".toml": "text/plain",
	}
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}

func writeResult(w *bufio.Writer, id interface{}, result interface{}) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}

func writeError(w *bufio.Writer, id interface{}, code int, message string) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message}}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}

func writeToolResult(w *bufio.Writer, id interface{}, text string) {
	writeResult(w, id, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	})
}

func writeToolError(w *bufio.Writer, id interface{}, text string) {
	writeResult(w, id, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": "Error: " + text},
		},
		"isError": true,
	})
}
