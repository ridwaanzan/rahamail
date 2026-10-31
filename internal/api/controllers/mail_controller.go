package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"mail-server/internal/backup"
	"mail-server/internal/delivery"
	"mail-server/internal/domain"
	"mail-server/internal/mailbuilder"
	"mail-server/internal/mailreader"
	"mail-server/internal/queue"
	"mail-server/internal/repository"
	"mail-server/internal/simulation"
	"mail-server/internal/sse"
	"mail-server/internal/storage"
	"mail-server/internal/trust"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type MailController struct{}

func NewMailController() *MailController {
	return &MailController{}
}

// RenderSetup merender wizard konfigurasi awal
func (mc *MailController) RenderSetup(c *gin.Context) {
	// Jika admin sudah ada, jangan izinkan setup ulang, redirect ke login
	var adminCount int64
	repository.DB.Model(&domain.User{}).Where("is_admin = ?", true).Count(&adminCount)
	if adminCount > 0 {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "setup.tmpl", gin.H{
		"title": "RaHaMail Server - Initial Setup Wizard",
	})
}

// CompleteSetup menerima konfigurasi wizard dari browser
func (mc *MailController) CompleteSetup(c *gin.Context) {
	// Proteksi one-time setup: Tolak jika sudah ada Super Admin
	var adminCount int64
	repository.DB.Model(&domain.User{}).Where("is_admin = ?", true).Count(&adminCount)
	if adminCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Setup telah selesai sebelumnya. Silakan login melalui halaman login."})
		return
	}

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

