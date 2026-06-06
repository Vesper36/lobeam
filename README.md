# LoBeam

**Modern Intelligent Managed Operations** -- Self-hosted file transfer platform.

The only self-hosted file transfer that combines **E2E encryption + P2P direct transfer + chunked upload/续传 + multi-user + brand customization + 网络剪贴板 + file requests + web folders** -- in a single binary.

[Quick Start](#quick-start) | [Features](#features) | [API](#api) | [Deployment](#deployment) | [CLI](#cli) | [Roadmap](#roadmap)

### Live

| Resource | URL |
|----------|-----|
| Demo | [lobeam.demo.vesper36.cc](https://lobeam.demo.vesper36.cc) |
| Docs | [lobeam.docs.vesper36.cc](https://lobeam.docs.vesper36.cc) |
| API Port | `50030` |

---

## Why LoBeam?

| Feature | WeTransfer | Send | LocalSend | croc | **LoBeam** |
|---------|:----------:|:----:|:---------:|:----:|:--------:|
| Self-hosted | - | Y | Y | Y | **Y** |
| Web UI | Y | Y | Y | - | **Y** |
| E2E Encryption | - | Y | - | Y | **Y** |
| P2P Direct Transfer | - | - | Y | Y | **Y** |
| Screen/Camera Sharing | - | - | - | - | **Y** |
| Chunked Upload / Resume | - | - | - | - | **Y** |
| Resume Download (HTTP Range) | - | - | - | - | **Y** |
| Multi-user / RBAC | Y | - | - | - | **Y** |
| Brand Customization | Pro | - | - | - | **Y** |
| Network Clipboard | - | - | - | - | **Y** |
| File Requests | Y | - | - | - | **Y** |
| Web Folders | Y | - | - | - | **Y** |
| File Previews | Y | - | Y | - | **Y** |
| QR Codes | - | - | - | - | **Y** |
| Email Notifications | Y | - | - | - | **Y** |
| CLI Tool | - | - | - | Y | **Y** |
| MCP Server (AI) | - | - | - | - | **Y** |
| Embed Form | - | - | - | - | **Y** |
| Single Binary Deploy | - | - | - | Y | **Y** |
| No Registration Required | - | Y | Y | Y | **Y** |

## Features

### Core Transfer
- **Dual Mode Transfer** -- Link mode (upload to server) + P2P mode (direct WebRTC transfer)
- **Chunked Upload / 续传** -- Automatic file splitting with parallel chunk upload, resume from interruption
- **No File Size Limit** -- Send files of any size
- **Real-time Progress** -- Upload speed, ETA, and percentage tracking
- **Real-time Sharing** -- Start sharing before upload completes

### Web Folders
- **双向共享文件箱** -- Multiple users can upload and download
- **Mode Control** -- `upload_only` (collect), `download_only` (distribute), or `both` (collaborate)
- **Anonymous Access** -- No registration required, simple token-based URL
- **Password Protection** -- Optional Argon2id password for sensitive folders

### File Requests
- **Request Files from Anyone** -- Generate a link to collect files from clients/users
- **Custom Form Fields** -- Collect sender info (name, email, custom fields)
- **File Type & Size Limits** -- Restrict what can be uploaded
- **Auto Expiry** -- Requests auto-close after a configured period

### Security
- **End-to-End Encryption** -- AES-256-GCM encryption on the client side
- **Zero Knowledge** -- Encryption key stays in the URL fragment, never sent to server
- **Password Protection** -- Optional password protection for transfers
- **Expiring Links** -- Auto-expire by time and/or download count
- **Audit Logs** -- Track all file sharing activities

### Enterprise
- **Multi-user System** -- Admin, member, and viewer roles
- **Brand Customization** -- Custom logo, colors, domain, CSS, HTML
- **REST API** -- Full programmatic access
- **SMTP Integration** -- Email notifications for uploads, downloads, expiry
- **Storage Quotas** -- Per-user storage limits

### Brand Customization (Whitelabel)
- **Custom Domain** -- `files.yourcompany.com` with matching brand
- **Custom Logo & Colors** -- Full color scheme control
- **Custom CSS/HTML** -- Match your brand guidelines exactly
- **Hide "Powered By"** -- White-label option
- **Custom Email Templates** -- Branded sender address and footer

### Embedded Upload Form
- **一行 HTML 集成** -- Drop the upload form into any website
- **Custom Form Fields** -- Add your own fields
- **Auto Brand Match** -- Inherits your LoBeam brand settings

### Extra Tools
- **Network Clipboard** -- Share text, code, notes via simple links
- **P2P Transfer** -- 6-digit code pairing for direct browser-to-browser
- **File Previews** -- Preview images, documents, media
- **QR Codes** -- Generate QR codes for mobile sharing
- **Real-time Sharing** -- Start sharing before upload completes

### CLI Tool
- **Cross-platform** -- Linux, macOS, Windows
- **Upload from Scripts** -- Automate file transfers
- **Auth Tokens** -- Save credentials securely

## Quick Start

### Docker (Recommended)

```bash
docker run -d \
  --name lobeam \
  -p 50030:50030 \
  -v lobeam-data:/data \
  -e LOBEAM_PUBLIC_URL=https://files.example.com \
  -e LOBEAM_JWT_SECRET=$(openssl rand -hex 32) \
  lobeam/lobeam:latest
```

### Docker Compose

```yaml
services:
  lobeam:
    image: lobeam/lobeam:latest
    ports:
      - "50030:50030"
    volumes:
      - lobeam-data:/data
    environment:
      - LOBEAM_PUBLIC_URL=https://files.example.com
      - LOBEAM_JWT_SECRET=your-secret-key
```

### Single Binary

```bash
# Linux/macOS
curl -L https://github.com/Vesper36/lobeam/releases/latest/download/lobeam-linux-amd64 -o lobeam
chmod +x lobeam
./lobeam
```

### Build from Source

```bash
git clone https://github.com/Vesper36/lobeam.git
cd lobeam
make build
./dist/lobeam
```

Visit `http://localhost:50030` -- the first registered user becomes admin.

## CLI

```bash
# Install CLI
curl -L https://github.com/Vesper36/lobeam/releases/latest/download/lobeam-cli-linux-amd64 -o lobeam-cli
chmod +x lobeam-cli

# Login
./lobeam-cli login -server https://lobeam.example.com -user alice -pass secret

# Upload
./lobeam-cli upload ./file.zip
./lobeam-cli upload -encrypted -expiry 168 -downloads 5 ./secret.pdf
./lobeam-cli upload -note "Quarterly report" ./Q4-2026.pdf
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LOBEAM_HOST` | `0.0.0.0` | Listen host |
| `LOBEAM_PORT` | `50030` | Listen port |
| `LOBEAM_PUBLIC_URL` | `http://localhost:50030` | Public-facing URL |
| `LOBEAM_DATA_DIR` | `./data` | Data storage directory |
| `LOBEAM_JWT_SECRET` | `change-me` | JWT signing secret |
| `LOBEAM_MAX_FILE_SIZE` | `0` (unlimited) | Max file size in bytes |
| `LOBEAM_TRANSFER_EXPIRY_HOURS` | `24` | Default transfer expiry |
| `LOBEAM_MAX_DOWNLOADS` | `100` | Default max downloads |
| `LOBEAM_ALLOW_ANONYMOUS` | `true` | Allow anonymous uploads |
| `LOBEAM_SMTP_HOST` | - | SMTP server for notifications |
| `LOBEAM_SMTP_PORT` | `587` | SMTP port |
| `LOBEAM_SMTP_USERNAME` | - | SMTP username |
| `LOBEAM_SMTP_PASSWORD` | - | SMTP password |
| `LOBEAM_SMTP_FROM` | - | Sender email address |

## API

### Public Endpoints

```bash
# Get brand config
GET /api/brand

# Get system settings
GET /api/settings

# Get transfer info
GET /api/t/{id}
GET /api/t/{id}/files
GET /api/t/{id}/download/{fileID}  # Supports HTTP Range for resumeable downloads

# Email a transfer link
POST /api/t/{id}/email

# Upload (no auth required for anonymous)
POST /api/upload/init               # Returns download_url immediately (real-time sharing)
POST /api/upload/chunk              # Supports resume: sends resumed=true if chunk exists
POST /api/upload/complete/{id}

# Network clipboard
POST /api/clipboard
GET /api/clipboard/{id}

# P2P transfer
POST /api/p2p/create
GET /api/p2p/{code}
GET /api/p2p/ws/{code}             # WebSocket for WebRTC signaling

# Web folders (public access)
GET /api/f/{token}
GET /api/f/{token}/files
POST /api/f/{token}/upload
GET /api/f/{token}/download/{fileID}

# File requests
GET /api/r/{id}
POST /api/r/{id}/submit
```

### Authenticated Endpoints

```bash
POST /api/auth/register
POST /api/auth/login
POST /api/auth/refresh

GET /api/me
GET /api/transfers
DELETE /api/transfers/{id}

POST /api/file-requests      # Create file request
GET /api/file-requests       # List my file requests

POST /api/folders            # Create web folder
GET /api/folders             # List my folders
```

### Admin Endpoints

```bash
GET /api/admin/users
GET /api/admin/logs
DELETE /api/admin/users/{id}
POST /api/brand              # Update brand
POST /api/settings           # Update system settings
```

## Embedded Upload Form

Add this to any website to embed an upload form:

```html
<iframe
  src="https://files.example.com/embed/upload"
  width="100%"
  height="500"
  frameborder="0"
  allow="camera"
></iframe>
```

Or use a button:

```html
<a href="https://files.example.com/embed/upload" target="_blank">
  Send us a file
</a>
```

## MCP Server (AI Tool Integration)

LoBeam includes a built-in MCP server, letting AI tools like Claude, Cursor, and other MCP-compatible clients upload files and get shareable download links.

```json
// Claude Desktop / Cursor config
{
  "mcpServers": {
    "lobeam": {
      "command": "lobeam-mcp",
      "args": ["-server", "https://files.example.com"]
    }
  }
}
```

Available tools:
- `upload_file` -- Upload a file and get a shareable download link
- `create_clipboard` -- Create a network clipboard entry
- `create_web_folder` -- Create a shared web folder
- `create_file_request` -- Create a file request link to collect files

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Client (Browser)                       │
├──────────────┬──────────────┬──────────────┬────────────────┤
│  Upload UI   │  Download UI │  P2P UI      │  Brand Config  │
│  (chunked)   │  (streamed)  │  (WebRTC)    │  (CSS vars)    │
└──────────────┴──────────────┴──────────────┴────────────────┘
       │              │              │              │
       ▼              ▼              ▼              ▼
┌──────────────────────────────────────────────────────────────┐
│                    Go HTTP Server (chi)                       │
├──────────────┬──────────────┬──────────────┬────────────────┤
│  /api/upload │  /api/t/...  │  /api/p2p    │  /api/brand    │
│  /api/folder │  /api/clip.. │  /api/req    │  /api/settings │
└──────────────┴──────────────┴──────────────┴────────────────┘
       │              │              │              │
       ▼              ▼              ▼              ▼
┌──────────────────────────────────────────────────────────────┐
│   SQLite (metadata) + Local FS (files) + WebSocket (signaling)│
└──────────────────────────────────────────────────────────────┘
```

### Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go + chi + SQLite |
| Frontend | SvelteKit + Tailwind CSS |
| P2P | WebRTC (browser-native) |
| Real-time | WebSocket (coder/websocket) |
| Encryption | AES-256-GCM + Argon2id |
| Storage | Local FS / MinIO / S3 |
| Auth | JWT (golang-jwt) |
| Deployment | Docker / Single binary |

## Roadmap

- [x] Chunked upload with 续传
- [x] E2E encryption (AES-256-GCM)
- [x] P2P transfer via WebRTC
- [x] P2P screen & camera sharing
- [x] Network clipboard
- [x] Multi-user system
- [x] Expiring links (time + download count)
- [x] Docker deployment
- [x] Brand customization (whitelabel)
- [x] Web folders (multi-user collections)
- [x] File requests
- [x] CLI tool
- [x] MCP server (AI tool integration)
- [x] Email notifications (SMTP + auto-send)
- [x] File previews (image/video/audio/document)
- [x] QR code generation
- [x] Embedded upload form (1-line HTML)
- [x] Real-time sharing (share before upload completes)
- [x] HTTP Range download (resumeable downloads)
- [x] Chunk resume upload (skip already uploaded chunks)
- [x] Admin panel (users, brand, settings, audit logs)
- [ ] SSO/OIDC integration
- [ ] ShareX integration
- [ ] Chrome extension
- [ ] Discord/Slack bot
- [ ] Outlook plugin
- [ ] Mobile apps (iOS/Android)
- [ ] WebTorrent multi-peer distribution

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<p align="center">
  Built with Go and SvelteKit
</p>
