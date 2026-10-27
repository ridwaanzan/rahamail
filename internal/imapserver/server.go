package imapserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/server"
	"golang.org/x/crypto/bcrypt"
)

// IMAPBackend mengimplementasikan backend.Backend untuk otentikasi login user IMAP
type IMAPBackend struct{}

func NewIMAPBackend() *IMAPBackend {
	return &IMAPBackend{}
}

// Login memvalidasi kredensial pengguna yang masuk melalui klien mail (Thunderbird/Outlook)
func (b *IMAPBackend) Login(conn *imap.ConnInfo, username, password string) (backend.User, error) {
	var user domain.User
	if err := repository.DB.Where("email = ? AND is_active = true", username).First(&user).Error; err != nil {
		log.Printf("⚠️ [IMAP-AUTH] Akun tidak ditemukan: %s", username)
		return nil, backend.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Printf("⚠️ [IMAP-AUTH] Password salah untuk akun: %s", username)
		return nil, backend.ErrInvalidCredentials
	}

	log.Printf("📥 [IMAP-AUTH] Pengguna berhasil login ke IMAP: %s", username)
	return &IMAPUser{user: user}, nil
}

// IMAPUser mengimplementasikan backend.User
type IMAPUser struct {
	user domain.User
}

func (u *IMAPUser) Username() string {
	return u.user.Email
}

// ListMailboxes mengembalikan daftar folder kotak surat (INBOX, Sent, Trash)
func (u *IMAPUser) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	return []backend.Mailbox{
		&IMAPMailbox{user: u.user, name: "INBOX"},
		&IMAPMailbox{user: u.user, name: "Sent"},
		&IMAPMailbox{user: u.user, name: "Trash"},
	}, nil
}

// GetMailbox mengambil instance folder kotak surat tertentu
func (u *IMAPUser) GetMailbox(name string) (backend.Mailbox, error) {
	return &IMAPMailbox{user: u.user, name: name}, nil
}

func (u *IMAPUser) CreateMailbox(name string) error {
	return nil
}

func (u *IMAPUser) DeleteMailbox(name string) error {
	return nil
}

func (u *IMAPUser) RenameMailbox(existingName, newName string) error {
	return nil
}

func (u *IMAPUser) Logout() error {
	log.Printf("👋 [IMAP] Pengguna logout: %s", u.user.Email)
	return nil
}

// IMAPMailbox mengimplementasikan backend.Mailbox
type IMAPMailbox struct {
	user domain.User
	name string
}

func (m *IMAPMailbox) Name() string {
	return m.name
}

// Info mengembalikan status jumlah pesan dan atribut mailbox
func (m *IMAPMailbox) Info() (*imap.MailboxInfo, error) {
	info := &imap.MailboxInfo{
		Attributes: []string{imap.NoInferiorsAttr},
		Delimiter:  "/",
		Name:       m.name,
	}
	return info, nil
}

// Status mengembalikan metadata dinamis (Messages count, Unseen, UIDNext)
func (m *IMAPMailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := imap.NewMailboxStatus(m.name, items)

	var count int64
	repository.DB.Model(&domain.Message{}).Where("user_id = ? AND mailbox = ?", m.user.ID, m.name).Count(&count)
	status.Messages = uint32(count)
	status.UidNext = uint32(count + 1)
	status.UidValidity = 1

	var unseen int64
	repository.DB.Model(&domain.Message{}).Where("user_id = ? AND mailbox = ? AND is_seen = false", m.user.ID, m.name).Count(&unseen)
	status.Unseen = uint32(unseen)

	return status, nil
}

func (m *IMAPMailbox) SetSubscribed(subscribed bool) error {
	return nil
}

func (m *IMAPMailbox) Check() error {
	return nil
}

// ListMessages mengambil daftar pesan dalam mailbox
func (m *IMAPMailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	var messages []domain.Message
	repository.DB.Where("user_id = ? AND mailbox = ?", m.user.ID, m.name).Order("id asc").Find(&messages)

	for i, msg := range messages {
		seq := uint32(i + 1)
		id := uint32(msg.ID)
		if uid {
			if !seqSet.Contains(id) {
				continue
			}
		} else {
			if !seqSet.Contains(seq) {
				continue
			}
		}

		mObj := imap.NewMessage(seq, items)
		mObj.Uid = id
		mObj.Envelope = &imap.Envelope{
			Date:    msg.CreatedAt,
			Subject: msg.Subject,
			From:    []*imap.Address{{PersonalName: msg.From, MailboxName: msg.From, HostName: ""}},
			To:      []*imap.Address{{PersonalName: msg.To, MailboxName: msg.To, HostName: ""}},
		}

		// Baca raw body file jika ada request fetch body
		for _, item := range items {
			if item == imap.FetchItem("BODY[]") || item == imap.FetchItem("RFC822") {
				var rawContent []byte
				if msg.StoragePath != "" {
					rawContent, _ = os.ReadFile(msg.StoragePath)
				}
				if len(rawContent) == 0 {
					rawContent = []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\n\r\n%s",
						msg.From, msg.To, msg.Subject, msg.CreatedAt.Format(time.RFC1123Z), msg.Snippet))
				}
				section, _ := imap.ParseBodySectionName(item)
				mObj.Body[section] = bytes.NewReader(rawContent)
			}
		}

		ch <- mObj
	}
	return nil
}

func (m *IMAPMailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	return nil, nil
}

func (m *IMAPMailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	msg := domain.Message{
		UserID:    m.user.ID,
		Mailbox:   m.name,
		Subject:   "Uploaded via IMAP",
		Snippet:   string(b),
		IsSeen:    true,
		CreatedAt: date,
	}
	return repository.DB.Create(&msg).Error
}

func (m *IMAPMailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	return nil
}

func (m *IMAPMailbox) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {
	return nil
}

func (m *IMAPMailbox) Expunge() error {
	return nil
}

// StartIMAPServer menjalankan IMAP daemon di port tertentu
func StartIMAPServer(addr string) *server.Server {
	be := NewIMAPBackend()
	s := server.New(be)
	s.Addr = addr
	s.AllowInsecureAuth = true // Diizinkan tanpa TLS untuk pengujian port 1143/143 lokal

	go func() {
		log.Printf("📬 [IMAP SERVER] Mendengarkan koneksi di %s...", addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("❌ IMAP Server error: %v", err)
		}
	}()

	return s
}
