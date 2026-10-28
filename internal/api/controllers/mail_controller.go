package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"mail-server/internal/delivery"
	"mail-server/internal/domain"
	"mail-server/internal/mailbuilder"
	"mail-server/internal/mailreader"
	"mail-server/internal/queue"
	"mail-server/internal/repository"
	"mail-server/internal/sse"
	"mail-server/internal/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type MailController struct{}

func NewMailController() *MailController {
	return &MailController{}
}

// RenderSetup merender wizard konfigurasi awal
func (mc *MailController) RenderSetup(c *gin.Context) {
	c.HTML(http.StatusOK, "setup.tmpl", gin.H{
		"title": "RaHaMail Server - Initial Setup Wizard",
	})
}

// CompleteSetup menerima konfigurasi wizard dari browser
func (mc *MailController) CompleteSetup(c *gin.Context) {
	var req struct {
		DBDriver      string `json:"db_driver"`
		PostgresDSN   string `json:"postgres_dsn"`
		DomainName    string `json:"domain_name"`
		DKIMSelector  string `json:"dkim_selector"`
		AdminEmail    string `json:"admin_email"`
		AdminPassword string `json:"admin_password"`
		AdminQuota    int64  `json:"admin_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Simpan atau update Domain
	var dom domain.Domain
	if err := repository.DB.Where("name = ?", req.DomainName).First(&dom).Error; err != nil {
		dom = domain.Domain{
			Name:         req.DomainName,
			DKIMSelector: req.DKIMSelector,
			IsActive:     true,
		}
		repository.DB.Create(&dom)
	}

	// 2. Buat atau update Super Admin
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	var user domain.User
	if err := repository.DB.Where("email = ?", req.AdminEmail).First(&user).Error; err != nil {
		user = domain.User{
			DomainID:     dom.ID,
			Email:        req.AdminEmail,
			PasswordHash: string(hash),
			QuotaBytes:   req.AdminQuota,
			IsAdmin:      true,
			IsActive:     true,
		}
		repository.DB.Create(&user)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Setup berhasil diselesaikan",
		"domain":  dom.Name,
		"admin":   user.Email,
	})
}

// RenderWebmail merender tampilan utama webmail user
func (mc *MailController) RenderWebmail(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	folder := c.DefaultQuery("folder", "INBOX")
	var messages []domain.Message
	var scheduledMails []domain.ScheduledMail

	if folder == "Scheduled" {
		repository.DB.Where("user_id = ?", currentUser.ID).
			Order("scheduled_at desc").
			Limit(100).
			Find(&scheduledMails)
	} else {
		repository.DB.Where("user_id = ? AND mailbox = ?", currentUser.ID, folder).
			Order("id desc").
			Limit(100).
			Find(&messages)
	}

	var scheduledCount int64
	repository.DB.Model(&domain.ScheduledMail{}).Where("user_id = ? AND status = 'pending'", currentUser.ID).Count(&scheduledCount)

	// Format kuota
	quotaMB := currentUser.QuotaBytes / (1024 * 1024)
	usedMB := currentUser.UsedBytes / (1024 * 1024)

	c.HTML(http.StatusOK, "webmail.tmpl", gin.H{
		"title":           "RaHaMail Webmail - " + currentUser.Email,
		"current_user":    currentUser,
		"current_folder":  folder,
		"messages":        messages,
		"scheduled_mails": scheduledMails,
		"scheduled_count": scheduledCount,
		"storage_used":    usedMB,
		"storage_quota":   quotaMB,
	})
}

// RenderIndex merender template Go HTML (.tmpl) dengan data awal
func (mc *MailController) RenderIndex(c *gin.Context) {
	var domains []domain.Domain
	var messages []domain.Message
	var domainCount, userCount, messageCount int64

	repository.DB.Find(&domains)
	repository.DB.Order("id desc").Limit(20).Find(&messages)

	repository.DB.Model(&domain.Domain{}).Count(&domainCount)
	repository.DB.Model(&domain.User{}).Count(&userCount)
	repository.DB.Model(&domain.Message{}).Count(&messageCount)

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title":     "RaHaMail Server - Live SSE Dashboard",
		"domains":   domains,
		"messages":  messages,
		"db_driver": repository.ActiveDriver,
		"stats": gin.H{
			"domains":  domainCount,
			"users":    userCount,
			"messages": messageCount,
		},
	})
}

// GetStats mengembalikan ringkasan statistik untuk dashboard
func (mc *MailController) GetStats(c *gin.Context) {
	var domainCount, userCount, messageCount int64
	repository.DB.Model(&domain.Domain{}).Count(&domainCount)
	repository.DB.Model(&domain.User{}).Count(&userCount)
	repository.DB.Model(&domain.Message{}).Count(&messageCount)

	c.JSON(http.StatusOK, gin.H{
		"domains":  domainCount,
		"users":    userCount,
		"messages": messageCount,
		"status":   "healthy",
	})
}

// ListDomains mengambil daftar domain
func (mc *MailController) ListDomains(c *gin.Context) {
	var domains []domain.Domain
	repository.DB.Find(&domains)
	c.JSON(http.StatusOK, domains)
}

// CreateDomain mendaftarkan domain baru
func (mc *MailController) CreateDomain(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Selector string `json:"selector"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Selector == "" {
		req.Selector = "mail"
	}

	d := domain.Domain{
		Name:          req.Name,
		DKIMSelector:  req.Selector,
		DKIMPublicKey: "v=DKIM1; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A...",
		IsActive:      true,
	}

	if err := repository.DB.Create(&d).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan domain: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, d)
}

