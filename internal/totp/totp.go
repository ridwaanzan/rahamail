package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GenerateSecret menghasilkan 160-bit (20-byte) random secret ter-encode Base32 (RFC 6238 / RFC 4226)
func GenerateSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateCode menghasilkan 6 digit kode TOTP untuk timestamp tertentu (step 30 detik)
func GenerateCode(secret string, t time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanSecret)
	if err != nil {
		// Coba dengan padding jika gagal
		key, err = base32.StdEncoding.DecodeString(cleanSecret)
		if err != nil {
			return "", fmt.Errorf("invalid base32 secret: %w", err)
		}
	}

	counter := uint64(t.Unix() / 30)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 section 5.4)
	offset := hash[len(hash)-1] & 0x0f
	binaryCode := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	code := binaryCode % 1000000

	return fmt.Sprintf("%06d", code), nil
}

// ValidateCode memverifikasi apakah passCode yang diberikan cocok dalam window time skew (default +/- 1 step = +/- 30s)
func ValidateCode(secret, passCode string, t time.Time) bool {
	passCode = strings.TrimSpace(passCode)
	if len(passCode) != 6 {
		return false
	}
	if _, err := strconv.Atoi(passCode); err != nil {
		return false
	}

	// Cek skew: -1 step, 0 step (current), +1 step
	for skew := -1; skew <= 1; skew++ {
		targetTime := t.Add(time.Duration(skew*30) * time.Second)
		expected, err := GenerateCode(secret, targetTime)
		if err != nil {
			continue
		}
		if hmac.Equal([]byte(expected), []byte(passCode)) {
			return true
		}
	}
	return false
}

// GenerateBackupCodes menghasilkan n recovery backup codes acak (misal 8 kode masing-masing 8 karakter hex)
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		codes[i] = fmt.Sprintf("%04x-%04x", bytes[0:2], bytes[2:4])
	}
	return codes, nil
}

// GenerateKeyURI menghasilkan URI standar `otpauth://totp/...` untuk pembacaan aplikasi Authenticator
func GenerateKeyURI(issuer, accountName, secret string) string {
	cleanSecret := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	v := url.Values{}
	v.Set("secret", cleanSecret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")

	label := fmt.Sprintf("%s:%s", issuer, accountName)
	return fmt.Sprintf("otpauth://totp/%s?%s", url.PathEscape(label), v.Encode())
}
