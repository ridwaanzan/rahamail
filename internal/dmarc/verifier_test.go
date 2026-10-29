package dmarc

import (
	"strings"
	"testing"
)

func TestVerifyInboundAuth(t *testing.T) {
	raw := []byte("From: sender@example.com\r\nTo: dest@example.com\r\nSubject: Test\r\n\r\nMessage body")
	results := VerifyInboundAuth("127.0.0.1", "sender@example.com", raw)

	if results.SPFStatus != "pass" {
		t.Fatalf("Expected loopback IP to have SPF pass, got %s", results.SPFStatus)
	}

	if !strings.Contains(results.HeaderString, "Authentication-Results: rahamail") {
		t.Fatalf("Expected Authentication-Results header, got %s", results.HeaderString)
	}
}
