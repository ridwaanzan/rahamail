package trust

import (
	"strings"
	"testing"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
)

func TestExtractEmailAddress(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"John Doe <john@example.com>", "john@example.com"},
		{"<admin@domain.org>", "admin@domain.org"},
		{"plain@user.net", "plain@user.net"},
		{"  SUPPORT <Support@Service.Co.Id> ", "support@service.co.id"},
	}

	for _, c := range cases {
		out := ExtractEmailAddress(c.input)
		if out != c.expected {
			t.Fatalf("ExtractEmailAddress(%q) = %q, expected %q", c.input, out, c.expected)
		}
	}
}

func TestContainsURLs(t *testing.T) {
	if !ContainsURLs("Klik disini: https://malicious-site.com/login untuk klaim") {
		t.Fatalf("Expected ContainsURLs to find https URL")
	}
	if !ContainsURLs("Buka http://example.com/file") {
		t.Fatalf("Expected ContainsURLs to find http URL")
	}
	if ContainsURLs("Email biasa tanpa link apapun") {
		t.Fatalf("Expected ContainsURLs to return false for text without link")
	}
}

func TestEvaluateSender_ReputationLevels(t *testing.T) {
	repository.InitDB()

	// 1. Uji kegagalan DMARC -> level danger
	alert := EvaluateSender(1, "spoofed@foreign.com", "Hello", "Hello", 0, "fail", "fail", "fail")
	if alert.Level != "danger" || !strings.Contains(alert.BadgeTitle, "Spoofing") {
		t.Fatalf("Expected danger level for failed DMARC, got %v", alert)
	}

	// 2. Uji domain lokal -> level none
	var dom domain.Domain
	repository.DB.FirstOrCreate(&dom, domain.Domain{Name: "example.com", IsActive: true})
	alertLocal := EvaluateSender(1, "colleague@example.com", "Internal memo", "Internal memo", 0, "pass", "pass", "pass")
	if alertLocal.Level != "none" || alertLocal.IsExternalDomain {
		t.Fatalf("Expected level none for local domain, got %v", alertLocal)
	}

	// 3. Uji domain eksternal baru dengan URL -> level warning
	alertWarning := EvaluateSender(9999, "stranger@unknown-vendor.xyz", "<a href='https://vendor.xyz/pay'>Bayar</a>", "", 0, "pass", "pass", "pass")
	if alertWarning.Level != "warning" || !alertWarning.IsFirstTimeSender || !alertWarning.HasExternalLinks {
		t.Fatalf("Expected warning level for new external sender with links, got %v", alertWarning)
	}

	// 4. Uji domain eksternal baru dengan Attachment -> level warning
	alertAtt := EvaluateSender(9999, "newuser@external.com", "Ini lampirannya", "", 1, "pass", "pass", "pass")
	if alertAtt.Level != "warning" || !alertAtt.HasAttachments {
		t.Fatalf("Expected warning level for external sender with attachments, got %v", alertAtt)
	}

	// 5. Uji domain eksternal baru teks polos -> level info
	alertInfo := EvaluateSender(9999, "simple@external.com", "Halo selamat pagi", "Halo selamat pagi", 0, "pass", "pass", "pass")
	if alertInfo.Level != "info" || !alertInfo.IsFirstTimeSender {
		t.Fatalf("Expected info level for plain text new sender, got %v", alertInfo)
	}
}
