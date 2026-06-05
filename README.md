<p align="center">
  <img src="docs/logo.svg" width="120" alt="MIMO Logo">
</p>

<h1 align="center">MIMO</h1>
<p align="center"><strong>Modern Intelligent Managed Operations</strong></p>
<p align="center">Self-hosted file transfer platform with E2E encryption, P2P direct transfer, and enterprise features.</p>

<p align="center">
  <a href="#features">Features</a> &bull;
  <a href="#quick-start">Quick Start</a> &bull;
  <a href="#deployment">Deployment</a> &bull;
  <a href="#api">API</a> &bull;
  <a href="#architecture">Architecture</a> &bull;
  <a href="#roadmap">Roadmap</a> &bull;
  <a href="#license">License</a>
</p>

---

## What is MIMO?

MIMO is a self-hosted file transfer platform that combines the simplicity of WeTransfer with the security of end-to-end encryption and the speed of peer-to-peer transfer -- all in a single binary.

**Why MIMO over alternatives?**

| Feature | WeTransfer | Send | LocalSend | croc | **MIMO** |
|---------|:----------:|:----:|:---------:|:----:|:--------:|
| Self-hosted | - | Y | Y | Y | **Y** |
| Web UI | Y | Y | Y | - | **Y** |
| E2E Encryption | - | Y | - | Y | **Y** |
| P2P Direct Transfer | - | - | Y | Y | **Y** |
| Chunked Upload / Resume | - | - | - | - | **Y** |
| Multi-user / RBAC | Y | - | - | - | **Y** |
| Brand Customization | Pro | - | - | - | **Y** |
| Network Clipboard | - | - | - | - | **Y** |
| Single Binary Deploy | - | - | - | Y | **Y** |
| No Registration Required | - | Y | Y | Y | **Y** |

## Features

### Core Transfer
- **Dual Mode Transfer** -- Link mode (upload to server) + P2P mode (direct WebRTC transfer)
- **Chunked Upload** -- Automatic file splitting with parallel chunk upload
- **Resume Support** -- Continue interrupted uploads from where they left off
- **No File Size Limit** -- Send files of any size
- **Real-time Progress** -- Upload speed, ETA, and percentage tracking

### Security
- **End-to-End Encryption** -- AES-256-GCM encryption on the client side
- **Zero Knowledge** -- Encryption key stays in the URL fragment, never sent to server
- **Password Protection** -- Optional Argon2id password protection for extra security
- **Expiring Links** -- Auto-expire by time and/or download count
- **Audit Logs** -- Track all file sharing activities

### Enterprise
- **Multi-user System** -- Admin, member, and viewer roles
- **Brand Customization** -- Custom logo, colors, and domain
- **REST API** -- Full programmatic access
- **SMTP Integration** -- Email notifications for uploads, downloads, and expiry
- **Storage Quotas** -- Per-user storage limits

### Extra Tools
- **Network Clipboard** -- Share text and code snippets via simple links
- **P2P Transfer** -- 6-digit code pairing for direct browser-to-browser transfer
- **File Previews** -- Preview images, documents, and media files
- **QR Codes** -- Generate QR codes for easy mobile sharing

## Quick Start

### Docker (Recommended)

```bash
docker run -d \
  --name mimo \
  -p 8080:8080 \
  -v mimo-data:/data \
  -e MIMO_PUBLIC_URL=https://files.example.com \
  -e MIMO_JWT_SECRET=your-secret-key-here \
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
      - MIMO_JWT_SECRET=your-secret-key-here
```

```bash
docker compose up -d
```

### Binary

```bash
# Download from releases
curl -L https://github.com/vesper/mimo/releases/latest/download/mimo-linux-amd64 -o mimo
chmod +x mimo
./mimo
```

### Build from Source

```bash
# Clone
git clone https://github.com/vesper/mimo.git
cd mimo

# Build frontend
cd web && npm install && npm run build && cd ..

# Build backend
go build -o mimo ./cmd/mimo

# Run
./mimo
```

Visit `http://localhost:8080` -- the first registered user becomes admin.

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MIMO_HOST` | `0.0.0.0` | Listen host |
| `MIMO_PORT` | `8080` | Listen port |
| `MIMO_PUBLIC_URL` | `http://localhost:8080` | Public-facing URL |
| `MIMO_DATA_DIR` | `./data` | Data storage directory |
| `MIMO_DB_PATH` | `./data/mimo.db` | SQLite database path |
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

### Upload a file

```bash
# 1. Initialize transfer
curl -X POST /api/upload/init \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-files", "encrypted": true}'

# 2. Upload chunks
curl -X POST /api/upload/chunk \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Transfer-ID: abc123" \
  -H "X-File-Name: photo.jpg" \
  -H "X-File-Size: 5242880" \
  -H "X-Chunk-Index: 0" \
  -H "X-Total-Chunks: 1" \
  --data-binary @chunk.bin

# 3. Complete transfer
curl -X POST /api/upload/complete/abc123 \
  -H "Authorization: Bearer $TOKEN"
```

### Download a file

```bash
curl /api/t/abc123/download/file456 -o output.jpg
```

### Create clipboard entry

```bash
curl -X POST /api/clipboard \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello, World!", "language": "text", "hours": 24}'
```

### Create P2P session

```bash
curl -X POST /api/p2p/create
# Returns: {"code": "A1B2C3", "url": "https://files.example.com/p2p/A1B2C3"}
```

## Architecture

```
Client (Browser)
  |
  |-- Upload: HTTP chunked POST -> Go server -> Local FS / S3
  |-- Download: HTTP GET -> Go server -> Stream from storage
  |-- P2P: WebSocket signaling -> WebRTC DataChannel (direct)
  |-- Clipboard: HTTP POST/GET -> SQLite
```

### Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go + chi + SQLite |
| Frontend | SvelteKit + Tailwind CSS |
| P2P | WebRTC (pion/webrtc) |
| Real-time | WebSocket (coder/websocket) |
| Encryption | AES-256-GCM + Argon2id |
| Storage | Local FS / MinIO / S3 |
| Auth | JWT (golang-jwt) |
| Deployment | Docker / Single binary |

### Project Structure

```
mimo/
  cmd/mimo/           # Entry point
  internal/
    config/           # Configuration
    crypto/           # E2E encryption
    db/               # SQLite database layer
    model/            # Data models
    server/           # HTTP/WebSocket handlers
    storage/          # Storage abstraction
    user/             # Authentication
  web/                # SvelteKit frontend
  migrations/         # Database migrations
  Dockerfile
  docker-compose.yml
```

## Roadmap

- [x] Chunked upload with resume
- [x] E2E encryption (AES-256-GCM)
- [x] P2P transfer via WebRTC
- [x] Network clipboard
- [x] Multi-user system
- [x] Expiring links (time + download count)
- [x] Docker deployment
- [ ] File previews (image/video/document)
- [ ] Email notifications (SMTP)
- [ ] Brand customization (logo, colors, domain)
- [ ] REST API + CLI tool
- [ ] SSO/OIDC integration
- [ ] Audit logs dashboard
- [ ] Storage quotas
- [ ] ShareX integration
- [ ] Chrome extension
- [ ] Mobile apps (iOS/Android)
- [ ] WebTorrent multi-peer distribution
- [ ] Screen sharing

## Contributing

Contributions welcome. Please read the contributing guidelines before submitting PRs.

1. Fork the repository
2. Create your feature branch (`git checkout -b feat/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feat/amazing-feature`)
5. Open a Pull Request

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<p align="center">
  Built with Go and SvelteKit
</p>