// RenderWebmail merender tampilan utama webmail user dengan dukungan pagination
func (mc *MailController) RenderWebmail(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	folder := c.DefaultQuery("folder", "INBOX")

	// Parsing parameter pagination: ?page=1&limit=25
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if err != nil || limit < 5 || limit > 100 {
		limit = 25
	}
	offset := (page - 1) * limit

	var messages []domain.Message
	var scheduledMails []domain.ScheduledMail
	var totalItems int64

	if folder == "Scheduled" {
		repository.DB.Model(&domain.ScheduledMail{}).
			Where("user_id = ?", currentUser.ID).
			Count(&totalItems)

		repository.DB.Where("user_id = ?", currentUser.ID).
			Order("scheduled_at desc").
			Offset(offset).
			Limit(limit).
			Find(&scheduledMails)
	} else {
		repository.DB.Model(&domain.Message{}).
			Where("user_id = ? AND mailbox = ?", currentUser.ID, folder).
			Count(&totalItems)

		repository.DB.Where("user_id = ? AND mailbox = ?", currentUser.ID, folder).
			Order("id desc").
			Offset(offset).
			Limit(limit).
			Find(&messages)
	}

	var scheduledCount int64
	repository.DB.Model(&domain.ScheduledMail{}).Where("user_id = ? AND status = 'pending'", currentUser.ID).Count(&scheduledCount)

	// Hitung total email per folder untuk badge counter di sidebar
	var inboxCount, sentCount, draftsCount, spamCount, trashCount int64
	type folderCountResult struct {
		Mailbox string
		Count   int64
	}
	var folderResults []folderCountResult
	repository.DB.Model(&domain.Message{}).
		Select("mailbox, count(*) as count").
		Where("user_id = ?", currentUser.ID).
		Group("mailbox").
		Scan(&folderResults)

	for _, fr := range folderResults {
		switch fr.Mailbox {
		case "INBOX":
			inboxCount = fr.Count
		case "Sent":
			sentCount = fr.Count
		case "Drafts":
			draftsCount = fr.Count
		case "Spam":
			spamCount = fr.Count
		case "Trash":
			trashCount = fr.Count
		}
	}

	// Format kuota
	quotaMB := currentUser.QuotaBytes / (1024 * 1024)
	usedMB := currentUser.UsedBytes / (1024 * 1024)

	// Hitung pagination info
	totalPages := int((totalItems + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	startItem := offset + 1
	if totalItems == 0 {
		startItem = 0
	}
	endItem := offset + limit
	if int64(endItem) > totalItems {
		endItem = int(totalItems)
	}

	c.HTML(http.StatusOK, "webmail.tmpl", gin.H{
		"title":           "RaHaMail Webmail - " + currentUser.Email,
		"current_user":    currentUser,
		"current_folder":  folder,
		"messages":        messages,
		"scheduled_mails": scheduledMails,
		"scheduled_count": scheduledCount,
		"inbox_count":     inboxCount,
		"sent_count":      sentCount,
		"drafts_count":    draftsCount,
		"spam_count":      spamCount,
		"trash_count":     trashCount,
		"storage_used":    usedMB,
		"storage_quota":   quotaMB,
		"page":            page,
		"limit":           limit,
		"total_pages":     totalPages,
		"total_items":     totalItems,
		"start_item":      startItem,
		"end_item":        endItem,
		"has_prev":        page > 1,
		"prev_page":       page - 1,
		"has_next":        page < totalPages,
		"next_page":       page + 1,
	})
}

// RenderIndex merender template Go HTML (.tmpl) untuk Overview Dashboard
func (mc *MailController) RenderIndex(c *gin.Context) {
	currentUser, _ := c.Get("user")
	var domains []domain.Domain
	var messages []domain.Message
	var domainCount, userCount, messageCount int64

	repository.DB.Find(&domains)
	repository.DB.Order("id desc").Limit(20).Find(&messages)

	repository.DB.Model(&domain.Domain{}).Count(&domainCount)
	repository.DB.Model(&domain.User{}).Count(&userCount)
	repository.DB.Model(&domain.Message{}).Count(&messageCount)

	c.HTML(http.StatusOK, "admin_dashboard.tmpl", gin.H{
		"title":       "RaHaMail Server - Live SSE Dashboard",
		"active_menu": "dashboard",
		"currentUser": currentUser,
		"domains":     domains,
		"messages":    messages,
		"db_driver":   repository.ActiveDriver,
		"stats": gin.H{
			"domains":  domainCount,
			"users":    userCount,
			"messages": messageCount,
		},
	})
}

// RenderAdminDomains merender halaman manajemen domain
func (mc *MailController) RenderAdminDomains(c *gin.Context) {
	currentUser, _ := c.Get("user")
	var domains []domain.Domain
	repository.DB.Find(&domains)

	c.HTML(http.StatusOK, "admin_domains.tmpl", gin.H{
		"title":       "RaHaMail - Domain Management & DKIM",
		"active_menu": "domains",
		"currentUser": currentUser,
		"domains":     domains,
		"db_driver":   repository.ActiveDriver,
	})
}

// RenderAdminSimulations merender halaman simulasi phishing & spam awareness
func (mc *MailController) RenderAdminSimulations(c *gin.Context) {
	currentUser, _ := c.Get("user")
	c.HTML(http.StatusOK, "admin_simulations.tmpl", gin.H{
		"title":       "RaHaMail - Phishing & Spam Awareness",
		"active_menu": "simulations",
		"currentUser": currentUser,
		"db_driver":   repository.ActiveDriver,
	})
}

// RenderAdminLogs merender halaman log stream SSE live
func (mc *MailController) RenderAdminLogs(c *gin.Context) {
	currentUser, _ := c.Get("user")
	c.HTML(http.StatusOK, "admin_logs.tmpl", gin.H{
		"title":       "RaHaMail - Live SSE Log Stream",
		"active_menu": "logs",
		"currentUser": currentUser,
		"db_driver":   repository.ActiveDriver,
	})
}

// RenderAdminSettings merender halaman pengaturan server & backup S3
func (mc *MailController) RenderAdminSettings(c *gin.Context) {
	currentUser, _ := c.Get("user")
	c.HTML(http.StatusOK, "admin_settings.tmpl", gin.H{
		"title":       "RaHaMail - Server Settings & Cloud Backup",
		"active_menu": "settings",
		"currentUser": currentUser,
		"db_driver":   repository.ActiveDriver,
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
	repository.DB.Preload("CatchAllUser").Find(&domains)
	c.JSON(http.StatusOK, domains)
}

// SetCatchAll mengonfigurasi atau menghapus user Catch-All untuk sebuah domain
func (mc *MailController) SetCatchAll(c *gin.Context) {
	domainID := c.Param("id")
	var req struct {
		UserID *uint `json:"user_id"` // null untuk menonaktifkan catch-all
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload tidak valid: " + err.Error()})
		return
	}

	var dom domain.Domain
	if err := repository.DB.First(&dom, domainID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain tidak ditemukan."})
		return
	}

	if req.UserID != nil {
		var user domain.User
		if err := repository.DB.First(&user, *req.UserID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Target user Catch-All tidak ditemukan."})
			return
		}
	}

	if err := repository.DB.Model(&dom).Update("catch_all_user_id", req.UserID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui konfigurasi Catch-All: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":             "success",
		"message":            "Pengaturan Catch-All domain berhasil diperbarui.",
		"domain_id":          dom.ID,
		"catch_all_user_id": req.UserID,
	})
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

	privKey, pubTXT, err := delivery.GenerateRSAKeyPair()
	if err != nil {
		pubTXT = "v=DKIM1; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA..."
	}

	d := domain.Domain{
		Name:           req.Name,
		DKIMSelector:   req.Selector,
		DKIMPrivateKey: privKey,
		DKIMPublicKey:  pubTXT,
		IsActive:       true,
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

	// Auto-harvest kontak: Simpan otomatis alamat penerima baru ke daftar kontak pribadi user
	go autoHarvestContacts(currentUser.ID, allRecipients)

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

	// Evaluasi Kepercayaan & Reputasi Pengirim (Smart Trust & Safety Alert)
	alert := trust.EvaluateSender(
		currentUser.ID,
		msg.From,
		parsed.BodyHTML,
		parsed.BodyText,
		len(parsed.Attachments),
		msg.SPFStatus,
		msg.DKIMStatus,
		msg.DMARCStatus,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":        msg,
		"content":        parsed,
		"security_alert": alert,
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

	// Jika dipindahkan ke Spam, periksa apakah email ini merupakan bagian dari simulasi phishing yang aktif
	if strings.EqualFold(req.TargetBox, "Spam") {
		senderAddr := trust.ExtractEmailAddress(msg.From)
		_, _ = simulation.RecordReport(currentUser.ID, senderAddr)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Email berhasil dipindahkan ke " + req.TargetBox})
}

// DeleteMessage menghapus email secara permanen (database dan file fisik .eml)
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

	// 1. Hapus file fisik .eml dari disk
	if msg.StoragePath != "" {
		_ = os.Remove(msg.StoragePath)
	}

	// 2. Pulihkan kuota penyimpanan pengguna
	if msg.Size > 0 {
		newUsed := currentUser.UsedBytes - msg.Size
		if newUsed < 0 {
			newUsed = 0
		}
		repository.DB.Model(&domain.User{}).Where("id = ?", currentUser.ID).Update("used_bytes", newUsed)
	}

	// 3. Hapus attachment terkait di database
	repository.DB.Where("message_id = ?", msg.ID).Delete(&domain.Attachment{})

	// 4. Hapus record pesan dari database
	repository.DB.Delete(&msg)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Email berhasil dihapus permanen"})
}

// EmptyTrash menghapus seluruh pesan di folder Trash secara permanen
func (mc *MailController) EmptyTrash(c *gin.Context) {
	val, _ := c.Get("user")
	currentUser := val.(*domain.User)

	var messages []domain.Message
	if err := repository.DB.Where("user_id = ? AND mailbox = 'Trash'", currentUser.ID).Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencari email di folder Trash"})
		return
	}

	var totalFreedBytes int64
	for _, msg := range messages {
		if msg.StoragePath != "" {
			_ = os.Remove(msg.StoragePath)
		}
		totalFreedBytes += msg.Size
		repository.DB.Where("message_id = ?", msg.ID).Delete(&domain.Attachment{})
		repository.DB.Delete(&msg)
	}

	if totalFreedBytes > 0 {
		newUsed := currentUser.UsedBytes - totalFreedBytes
		if newUsed < 0 {
			newUsed = 0
		}
		repository.DB.Model(&domain.User{}).Where("id = ?", currentUser.ID).Update("used_bytes", newUsed)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("%d email di folder Trash berhasil dihapus permanen", len(messages)),
		"count":   len(messages),
	})
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

// SendTransactionalMail menangani pengiriman email transaksional developer via REST API menggunakan API Key
func (mc *MailController) SendTransactionalMail(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized API token."})
		return
	}
	currentUser := val.(*domain.User)

	var req struct {
		To          []string `json:"to" binding:"required"`
		Cc          []string `json:"cc"`
		Bcc         []string `json:"bcc"`
		Subject     string   `json:"subject" binding:"required"`
		HTML        string   `json:"html"`
		Text        string   `json:"text"`
		SenderName  string   `json:"from_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload JSON tidak valid: " + err.Error()})
		return
	}

	if len(req.To) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field 'to' harus memiliki minimal 1 alamat penerima."})
		return
	}

	fromHeader := currentUser.Email
	if req.SenderName != "" {
		fromHeader = req.SenderName + " <" + currentUser.Email + ">"
	}

	mailData := mailbuilder.OutgoingEmail{
		From:     fromHeader,
		To:       req.To,
		Cc:       req.Cc,
		Bcc:      req.Bcc,
		Subject:  req.Subject,
		BodyHTML: req.HTML,
		BodyText: req.Text,
	}

	rawBytes, messageID, err := mailbuilder.BuildRFC5322Message(mailData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyusun pesan RFC 5322: " + err.Error()})
		return
	}

	// Sign DKIM jika domain memiliki kunci
	fromDomain := ""
	parts := strings.Split(currentUser.Email, "@")
	if len(parts) == 2 {
		fromDomain = parts[1]
	}

	var dom domain.Domain
	if err := repository.DB.Where("name = ?", fromDomain).First(&dom).Error; err == nil && dom.DKIMPrivateKey != "" {
		if signed, err := delivery.SignDKIM(rawBytes, dom.Name, dom.DKIMSelector, dom.DKIMPrivateKey); err == nil {
			rawBytes = signed
		}
	}

	// Simpan ke storage Maildir sent secara aman
	filePath, err := storage.SaveMailToFile("sent", currentUser.ID, messageID, rawBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file sent: " + err.Error()})
		return
	}

	snippet := req.Text
	if snippet == "" && req.HTML != "" {
		snippet = "(Transactional HTML)"
	}
	if len(snippet) > 200 {
		snippet = snippet[:197] + "..."
	}

	sentMsg := domain.Message{
		UserID:      currentUser.ID,
		Mailbox:     "Sent",
		MessageID:   messageID,
		From:        currentUser.Email,
		To:          strings.Join(req.To, ", "),
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

	// Kirim langsung ke setiap penerima (inbound lokal atau outbound MTA)
	allRecipients := append(req.To, req.Cc...)
	allRecipients = append(allRecipients, req.Bcc...)

	go func(recipients []string, b []byte, s domain.Message) {
		for _, recipient := range recipients {
			rec := strings.TrimSpace(recipient)
			if rec == "" {
				continue
			}

			// Cek apakah lokal
			var localUser domain.User
			if err := repository.DB.Where("email = ?", rec).First(&localUser).Error; err == nil {
				inboxPath, err := storage.SaveMailToFile("inbox", localUser.ID, s.MessageID, b)
				if err == nil {
					inboxMsg := domain.Message{
						UserID:      localUser.ID,
						Mailbox:     "INBOX",
						MessageID:   s.MessageID,
						From:        s.From,
						To:          rec,
						Subject:     s.Subject,
						Snippet:     s.Snippet,
						Size:        s.Size,
						StoragePath: inboxPath,
						SPFStatus:   "pass",
						DKIMStatus:  "pass",
						DMARCStatus: "pass",
						ReceivedAt:  time.Now(),
					}
					repository.DB.Create(&inboxMsg)
					sse.GlobalHub.Broadcast("new_email", inboxMsg)
				}
			} else {
				// Outbound eksternal ke internet via MTA
				_ = delivery.DeliverOutbound(s.From, rec, b)
			}
		}
	}(allRecipients, rawBytes, sentMsg)

	c.JSON(http.StatusOK, gin.H{
		"status":     "sent",
		"message":    "Transactional email accepted and dispatched.",
		"message_id": messageID,
		"recipients": len(allRecipients),
	})
}

// autoHarvestContacts mengekstrak alamat email unik dan menyimpannya secara otomatis ke buku kontak user jika belum ada
func autoHarvestContacts(userID uint, rawAddresses []string) {
	for _, raw := range rawAddresses {
		email := strings.TrimSpace(strings.ToLower(raw))
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		var count int64
		repository.DB.Model(&domain.Contact{}).Where("user_id = ? AND email = ?", userID, email).Count(&count)
		if count == 0 {
			name := strings.Split(email, "@")[0]
			repository.DB.Create(&domain.Contact{
				UserID: userID,
				Name:   strings.Title(name),
				Email:  email,
			})
		}
	}
}

// ListContacts mengambil daftar kontak milik user yang sedang login
func (mc *MailController) ListContacts(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)

	query := strings.TrimSpace(c.Query("q"))
	dbQuery := repository.DB.Where("user_id = ?", user.ID)
	if query != "" {
		likePattern := "%" + query + "%"
		dbQuery = dbQuery.Where("name LIKE ? OR email LIKE ? OR company LIKE ?", likePattern, likePattern, likePattern)
	}

	var contacts []domain.Contact
	dbQuery.Order("name asc, email asc").Find(&contacts)

	c.JSON(http.StatusOK, contacts)
}

// CreateContact membuat kontak baru di buku alamat user
func (mc *MailController) CreateContact(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)

	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email" binding:"required"`
		Company string `json:"company"`
		Notes   string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alamat email wajib diisi."})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if !strings.Contains(email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format alamat email tidak valid."})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	var existing domain.Contact
	if err := repository.DB.Where("user_id = ? AND email = ?", user.ID, email).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kontak dengan email ini sudah ada."})
		return
	}

	contact := domain.Contact{
		UserID:  user.ID,
		Name:    name,
		Email:   email,
		Company: strings.TrimSpace(req.Company),
		Notes:   strings.TrimSpace(req.Notes),
	}

	if err := repository.DB.Create(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan kontak: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kontak berhasil ditambahkan.",
		"contact": contact,
	})
}

