package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"mail-server/internal/delivery"
	"mail-server/internal/domain"
	"mail-server/internal/repository"
	"mail-server/internal/sse"

	"github.com/hibiken/asynq"
)

const (
	TypeEmailDelivery = "email:deliver"
)

// EmailDeliveryPayload menyimpan data email yang akan dikirim oleh worker
type EmailDeliveryPayload struct {
	MessageID uint   `json:"message_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

// Manager mengelola client enqueuer dan background worker
type Manager struct {
	client *asynq.Client
	server *asynq.Server
}

var GlobalQueue *Manager

// InitQueue menginisialisasi Asynq jika Redis tersedia, atau direct goroutine queue jika Redis offline
func InitQueue() *Manager {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}

	// Cek konektivitas TCP ke Redis terlebih dahulu sebelum polling terus-menerus
	conn, err := net.DialTimeout("tcp", redisAddr, 800*time.Millisecond)
	var client *asynq.Client
	var server *asynq.Server

	if err == nil {
		conn.Close()
		redisOpt := asynq.RedisClientOpt{Addr: redisAddr}
		client = asynq.NewClient(redisOpt)
		server = asynq.NewServer(
			redisOpt,
			asynq.Config{
				Concurrency: 5,
				Queues: map[string]int{
					"default": 1,
				},
			},
		)

		go func() {
			mux := asynq.NewServeMux()
			mux.HandleFunc(TypeEmailDelivery, HandleEmailDeliveryTask)
			log.Printf("📬 [ASYNQ QUEUE] Background worker Redis terhubung di %s", redisAddr)
			if err := server.Run(mux); err != nil {
				log.Printf("⚠️ Asynq server stopped: %v", err)
			}
		}()
	} else {
		log.Printf("ℹ️ [QUEUE MODE] Redis offline (%s tidak aktif). Sistem beralih ke Mode Asynchronous In-Memory Goroutine.", redisAddr)
	}

	GlobalQueue = &Manager{
		client: client,
		server: server,
	}

	return GlobalQueue
}

// EnqueueEmail memasukkan tugas pengiriman email ke antrean
func (m *Manager) EnqueueEmail(msg domain.Message, body string) error {
	if m.client != nil {
		payload, err := json.Marshal(EmailDeliveryPayload{
			MessageID: msg.ID,
			From:      msg.From,
			To:        msg.To,
			Subject:   msg.Subject,
			Body:      body,
		})
		if err == nil {
			task := asynq.NewTask(TypeEmailDelivery, payload, asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
			if info, err := m.client.Enqueue(task); err == nil {
				log.Printf("📥 [ASYNQ ENQUEUED] Email ID #%d dimasukkan ke antrean Redis (Task: %s)", msg.ID, info.ID)
				return nil
			}
		}
	}

	// Fallback In-Memory Goroutine non-blocking
	go processDeliverySync(msg.From, msg.To, msg.Subject, body, msg.ID)
	return nil
}

// HandleEmailDeliveryTask dipanggil oleh Asynq Worker untuk mengeksekusi pengiriman
func HandleEmailDeliveryTask(ctx context.Context, t *asynq.Task) error {
	var p EmailDeliveryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("gagal unmarshal payload task: %w", err)
	}

	return processDeliverySync(p.From, p.To, p.Subject, p.Body, p.MessageID)
}

func processDeliverySync(from, to, subject, body string, messageID uint) error {
	log.Printf("⚡ [MTA WORKER] Memproses pengiriman outbound: <%s> -> <%s>", from, to)

	// Buat pesan RFC 5322 standar
	dateStr := time.Now().Format(time.RFC1123Z)
	rawMsg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%d.%d@rahamail.local>\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, dateStr, messageID, time.Now().UnixNano(), body)

	fromParts := strings.Split(from, "@")
	if len(fromParts) == 2 {
		fromDomain := fromParts[1]
		var dom domain.Domain
		if err := repository.DB.Where("name = ?", fromDomain).First(&dom).Error; err == nil && dom.DKIMPrivateKey != "" {
			// Tandatangani email dengan DKIM signature
			signed, err := delivery.SignDKIM([]byte(rawMsg), dom.Name, dom.DKIMSelector, dom.DKIMPrivateKey)
			if err == nil {
				rawMsg = string(signed)
				log.Printf("🔑 [DKIM SIGNED] Email ditandatangani menggunakan RSA kunci domain: %s", dom.Name)
			}
		}
	}

	// Lakukan pengiriman via SMTP Socket ke MX target
	err := delivery.DeliverOutbound(from, to, []byte(rawMsg))
	if err != nil {
		log.Printf("❌ [MTA DELIVERY FAILED] Gagal mengirim ke %s: %v", to, err)
		sse.GlobalHub.Broadcast("log", map[string]string{
			"level":   "error",
			"message": fmt.Sprintf("Delivery failed to %s: %v", to, err),
		})
		return err
	}

	sse.GlobalHub.Broadcast("log", map[string]string{
		"level":   "success",
		"message": fmt.Sprintf("Email successfully delivered to %s", to),
	})
	return nil
}
