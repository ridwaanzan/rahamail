package domain

import (
	"time"

	"gorm.io/gorm"
)

// Domain merepresentasikan domain email yang dilayani oleh mail server
type Domain struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Name           string         `gorm:"size:255;not null;uniqueIndex" json:"name"` // misal: example.com
	DKIMSelector   string         `gorm:"size:64;default:'mail'" json:"dkim_selector"`
	DKIMPrivateKey string         `gorm:"type:text" json:"-"`                       // Kunci privat untuk menandatangani email keluar
	DKIMPublicKey  string         `gorm:"type:text" json:"dkim_public_key"`         // Rekomendasi DNS TXT DKIM
	CatchAllUserID *uint          `gorm:"index" json:"catch_all_user_id,omitempty"` // User ID penerima seluruh email tak dikenal (Catch-All)
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	CatchAllUser *User    `gorm:"foreignKey:CatchAllUserID" json:"catch_all_user,omitempty"`
	Users        []User   `gorm:"foreignKey:DomainID" json:"users,omitempty"`
	Aliases      []Alias  `gorm:"foreignKey:DomainID" json:"aliases,omitempty"`
}

// User merepresentasikan akun kotak surat (mailbox) pengguna
type User struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	DomainID         uint           `gorm:"not null;index" json:"domain_id"`
	Email            string         `gorm:"size:255;not null;uniqueIndex" json:"email"` // user@example.com
	PasswordHash     string         `gorm:"size:255;not null" json:"-"`
	QuotaBytes       int64          `gorm:"default:1073741824" json:"quota_bytes"` // Default 1 GB
	UsedBytes        int64          `gorm:"default:0" json:"used_bytes"`
	IsAdmin          bool           `gorm:"default:false" json:"is_admin"`
	IsActive         bool           `gorm:"default:true" json:"is_active"`
	TwoFactorEnabled bool           `gorm:"default:false" json:"two_factor_enabled"`
	TwoFactorSecret  string         `gorm:"size:128" json:"-"` // Base32 encoded TOTP Secret
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	Domain      *Domain      `gorm:"foreignKey:DomainID" json:"domain,omitempty"`
	Messages    []Message    `gorm:"foreignKey:UserID" json:"messages,omitempty"`
	BackupCodes []BackupCode `gorm:"foreignKey:UserID" json:"-"`
	APIKeys     []APIKey     `gorm:"foreignKey:UserID" json:"-"`
}

// BackupCode merepresentasikan kode darurat 2FA sekali pakai
type BackupCode struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	CodeHash  string         `gorm:"size:255;not null" json:"-"` // Bcrypt hash dari kode backup
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// APIKey merepresentasikan token otentikasi developer untuk pengiriman transactional email via REST API
type APIKey struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"not null;index" json:"user_id"`
	Name       string         `gorm:"size:128;not null" json:"name"`       // misal "Billing Service", "Staging"
	Prefix     string         `gorm:"size:16;not null;index" json:"prefix"` // e.g. "rhm_live_a1b2"
	TokenHash  string         `gorm:"size:64;not null;uniqueIndex" json:"-"` // SHA-256 hash
	LastUsedAt *time.Time     `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	IsActive   bool           `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// ScheduledMail merepresentasikan email yang dijadwalkan untuk dikirim di masa mendatang
type ScheduledMail struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"not null;index" json:"user_id"`
	From         string         `gorm:"size:255;not null" json:"from"`
	To           string         `gorm:"type:text;not null" json:"to"`
	Cc           string         `gorm:"type:text" json:"cc"`
	Bcc          string         `gorm:"type:text" json:"bcc"`
	Subject      string         `gorm:"type:text" json:"subject"`
	BodyHTML     string         `gorm:"type:text" json:"body_html"`
	BodyText     string         `gorm:"type:text" json:"body_text"`
	ScheduledAt  time.Time      `gorm:"index;not null" json:"scheduled_at"`
	Status       string         `gorm:"size:32;default:'pending';index" json:"status"` // pending, sent, cancelled, failed
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Attachment merepresentasikan file lampiran email
type Attachment struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	MessageID   uint           `gorm:"not null;index" json:"message_id"`
	FileName    string         `gorm:"size:255;not null" json:"file_name"`
	ContentType string         `gorm:"size:128" json:"content_type"`
	Size        int64          `gorm:"default:0" json:"size"`
	StoragePath string         `gorm:"size:512;not null" json:"storage_path"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Contact merepresentasikan buku alamat / daftar kontak pengguna
type Contact struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Name      string         `gorm:"size:255" json:"name"`
	Email     string         `gorm:"size:255;not null;index" json:"email"`
	Company   string         `gorm:"size:255" json:"company"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// AutoResponder merepresentasikan pengaturan pesan balasan otomatis (Vacation / Out-of-Office Notice)
type AutoResponder struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;uniqueIndex" json:"user_id"`
	Enabled   bool           `gorm:"default:false" json:"enabled"`
	StartTime *time.Time     `json:"start_time,omitempty"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
	Subject   string         `gorm:"size:255;default:'Out of Office'" json:"subject"`
	BodyHTML  string         `gorm:"type:text" json:"body_html"`
	BodyText  string         `gorm:"type:text" json:"body_text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// AutoResponderLog mencatat pengiriman balasan otomatis untuk mencegah looping dan spam
