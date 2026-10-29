package simulation

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/mailbuilder"
	"mail-server/internal/repository"
	"mail-server/internal/storage"
)

// PhishingPreset merepresentasikan template simulasi phishing bawaan sistem
type PhishingPreset struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	SenderName  string `json:"sender_name"`
	SenderEmail string `json:"sender_email"`
	Subject     string `json:"subject"`
	TemplateHTML string `json:"template_html"`
	Description string `json:"description"`
}

// GetDefaultPresets mengembalikan daftar template skenario phishing bawaan perusahaan
func GetDefaultPresets() []PhishingPreset {
	return []PhishingPreset{
		{
			ID:          "it_password_reset",
			Title:       "Pemberitahuan Kadaluarsa Kata Sandi IT",
			SenderName:  "IT Corporate Support",
			SenderEmail: "it-support@corp-security-service.xyz",
			Subject:     "[PENTING] Kata Sandi Email Anda Akan Kedaluwarsa Hari Ini",
			Description: "Skenario mendesak yang meminta karyawan memperbarui kredensial akun.",
			TemplateHTML: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 8px;">
    <h3 style="color: #d9534f;">Pemberitahuan Keamanan Sistem IT</h3>
    <p>Halo,</p>
    <p>Kata sandi untuk akun email perusahaan Anda akan kedaluwarsa dalam <strong>24 jam</strong> sesuai kebijakan rotasi keamanan triwulanan.</p>
    <p>Untuk tetap mempertahankan akses ke seluruh layanan kerja tanpa gangguan, silakan perbarui kata sandi Anda melalui tautan portal resmi berikut:</p>
    <p style="text-align: center; margin: 30px 0;">
        <a href="{{.TrackingLink}}" style="background-color: #0284c7; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block;">Perbarui Kata Sandi Sekarang</a>
    </p>
    <p style="color: #666; font-size: 12px;">Jika Anda mengabaikan pesan ini, akses ke email dan berkas kerja akan dinonaktifkan sementara oleh Administrator Sistem.</p>
</div>`,
		},
		{
			ID:          "hr_payroll_bonus",
			Title:       "Konfirmasi Penyesuaian Gaji & Bonus HR",
			SenderName:  "Departemen SDM & HR",
			SenderEmail: "hr-payroll@hr-internal-portal.info",
			Subject:     "Konfirmasi Penyesuaian Tunjangan Kinerja dan Bonus Akhir Tahun",
			Description: "Skenario finansial yang memancing rasa penasaran karyawan terkait kompensasi.",
			TemplateHTML: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 8px;">
    <h3 style="color: #15803d;">Informasi Pembayaran Bonus Kinerja</h3>
    <p>Yth. Karyawan,</p>
    <p>Manajemen telah menyetujui rincian penyesuaian tunjangan dan bonus kinerja akhir periode. Dokumen rincian slip telah tersedia di sistem payroll.</p>
    <p>Silakan klik tautan di bawah ini untuk mengonfirmasi nomor rekening dan melihat rincian kalkulasi:</p>
    <p style="text-align: center; margin: 30px 0;">
        <a href="{{.TrackingLink}}" style="background-color: #16a34a; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block;">Lihat Rincian Slip Bonus</a>
    </p>
    <p style="color: #666; font-size: 12px;">Batas akhir konfirmasi data rekening adalah hari Jumat pukul 17:00 WIB.</p>
</div>`,
		},
		{
			ID:          "courier_tracking",
			Title:       "Paket Dokumen Kantor Tertunda",
			SenderName:  "Ekspedisi Logistik Kantor",
			SenderEmail: "dispatch@courier-express-delivery.net",
			Subject:     "Pemberitahuan Pengiriman Dokumen Penting Tertunda - Resi #EXP-9921",
			Description: "Skenario pengiriman paket fisik yang meminta konfirmasi alamat.",
			TemplateHTML: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: auto; padding: 20px; border: 1px solid #e0e0e0; border-radius: 8px;">
    <h3 style="color: #b45309;">Pengiriman Paket Tertunda</h3>
    <p>Paket dokumen fisik untuk Anda tidak dapat diantar ke meja kantor dikarenakan alamat divisi kurang lengkap.</p>
    <p>Harap konfirmasi nomor ruangan atau verifikasi resi pengiriman Anda melalui sistem pelacakan kami:</p>
    <p style="text-align: center; margin: 30px 0;">
        <a href="{{.TrackingLink}}" style="background-color: #f59e0b; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 6px; font-weight: bold; display: inline-block;">Lacak & Konfirmasi Alamat</a>
    </p>
</div>`,
		},
	}
}

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

// LaunchCampaign mengeksekusi pengiriman email simulasi ke kotak masuk karyawan secara aman (Internal Mailbox Delivery)
func LaunchCampaign(campaignID uint, baseURL string) (int, error) {
	var campaign domain.PhishingCampaign
	if err := repository.DB.First(&campaign, campaignID).Error; err != nil {
		return 0, fmt.Errorf("campaign not found: %w", err)
	}

	// Ambil target pengguna: jika TargetDomainID diisi, filter berdasarkan domain tersebut; jika tidak, semua user aktif non-admin
	var users []domain.User
	query := repository.DB.Where("is_active = true")
	if campaign.TargetDomainID != nil && *campaign.TargetDomainID > 0 {
		query = query.Where("domain_id = ?", *campaign.TargetDomainID)
	}
	if err := query.Find(&users).Error; err != nil {
		return 0, err
	}

	if len(users) == 0 {
		return 0, fmt.Errorf("tidak ada target pengguna yang memenuhi kriteria")
	}

	sentCount := 0
	now := time.Now()

	for _, user := range users {
		code := GenerateTrackingCode()
		target := domain.PhishingTarget{
			CampaignID:   campaign.ID,
			UserID:       user.ID,
			TrackingCode: code,
			Status:       "sent",
			SentAt:       now,
		}
		if err := repository.DB.Create(&target).Error; err != nil {
			log.Printf("⚠️ Gagal membuat target simulasi untuk user %s: %v", user.Email, err)
			continue
		}

		// Siapkan isi email yang diinjeksi link pelacak unik
		trackingURL := BuildTrackingURL(baseURL, code)
		personalizedHTML := InjectTrackingLinks(campaign.TemplateBody, trackingURL)

		// Buat MIME email RFC 5322
		rawBytes, messageID, err := mailbuilder.BuildRFC5322Message(mailbuilder.OutgoingEmail{
			From:     fmt.Sprintf(`"%s" <%s>`, campaign.SenderName, campaign.SenderEmail),
			To:       []string{user.Email},
			Subject:  campaign.Subject,
			BodyHTML: personalizedHTML,
			BodyText: "Simulasi Pelatihan Keamanan Siber Perusahaan. Silakan buka versi HTML.",
		})
		if err != nil {
			log.Printf("⚠️ Gagal menyusun RFC 5322 untuk user %s: %v", user.Email, err)
			continue
		}

		// Simpan file .eml ke direktori storage inbox user
		storagePath, err := storage.SaveMailToFile("inbox", user.ID, messageID, rawBytes)
		if err != nil {
			log.Printf("⚠️ Gagal menyimpan file email simulasi untuk user %s: %v", user.Email, err)
			continue
		}

		// Buat record Message di mailbox INBOX
		msg := domain.Message{
			UserID:      user.ID,
			Mailbox:     "INBOX",
			MessageID:   messageID,
			From:        fmt.Sprintf(`"%s" <%s>`, campaign.SenderName, campaign.SenderEmail),
			To:          user.Email,
			Subject:     campaign.Subject,
			Snippet:     "Pemberitahuan simulasi uji coba keamanan...",
			Size:        int64(len(rawBytes)),
			StoragePath: storagePath,
			SPFStatus:   "pass",
			DKIMStatus:  "pass",
			DMARCStatus: "pass",
			VirusStatus: "clean",
			ReceivedAt:  now,
			IsSeen:      false,
		}
		if err := repository.DB.Create(&msg).Error; err != nil {
			log.Printf("⚠️ Gagal membuat pesan database untuk user %s: %v", user.Email, err)
			continue
		}

		sentCount++
	}

	// Update status kampanye menjadi running/completed
	campaign.Status = "running"
	campaign.TotalSent = sentCount
	repository.DB.Save(&campaign)

	return sentCount, nil
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
