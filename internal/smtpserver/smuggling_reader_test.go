package smtpserver

import (
	"bytes"
	"io"
	"testing"
)

func TestSmugglingReader(t *testing.T) {
	// 1. Valid RFC 5321 stream (All LFs are preceded by CR)
	validData := []byte("Subject: Test\r\nFrom: user@domain.com\r\n\r\nHello World\r\n")
	sr := NewSmugglingReader(bytes.NewReader(validData))
	out, err := io.ReadAll(sr)
	if err != nil {
		t.Fatalf("Expected valid stream to pass, got err: %v", err)
	}
	if !bytes.Equal(out, validData) {
		t.Errorf("Expected output to match valid input")
	}

	// 2. Malicious stream with Naked LF (\n without \r)
	evilData := []byte("Subject: Test\r\nFrom: user@domain.com\n\nNaked LF attack\n")
	srEvil := NewSmugglingReader(bytes.NewReader(evilData))
	_, err = io.ReadAll(srEvil)
	if err != ErrNakedLFDetected {
		t.Errorf("Expected ErrNakedLFDetected, got: %v", err)
	}
}