type AutoResponderLog struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AutoResponderID uint      `gorm:"not null;index" json:"auto_responder_id"`
	RecipientEmail  string    `gorm:"size:255;not null;index" json:"recipient_email"`
	LastSentAt      time.Time `gorm:"index" json:"last_sent_at"`
}

// CannedResponse merepresentasikan template pesan siap pakai (email snippet)
type CannedResponse struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Title     string         `gorm:"size:128;not null" json:"title"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Alias merepresentasikan email forwarder atau alias lokal
type Alias struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	DomainID    uint           `gorm:"not null;index" json:"domain_id"`
	Source      string         `gorm:"size:255;not null;index" json:"source"`      // info@example.com
	Destination string         `gorm:"size:255;not null" json:"destination"`      // user1@example.com,user2@example.com
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Message merepresentasikan metadata email yang tersimpan
type Message struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	MessageID   string         `gorm:"size:255;index" json:"message_id"` // Standard RFC 5322 Message-ID
	Mailbox     string         `gorm:"size:64;default:'INBOX';index" json:"mailbox"` // INBOX, Sent, Trash, Spam
	From        string         `gorm:"size:255;not null" json:"from"`
	To          string         `gorm:"type:text;not null" json:"to"`
	Subject     string         `gorm:"type:text" json:"subject"`
	Snippet     string         `gorm:"size:255" json:"snippet"`
	Size        int64          `gorm:"default:0" json:"size"`
	StoragePath string         `gorm:"size:512;not null" json:"-"` // Lokasi fisik file .eml di storage disk
	IsSeen      bool           `gorm:"default:false;index" json:"is_seen"`
	IsFlagged   bool           `gorm:"default:false" json:"is_flagged"`
	SPFStatus   string         `gorm:"size:32;default:'neutral'" json:"spf_status"`
	DKIMStatus  string         `gorm:"size:32;default:'neutral'" json:"dkim_status"`
	DMARCStatus string         `gorm:"size:32;default:'neutral'" json:"dmarc_status"`
	VirusStatus string         `gorm:"size:64;default:'clean'" json:"virus_status"`
	SpamScore   float64        `gorm:"default:0.0" json:"spam_score"`
	ReceivedAt  time.Time      `json:"received_at"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// SystemConfig menyimpan konfigurasi sistem global (seperti S3 Cloud Backup)
type SystemConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Key       string         `gorm:"size:64;not null;uniqueIndex" json:"key"` // e.g. "s3_backup"
	Value     string         `gorm:"type:text;not null" json:"value"`         // JSON payload
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// PhishingCampaign merepresentasikan program simulasi uji coba phishing / spam internal perusahaan
type PhishingCampaign struct {
	ID             uint             `gorm:"primaryKey" json:"id"`
	Title          string           `gorm:"size:255;not null" json:"title"` // misal: "Simulasi Q4: Pembaruan Kebijakan HR"
	SenderName     string           `gorm:"size:128;not null" json:"sender_name"` // misal: "HR Support Portal"
	SenderEmail    string           `gorm:"size:255;not null" json:"sender_email"` // misal: "hr-support@company-portal.xyz"
	Subject        string           `gorm:"size:255;not null" json:"subject"` // misal: "[PENTING] Verifikasi Data Karyawan 2026"
	TemplateBody   string           `gorm:"type:text;not null" json:"template_body"` // HTML Template dengan placeholder {{.TrackingLink}}
	TargetDomainID *uint            `gorm:"index" json:"target_domain_id,omitempty"` // Filter domain karyawan tertentu
	Status         string           `gorm:"size:32;default:'draft';index" json:"status"` // draft, scheduled, running, completed
	ScheduledAt    *time.Time       `json:"scheduled_at,omitempty"`
	CompletedAt    *time.Time       `json:"completed_at,omitempty"`
	TotalSent      int              `gorm:"default:0" json:"total_sent"`
	TotalClicked   int              `gorm:"default:0" json:"total_clicked"`
	TotalReported  int              `gorm:"default:0" json:"total_reported"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"-"`

	Targets        []PhishingTarget `gorm:"foreignKey:CampaignID" json:"targets,omitempty"`
}

// PhishingTarget merepresentasikan partisipan/karyawan target dari simulasi phishing
type PhishingTarget struct {
	ID           uint              `gorm:"primaryKey" json:"id"`
	CampaignID   uint              `gorm:"not null;index" json:"campaign_id"`
	UserID       uint              `gorm:"not null;index" json:"user_id"`
	TrackingCode string            `gorm:"size:64;not null;uniqueIndex" json:"tracking_code"` // Token acak unik
	Status       string            `gorm:"size:32;default:'sent';index" json:"status"` // sent, opened, clicked, reported
	SentAt       time.Time         `json:"sent_at"`
	OpenedAt     *time.Time        `json:"opened_at,omitempty"`
	ClickedAt    *time.Time        `json:"clicked_at,omitempty"` // Waktu karyawan mengklik link (Not Passed)
	ReportedAt   *time.Time        `json:"reported_at,omitempty"` // Waktu karyawan menandai spam (Passed)
	IPAddress    string            `gorm:"size:64" json:"ip_address,omitempty"`
	UserAgent    string            `gorm:"size:255" json:"user_agent,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	DeletedAt    gorm.DeletedAt    `gorm:"index" json:"-"`

	User         *User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Campaign     *PhishingCampaign `gorm:"foreignKey:CampaignID" json:"campaign,omitempty"`
}
