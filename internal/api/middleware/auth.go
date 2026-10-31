package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"mail-server/internal/apikey"
	"mail-server/internal/domain"
	"mail-server/internal/repository"

	"github.com/gin-gonic/gin"
)

var sessionSecret = []byte(getSecretKey())

func getSecretKey() string {
	key := os.Getenv("SESSION_SECRET")
	if key == "" {
		key = "rahamail-super-secure-production-secret-session-key-2026"
	}
	return key
}

// GenerateSessionToken membuat token session terenkripsi HMAC-SHA256: "userID:timestamp:signature"
func GenerateSessionToken(userID uint) string {
	ts := time.Now().Unix()
	payload := fmt.Sprintf("%d:%d", userID, ts)
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%s", payload, sig)
}

// ValidateSessionToken memeriksa validitas signature dan masa berlaku token (7 hari)
func ValidateSessionToken(token string) (uint, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return 0, false
	}

	payload := fmt.Sprintf("%s:%s", parts[0], parts[1])
	sig := parts[2]

	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, false
	}

	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix()-ts > 7*86400 {
		return 0, false // Expired jika lebih dari 7 hari
	}

	uid, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}

	return uint(uid), true
}

// UserAuthMiddleware memastikan pengguna memiliki cookie sesi yang valid
func UserAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("rahamail_session")
		if err != nil || token == "" {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan. Silakan login."})
			} else {
				c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
			}
			c.Abort()
			return
		}

		userID, ok := ValidateSessionToken(token)
		if !ok {
			c.SetCookie("rahamail_session", "", -1, "/", "", false, true)
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi telah berakhir. Silakan login kembali."})
			} else {
				c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
			}
			c.Abort()
			return
		}

		var user domain.User
		if err := repository.DB.Preload("Domain").First(&user, userID).Error; err != nil || !user.IsActive {
			c.SetCookie("rahamail_session", "", -1, "/", "", false, true)
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Akun tidak ditemukan atau non-aktif."})
			} else {
				c.Redirect(http.StatusFound, "/login")
			}
			c.Abort()
			return
		}

		// Simpan user ke context Gin
		c.Set("user", &user)
		c.Set("user_id", user.ID)
		c.Next()
	}
}





// AdminAuthMiddleware memastikan pengguna adalah admin
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		val, exists := c.Get("user")
		if !exists {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan. Silakan login."})
			} else {
				c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
			}
			c.Abort()
			return
		}

		user, ok := val.(*domain.User)
		if !ok || !user.IsAdmin {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Hanya Super Admin yang diizinkan."})
			} else {
				// Redirect user non-admin ke webmail miliknya sendiri
				c.Redirect(http.StatusFound, "/mail")
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// Generate2FAPendingToken membuat token sementara berdurasi pendek (5 menit) untuk tahap verifikasi 2FA
func Generate2FAPendingToken(userID uint) string {
	ts := time.Now().Unix()
	payload := fmt.Sprintf("2fa_pending:%d:%d", userID, ts)
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%s", payload, sig)
}

// Validate2FAPendingToken memvalidasi token pending 2FA (maksimal 5 menit)
func Validate2FAPendingToken(token string) (uint, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 4 || parts[0] != "2fa_pending" {
		return 0, false
	}

	payload := fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2])
	sig := parts[3]

	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, false
	}

	ts, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || time.Now().Unix()-ts > 300 { // 5 menit
		return 0, false
	}

	uid, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}

	return uint(uid), true
}

// APIKeyAuthMiddleware memvalidasi token transaksi developer `Bearer rhm_live_...`
func APIKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required (Bearer rhm_live_...)"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format. Format: Bearer <token>"})
			c.Abort()
			return
		}

		rawToken := strings.TrimSpace(parts[1])
		if !apikey.ValidateFormat(rawToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key format. Must start with rhm_live_ followed by 64 hex characters."})
			c.Abort()
			return
		}

		tokenHash := apikey.HashToken(rawToken)

		var key domain.APIKey
		if err := repository.DB.Preload("User").Preload("User.Domain").Where("token_hash = ? AND is_active = true", tokenHash).First(&key).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is invalid, inactive, or revoked."})
			c.Abort()
			return
		}

		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key has expired."})
			c.Abort()
			return
		}

		if !key.User.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "Associated mailbox account has been disabled."})
			c.Abort()
			return
		}

		// Update last_used_at secara asynchronous
		now := time.Now()
		go func(id uint) {
			repository.DB.Model(&domain.APIKey{}).Where("id = ?", id).Update("last_used_at", now)
		}(key.ID)

		c.Set("user", key.User)
		c.Set("user_id", key.UserID)
		c.Set("api_key", &key)
		c.Next()
	}
}




