# MIMO

**Modern Intelligent Managed Operations** -- Self-hosted file transfer platform.

The only self-hosted file transfer that combines **E2E encryption + P2P direct transfer + chunked upload/续传 + multi-user + brand customization + 网络剪贴板 + file requests + web folders** -- in a single binary.

[Quick Start](#quick-start) | [Features](#features) | [API](#api) | [Deployment](#deployment) | [CLI](#cli) | [Roadmap](#roadmap)

---

## Why MIMO?

| Feature | WeTransfer | Send | LocalSend | croc | **MIMO** |
|---------|:----------:|:----:|:---------:|:----:|:--------:|
| Self-hosted | - | Y | Y | Y | **Y** |
| Web UI | Y | Y | Y | - | **Y** |
| E2E Encryption | - | Y | - | Y | **Y** |
| P2P Direct Transfer | - | - | Y | Y | **Y** |
| Chunked Upload / 续传 | - | - | - | - | **Y** |
| Multi-user / RBAC | Y | - | - | - | **Y** |
| Brand Customization | Pro | - | - | - | **Y** |
| Network Clipboard | - | - | - | - | **Y** |
| File Requests | Y | - | - | - | **Y** |
| Web Folders | Y | - | - | - | **Y** |
| CLI Tool | - | - | - | Y | **Y** |
| Single Binary Deploy | - | - | - | Y | **Y** |
| No Registration Required | - | Y | Y | Y | **Y** |
| Embed via 1-line HTML | - | - | - | - | **Y** |

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
- **Auto Brand Match** -- Inherits your MIMO brand settings

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
  --name mimo \
  -p 8080:8080 \
  -v mimo-data:/data \
  -e MIMO_PUBLIC_URL=https://files.example.com \
  -e MIMO_JWT_SECRET=$(openssl rand -hex 32) \
  mimo/mimo:latest
```

### Docker Compose

```yaml
services:
  mimo:
    image: mimo/mimo:latest
    ports:
      - "8080:8080"
    volumes:
      - mimo-data:/data
    environment:
      - MIMO_PUBLIC_URL=https://files.example.com
      - MIMO_JWT_SECRET=your-secret-key
```

### Single Binary

```bash
# Linux/macOS
curl -L https://github.com/Vesper36/mimo/releases/latest/download/mimo-linux-amd64 -o mimo
chmod +x mimo
./mimo
```

### Build from Source

```bash
git clone https://github.com/Vesper36/mimo.git
cd mimo
make build
./dist/mimo
```

Visit `http://localhost:8080` -- the first registered user becomes admin.

## CLI

```bash
# Install CLI
curl -L https://github.com/Vesper36/mimo/releases/latest/download/mimo-cli-linux-amd64 -o mimo-cli
chmod +x mimo-cli

# Login
./mimo-cli login -server https://mimo.example.com -user alice -pass secret

# Upload
./mimo-cli upload ./file.zip
./mimo-cli upload -encrypted -expiry 168 -downloads 5 ./secret.pdf
./mimo-cli upload -note "Quarterly report" ./Q4-2026.pdf
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MIMO_HOST` | `0.0.0.0` | Listen host |
| `MIMO_PORT` | `8080` | Listen port |
| `MIMO_PUBLIC_URL` | `http://localhost:8080` | Public-facing URL |
| `MIMO_DATA_DIR` | `./data` | Data storage directory |
| `MIMO_JWT_SECRET` | `change-me` | JWT signing secret |
| `MIMO_MAX_FILE_SIZE` | `0` (unlimited) | Max file size in bytes |
| `MIMO_TRANSFER_EXPIRY_HOURS` | `24` | Default transfer expiry |
| `MIMO_MAX_DOWNLOADS` | `100` | Default max downloads |
| `MIMO_ALLOW_ANONYMOUS` | `true` | Allow anonymous uploads |
| `MIMO_SMTP_HOST` | - | SMTP server for notifications |
| `MIMO_SMTP_PORT` | `587` | SMTP port |
| `MIMO_SMTP_USERNAME` | - | SMTP username |
| `MIMO_SMTP_PASSWORD` | - | SMTP password |
| `MIMO_SMTP_FROM` | - | Sender email address |

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
GET /api/t/{id}/download/{fileID}

# Upload (no auth required for anonymous)
POST /api/upload/init
POST /api/upload/chunk
POST /api/upload/complete/{id}

# Network clipboard
POST /api/clipboard
GET /api/clipboard/{id}

# P2P transfer
POST /api/p2p/create
GET /api/p2p/{code}

# Web folders (public access)
GET /api/f/{token}
GET /api/f/{token}/files
POST /api/f/{token}/upload
GET /api/f/{token}/download/{fileID}

# File requests
GET /api/r/{id}
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
- [x] Network clipboard
- [x] Multi-user system
- [x] Expiring links (time + download count)
- [x] Docker deployment
- [x] Brand customization (whitelabel)
- [x] Web folders (multi-user collections)
- [x] File requests
- [x] CLI tool
- [x] Email notifications (SMTP)
- [ ] File previews (image/video/document)
- [ ] SSO/OIDC integration
- [ ] Audit logs dashboard
- [ ] ShareX integration
- [ ] Chrome extension
- [ ] Discord/Slack bot
- [ ] Mobile apps (iOS/Android)
- [ ] WebTorrent multi-peer distribution
- [ ] Screen sharing

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<p align="center">
  Built with Go and SvelteKit
</p>
