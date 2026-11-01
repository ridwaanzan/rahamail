package smtpserver

import (
	"errors"
	"testing"
)

func TestSASLLoginFlow(t *testing.T) {
	mockAuth := func(u, p string) error {
		if u == "user@domain.com" && p == "secret123" {
			return nil
		}
		return errors.New("invalid credentials")
	}

	t.Run("Standard 2-Step LOGIN Flow", func(t *testing.T) {
		s := NewLoginServer(mockAuth)

		// Step 0: Initial request (empty response)
		ch, done, err := s.Next(nil)
		if err != nil || done || string(ch) != "Username:" {
			t.Fatalf("Step 0 failed: ch=%s, done=%v, err=%v", string(ch), done, err)
		}

		// Step 1: Client sends username
		ch, done, err = s.Next([]byte("user@domain.com"))
		if err != nil || done || string(ch) != "Password:" {
			t.Fatalf("Step 1 failed: ch=%s, done=%v, err=%v", string(ch), done, err)
		}

		// Step 2: Client sends password
		ch, done, err = s.Next([]byte("secret123"))
		if err != nil || !done {
			t.Fatalf("Step 2 failed: done=%v, err=%v", done, err)
		}
	})

	t.Run("LOGIN with Initial Response", func(t *testing.T) {
		s := NewLoginServer(mockAuth)

		// Step 0 with initial username payload
		ch, done, err := s.Next([]byte("user@domain.com"))
		if err != nil || done || string(ch) != "Password:" {
			t.Fatalf("Step 0 (with payload) failed: ch=%s, done=%v, err=%v", string(ch), done, err)
		}

		// Step 2: Client sends password
		ch, done, err = s.Next([]byte("secret123"))
		if err != nil || !done {
			t.Fatalf("Step 2 failed: done=%v, err=%v", done, err)
		}
	})

	t.Run("LOGIN with Wrong Password", func(t *testing.T) {
		s := NewLoginServer(mockAuth)

		_, _, _ = s.Next(nil)
		_, _, _ = s.Next([]byte("user@domain.com"))
		_, done, err := s.Next([]byte("wrongpass"))
		if err == nil || !done {
			t.Fatalf("Expected error for wrong password, got err=%v, done=%v", err, done)
		}
	})
}
