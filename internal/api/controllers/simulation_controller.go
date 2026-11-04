package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
	"mail-server/internal/simulation"

	"github.com/gin-gonic/gin"
)

type SimulationController struct{}

func NewSimulationController() *SimulationController {
	return &SimulationController{}
}

// HandleTrackingClick menangani klik tautan tracking simulasi: GET /sim/c/:code
func (sc *SimulationController) HandleTrackingClick(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.String(http.StatusBadRequest, "Invalid tracking code")
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	target, err := simulation.RecordClick(code, ipAddress, userAgent)
	if err != nil {
		c.String(http.StatusNotFound, "Tautan simulasi tidak valid atau telah kedaluwarsa.")
		return
	}

	senderEmail := "unknown@suspicious-domain.xyz"
	if target.Campaign != nil && target.Campaign.SenderEmail != "" {
		senderEmail = target.Campaign.SenderEmail
	}

	targetEmail := ""
	if target.User != nil {
		targetEmail = target.User.Email
	}

	// Jika target belum memasukkan kredensial, tampilkan mock login portal (Credential Harvester simulation)
	if !target.CredentialSubmitted {
		c.HTML(http.StatusOK, "simulation_login", gin.H{
			"title":         "Konfirmasi Kredensial - Portal Internal",
			"tracking_code": target.TrackingCode,
			"target_email":  targetEmail,
		})
		return
	}

	// Render halaman Teachable Moment (Edukasi Keamanan)
	c.HTML(http.StatusOK, "simulation_teachable", gin.H{
		"title":        "Peringatan Edukasi Keamanan",
		"sender_email": senderEmail,
		"campaign":     target.Campaign,
		"target":       target,
	})
}

// HandleCredentialSubmit menangani saat target memasukkan password di mock portal: POST /sim/c/:code/submit
func (sc *SimulationController) HandleCredentialSubmit(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.String(http.StatusBadRequest, "Invalid tracking code")
		return
	}

	target, err := simulation.RecordCredentialSubmission(code)
	if err != nil {
		c.String(http.StatusNotFound, "Tautan simulasi tidak valid atau telah kedaluwarsa.")
		return
	}

	senderEmail := "unknown@suspicious-domain.xyz"
	if target.Campaign != nil && target.Campaign.SenderEmail != "" {
		senderEmail = target.Campaign.SenderEmail
	}

	// Setelah submit, tampilkan halaman Teachable Moment edukasi
	c.HTML(http.StatusOK, "simulation_teachable", gin.H{
		"title":        "Peringatan Edukasi Keamanan",
		"sender_email": senderEmail,
		"campaign":     target.Campaign,
		"target":       target,
	})
}

// ListPresets mengambil daftar template simulasi phishing bawaan sistem: GET /api/v1/admin/simulations/presets
func (sc *SimulationController) ListPresets(c *gin.Context) {
	presets := simulation.GetDefaultPresets()
	c.JSON(http.StatusOK, gin.H{
		"presets": presets,
	})
}

// ListCampaigns mengambil daftar riwayat kampanye simulasi: GET /api/v1/admin/simulations
func (sc *SimulationController) ListCampaigns(c *gin.Context) {
	var campaigns []domain.PhishingCampaign
	if err := repository.DB.Preload("Targets").Order("id DESC").Find(&campaigns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar kampanye: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"campaigns": campaigns,
	})
}

