package imapserver

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"mail-server/internal/repository"
	"mail-server/internal/tlsconfig"

	"github.com/emersion/go-imap/client"
	imapserver "github.com/emersion/go-imap/server"
)

func TestIMAPStartTLSAndSSL(t *testing.T) {
	repository.InitDB()

	tlsConfig, err := tlsconfig.LoadOrGenerateTLSConfig("mail.testdomain.com")
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	// Listen langsung pada port :0
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Cannot listen l1: %v", err)
	}
	imapAddr := l1.Addr().String()

	l2, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Cannot listen l2: %v", err)
	}
	imapsAddr := l2.Addr().String()

	be := NewIMAPBackend()
	s1 := imapserver.New(be)
	s1.Addr = imapAddr
	s1.TLSConfig = tlsConfig
	go func() {
		_ = s1.Serve(l1)
	}()
	defer s1.Close()

	s2 := imapserver.New(be)
	s2.Addr = imapsAddr
	s2.TLSConfig = tlsConfig
	go func() {
		_ = s2.Serve(l2)
	}()
	defer s2.Close()

	time.Sleep(100 * time.Millisecond)

	// 1. Uji koneksi Plaintext & STARTTLS ke Port IMAP
	t.Run("IMAP STARTTLS Upgrade", func(t *testing.T) {
		c, err := client.Dial(imapAddr)
		if err != nil {
			t.Fatalf("Dial IMAP failed: %v", err)
		}
		defer c.Logout()

		clientTLSConfig := &tls.Config{InsecureSkipVerify: true}
		if err := c.StartTLS(clientTLSConfig); err != nil {
			t.Fatalf("StartTLS on IMAP failed: %v", err)
		}
	})

	// 2. Uji koneksi Direct SSL/TLS ke Port IMAPS
	t.Run("IMAPS Direct TLS Dial", func(t *testing.T) {
		clientTLSConfig := &tls.Config{InsecureSkipVerify: true}
		c, err := client.DialTLS(imapsAddr, clientTLSConfig)
		if err != nil {
			t.Fatalf("Dial IMAPS TLS failed: %v", err)
		}
		defer c.Logout()
	})
}
