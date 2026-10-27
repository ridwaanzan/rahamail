package controllers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"mail-server/internal/api/middleware"
	"mail-server/internal/domain"
	"mail-server/internal/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthController struct{}

func NewAuthController() *AuthController {
	return &AuthController{}
}

func getCaptchaSecret() []byte {
	secret := os.Getenv("CAPTCHA_SECRET")
	if secret == "" {
		secret = "rahamail-captcha-secret-key-2026"
	}
	return []byte(secret)
}

// generateCaptchaToken menghasilkan soal matematika dan token HMAC yang mengikat jawaban yang benar
func generateCaptchaToken() (question string, token string, err error) {
	n1Big, _ := rand.Int(rand.Reader, big.NewInt(15))
	n2Big, _ := rand.Int(rand.Reader, big.NewInt(15))
	n1 := int(n1Big.Int64()) + 2
	n2 := int(n2Big.Int64()) + 1
	sum := n1 + n2

	ts := time.Now().Unix()
	payload := fmt.Sprintf("%d:%d", sum, ts)

	mac := hmac.New(sha256.New, getCaptchaSecret())
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	fullToken := fmt.Sprintf("%s:%s", payload, sig)
	question = fmt.Sprintf("Berapa hasil dari %d + %d ?", n1, n2)
	return question, fullToken, nil
}

// verifyCaptcha memeriksa jawaban matematika user
func verifyCaptcha(token string, answerStr string) bool {
	ans, err := strconv.Atoi(strings.TrimSpace(answerStr))
	if err != nil {
		return false
	}

	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return false
	}

	expectedSum, err := strconv.Atoi(parts[0])
	if err != nil || expectedSum != ans {
		return false
	}

	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix()-ts > 600 { // Captcha berlaku maksimal 10 menit
		return false
	}

	payload := fmt.Sprintf("%s:%s", parts[0], parts[1])
	sig := parts[2]

	mac := hmac.New(sha256.New, getCaptchaSecret())
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

// RenderRegister menampilkan halaman pendaftaran mailbox baru
func (ac *AuthController) RenderRegister(c *gin.Context) {
	var domains []domain.Domain
	repository.DB.Where("is_active = ?", true).Find(&domains)

	question, captchaToken, _ := generateCaptchaToken()

	c.HTML(http.StatusOK, "register.tmpl", gin.H{
		"title":         "Daftar Akun Email Baru - RaHaMail",
		"domains":       domains,
		"captcha_q":     question,
		"captcha_token": captchaToken,
	})
}

// RenderLogin menampilkan halaman login webmail
func (ac *AuthController) RenderLogin(c *gin.Context) {
	// Jika user sudah login, langsung redirect ke webmail
	if token, err := c.Cookie("rahamail_session"); err == nil && token != "" {
		if _, ok := middleware.ValidateSessionToken(token); ok {
			c.Redirect(http.StatusFound, "/mail")
			return
		}
	}

	c.HTML(http.StatusOK, "login.tmpl", gin.H{
		"title": "Masuk ke RaHaMail Webmail",
		"next":  c.Query("next"),
	})
}

// CheckUsername memeriksa ketersediaan email via AJAX
func (ac *AuthController) CheckUsername(c *gin.Context) {
	username := strings.TrimSpace(strings.ToLower(c.Query("username")))
	domainName := strings.TrimSpace(strings.ToLower(c.Query("domain")))

	if username == "" || domainName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"available": false, "message": "Username dan domain harus diisi"})
		return
	}

	// Validasi karakter username
	for _, ch := range username {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= '0' && ch <= '9') && ch != '.' && ch != '_' && ch != '-' {
			c.JSON(http.StatusOK, gin.H{"available": false, "message": "Karakter tidak valid (hanya a-z, 0-9, titik, dash, underscore)"})
			return
		}
	}

	fullEmail := fmt.Sprintf("%s@%s", username, domainName)

	var count int64
	repository.DB.Model(&domain.User{}).Where("email = ?", fullEmail).Count(&count)

	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"available": false, "message": "Email sudah digunakan"})
	} else {
		c.JSON(http.StatusOK, gin.H{"available": true, "message": "Email tersedia!"})
	}
}

