# Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# Build backend
FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/server/static ./internal/server/static
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /mimo ./cmd/mimo

# Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates sqlite
COPY --from=backend /mimo /usr/local/bin/mimo

VOLUME /data
ENV MIMO_DATA_DIR=/data
ENV MIMO_DB_PATH=/data/mimo.db
ENV MIMO_HOST=0.0.0.0
ENV MIMO_PORT=8080

EXPOSE 8080

ENTRYPOINT ["mimo"]
