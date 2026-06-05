# Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# Build backend (alpine + gcc for musl-linked CGO binary)
FROM golang:1.24-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=auto go mod download
COPY . .
COPY --from=frontend /app/cmd/lobeam/static ./cmd/lobeam/static
RUN CGO_ENABLED=1 GOTOOLCHAIN=auto go build -ldflags="-s -w" -o /lobeam ./cmd/lobeam

# Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates sqlite
COPY --from=backend /lobeam /usr/local/bin/lobeam

VOLUME /data
ENV LOBEAM_DATA_DIR=/data
ENV LOBEAM_DB_PATH=/data/lobeam.db
ENV LOBEAM_HOST=0.0.0.0
ENV LOBEAM_PORT=50030

EXPOSE 50030

ENTRYPOINT ["lobeam"]