// Register proses registrasi mailbox baru
func (ac *AuthController) Register(c *gin.Context) {
	var req struct {
		Username        string `json:"username" binding:"required"`
		DomainID        uint   `json:"domain_id" binding:"required"`
		Password        string `json:"password" binding:"required,min=6"`
		ConfirmPassword string `json:"confirm_password" binding:"required"`
		CaptchaToken    string `json:"captcha_token" binding:"required"`
		CaptchaAnswer   string `json:"captcha_answer" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	// 1. Verifikasi Captcha anti-bot
	if !verifyCaptcha(req.CaptchaToken, req.CaptchaAnswer) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jawaban tantangan keamanan (Captcha) salah atau kedaluwarsa. Silakan refresh."})
		return
	}

	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Konfirmasi kata sandi tidak cocok."})
		return
	}

	// 2. Ambil domain
	var dom domain.Domain
	if err := repository.DB.First(&dom, req.DomainID).Error; err != nil || !dom.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Domain email tidak valid atau sedang nonaktif."})
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	fullEmail := fmt.Sprintf("%s@%s", username, dom.Name)

	// 3. Cek apakah email sudah terdaftar
	var existingCount int64
	repository.DB.Model(&domain.User{}).Where("email = ?", fullEmail).Count(&existingCount)
	if existingCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alamat email " + fullEmail + " sudah terdaftar."})
		return
	}

	// 4. Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengenkripsi kata sandi."})
		return
	}

	// 5. Buat Mailbox User
	newUser := domain.User{
		DomainID:     dom.ID,
		Email:        fullEmail,
		PasswordHash: string(hash),
		QuotaBytes:   1073741824, // 1 GB default
		UsedBytes:    0,
		IsAdmin:      false,
		IsActive:     true,
	}

	if err := repository.DB.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan akun: " + err.Error()})
		return
	}

	// Buat pesan ucapan selamat datang di mailbox baru
	repository.DB.Create(&domain.Message{
		UserID:      newUser.ID,
		MessageID:   fmt.Sprintf("<welcome.%d@%s>", time.Now().Unix(), dom.Name),
		Mailbox:     "INBOX",
		From:        fmt.Sprintf("support@%s", dom.Name),
		To:          fullEmail,
		Subject:     "Selamat Datang di Kotak Surat RaHaMail Anda!",
		Snippet:     "Akun email Anda telah berhasil dibuat dan siap untuk mengirim serta menerima email...",
		SPFStatus:   "pass",
		DKIMStatus:  "pass",
		DMARCStatus: "pass",
		ReceivedAt:  time.Now(),
	})

	// Buat sesi login otomatis
	token := middleware.GenerateSessionToken(newUser.ID)
	c.SetCookie("rahamail_session", token, 7*86400, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Pendaftaran berhasil! Mengalihkan ke webmail...",
		"email":    fullEmail,
		"redirect": "/mail",
	})
}

// Login memproses autentikasi pengguna
func (ac *AuthController) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email dan kata sandi harus diisi."})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))

	var user domain.User
	if err := repository.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau kata sandi salah."})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akun Anda telah dinonaktifkan oleh administrator."})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau kata sandi salah."})
		return
	}

	// Sukses, buat cookie session
	token := middleware.GenerateSessionToken(user.ID)
	c.SetCookie("rahamail_session", token, 7*86400, "/", "", false, true)

	redirectURL := "/mail"
	if user.IsAdmin {
		redirectURL = "/" // Admin dashboard
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Login berhasil!",
		"is_admin": user.IsAdmin,
		"redirect": redirectURL,
	})
}

// Logout menghapus sesi dan cookie
func (ac *AuthController) Logout(c *gin.Context) {
	c.SetCookie("rahamail_session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