// ListUsers mengambil daftar user
func (mc *MailController) ListUsers(c *gin.Context) {
	var users []domain.User
	repository.DB.Preload("Domain").Find(&users)
	c.JSON(http.StatusOK, users)
}

// CreateUser membuat kotak surat baru
func (mc *MailController) CreateUser(c *gin.Context) {
	var req struct {
		DomainID uint   `json:"domain_id" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal hashing password"})
		return
	}

	u := domain.User{
		DomainID:     req.DomainID,
		Email:        req.Email,
		PasswordHash: string(hash),
		IsActive:     true,
	}

	if err := repository.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat user: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, u)
}

// ScheduleEmail menangani pendaftaran email yang dijadwalkan
func (mc *MailController) ScheduleEmail(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := val.(*domain.User)

	var req struct {
		To          string `json:"to" binding:"required"`
		Cc          string `json:"cc"`
		Bcc         string `json:"bcc"`
		Subject     string `json:"subject" binding:"required"`
		BodyHTML    string `json:"body_html"`
		BodyText    string `json:"body_text"`
		ScheduledAt string `json:"scheduled_at" binding:"required"` // Format ISO: 2026-10-28T09:00
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak lengkap: " + err.Error()})
		return
	}

	schedTime, err := time.Parse("2006-01-02T15:04", req.ScheduledAt)
	if err != nil {
		// Coba format RFC3339
		schedTime, err = time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format waktu tidak valid (gunakan format YYYY-MM-DDTHH:MM)"})
			return
		}
	}

	if schedTime.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Waktu penjadwalan harus di masa mendatang"})
		return
	}

	scheduledMail := domain.ScheduledMail{
		UserID:      currentUser.ID,
		From:        currentUser.Email,
		To:          req.To,
		Cc:          req.Cc,
		Bcc:         req.Bcc,
		Subject:     req.Subject,
		BodyHTML:    req.BodyHTML,
		BodyText:    req.BodyText,
		ScheduledAt: schedTime,
		Status:      "pending",
	}

	if err := repository.DB.Create(&scheduledMail).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan jadwal email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      "Email berhasil dijadwalkan!",
		"scheduled_at": schedTime.Format("02 Jan 2006 15:04"),
	})
}

// CancelScheduledMail membatalkan pengiriman email terjadwal
func (mc *MailController) CancelScheduledMail(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	idStr := c.Param("id")
	schedID, _ := strconv.ParseUint(idStr, 10, 64)

	var sched domain.ScheduledMail
	if err := repository.DB.Where("id = ? AND user_id = ?", schedID, currentUser.ID).First(&sched).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Jadwal email tidak ditemukan"})
		return
	}

	if sched.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email ini sudah terkirim atau dibatalkan sebelumnya"})
		return
	}

	repository.DB.Model(&sched).Update("status", "cancelled")
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Jadwal pengiriman email berhasil dibatalkan"})
}

// ComposeAndSendEmail menangani pengiriman email dari Webmail UI (mendukung HTML, CC, BCC, Attachments)
func (mc *MailController) ComposeAndSendEmail(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := val.(*domain.User)

	var req struct {
		To          string `json:"to" binding:"required"`
		Cc          string `json:"cc"`
		Bcc         string `json:"bcc"`
		Subject     string `json:"subject" binding:"required"`
		BodyHTML    string `json:"body_html"`
		BodyText    string `json:"body_text"`
		Attachments []struct {
			FileName    string `json:"file_name"`
			ContentType string `json:"content_type"`
			DataBase64  string `json:"data_base64"`
		} `json:"attachments"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format input salah: " + err.Error()})
		return
	}

	toList := mailbuilder.ParseAddresses(req.To)
	if len(toList) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alamat tujuan (To) harus diisi dengan valid"})
		return
	}

	ccList := mailbuilder.ParseAddresses(req.Cc)
	bccList := mailbuilder.ParseAddresses(req.Bcc)

	var attInputs []mailbuilder.AttachmentInput
	for _, a := range req.Attachments {
		dataBytes, err := mailbuilder.DecodeBase64(a.DataBase64)
		if err == nil {
			attInputs = append(attInputs, mailbuilder.AttachmentInput{
				FileName:    a.FileName,
				ContentType: a.ContentType,
				Data:        dataBytes,
			})
		}
	}

	mailData := mailbuilder.OutgoingEmail{
		From:        currentUser.Email,
		To:          toList,
		Cc:          ccList,
		Bcc:         bccList,
		Subject:     req.Subject,
		BodyHTML:    req.BodyHTML,
		BodyText:    req.BodyText,
		Attachments: attInputs,
	}

	rawBytes, messageID, err := mailbuilder.BuildRFC5322Message(mailData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyusun pesan email: " + err.Error()})
		return
	}

	// Tanda tangani dengan DKIM jika domain pengirim memiliki kunci
	fromDomain := currentUser.Domain.Name
	if fromDomain == "" {
		parts := strings.Split(currentUser.Email, "@")
		if len(parts) == 2 {
			fromDomain = parts[1]
		}
	}

	var dom domain.Domain
	if err := repository.DB.Where("name = ?", fromDomain).First(&dom).Error; err == nil && dom.DKIMPrivateKey != "" {
		signed, err := delivery.SignDKIM(rawBytes, dom.Name, dom.DKIMSelector, dom.DKIMPrivateKey)
		if err == nil {
			rawBytes = signed
		}
	}

	// Simpan ke storage Maildir sent secara aman
	filePath, err := storage.SaveMailToFile("sent", currentUser.ID, messageID, rawBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file email sent: " + err.Error()})
		return
	}

	// Simpan riwayat ke folder 'Sent' di database
	snippet := req.BodyText
	if snippet == "" && req.BodyHTML != "" {
		snippet = "(HTML Content)"
	}
	if len(snippet) > 200 {
		snippet = snippet[:197] + "..."
	}

	sentMsg := domain.Message{
		UserID:      currentUser.ID,
		Mailbox:     "Sent",
		MessageID:   messageID,
		From:        currentUser.Email,
		To:          req.To,
		Subject:     req.Subject,
		Snippet:     snippet,
		Size:        int64(len(rawBytes)),
		StoragePath: filePath,
		IsSeen:      true,
		SPFStatus:   "pass",
		DKIMStatus:  "pass",
		DMARCStatus: "pass",
		ReceivedAt:  time.Now(),
	}
	repository.DB.Create(&sentMsg)

	// Kirim ke semua recipient (To, Cc, Bcc)
	allRecipients := append(toList, ccList...)
	allRecipients = append(allRecipients, bccList...)

	for _, recipient := range allRecipients {
		rec := strings.TrimSpace(recipient)
		if rec == "" {
			continue
		}
		// Cek apakah recipient lokal atau remote
		parts := strings.Split(rec, "@")
		if len(parts) == 2 {
			recDomain := parts[1]
			var localDom domain.Domain
			if err := repository.DB.Where("name = ? AND is_active = true", recDomain).First(&localDom).Error; err == nil {
				// Penerima Lokal: masukkan langsung ke INBOX penerima
				var recUser domain.User
				if err := repository.DB.Where("email = ?", rec).First(&recUser).Error; err == nil {
					localPath, err := storage.SaveMailToFile("inbox", recUser.ID, messageID, rawBytes)
					if err != nil {
						localPath = filePath
					}

					localMsg := domain.Message{
						UserID:      recUser.ID,
						Mailbox:     "INBOX",
						MessageID:   messageID,
						From:        currentUser.Email,
						To:          rec,
						Subject:     req.Subject,
						Snippet:     snippet,
						Size:        int64(len(rawBytes)),
						StoragePath: localPath,
						SPFStatus:   "pass",
						DKIMStatus:  "pass",
						DMARCStatus: "pass",
						ReceivedAt:  time.Now(),
						IsSeen:      false,
					}
					repository.DB.Create(&localMsg)
					sse.GlobalHub.Broadcast("new_email", localMsg)
					continue
				}
			}
		}

		// Penerima Remote: kirim ke background Outbound MTA delivery
		go func(targetRec string) {
			_ = delivery.DeliverOutbound(currentUser.Email, targetRec, rawBytes)
		}(rec)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Email berhasil dikirim!",
		"message_id": messageID,
	})
}

