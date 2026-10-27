# Multi-Stage Build: 100% Pure Go Static Binary (Zero Node.js, Zero CGO, Embedded Web Assets)
FROM golang:alpine AS builder

WORKDIR /app

# Salin dependency Go
COPY go.mod go.sum ./
RUN go mod download

# Salin source code (termasuk folder web/static dan web/views untuk disematkan oleh go:embed)
COPY . .

# Kompilasi Single Binary Mandiri (Self-Contained)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o rahamail main.go

# Runtime Stage: Ultra Minimalis (Hanya butuh 1 file binary!)
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# HANYA MENYALIN BINARY RAHAMAIL!
# Tidak perlu lagi menyalin folder web/public atau web/views karena sudah tertanam di dalam binary
COPY --from=builder /app/rahamail /app/rahamail

# Buat direktori volume persisten (Database SQLite & Penyimpanan Fisik Email)
RUN mkdir -p /app/data /app/var/mail/inbox

EXPOSE 8080 25 587 143 993

ENV GIN_MODE=release
ENV PORT=:8080
ENV SMTP_PORT=:25
ENV IMAP_PORT=:143
ENV DB_DRIVER=sqlite
ENV SQLITE_PATH=/app/data/rahamail.db

ENTRYPOINT ["/app/rahamail"]
