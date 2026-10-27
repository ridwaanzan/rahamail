package scheduler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mail-server/internal/delivery"
	"mail-server/internal/domain"
	"mail-server/internal/mailbuilder"
	"mail-server/internal/repository"
	"mail-server/internal/sse"
)

// SchedulerEngine mengelola ticker periodik untuk mengeksekusi email terjadwal
type SchedulerEngine struct {
	ticker   *time.Ticker
	stopChan chan struct{}
	mu       sync.Mutex
	running  bool
}

var GlobalScheduler *SchedulerEngine

func InitScheduler() *SchedulerEngine {
	GlobalScheduler = &SchedulerEngine{
		stopChan: make(chan struct{}),
	}
	GlobalScheduler.Start()
	return GlobalScheduler
}

// Start menjalankan background loop dengan interval setiap 15 detik
func (s *SchedulerEngine) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}
	s.running = true
	s.ticker = time.NewTicker(15 * time.Second)

	log.Printf("⏰ [SCHEDULER ENGINE] Background Mail Scheduler aktif (Interval: 15s)...")

	go func() {
		// Eksekusi segera saat startup
		s.processDueEmails()

		for {
			select {
			case <-s.ticker.C:
				s.processDueEmails()
			case <-s.stopChan:
				s.ticker.Stop()
				return
			}
		}
	}()
}

// Stop menghentikan scheduler
func (s *SchedulerEngine) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
}

// processDueEmails mencari dan mengeksekusi semua email berstatus 'pending' yang sudah jatuh tempo
func (s *SchedulerEngine) processDueEmails() {
	now := time.Now()
	var dueMails []domain.ScheduledMail

	// Cari email pending yang scheduled_at <= waktu sekarang
	err := repository.DB.Preload("User").
		Where("status = ? AND scheduled_at <= ?", "pending", now).
		Find(&dueMails).Error

	if err != nil || len(dueMails) == 0 {
		return
	}

	log.Printf("⏳ [SCHEDULER ENGINE] Menemukan %d email terjadwal siap dikirim.", len(dueMails))

	for _, item := range dueMails {
		s.dispatchScheduledMail(item)
	}
}

// dispatchScheduledMail menyusun dan mengirimkan email terjadwal
func (s *SchedulerEngine) dispatchScheduledMail(item domain.ScheduledMail) {
	log.Printf("🚀 [SCHEDULER] Mengirim email terjadwal ID #%d: Dari <%s> Ke <%s> | Subjek: %s", item.ID, item.From, item.To, item.Subject)

	toList := mailbuilder.ParseAddresses(item.To)
	ccList := mailbuilder.ParseAddresses(item.Cc)
	bccList := mailbuilder.ParseAddresses(item.Bcc)

	mailData := mailbuilder.OutgoingEmail{
		From:     item.From,
		To:       toList,
		Cc:       ccList,
		Bcc:      bccList,
		Subject:  item.Subject,
		BodyHTML: item.BodyHTML,
		BodyText: item.BodyText,
	}

	rawBytes, messageID, err := mailbuilder.BuildRFC5322Message(mailData)
	if err != nil {
		repository.DB.Model(&item).Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": err.Error(),
		})
		return
	}

	// Sign DKIM jika ada
	parts := strings.Split(item.From, "@")
	if len(parts) == 2 {
		fromDomain := parts[1]
		var dom domain.Domain
		if err := repository.DB.Where("name = ?", fromDomain).First(&dom).Error; err == nil && dom.DKIMPrivateKey != "" {
			if signed, err := delivery.SignDKIM(rawBytes, dom.Name, dom.DKIMSelector, dom.DKIMPrivateKey); err == nil {
				rawBytes = signed
			}
		}
	}

	// Simpan ke storage Maildir sent
	storageDir := filepath.Join("var", "mail", "sent")
	os.MkdirAll(storageDir, 0755)
	filename := fmt.Sprintf("%d_scheduled_sent_%d.eml", time.Now().UnixNano(), item.UserID)
	filePath := filepath.Join(storageDir, filename)
	_ = os.WriteFile(filePath, rawBytes, 0644)

	snippet := item.BodyText
	if snippet == "" && item.BodyHTML != "" {
		snippet = "(HTML Content)"
	}
	if len(snippet) > 200 {
		snippet = snippet[:197] + "..."
	}

	// Rekam di folder Sent user
	sentMsg := domain.Message{
		UserID:      item.UserID,
		Mailbox:     "Sent",
		MessageID:   messageID,
		From:        item.From,
		To:          item.To,
		Subject:     item.Subject,
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

	// Update status ScheduledMail menjadi 'sent'
	repository.DB.Model(&item).Updates(map[string]interface{}{
		"status": "sent",
	})

	// Kirim ke semua recipient
	allRecipients := append(toList, ccList...)
	allRecipients = append(allRecipients, bccList...)

	for _, recipient := range allRecipients {
		rec := strings.TrimSpace(recipient)
		if rec == "" {
			continue
		}

		recParts := strings.Split(rec, "@")
		if len(recParts) == 2 {
			recDomain := recParts[1]
			var localDom domain.Domain
			if err := repository.DB.Where("name = ? AND is_active = true", recDomain).First(&localDom).Error; err == nil {
				var recUser domain.User
				if err := repository.DB.Where("email = ?", rec).First(&recUser).Error; err == nil {
					localMsg := domain.Message{
						UserID:      recUser.ID,
						Mailbox:     "INBOX",
						MessageID:   messageID,
						From:        item.From,
						To:          rec,
						Subject:     item.Subject,
						Snippet:     snippet,
						Size:        int64(len(rawBytes)),
						StoragePath: filePath,
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

		go func(targetRec string) {
			_ = delivery.DeliverOutbound(item.From, targetRec, rawBytes)
		}(rec)
	}

	// Siarkan log sukses lewat SSE
	sse.GlobalHub.Broadcast("log", map[string]string{
		"level":   "success",
		"message": fmt.Sprintf("Scheduled email #%d sent to %s", item.ID, item.To),
	})
}
