package simulation

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
)

// GenerateTrackingCode menghasilkan token acak 32 hex chars (128-bit entropy)
func GenerateTrackingCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BuildTrackingURL merangkai URL pelacak berbasis baseURL dan code
func BuildTrackingURL(baseURL string, code string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/sim/c/%s", baseURL, code)
}

// InjectTrackingLinks menggantikan placeholder {{.TrackingLink}} atau tautan umum dalam body HTML
func InjectTrackingLinks(htmlBody string, trackingURL string) string {
	if strings.Contains(htmlBody, "{{.TrackingLink}}") {
		return strings.ReplaceAll(htmlBody, "{{.TrackingLink}}", trackingURL)
	}

	// Fallback jika template memiliki href="#" atau link statis
	re := regexp.MustCompile(`(?i)href=["'](https?://[^"']+|#)["']`)
	return re.ReplaceAllString(htmlBody, fmt.Sprintf(`href="%s"`, trackingURL))
}

// RecordClick mencatat interaksi saat karyawan mengklik tautan phishing simulasi (Status: clicked -> NOT PASSED)
func RecordClick(trackingCode string, ipAddress string, userAgent string) (*domain.PhishingTarget, error) {
	var target domain.PhishingTarget
	err := repository.DB.Preload("Campaign").Preload("User").
		Where("tracking_code = ?", trackingCode).First(&target).Error
	if err != nil {
		return nil, fmt.Errorf("tracking code not found: %w", err)
	}

	now := time.Now()
	// Jika belum pernah diklik, perbarui status menjadi clicked dan tambahkan counter di Campaign
	if target.Status != "clicked" {
		target.Status = "clicked"
		target.ClickedAt = &now
		target.IPAddress = ipAddress
		target.UserAgent = userAgent

		if err := repository.DB.Save(&target).Error; err != nil {
			return nil, err
		}

		// Update total_clicked pada campaign
		repository.DB.Model(&domain.PhishingCampaign{}).
			Where("id = ?", target.CampaignID).
			UpdateColumn("total_clicked", repository.DB.Raw("total_clicked + 1"))
	}

	return &target, nil
}

// RecordReport mencatat saat karyawan melaporkan email simulasi sebagai spam/phishing (Status: reported -> PASSED)
func RecordReport(userID uint, senderEmail string) (*domain.PhishingTarget, error) {
	// Cari campaign aktif yang cocok dengan senderEmail dan mentarget user ini
	var target domain.PhishingTarget
	err := repository.DB.Preload("Campaign").
		Joins("JOIN phishing_campaigns ON phishing_campaigns.id = phishing_targets.campaign_id").
		Where("phishing_targets.user_id = ? AND phishing_campaigns.sender_email = ? AND phishing_targets.status != 'reported'", userID, senderEmail).
		Order("phishing_targets.id DESC").
		First(&target).Error
	if err != nil {
		return nil, err
	}

	now := time.Now()
	target.Status = "reported"
	target.ReportedAt = &now

	if err := repository.DB.Save(&target).Error; err != nil {
		return nil, err
	}

	repository.DB.Model(&domain.PhishingCampaign{}).
		Where("id = ?", target.CampaignID).
		UpdateColumn("total_reported", repository.DB.Raw("total_reported + 1"))

	return &target, nil
}

// SafeExtractHost mengekstrak host/domain dari string URL
func SafeExtractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}
