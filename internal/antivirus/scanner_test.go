package antivirus

import (
	"testing"
)

func TestGracefulBypassWhenScannerOffline(t *testing.T) {
	sampleEmail := []byte("From: user@example.com\r\nTo: dest@example.com\r\nSubject: Test\r\n\r\nHello World")

	res := ScanIncomingMessage(sampleEmail)
	if res.IsInfected {
		t.Fatalf("Expected offline ClamAV to bypass, got infected")
	}
	if res.IsSpam {
		t.Fatalf("Expected offline Rspamd to bypass, got isSpam")
	}
	if res.Action != "no_action" {
		t.Fatalf("Expected no_action, got %s", res.Action)
	}
}
