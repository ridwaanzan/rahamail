package tlsconfig

import (
	"crypto/tls"
	"testing"
)

func TestLoadOrGenerateTLSConfig(t *testing.T) {
	cfg, err := LoadOrGenerateTLSConfig("mail.testdomain.com")
	if err != nil {
		t.Fatalf("LoadOrGenerateTLSConfig failed: %v", err)
	}

	if cfg == nil || len(cfg.Certificates) == 0 {
		t.Fatalf("Expected valid tls.Config with at least 1 certificate")
	}

	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("Expected MinVersion TLS 1.2, got %v", cfg.MinVersion)
	}
}
