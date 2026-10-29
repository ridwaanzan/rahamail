package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"

	"github.com/gin-gonic/gin"
)

func TestPhishingSimulationPhase2Flow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	repository.InitDB()

	simCtrl := NewSimulationController()

	api := r.Group("/api/v1")
	{
		api.GET("/admin/simulations/presets", simCtrl.ListPresets)
		api.GET("/admin/simulations", simCtrl.ListCampaigns)
		api.POST("/admin/simulations", simCtrl.CreateCampaign)
		api.POST("/admin/simulations/:id/launch", simCtrl.LaunchCampaign)
		api.GET("/admin/simulations/:id/stats", simCtrl.GetCampaignStats)
	}

	// 1. Test GET /admin/simulations/presets
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/simulations/presets", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from ListPresets, got %d", w.Code)
	}

	var presetsResp struct {
		Presets []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"presets"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &presetsResp)
	if len(presetsResp.Presets) == 0 {
		t.Fatalf("Expected at least 1 default preset, got 0")
	}

	// 2. Prepare test domain and employee user
	testDomain := domain.Domain{
		Name:     fmt.Sprintf("campaign-domain-%d.org", time.Now().UnixNano()),
		IsActive: true,
	}
	repository.DB.Create(&testDomain)

	testUser := domain.User{
		DomainID:     testDomain.ID,
		Email:        fmt.Sprintf("employee_%d@%s", time.Now().UnixNano(), testDomain.Name),
		PasswordHash: "pass123",
		IsActive:     true,
	}
	repository.DB.Create(&testUser)

	// 3. Test POST /admin/simulations (Create Campaign)
	createPayload, _ := json.Marshal(map[string]interface{}{
		"title":            "Simulasi Q4: Pembaruan Kebijakan HR",
		"sender_name":      "HR Corporate Portal",
		"sender_email":     "hr-alert@fake-company-portal.xyz",
		"subject":          "[PENTING] Konfirmasi Data Karyawan",
		"template_body":    `<p>Halo, harap verifikasi akun kerja Anda: <a href="{{.TrackingLink}}">Verifikasi Sekarang</a></p>`,
		"target_domain_id": testDomain.ID,
	})

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/admin/simulations", bytes.NewBuffer(createPayload))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from CreateCampaign, got %d: %s", w2.Code, w2.Body.String())
	}

	var createResp struct {
		Campaign domain.PhishingCampaign `json:"campaign"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &createResp)
	campaignID := createResp.Campaign.ID
	if campaignID == 0 {
		t.Fatalf("Expected created campaign to have non-zero ID")
	}

	// 4. Test POST /admin/simulations/:id/launch (Launch Campaign)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/simulations/%d/launch", campaignID), nil)
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from LaunchCampaign, got %d: %s", w3.Code, w3.Body.String())
	}

	var launchResp struct {
		Status    string `json:"status"`
		SentCount int    `json:"sent_count"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &launchResp)
	if launchResp.SentCount < 1 {
		t.Errorf("Expected at least 1 user targeted in launch, got %d", launchResp.SentCount)
	}

	// Verify that email was injected into employee's mailbox
	var msgCount int64
	repository.DB.Model(&domain.Message{}).
		Where("user_id = ? AND mailbox = 'INBOX' AND `from` LIKE ?", testUser.ID, "%fake-company-portal.xyz%").
		Count(&msgCount)
	if msgCount == 0 {
		t.Errorf("Expected simulation email to be delivered to user INBOX")
	}

	// 5. Test GET /admin/simulations/:id/stats
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/simulations/%d/stats", campaignID), nil)
	r.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from GetCampaignStats, got %d: %s", w4.Code, w4.Body.String())
	}

	var statsResp struct {
		CampaignID  uint   `json:"campaign_id"`
		TotalSent   int    `json:"total_sent"`
		FailureRate string `json:"failure_rate"`
	}
	_ = json.Unmarshal(w4.Body.Bytes(), &statsResp)
	if statsResp.TotalSent < 1 {
		t.Errorf("Expected TotalSent in stats to be >= 1, got %d", statsResp.TotalSent)
	}
}
