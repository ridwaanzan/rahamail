package totp

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAndValidateTOTP(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	if len(secret) < 16 {
		t.Fatalf("Secret length too short: %s", secret)
	}

	now := time.Now()
	code, err := GenerateCode(secret, now)
	if err != nil {
		t.Fatalf("Failed to generate code: %v", err)
	}

	if len(code) != 6 {
		t.Fatalf("Expected 6 digits code, got %s", code)
	}

	// Validasi kode sekarang harus valid
	if !ValidateCode(secret, code, now) {
		t.Fatalf("Expected code %s to be valid for secret %s", code, secret)
	}

	// Validasi dalam rentang skew 25 detik ke depan (harus tetap valid karena skew +/- 1 step)
	if !ValidateCode(secret, code, now.Add(25*time.Second)) {
		t.Fatalf("Expected code %s to be valid within skew", code)
	}

	// Validasi kode salah
	if ValidateCode(secret, "999999", now) && code != "999999" {
		t.Fatalf("Invalid code was accepted")
	}

	// Validasi format URI
	uri := GenerateKeyURI("RaHaMail", "user@example.com", secret)
	if !strings.HasPrefix(uri, "otpauth://totp/RaHaMail:user@example.com?") && !strings.HasPrefix(uri, "otpauth://totp/RaHaMail%3Auser") {
		t.Fatalf("Unexpected otpauth URI: %s", uri)
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("Failed to generate backup codes: %v", err)
	}
	if len(codes) != 8 {
		t.Fatalf("Expected 8 backup codes, got %d", len(codes))
	}
	for _, c := range codes {
		if len(c) != 9 || !strings.Contains(c, "-") {
			t.Fatalf("Unexpected backup code format: %s", c)
		}
	}
}
