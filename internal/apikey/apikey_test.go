package apikey

import (
	"strings"
	"testing"
)

func TestGenerateAndVerifyAPIKey(t *testing.T) {
	rawToken, prefix, tokenHash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	if !strings.HasPrefix(rawToken, "rhm_live_") {
		t.Fatalf("Expected token to start with rhm_live_, got %s", rawToken)
	}

	if !ValidateFormat(rawToken) {
		t.Fatalf("ValidateFormat failed for valid token: %s", rawToken)
	}

	if len(prefix) != 13 {
		t.Fatalf("Expected prefix of length 13, got %s (len: %d)", prefix, len(prefix))
	}

	if !VerifyToken(rawToken, tokenHash) {
		t.Fatalf("VerifyToken failed for valid raw token and hash")
	}

	// Token tidak valid
	if VerifyToken("rhm_live_invalidtoken123", tokenHash) {
		t.Fatalf("VerifyToken passed for invalid token")
	}

	if ValidateFormat("some_other_prefix_abcdef") {
		t.Fatalf("ValidateFormat passed for non-rhm_live prefix")
	}
}
