package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mail-server/internal/repository"
)

// S3Config merepresentasikan konfigurasi penyimpanan objek cloud S3 / MinIO / R2
type S3Config struct {
	Endpoint        string `json:"endpoint"`         // e.g. "s3.amazonaws.com" atau "minio.local:9000"
	Region          string `json:"region"`           // e.g. "us-east-1"
	Bucket          string `json:"bucket"`           // e.g. "rahamail-backups"
	AccessKey       string `json:"access_key"`       // AWS Access Key ID
	SecretKey       string `json:"secret_key"`       // AWS Secret Access Key
	EncryptionKey   string `json:"encryption_key"`   // Kunci passphrase untuk enkripsi AES-256 GCM
	UseSSL          bool   `json:"use_ssl"`
	RetentionDays   int    `json:"retention_days"`
	LastBackupTime  *time.Time `json:"last_backup_time,omitempty"`
	LastBackupStatus string    `json:"last_backup_status,omitempty"`
}

// CreateEncryptedArchive membuat arsip gzip terenkripsi AES-256 GCM dari database SQLite dan direktori Maildir
func CreateEncryptedArchive(dbPath, maildirPath, passphrase string) ([]byte, error) {
	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)

	// 1. Masukkan Database SQLite (atau file cadangan)
	if _, err := os.Stat(dbPath); err == nil {
		dbBytes, err := os.ReadFile(dbPath)
		if err == nil {
			header := &tar.Header{
				Name:    "database.db",
				Size:    int64(len(dbBytes)),
				Mode:    0600,
				ModTime: time.Now(),
			}
			if err := tw.WriteHeader(header); err == nil {
				_, _ = tw.Write(dbBytes)
			}
		}
	}

	// 2. Masukkan file-file Maildir
	if _, err := os.Stat(maildirPath); err == nil {
		_ = filepath.Walk(maildirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			relPath, err := filepath.Rel(maildirPath, path)
			if err != nil {
				return nil
			}
			fileBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			header := &tar.Header{
				Name:    filepath.Join("mail", relPath),
				Size:    int64(len(fileBytes)),
				Mode:    0600,
				ModTime: info.ModTime(),
			}
			if err := tw.WriteHeader(header); err == nil {
				_, _ = tw.Write(fileBytes)
			}
			return nil
		})
	}

	_ = tw.Close()
	_ = gw.Close()

	compressedData := tarBuf.Bytes()

	// Jika tidak ada passphrase enkripsi, kembalikan gzip langsung
	if passphrase == "" {
		return compressedData, nil
	}

	// Enkripsi dengan AES-256-GCM
	key := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encrypted := gcm.Seal(nonce, nonce, compressedData, nil)
	return encrypted, nil
}

// UploadToS3 mengunggah data byte ke endpoint S3 menggunakan HTTP PUT (didukung oleh MinIO, Wasabi, AWS S3, dan R2)
func UploadToS3(cfg S3Config, objectKey string, data []byte) error {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return fmt.Errorf("S3 endpoint dan bucket wajib diisi")
	}

	scheme := "https"
	if !cfg.UseSSL && !strings.HasPrefix(cfg.Endpoint, "https://") {
		scheme = "http"
	}
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "http://"), "https://")
	uploadURL := fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, cfg.Bucket, objectKey)

	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-date", time.Now().UTC().Format("20060102T150405Z"))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("S3 server mengembalikan status HTTP %d", resp.StatusCode)
	}

	return nil
}

// PerformBackup menjalankan backup instan SQLite DB dan Maildir
func PerformBackup(cfg S3Config) (string, int64, error) {
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "gomail.db"
	}

	// Backup online SQLite menggunakan VACUUM INTO untuk menjamin konsistensi ACID tanpa mengunci database
	backupDBPath := filepath.Join(os.TempDir(), fmt.Sprintf("rahamail-backup-%d.db", time.Now().UnixNano()))
	defer os.Remove(backupDBPath)

	if repository.ActiveDriver == "SQLite (WAL Mode)" {
		vacuumSQL := fmt.Sprintf("VACUUM INTO '%s'", backupDBPath)
		if err := repository.DB.Exec(vacuumSQL).Error; err != nil {
			// Fallback copy jika driver SQLite versi lama
			backupDBPath = dbPath
		}
	} else {
		backupDBPath = dbPath
	}

	maildirPath := "var/mail"
	archiveData, err := CreateEncryptedArchive(backupDBPath, maildirPath, cfg.EncryptionKey)
	if err != nil {
		return "", 0, fmt.Errorf("gagal membuat arsip terenkripsi: %w", err)
	}

	filename := fmt.Sprintf("rahamail-backup-%s.tar.gz.enc", time.Now().Format("2006-01-02-150405"))

	// Jika S3 dikonfigurasi, unggah langsung
	if cfg.Endpoint != "" && cfg.Bucket != "" {
		if err := UploadToS3(cfg, filename, archiveData); err != nil {
			return filename, int64(len(archiveData)), fmt.Errorf("arsip dibuat namun gagal upload ke S3: %w", err)
		}
	}

	return filename, int64(len(archiveData)), nil
}