// GetMessageDetail mengambil konten detail email (HTML / Text / Lampiran)
func (mc *MailController) GetMessageDetail(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := val.(*domain.User)

	idStr := c.Param("id")
	msgID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID pesan tidak valid"})
		return
	}

	var msg domain.Message
	if err := repository.DB.Where("id = ? AND user_id = ?", msgID, currentUser.ID).First(&msg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesan tidak ditemukan"})
		return
	}

	// Validasi keamanan Anti-Path Traversal pada storage file
	if msg.StoragePath != "" && !storage.IsPathSafe(msg.StoragePath) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insecure file path access detected"})
		return
	}

	// Tandai sudah dibaca jika belum
	if !msg.IsSeen {
		repository.DB.Model(&msg).Update("is_seen", true)
		msg.IsSeen = true
	}

	parsed, err := mailreader.ParseEML(msg.StoragePath, msg.Snippet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca isi email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": msg,
		"content": parsed,
	})
}

// MoveMessage memindahkan email ke folder lain (Trash, Spam, Inbox, Archive)
func (mc *MailController) MoveMessage(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	var req struct {
		MessageID uint   `json:"message_id" binding:"required"`
		TargetBox string `json:"target_mailbox" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var msg domain.Message
	if err := repository.DB.Where("id = ? AND user_id = ?", req.MessageID, currentUser.ID).First(&msg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesan tidak ditemukan"})
		return
	}

	repository.DB.Model(&msg).Update("mailbox", req.TargetBox)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Email berhasil dipindahkan ke " + req.TargetBox})
}

// DeleteMessage menghapus email secara permanen
func (mc *MailController) DeleteMessage(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	idStr := c.Param("id")
	msgID, _ := strconv.ParseUint(idStr, 10, 64)

	var msg domain.Message
	if err := repository.DB.Where("id = ? AND user_id = ?", msgID, currentUser.ID).First(&msg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesan tidak ditemukan"})
		return
	}

	repository.DB.Delete(&msg)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Email berhasil dihapus permanen"})
}

// ToggleStarMessage menandai atau membatalkan tanda bintang (flagged)
func (mc *MailController) ToggleStarMessage(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	idStr := c.Param("id")
	msgID, _ := strconv.ParseUint(idStr, 10, 64)

	var msg domain.Message
	if err := repository.DB.Where("id = ? AND user_id = ?", msgID, currentUser.ID).First(&msg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pesan tidak ditemukan"})
		return
	}

	newStatus := !msg.IsFlagged
	repository.DB.Model(&msg).Update("is_flagged", newStatus)
	c.JSON(http.StatusOK, gin.H{"status": "success", "is_flagged": newStatus})
}

// ListMessages mengambil daftar pesan
func (mc *MailController) ListMessages(c *gin.Context) {
	var messages []domain.Message
	repository.DB.Order("id desc").Find(&messages)
	c.JSON(http.StatusOK, messages)
}

// SendEmail menangani pengiriman email via REST API
func (mc *MailController) SendEmail(c *gin.Context) {
	var req struct {
		From    string `json:"from" binding:"required,email"`
		To      string `json:"to" binding:"required,email"`
		Subject string `json:"subject" binding:"required"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Simpan ke database
	msg := domain.Message{
		UserID:      1,
		Mailbox:     "Sent",
		From:        req.From,
		To:          req.To,
		Subject:     req.Subject,
		Snippet:     req.Body,
		StoragePath: "/var/mail/sent.eml",
		SPFStatus:   "pass",
		DKIMStatus:  "pass",
		DMARCStatus: "pass",
		IsSeen:      true,
	}
	repository.DB.Create(&msg)

	// Masukkan ke antrean pengiriman background Asynq / MTA Worker
	queue.GlobalQueue.EnqueueEmail(msg, req.Body)

	// Broadcast ke semua client SSE bahwa ada email baru
	sse.GlobalHub.Broadcast("new_email", msg)

	// Broadcast juga statistik terbaru
	var messageCount, domainCount, userCount int64
	repository.DB.Model(&domain.Message{}).Count(&messageCount)
	repository.DB.Model(&domain.Domain{}).Count(&domainCount)
	repository.DB.Model(&domain.User{}).Count(&userCount)
	sse.GlobalHub.Broadcast("stats", gin.H{
		"messages": messageCount,
		"domains":  domainCount,
		"users":    userCount,
	})

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "queued",
		"message": "Email berhasil dimasukkan ke antrean MTA",
		"data":    msg,
	})
}