// CreateCampaign membuat rancangan kampanye simulasi baru: POST /api/v1/admin/simulations
func (sc *SimulationController) CreateCampaign(c *gin.Context) {
	var req struct {
		Title          string `json:"title" binding:"required"`
		SenderName     string `json:"sender_name" binding:"required"`
		SenderEmail    string `json:"sender_email" binding:"required,email"`
		Subject        string `json:"subject" binding:"required"`
		TemplateBody   string `json:"template_body" binding:"required"`
		TargetDomainID *uint  `json:"target_domain_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	campaign := domain.PhishingCampaign{
		Title:          strings.TrimSpace(req.Title),
		SenderName:     strings.TrimSpace(req.SenderName),
		SenderEmail:    strings.TrimSpace(req.SenderEmail),
		Subject:        strings.TrimSpace(req.Subject),
		TemplateBody:   req.TemplateBody,
		TargetDomainID: req.TargetDomainID,
		Status:         "draft",
	}

	if err := repository.DB.Create(&campaign).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat kampanye: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Kampanye simulasi berhasil dibuat.",
		"campaign": campaign,
	})
}

// LaunchCampaign meluncurkan pengiriman email simulasi ke kotak masuk karyawan: POST /api/v1/admin/simulations/:id/launch
func (sc *SimulationController) LaunchCampaign(c *gin.Context) {
	idStr := c.Param("id")
	campaignID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kampanye tidak valid"})
		return
	}

	// Tentukan BaseURL dari request scheme dan host
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	sentCount, err := simulation.LaunchCampaign(uint(campaignID), baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal meluncurkan kampanye: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Kampanye berhasil diluncurkan ke %d target pengguna.", sentCount),
		"sent_count": sentCount,
	})
}

// GetCampaignStats mengambil rekap statistik sebuah kampanye: GET /api/v1/admin/simulations/:id/stats
func (sc *SimulationController) GetCampaignStats(c *gin.Context) {
	idStr := c.Param("id")
	campaignID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kampanye tidak valid"})
		return
	}

	var campaign domain.PhishingCampaign
	if err := repository.DB.Preload("Targets.User").First(&campaign, campaignID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kampanye tidak ditemukan"})
		return
	}

	totalSent := len(campaign.Targets)
	clickedCount := 0
	reportedCount := 0
	unresponsiveCount := 0

	for _, t := range campaign.Targets {
		switch t.Status {
		case "clicked":
			clickedCount++
		case "reported":
			reportedCount++
		default:
			unresponsiveCount++
		}
	}

	failureRate := 0.0
	if totalSent > 0 {
		failureRate = (float64(clickedCount) / float64(totalSent)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"campaign_id":        campaign.ID,
		"title":              campaign.Title,
		"status":             campaign.Status,
		"total_sent":         totalSent,
		"total_clicked":      clickedCount,
		"total_reported":     reportedCount,
		"total_unresponsive": unresponsiveCount,
		"failure_rate":       fmt.Sprintf("%.1f%%", failureRate),
		"targets":            campaign.Targets,
	})
}

// ExportCampaignCSV mengekspor hasil simulasi ke format CSV untuk audit compliance: GET /api/v1/admin/simulations/:id/export
func (sc *SimulationController) ExportCampaignCSV(c *gin.Context) {
	idStr := c.Param("id")
	campaignID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kampanye tidak valid"})
		return
	}

	var campaign domain.PhishingCampaign
	if err := repository.DB.Preload("Targets.User").First(&campaign, campaignID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kampanye tidak ditemukan"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=phishing_report_campaign_%d.csv", campaign.ID))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	var buf strings.Builder
	buf.WriteString("ID,Email Karyawan,Status,Waktu Kirim,Waktu Klik,Kredensial Dimasukkan,Waktu Lapor Spam,IP Address\n")

	for _, t := range campaign.Targets {
		email := ""
		if t.User != nil {
			email = t.User.Email
		}
		sentTime := t.SentAt.Format(time.RFC3339)
		clickTime := ""
		if t.ClickedAt != nil {
			clickTime = t.ClickedAt.Format(time.RFC3339)
		}
		reportTime := ""
		if t.ReportedAt != nil {
			reportTime = t.ReportedAt.Format(time.RFC3339)
		}

		credSubmitted := "Tidak"
		if t.CredentialSubmitted {
			credSubmitted = "Ya (Vulnerable)"
		}

		buf.WriteString(fmt.Sprintf("%d,%s,%s,%s,%s,%s,%s,%s\n",
			t.ID,
			email,
			t.Status,
			sentTime,
			clickTime,
			credSubmitted,
			reportTime,
			t.IPAddress,
		))
	}

	c.String(http.StatusOK, buf.String())
}
