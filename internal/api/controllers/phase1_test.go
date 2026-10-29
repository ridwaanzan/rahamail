package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mail-server/internal/api/middleware"
	"mail-server/internal/apikey"
	"mail-server/internal/domain"
	"mail-server/internal/repository"
	"mail-server/internal/totp"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	repository.InitDB()
	return r
}

func TestPhase1Flow(t *testing.T) {
	r := setupTestRouter()
	authCtrl := NewAuthController()
	mailCtrl := NewMailController()

	// 1. Setup routes
	api := r.Group("/api/v1")
	{
		api.POST("/auth/login", authCtrl.Login)
		api.POST("/auth/verify-2fa", authCtrl.Verify2FA)

		twoFA := api.Group("/2fa", middleware.UserAuthMiddleware())
		{
			twoFA.GET("/setup", authCtrl.Setup2FA)
			twoFA.POST("/enable", authCtrl.Enable2FA)
			twoFA.POST("/disable", authCtrl.Disable2FA)
		}

		apiKeys := api.Group("/api-keys", middleware.UserAuthMiddleware())
		{
			apiKeys.GET("", authCtrl.ListAPIKeys)
			apiKeys.POST("", authCtrl.CreateAPIKey)
			apiKeys.DELETE("/:id", authCtrl.RevokeAPIKey)
		}

		api.POST("/send", middleware.APIKeyAuthMiddleware(), mailCtrl.SendTransactionalMail)
		api.POST("/domains/:id/catch-all", mailCtrl.SetCatchAll)
	}

	// 2. Setup user test
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	uniqueEmail := fmt.Sprintf("dev-%d@example.com", time.Now().UnixNano())

	testUser := domain.User{
		DomainID:     1,
		Email:        uniqueEmail,
		PasswordHash: string(hashedPwd),
		IsActive:     true,
	}
	if err := repository.DB.Create(&testUser).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionCookie := http.Cookie{
		Name:  "rahamail_session",
		Value: middleware.GenerateSessionToken(testUser.ID),
	}

	// 3. Test 2FA Setup & Enable
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	code, _ := totp.GenerateCode(secret, time.Now())

	enablePayload, _ := json.Marshal(map[string]string{
		"secret": secret,
		"code":   code,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/2fa/enable", bytes.NewBuffer(enablePayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for 2FA enable, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Test Login requiring 2FA
	loginPayload, _ := json.Marshal(map[string]string{
		"email":    uniqueEmail,
		"password": "secret123",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for login, got %d", w.Code)
	}

	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	if loginResp["status"] != "2fa_required" {
		t.Fatalf("Expected 2fa_required status, got %v", loginResp["status"])
	}
	pendingToken := loginResp["pending_token"].(string)

	// 5. Test Verify 2FA
	verifyCode, _ := totp.GenerateCode(secret, time.Now())
	verifyPayload, _ := json.Marshal(map[string]string{
		"pending_token": pendingToken,
		"code":          verifyCode,
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/verify-2fa", bytes.NewBuffer(verifyPayload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for verify-2fa, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Test Create Developer API Key
	apiKeyPayload, _ := json.Marshal(map[string]string{
		"name": "Integration Test Key",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/api-keys", bytes.NewBuffer(apiKeyPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for API key creation, got %d: %s", w.Code, w.Body.String())
	}

	var keyResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &keyResp)
	rawToken := keyResp["api_key"].(string)
	if !strings.HasPrefix(rawToken, apikey.Prefix) {
		t.Fatalf("Expected token to start with %s, got %s", apikey.Prefix, rawToken)
	}

	// 7. Test Developer Transactional Send using API Key
	sendPayload, _ := json.Marshal(map[string]interface{}{
		"to":      []string{"recipient@example.com"},
		"subject": "Automated Transactional Test",
		"text":    "Hello from API",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/send", bytes.NewBuffer(sendPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rawToken)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for transactional send, got %d: %s", w.Code, w.Body.String())
	}

	// 8. Test Set Catch-All User for Domain
	var dom domain.Domain
	repository.DB.FirstOrCreate(&dom, domain.Domain{Name: "example.com"})

	caPayload, _ := json.Marshal(map[string]interface{}{
		"user_id": testUser.ID,
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/domains/1/catch-all", bytes.NewBuffer(caPayload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for set catch-all, got %d: %s", w.Code, w.Body.String())
	}
}
