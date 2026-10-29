package simulation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
)

func TestGenerateTrackingCode(t *testing.T) {
	code1 := GenerateTrackingCode()
	code2 := GenerateTrackingCode()

	if len(code1) != 32 {
		t.Fatalf("Expected 32 hex chars tracking code, got length %d", len(code1))
	}
	if code1 == code2 {
		t.Fatalf("Tracking codes should be unique and random")
	}
}

func TestBuildAndInjectTrackingLinks(t *testing.T) {
	baseURL := "https://mail.company.com/"
	code := "abcd1234efgh5678"
	trackingURL := BuildTrackingURL(baseURL, code)

	expectedURL := "https://mail.company.com/sim/c/abcd1234efgh5678"
	if trackingURL != expectedURL {
		t.Fatalf("Expected tracking URL %s, got %s", expectedURL, trackingURL)
	}

	// Case 1: Placeholder {{.TrackingLink}}
	tpl1 := `<div>Silakan klik <a href="{{.TrackingLink}}">Verifikasi Akun</a></div>`
	res1 := InjectTrackingLinks(tpl1, trackingURL)
	if !strings.Contains(res1, fmt.Sprintf(`href="%s"`, expectedURL)) {
		t.Errorf("Expected placeholder replacement, got: %s", res1)
	}

	// Case 2: Standard href
	tpl2 := `<div>Silakan klik <a href="http://login-portal.fake/login">Masuk</a></div>`
	res2 := InjectTrackingLinks(tpl2, trackingURL)
	if !strings.Contains(res2, fmt.Sprintf(`href="%s"`, expectedURL)) {
		t.Errorf("Expected regex href replacement, got: %s", res2)
	}
}

func TestRecordClickAndReport(t *testing.T) {
	repository.InitDB()

	// 1. Setup mock campaign & target
	campaign := domain.PhishingCampaign{
		Title:        "Uji Coba Phishing HR Q4",
		SenderName:   "HR Portal",
		SenderEmail:  fmt.Sprintf("hr-fake-%d@phish.xyz", time.Now().UnixNano()),
		Subject:      "Pembaruan Gaji 2026",
		TemplateBody: "Silakan verifikasi data Anda.",
		Status:       "running",
	}
	if err := repository.DB.Create(&campaign).Error; err != nil {
		t.Fatalf("Failed to create campaign: %v", err)
	}

	user := domain.User{
		DomainID:     1,
		Email:        fmt.Sprintf("karyawan_%d@company.com", time.Now().UnixNano()),
		PasswordHash: "secret",
		IsActive:     true,
	}
	if err := repository.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	code := GenerateTrackingCode()
	target := domain.PhishingTarget{
		CampaignID:   campaign.ID,
		UserID:       user.ID,
		TrackingCode: code,
		Status:       "sent",
		SentAt:       time.Now(),
	}
	if err := repository.DB.Create(&target).Error; err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	// 2. Test RecordClick (Simulasi Karyawan Klik Link -> NOT PASSED)
	clickedTarget, err := RecordClick(code, "192.168.1.100", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("Failed to record click: %v", err)
	}
	if clickedTarget.Status != "clicked" {
		t.Errorf("Expected status 'clicked', got '%s'", clickedTarget.Status)
	}
	if clickedTarget.ClickedAt == nil {
		t.Errorf("Expected ClickedAt to be set")
	}

	var updatedCampaign domain.PhishingCampaign
	repository.DB.First(&updatedCampaign, campaign.ID)
	if updatedCampaign.TotalClicked != 1 {
		t.Errorf("Expected campaign TotalClicked to be 1, got %d", updatedCampaign.TotalClicked)
	}

	// 3. Test RecordReport (Simulasi Karyawan Waspada & Tandai Spam -> PASSED)
	reportedTarget, err := RecordReport(user.ID, campaign.SenderEmail)
	if err != nil {
		t.Fatalf("Failed to record report: %v", err)
	}
	if reportedTarget.Status != "reported" {
		t.Errorf("Expected status 'reported', got '%s'", reportedTarget.Status)
	}
	if reportedTarget.ReportedAt == nil {
		t.Errorf("Expected ReportedAt to be set")
	}

	repository.DB.First(&updatedCampaign, campaign.ID)
	if updatedCampaign.TotalReported != 1 {
		t.Errorf("Expected campaign TotalReported to be 1, got %d", updatedCampaign.TotalReported)
	}
}
