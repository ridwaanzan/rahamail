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

	// Verify admin_simulations renders cleanly with presets
	simBuf := new(bytes.Buffer)
	err = engine.Render(simBuf, "admin_simulations", map[string]any{
		"title":     "Test Simulations",
		"db_driver": "sqlite3",
		"presets": []map[string]any{
			{
				"Title":        "IT Password Reset",
				"SenderName":   "IT Admin",
				"SenderEmail":  "it@fake.xyz",
				"Subject":      "Reset Pass",
				"TemplateHTML": "<p>Hello</p>",
			},
		},
	})
	if err != nil {
		t.Fatalf("Render admin_simulations failed: %v", err)
	}
	if simBuf.Len() == 0 {
		t.Fatalf("Rendered admin_simulations buffer is empty")
	}
}

func BenchmarkRHUIEngineRender(b *testing.B) {
	engine, err := GetRHUIEngine()
	if err != nil {
		b.Fatalf("GetRHUIEngine failed: %v", err)
	}

	data := map[string]any{
		"title":     "Benchmark Domains",
		"db_driver": "sqlite3",
		"domains": []map[string]any{
			{"Name": "alpha.example.com", "DKIMSelector": "mail", "DKIMPublicKey": "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ=="},
			{"Name": "beta.example.com", "DKIMSelector": "mail", "DKIMPublicKey": "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ=="},
			{"Name": "gamma.example.com", "DKIMSelector": "mail", "DKIMPublicKey": "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ=="},
		},
	}

	buf := new(bytes.Buffer)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = engine.Render(buf, "admin_domains", data)
	}
}
