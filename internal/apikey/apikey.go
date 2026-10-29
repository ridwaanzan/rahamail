package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	Prefix = "rhm_live_"
)

// GenerateAPIKey membuat token API baru berawalan `rhm_live_` dengan 32 bytes entropi
// Return: plaintextToken (hanya ditampilkan sekali kepada user), prefix (untuk identifikasi di UI), hash (untuk disimpan di DB)
func GenerateAPIKey() (rawToken string, prefix string, tokenHash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", err
	}
	rawToken = Prefix + hex.EncodeToString(bytes)
	prefix = rawToken[:13] // e.g. "rhm_live_a1b2"
	tokenHash = HashToken(rawToken)
	return rawToken, prefix, tokenHash, nil
}

// HashToken melakukan SHA-256 hash pada raw token
func HashToken(rawToken string) string {
	rawToken = strings.TrimSpace(rawToken)
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}

// VerifyToken memverifikasi apakah rawToken cocok dengan storedHash
func VerifyToken(rawToken, storedHash string) bool {
	computed := HashToken(rawToken)
	return computed == strings.TrimSpace(storedHash)
}

// ValidateFormat memeriksa apakah token memiliki format yang valid
func ValidateFormat(rawToken string) bool {
	rawToken = strings.TrimSpace(rawToken)
	if !strings.HasPrefix(rawToken, Prefix) {
		return false
	}
	body := strings.TrimPrefix(rawToken, Prefix)
	if len(body) != 64 { // 32 bytes hex = 64 chars
		return false
	}
	_, err := hex.DecodeString(body)
	return err == nil
}
