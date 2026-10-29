package trust

import (
	"regexp"
	"strings"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
)

var (
	// Regex mendeteksi tautan HTTP/HTTPS di dalam body email
	urlRegex = regexp.MustCompile(`(?i)\b(https?://[^\s<>"']+)\b`)
)

// SecurityAlert merepresentasikan tingkat kewaspadaan dan detail peringatan pada email masuk
type SecurityAlert struct {
	Level                string `json:"level"`                 // "none", "info", "warning", "danger"
	IsFirstTimeSender    bool   `json:"is_first_time_sender"`   // true jika penerima belum pernah berinteraksi sebelumnya
	IsExternalDomain     bool   `json:"is_external_domain"`     // true jika domain pengirim bukan domain lokal server
	IsInContacts         bool   `json:"is_in_contacts"`         // true jika pengirim ada di buku kontak user
	HasAttachments       bool   `json:"has_attachments"`        // true jika email memuat file lampiran
	HasExternalLinks     bool   `json:"has_external_links"`      // true jika email memuat tautan URL luar
	AuthPass             bool   `json:"auth_pass"`              // true jika SPF/DKIM/DMARC lulus
	BadgeTitle           string `json:"badge_title"`            // Judul singkat badge
	Description          string `json:"description"`            // Penjelasan ramah pengguna
	SenderEmail          string `json:"sender_email"`           // Alamat email bersih pengirim
	ActionRecommendation string `json:"action_recommendation"`  // Rekomendasi tindakan (misal: "Mark as Trusted")
}

// ExtractEmailAddress mengekstrak email bersih dari format "Name <user@domain.com>" atau "user@domain.com"
func ExtractEmailAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "<")
	end := strings.Index(raw, ">")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(strings.ToLower(raw[start+1 : end]))
	}
	return strings.TrimSpace(strings.ToLower(raw))
}

// ContainsURLs mendeteksi apakah teks/HTML memuat tautan URL luar
func ContainsURLs(text string) bool {
	return urlRegex.MatchString(text)
}

// EvaluateSender mengevaluasi tingkat kepercayaan email masuk bagi pengguna penerima
func EvaluateSender(userID uint, rawSender string, bodyHTML string, bodyText string, attachmentCount int, spfStatus, dkimStatus, dmarcStatus string) SecurityAlert {
	cleanSender := ExtractEmailAddress(rawSender)
	alert := SecurityAlert{
		Level:           "none",
		SenderEmail:     cleanSender,
		HasAttachments:  attachmentCount > 0,
		HasExternalLinks: ContainsURLs(bodyHTML) || ContainsURLs(bodyText),
		AuthPass:        true,
	}

	parts := strings.Split(cleanSender, "@")
	if len(parts) != 2 {
		return alert
	}
	senderDomain := parts[1]

	// 1. Evaluasi Status Autentikasi Pengirim (SPF / DKIM / DMARC)
	if strings.ToLower(spfStatus) == "fail" || strings.ToLower(dkimStatus) == "fail" || strings.ToLower(dmarcStatus) == "fail" {
		alert.AuthPass = false
		alert.Level = "danger"
		alert.BadgeTitle = "Peringatan Spoofing / Keamanan"
		alert.Description = "Email ini gagal melewati verifikasi autentikasi pengirim (SPF/DKIM/DMARC). Pengirim mungkin memalsukan identitas aslinya. Jangan klik tautan atau unduh lampiran apa pun."
		alert.ActionRecommendation = "Pindahkan ke Spam"
		return alert
	}

	// 2. Periksa apakah Domain Pengirim adalah Domain Lokal Organisasi
	var localDomain domain.Domain
	if err := repository.DB.Where("name = ? AND is_active = true", senderDomain).First(&localDomain).Error; err == nil {
		// Domain lokal dianggap terpercaya di dalam organisasi
		alert.IsExternalDomain = false
		alert.IsFirstTimeSender = false
		alert.Level = "none"
		return alert
	}
	alert.IsExternalDomain = true

	// 3. Periksa apakah Pengirim Terdaftar di Buku Kontak (Address Book) Pengguna
	var contact domain.Contact
	if err := repository.DB.Where("user_id = ? AND email = ?", userID, cleanSender).First(&contact).Error; err == nil {
		alert.IsInContacts = true
		alert.IsFirstTimeSender = false
		alert.Level = "none"
		return alert
	}

	// 4. Periksa Riwayat Interaksi Sebelumnya (Apakah user pernah mengirim email ke pengirim ini atau menerima sebelumnya)
	var sentInteractionCount int64
	likeSender := "%" + cleanSender + "%"
	repository.DB.Model(&domain.Message{}).
		Where("user_id = ? AND mailbox = 'Sent' AND `to` LIKE ?", userID, likeSender).
		Count(&sentInteractionCount)

	if sentInteractionCount > 0 {
		// Pengguna pernah membalas/mengirim email ke alamat ini sebelumnya -> Dikenal
		alert.IsFirstTimeSender = false
		alert.Level = "none"
		return alert
	}

	// Periksa juga apakah ada riwayat pesan Inbox lama dari pengirim ini (lebih dari 1 pesan)
	var previousInboxCount int64
	repository.DB.Model(&domain.Message{}).
		Where("user_id = ? AND `from` LIKE ?", userID, likeSender).
		Count(&previousInboxCount)

	if previousInboxCount > 1 {
		// Sudah ada interaksi pesan sebelumnya
		alert.IsFirstTimeSender = false
		alert.Level = "none"
		return alert
	}

	// 5. Teridentifikasi sebagai First-Time Sender dari Domain Eksternal!
	alert.IsFirstTimeSender = true

	if alert.HasAttachments && alert.HasExternalLinks {
		alert.Level = "warning"
		alert.BadgeTitle = "Domain Eksternal & Pengirim Pertama Kali"
		alert.Description = "Anda belum pernah berinteraksi dengan pengirim ini dan pesan berasal dari domain eksternal. Email ini memuat lampiran file dan tautan URL luar. Berhati-hatilah sebelum mengklik atau mengunduh."
		alert.ActionRecommendation = "Tandai sebagai Kontak Aman"
	} else if alert.HasAttachments {
		alert.Level = "warning"
		alert.BadgeTitle = "Lampiran dari Domain Tidak Dikenal"
		alert.Description = "Email ini berasal dari domain eksternal baru dan memuat lampiran file. Pastikan Anda mengenali pengirim sebelum membuka lampiran."
		alert.ActionRecommendation = "Tandai sebagai Kontak Aman"
	} else if alert.HasExternalLinks {
		alert.Level = "warning"
		alert.BadgeTitle = "Tautan URL dari Pengirim Baru"
		alert.Description = "Pesan ini memuat tautan web dari domain yang belum pernah berinteraksi dengan Anda sebelumnya. Waspadai percobaan penipuan (phishing)."
		alert.ActionRecommendation = "Tandai sebagai Kontak Aman"
	} else {
		alert.Level = "info"
		alert.BadgeTitle = "Pengirim Baru dari Luar Organisasi"
		alert.Description = "Ini adalah pertama kalinya Anda menerima pesan dari alamat email ini. Pastikan Anda memverifikasi keaslian pengirim sebelum membalas data sensitif."
		alert.ActionRecommendation = "Tambah ke Buku Kontak"
	}

	return alert
}
