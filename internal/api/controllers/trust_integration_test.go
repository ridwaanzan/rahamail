package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"

	"github.com/gin-gonic/gin"
)

func TestSecurityAlertIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	repository.InitDB()
	mailCtrl := NewMailController()

	// 1. Create a test domain and user
	testDomain := domain.Domain{
		Name:     fmt.Sprintf("trustdomain-%d.org", time.Now().UnixNano()),
		IsActive: true,
	}
	if err := repository.DB.Create(&testDomain).Error; err != nil {
		t.Fatalf("Failed to create test domain: %v", err)
	}

	user := domain.User{
		DomainID:     testDomain.ID,
		Email:        fmt.Sprintf("trustuser_%d@%s", time.Now().UnixNano(), testDomain.Name),
		PasswordHash: "mock-hash",
		IsActive:     true,
	}
	if err := repository.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// 2. Prepare mock .eml file in var/mail/INBOX/{userID}/
	shaName := fmt.Sprintf("testmsg_%d", time.Now().UnixNano())
	msgDir := filepath.Join("var", "mail", "INBOX", fmt.Sprintf("%d", user.ID))
	_ = os.MkdirAll(msgDir, 0755)
	emlPath := filepath.Join(msgDir, shaName+".eml")

	rawEML := `From: "Unknown Vendor" <stranger@untrusted-vendor.xyz>
To: ` + user.Email + `
Subject: Project Proposal with Link
Date: Thu, 29 Oct 2026 08:00:00 +0000
Content-Type: text/html; charset="UTF-8"

<html><body><p>Please check this <a href="https://untrusted-vendor.xyz/file.pdf">proposal link</a></p></body></html>
`
	if err := os.WriteFile(emlPath, []byte(rawEML), 0644); err != nil {
		t.Fatalf("Failed to write mock eml: %v", err)
	}
	defer os.Remove(emlPath)

	// 3. Insert record into messages table
	msg := domain.Message{
		UserID:      user.ID,
		Mailbox:     "INBOX",
		From:        "stranger@untrusted-vendor.xyz",
		To:          user.Email,
		Subject:     "Project Proposal with Link",
		StoragePath: emlPath,
		Size:        int64(len(rawEML)),
		DMARCStatus: "pass",
		SPFStatus:   "pass",
		DKIMStatus:  "pass",
	}
	if err := repository.DB.Create(&msg).Error; err != nil {
		t.Fatalf("Failed to create message record: %v", err)
	}

	// 4. Setup route with mock auth middleware
	r.GET("/api/v1/webmail/messages/:id", func(c *gin.Context) {
		c.Set("user", &user)
		c.Next()
	}, mailCtrl.GetMessageDetail)

	// 5. Perform GET /api/v1/webmail/messages/:id
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/webmail/messages/%d", msg.ID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GetMessageDetail, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Message       domain.Message `json:"message"`
		SecurityAlert struct {
			Level            string `json:"level"`
			Title            string `json:"title"`
			Description      string `json:"description"`
			SenderEmail      string `json:"sender_email"`
			HasExternalLinks bool   `json:"has_external_links"`
			IsExternal       bool   `json:"is_external_domain"`
			IsInContacts     bool   `json:"is_in_contacts"`
		} `json:"security_alert"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if resp.SecurityAlert.Level != "warning" {
		t.Errorf("Expected security alert level 'warning', got '%s'", resp.SecurityAlert.Level)
	}
	if !resp.SecurityAlert.HasExternalLinks {
		t.Errorf("Expected HasExternalLinks to be true")
	}
	if resp.SecurityAlert.SenderEmail != "stranger@untrusted-vendor.xyz" {
		t.Errorf("Expected SenderEmail 'stranger@untrusted-vendor.xyz', got '%s'", resp.SecurityAlert.SenderEmail)
	}

	// 6. Now add contact to simulate "Mark as Trusted" and verify alert turns to "none"
	contact := domain.Contact{
		UserID: user.ID,
		Name:   "Unknown Vendor",
		Email:  "stranger@untrusted-vendor.xyz",
	}
	if err := repository.DB.Create(&contact).Error; err != nil {
		t.Fatalf("Failed to insert contact: %v", err)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/webmail/messages/%d", msg.ID), nil)
	r.ServeHTTP(w2, req2)

	var resp2 struct {
		SecurityAlert struct {
			Level string `json:"level"`
		} `json:"security_alert"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp2.SecurityAlert.Level != "none" {
		t.Errorf("Expected security alert level to be 'none' after adding contact, got '%s'", resp2.SecurityAlert.Level)
	}
}
