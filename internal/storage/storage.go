package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const BaseMailDir = "var/mail"

// GenerateSecurePath menghasilkan path penyimpanan Maildir berbasis SHA-256 hash
// Format: var/mail/{folder}/{userID}/{sha256(messageID+timestamp)}.eml
func GenerateSecurePath(folder string, userID uint, messageID string) (string, error) {
	// Sanitasi folder untuk mencegah directory traversal
	cleanFolder := filepath.Base(filepath.Clean(folder))
	if cleanFolder == "." || cleanFolder == "/" || cleanFolder == ".." {
		cleanFolder = "inbox"
	}

	// Buat hash unik berdasarkan messageID, timestamp, dan userID
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%d:%s:%d", userID, messageID, time.Now().UnixNano())))
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	userDir := filepath.Join(BaseMailDir, cleanFolder, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(userDir, 0750); err != nil {
		return "", fmt.Errorf("gagal membuat direktori penyimpanan surat: %w", err)
	}

	fullPath := filepath.Join(userDir, fileHash+".eml")
	return fullPath, nil
}

// IsPathSafe memverifikasi apakah path target berada di dalam BaseMailDir (Anti-Path Traversal)
func IsPathSafe(targetPath string) bool {
	cleanBase, err := filepath.Abs(BaseMailDir)
	if err != nil {
		return false
	}

	cleanTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}

	// Pastikan cleanTarget memiliki prefix cleanBase
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "\x00") {
		return false
	}

	return true
}

// SaveMailToFile menyimpan bytes email secara aman ke disk
func SaveMailToFile(folder string, userID uint, messageID string, data []byte) (string, error) {
	targetPath, err := GenerateSecurePath(folder, userID, messageID)
	if err != nil {
		return "", err
	}

	if !IsPathSafe(targetPath) {
		return "", fmt.Errorf("insecure path detected: %s", targetPath)
	}

	if err := os.WriteFile(targetPath, data, 0600); err != nil {
		return "", fmt.Errorf("gagal menulis file email: %w", err)
	}

	return targetPath, nil
}