// DeleteContact menghapus kontak dari buku alamat
func (mc *MailController) DeleteContact(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)
	contactID := c.Param("id")

	res := repository.DB.Where("id = ? AND user_id = ?", contactID, user.ID).Delete(&domain.Contact{})
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kontak tidak ditemukan."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kontak berhasil dihapus.",
	})
}

// GetAutoResponder mengambil konfigurasi Out-of-Office / Vacation Responder user
func (mc *MailController) GetAutoResponder(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)

	var ar domain.AutoResponder
	if err := repository.DB.Where("user_id = ?", user.ID).First(&ar).Error; err != nil {
		ar = domain.AutoResponder{
			UserID:   user.ID,
			Enabled:  false,
			Subject:  "Out of Office: " + user.Email,
			BodyText: "Halo,\n\nSaat ini saya sedang tidak berada di tempat dan memiliki akses email yang terbatas. Saya akan membalas pesan Anda setelah kembali.\n\nTerima kasih.",
		}
	}

	c.JSON(http.StatusOK, ar)
}

// SaveAutoResponder menyimpan pengaturan balasan liburan otomatis
func (mc *MailController) SaveAutoResponder(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)

	var req struct {
		Enabled   bool       `json:"enabled"`
		StartTime *time.Time `json:"start_time"`
		EndTime   *time.Time `json:"end_time"`
		Subject   string     `json:"subject"`
		BodyHTML  string     `json:"body_html"`
		BodyText  string     `json:"body_text"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload tidak valid: " + err.Error()})
		return
	}

	var ar domain.AutoResponder
	err := repository.DB.Where("user_id = ?", user.ID).First(&ar).Error
	if err != nil {
		ar = domain.AutoResponder{
			UserID:    user.ID,
			Enabled:   req.Enabled,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
			Subject:   strings.TrimSpace(req.Subject),
			BodyHTML:  req.BodyHTML,
			BodyText:  req.BodyText,
		}
		if err := repository.DB.Create(&ar).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan auto responder: " + err.Error()})
			return
		}
	} else {
		ar.Enabled = req.Enabled
		ar.StartTime = req.StartTime
		ar.EndTime = req.EndTime
		ar.Subject = strings.TrimSpace(req.Subject)
		ar.BodyHTML = req.BodyHTML
		ar.BodyText = req.BodyText
		if err := repository.DB.Save(&ar).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui auto responder: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pengaturan Vacation Auto-Responder berhasil disimpan.",
		"data":    ar,
	})
}

// ListCannedResponses mengambil daftar template email siap pakai (snippets)
func (mc *MailController) ListCannedResponses(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)

	var responses []domain.CannedResponse
	repository.DB.Where("user_id = ?", user.ID).Order("created_at desc").Find(&responses)

	c.JSON(http.StatusOK, responses)
}

// CreateCannedResponse membuat template snippet email baru
func (mc *MailController) CreateCannedResponse(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)

	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Judul dan isi template harus diisi."})
		return
	}

	template := domain.CannedResponse{
		UserID:  user.ID,
		Title:   strings.TrimSpace(req.Title),
		Content: req.Content,
	}

	if err := repository.DB.Create(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat template: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Template email berhasil dibuat.",
		"template": template,
	})
}

// DeleteCannedResponse menghapus template snippet
func (mc *MailController) DeleteCannedResponse(c *gin.Context) {
	val, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Autentikasi diperlukan."})
		return
	}
	user := val.(*domain.User)
	templateID := c.Param("id")

	res := repository.DB.Where("id = ? AND user_id = ?", templateID, user.ID).Delete(&domain.CannedResponse{})
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template tidak ditemukan."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Template berhasil dihapus.",
	})
}

// GetBackupConfig mengambil konfigurasi S3 cloud backup
func (mc *MailController) GetBackupConfig(c *gin.Context) {
	var cfg domain.SystemConfig
	err := repository.DB.Where("key = ?", "s3_backup").First(&cfg).Error
	if err != nil {
		c.JSON(http.StatusOK, backup.S3Config{
			Region:        "us-east-1",
			UseSSL:        true,
			RetentionDays: 30,
		})
		return
	}
	c.Data(http.StatusOK, "application/json", []byte(cfg.Value))
}

// SaveBackupConfig menyimpan konfigurasi cloud backup
func (mc *MailController) SaveBackupConfig(c *gin.Context) {
	var s3Cfg backup.S3Config
	if err := c.ShouldBindJSON(&s3Cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload konfigurasi S3 tidak valid: " + err.Error()})
		return
	}

	rawBytes, _ := json.Marshal(s3Cfg)
	var sysCfg domain.SystemConfig
	err := repository.DB.Where("key = ?", "s3_backup").First(&sysCfg).Error
	if err != nil {
		sysCfg = domain.SystemConfig{
			Key:   "s3_backup",
			Value: string(rawBytes),
		}
		repository.DB.Create(&sysCfg)
	} else {
		sysCfg.Value = string(rawBytes)
		repository.DB.Save(&sysCfg)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Konfigurasi S3 Cloud Backup berhasil disimpan.",
		"data":    s3Cfg,
	})
}

// TriggerManualBackup menjalankan snapshot backup saat ini juga
func (mc *MailController) TriggerManualBackup(c *gin.Context) {
	var s3Cfg backup.S3Config
	var sysCfg domain.SystemConfig
	if err := repository.DB.Where("key = ?", "s3_backup").First(&sysCfg).Error; err == nil {
		_ = json.Unmarshal([]byte(sysCfg.Value), &s3Cfg)
	}

	filename, size, err := backup.PerformBackup(s3Cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "failed",
			"error":   err.Error(),
			"file":    filename,
			"size":    size,
		})
		return
	}

	now := time.Now()
	s3Cfg.LastBackupTime = &now
	s3Cfg.LastBackupStatus = "success"
	updatedRaw, _ := json.Marshal(s3Cfg)
	repository.DB.Model(&domain.SystemConfig{}).Where("key = ?", "s3_backup").Update("value", string(updatedRaw))

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Backup database dan Maildir berhasil dibuat.",
		"filename": filename,
		"size":     size,
		"time":     now,
	})
}
