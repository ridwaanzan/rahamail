package smtpserver

import (
	"crypto/tls"
	"net"
	"net/smtp"
	"testing"
	"time"

	"mail-server/internal/repository"
	"mail-server/internal/tlsconfig"
	gosmtp "github.com/emersion/go-smtp"
)

func TestSMTPSTARTTLSAndSubmission(t *testing.T) {
	repository.InitDB()

	tlsConfig, err := tlsconfig.LoadOrGenerateTLSConfig("mail.testdomain.com")
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	addr := l.Addr().String()

	s := gosmtp.NewServer(NewBackend())
	s.Addr = addr
	s.Domain = "mail.testdomain.com"
	s.TLSConfig = tlsConfig
	s.AllowInsecureAuth = true

	go func() {
		_ = s.Serve(l)
	}()
	defer s.Close()

	time.Sleep(100 * time.Millisecond)

	t.Run("SMTP STARTTLS Handshake", func(t *testing.T) {
		c, err := smtp.Dial(addr)
		if err != nil {
			t.Fatalf("Dial failed: %v", err)
		}
		defer c.Close()

		if ok, _ := c.Extension("STARTTLS"); !ok {
			t.Fatalf("Expected STARTTLS extension advertised")
		}

		clientTLSConfig := &tls.Config{InsecureSkipVerify: true}
		if err := c.StartTLS(clientTLSConfig); err != nil {
			t.Fatalf("StartTLS handshake failed: %v", err)
		}
	})
}
