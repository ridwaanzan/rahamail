package web

import (
	"bytes"
	"testing"
)

func TestRHUIEngineInitAndRender(t *testing.T) {
	engine, err := GetRHUIEngine()
	if err != nil {
		t.Fatalf("GetRHUIEngine failed: %v", err)
	}

	// Verify login renders cleanly
	loginBuf := new(bytes.Buffer)
	err = engine.Render(loginBuf, "login", map[string]any{
		"title": "Masuk ke RaHaMail",
		"next":  "/mail",
	})
	if err != nil {
		t.Fatalf("Render login failed: %v", err)
	}
	if loginBuf.Len() == 0 {
		t.Fatalf("Rendered login buffer is empty")
	}

	// Verify admin_domains renders cleanly with mock data
	domainsBuf := new(bytes.Buffer)
	err = engine.Render(domainsBuf, "admin_domains", map[string]any{
		"title":     "Test Domains",
		"db_driver": "sqlite3",
		"domains": []map[string]any{
			{
				"Name":          "example.com",
				"DKIMSelector":  "mail",
				"DKIMPublicKey": "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ==",
			},
		},
	})
	if err != nil {
		t.Fatalf("Render admin_domains failed: %v", err)
	}
	t.Logf("admin_domains rendered %d bytes:\n%s", domainsBuf.Len(), domainsBuf.String()[:1000])
}
