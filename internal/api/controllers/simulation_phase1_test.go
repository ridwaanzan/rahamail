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

	"mail-server/internal/domain"
	"mail-server/internal/repository"
	"mail-server/internal/simulation"
	"mail-server/web"

	"github.com/gin-gonic/gin"
)

func TestPhishingSimulationPhase1Flow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	repository.InitDB()

	tmpl, err := web.GetTemplates()
	if err != nil {
		t.Fatalf("Failed to load embedded templates: %v", err)
	}
	r.SetHTMLTemplate(tmpl)

	mailCtrl := NewMailController()
	simCtrl := NewSimulationController()

	// 1. Setup Routes
	r.GET("/sim/c/:code", simCtrl.HandleTrackingClick)

	userGroup := r.Group("/api/v1/webmail")
	var currentUser *domain.User
	userGroup.Use(func(c *gin.Context) {
		c.Set("user", currentUser)
		c.Next()
	})
	userGroup.POST("/messages/move", mailCtrl.MoveMessage)

	// 2. Prepare test domain, user, campaign, target
	testDomain := domain.Domain{
		Name:     fmt.Sprintf("simdomain-%d.org", time.Now().UnixNano()),
		IsActive: true,
	}
	repository.DB.Create(&testDomain)

	testUser := domain.User{
		DomainID:     testDomain.ID,
		Email:        fmt.Sprintf("targetuser_%d@%s", time.Now().UnixNano(), testDomain.Name),
		PasswordHash: "secret",
		IsActive:     true,
	}
	repository.DB.Create(&testUser)
	currentUser = &testUser

	fakeSender := fmt.Sprintf("it-helpdesk-%d@phishing-test.xyz", time.Now().UnixNano())
	campaign := domain.PhishingCampaign{
		Title:        "Simulasi Phishing Q4: IT Account Reset",
		SenderName:   "IT Helpdesk Support",
		SenderEmail:  fakeSender,
		Subject:      "[URGENT] Akun Anda Akan Dibekukan",
		TemplateBody: `<p>Klik tautan berikut untuk memulihkan akun: <a href="{{.TrackingLink}}">Reset Password</a></p>`,
		Status:       "running",
	}
	repository.DB.Create(&campaign)

	code := simulation.GenerateTrackingCode()
	target := domain.PhishingTarget{
		CampaignID:   campaign.ID,
		UserID:       testUser.ID,
		TrackingCode: code,
		Status:       "sent",
		SentAt:       time.Now(),
	}
	repository.DB.Create(&target)

	// 3. Test Clicking Tracking Link: GET /sim/c/:code -> Should record 'clicked' and render teachable page
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sim/c/"+code, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from tracking click, got %d: %s", w.Code, w.Body.String())
	}

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "NOT PASSED") {
		t.Errorf("Expected Teachable Moment page to show NOT PASSED status")
	}
	if !strings.Contains(bodyStr, fakeSender) {
		t.Errorf("Expected Teachable Moment page to show fake sender: %s", fakeSender)
	}

	// Verify database record
	var verifyTarget domain.PhishingTarget
	repository.DB.First(&verifyTarget, target.ID)
	if verifyTarget.Status != "clicked" {
		t.Errorf("Expected target status to be 'clicked', got '%s'", verifyTarget.Status)
	}
	if verifyTarget.ClickedAt == nil {
		t.Errorf("Expected target ClickedAt to be populated")
	}

	// 4. Test Reporting as Spam via /api/v1/webmail/messages/move -> Should update status to 'reported' (PASSED)
	// Create mock message from the fake sender in INBOX
	msg := domain.Message{
		UserID:      testUser.ID,
		Mailbox:     "INBOX",
		From:        fmt.Sprintf(`"IT Helpdesk" <%s>`, fakeSender),
		To:          testUser.Email,
		Subject:     campaign.Subject,
		StoragePath: "/dummy/path.eml",
	}
	repository.DB.Create(&msg)

	movePayload, _ := json.Marshal(map[string]interface{}{
		"message_id":     msg.ID,
		"target_mailbox": "Spam",
	})

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/webmail/messages/move", bytes.NewBuffer(movePayload))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from MoveMessage, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify target status is updated to 'reported'
	repository.DB.First(&verifyTarget, target.ID)
	if verifyTarget.Status != "reported" {
		t.Errorf("Expected target status to be 'reported' after moving to spam, got '%s'", verifyTarget.Status)
	}
	if verifyTarget.ReportedAt == nil {
		t.Errorf("Expected target ReportedAt to be populated")
	}
}
