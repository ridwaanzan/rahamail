package storage

import (
	"strings"
	"testing"
)

func TestStorageSecurity(t *testing.T) {
	// 1. Test IsPathSafe
	if !IsPathSafe("var/mail/inbox/1/sample.eml") {
		t.Errorf("Expected valid path to be safe")
	}

	// 2. Test Directory Traversal Detection
	evilPaths := []string{
		"../../etc/passwd",
		"var/mail/inbox/../../etc/shadow",
		"var/mail/inbox/1/../../../secret.key",
	}

	for _, p := range evilPaths {
		if IsPathSafe(p) {
			t.Errorf("Expected evil path %q to be rejected", p)
		}
	}

	// 3. Test GenerateSecurePath
	path, err := GenerateSecurePath("inbox", 5, "<test.msg.123@domain.com>")
	if err != nil {
		t.Fatalf("GenerateSecurePath failed: %v", err)
	}

	if !strings.HasPrefix(path, "var/mail/inbox/5/") {
		t.Errorf("Expected path to start with var/mail/inbox/5/, got %s", path)
	}
	if !strings.HasSuffix(path, ".eml") {
		t.Errorf("Expected path to end with .eml, got %s", path)
	}
}
