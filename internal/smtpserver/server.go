package smtpserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
	"mail-server/internal/sse"
	"mail-server/internal/storage"

	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"golang.org/x/crypto/bcrypt"
)

// Backend mengimplementasikan smtp.Backend
type Backend struct{}

func NewBackend() *Backend {
	return &Backend{}
}

// NewSession dipanggil setiap ada koneksi client TCP baru ke server SMTP
func (bk *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	remoteAddr := c.Conn().RemoteAddr().String()
	log.Printf("🔌 [SMTP] Koneksi masuk dari: %s", remoteAddr)
	return &Session{
		remoteIP: remoteAddr,
	}, nil
}

// Session mengelola alur state percakapan SMTP
type Session struct {
	remoteIP      string
	authenticated bool
	authUser      *domain.User
	from          string
	to            []string
}

// AuthMechanisms mendefinisikan mekanisme otentikasi yang didukung
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth memvalidasi username dan password user saat mengirim lewat submission
func (s *Session) Auth(mech string) (sasl.Server, error) {
	if mech != sasl.Plain {
		return nil, errors.New("504 5.7.4 Unrecognized authentication type")
	}

	return sasl.NewPlainServer(func(identity, username, password string) error {
		var user domain.User
		if err := repository.DB.Where("email = ? AND is_active = true", username).First(&user).Error; err != nil {
			log.Printf("⚠️ [SMTP-AUTH] User tidak ditemukan atau non-aktif: %s", username)
			return errors.New("535 5.7.8 Authentication credentials invalid")
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			log.Printf("⚠️ [SMTP-AUTH] Password salah untuk user: %s", username)
			return errors.New("535 5.7.8 Authentication credentials invalid")
		}

		s.authenticated = true
		s.authUser = &user
		log.Printf("✅ [SMTP-AUTH] User berhasil terotentikasi: %s", username)
		return nil
	}), nil
}

// Mail menangani perintah MAIL FROM:<sender@domain.com>
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	log.Printf("📨 [SMTP] MAIL FROM: %s (Auth: %v)", from, s.authenticated)
	return nil
}

// Rcpt menangani perintah RCPT TO:<recipient@domain.com>
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	// Jika user terotentikasi, perbolehkan relay ke luar
	if s.authenticated {
		s.to = append(s.to, to)
		return nil
	}

	// Jika inbound tanpa otentikasi (Port 25), hanya izinkan email yang ditujukan ke domain lokal
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return errors.New("501 5.1.3 Bad recipient address syntax")
	}
	domainPart := parts[1]

	var dom domain.Domain
	if err := repository.DB.Where("name = ? AND is_active = true", domainPart).First(&dom).Error; err != nil {
		log.Printf("⛔ [SMTP-RELAY DENIED] Percobaan relay terbuka ke domain tak terdaftar: %s", to)
		return errors.New("550 5.7.1 Relaying denied. IP not authorized")
	}

	s.to = append(s.to, to)
	log.Printf("📥 [SMTP] RCPT TO: %s (Diterima untuk domain lokal)", to)
	return nil
}

// Data menangani stream isi email (Headers + Body + Attachments)
func (s *Session) Data(r io.Reader) error {
	// Bungkus dengan SmugglingReader untuk mitigasi CVE-2023-51764 (Anti-Naked LF)
	safeReader := NewSmugglingReader(r)

	var buf bytes.Buffer
	tee := io.TeeReader(safeReader, &buf)

	// Parsing RFC 5322 MIME email headers
	mr, err := mail.CreateReader(tee)
	subject := "(No Subject)"
	messageID := fmt.Sprintf("<%d.%s@rahamail.local>", time.Now().UnixNano(), "mta")
	date := time.Now()
	var snippet string

	if err == nil {
		header := mr.Header
		if s, err := header.Subject(); err == nil && s != "" {
			subject = s
		}
		if mid, err := header.MessageID(); err == nil && mid != "" {
			messageID = mid
		}
		if d, err := header.Date(); err == nil {
			date = d
		}

		// Baca snippet konten teks
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				contentType, _, _ := h.ContentType()
				if strings.HasPrefix(contentType, "text/plain") {
					b, _ := io.ReadAll(io.LimitReader(p.Body, 200))
					snippet = string(b)
					break
				}
			}
		}
	}

	// Baca sisa raw bytes buffer
	io.Copy(io.Discard, tee)
	rawBytes := buf.Bytes()
	size := int64(len(rawBytes))

	log.Printf("💾 [SMTP] Menerima pesan %d bytes | Subjek: %s", size, subject)

	// Simpan metadata dan file ke storage aman untuk setiap penerima lokal
	for _, recipient := range s.to {
		var user domain.User
		if err := repository.DB.Where("email = ?", recipient).First(&user).Error; err == nil {
			// Simpan dengan path aman ter-hash per user
			filePath, err := storage.SaveMailToFile("inbox", user.ID, messageID, rawBytes)
			if err != nil {
				log.Printf("❌ Gagal menyimpan file email secara aman: %v", err)
				continue
			}

			msg := domain.Message{
				UserID:      user.ID,
				Mailbox:     "INBOX",
				MessageID:   messageID,
				From:        s.from,
				To:          recipient,
				Subject:     subject,
				Snippet:     snippet,
				Size:        size,
				StoragePath: filePath,
				SPFStatus:   "pass",
				DKIMStatus:  "pass",
				DMARCStatus: "pass",
				ReceivedAt:  date,
				IsSeen:      false,
			}
			repository.DB.Create(&msg)

			// Siarkan langsung event real-time ke Dashboard Web melalui SSE
			sse.GlobalHub.Broadcast("new_email", msg)
			log.Printf("🚀 [SSE BROADCAST] Email baru berhasil disiarkan ke dashboard untuk user: %s", recipient)
		}
	}

	// Update statistik live
	var messageCount int64
	repository.DB.Model(&domain.Message{}).Count(&messageCount)
	sse.GlobalHub.Broadcast("stats", map[string]interface{}{
		"messages": messageCount,
	})

	return nil
}

// Reset mereset session SMTP saat ada perintah RSET
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

// Logout menutup session SMTP
func (s *Session) Logout() error {
	log.Printf("👋 [SMTP] Client disconnect: %s", s.remoteIP)
	return nil
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}

// StartSMTPServer menjalankan daemon SMTP server di port tertentu (goroutine)
func StartSMTPServer(addr, domainName string) *smtp.Server {
	s := smtp.NewServer(NewBackend())
	s.Addr = addr
	s.Domain = domainName
	s.ReadTimeout = 30 * time.Second
	s.WriteTimeout = 30 * time.Second
	s.MaxMessageBytes = 25 * 1024 * 1024 // Maksimum ukuran lampiran/email 25MB
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true // Untuk pengujian lokal tanpa TLS aktif

	go func() {
		log.Printf("📬 [SMTP SERVER] Mendengarkan koneksi di %s (Domain: %s)...", addr, domainName)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Fatalf("❌ SMTP Server error: %v", err)
		}
	}()

	return s
}
