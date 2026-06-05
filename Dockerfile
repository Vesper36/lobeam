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
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /lobeam ./cmd/lobeam

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
