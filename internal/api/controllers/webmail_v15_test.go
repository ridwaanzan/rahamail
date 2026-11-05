package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mail-server/internal/domain"
	"mail-server/internal/repository"
	"mail-server/web"

	"github.com/gin-gonic/gin"
	rhuigin "github.com/suserc0de/rhui/adapter/gin"
)

func TestWebmailServerSearchAndFragmentPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository.InitDB()

	// Pastikan RHUI Engine terinisialisasi
	rhuiEngine, err := web.GetRHUIEngine()
	if err != nil {
		t.Fatalf("Gagal memuat RHUI Engine: %v", err)
	}
	web.SetGlobalRHUIEngine(rhuiEngine)

	// Pastikan domain terbuat untuk foreign key constraint User.DomainID
	var dom domain.Domain
	if err := repository.DB.Where("name = ?", "example.com").First(&dom).Error; err != nil {
		dom = domain.Domain{
			Name:     "example.com",
			IsActive: true,
		}
		repository.DB.Create(&dom)
	}

	// Buat mock user & messages untuk testing
	var user domain.User
	if err := repository.DB.Where("email = ?", "search_test_user@example.com").First(&user).Error; err != nil {
		user = domain.User{
			DomainID:   dom.ID,
			Email:      "search_test_user@example.com",
			QuotaBytes: 1024 * 1024 * 100,
			IsActive:   true,
		}
		repository.DB.Create(&user)
	}

	// Buat sample message
	msg1 := domain.Message{
		UserID:      user.ID,
		Mailbox:     "INBOX",
		From:        "ceo@company.com",
		To:          user.Email,
		Subject:     "Project Alpha Q4 Strategic Review",
		Snippet:     "Here are the meeting notes for Project Alpha Q4 roadmap.",
		ReceivedAt:  time.Now(),
		StoragePath: "data/mock_alpha.eml",
	}
	msg2 := domain.Message{
		UserID:      user.ID,
		Mailbox:     "INBOX",
		From:        "newsletter@techwire.org",
		To:          user.Email,
		Subject:     "Weekly Go & Cloud Security Digest",
		Snippet:     "Top stories this week: pure-Go mail servers and memory safety.",
		ReceivedAt:  time.Now(),
		StoragePath: "data/mock_newsletter.eml",
	}
	repository.DB.Create(&msg1)
	repository.DB.Create(&msg2)

	mailCtrl := NewMailController()
	r := gin.New()
	r.HTMLRender = rhuigin.New(rhuiEngine)

	// Mock auth context
	r.Use(func(c *gin.Context) {
		c.Set("user", &user)
		c.Next()
	})

	r.GET("/mail", mailCtrl.RenderWebmail)
	r.GET("/api/v1/webmail/messages/fragment", mailCtrl.GetMessagesFragment)

	// 1. Test Server-side search via GET /api/v1/webmail/messages/fragment with query ?q=Alpha&format=json
	t.Run("ServerSearch_JSON_FilterQuery", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/webmail/messages/fragment?folder=INBOX&q=Alpha&format=json", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var res struct {
			TotalItems int              `json:"total_items"`
			Messages   []domain.Message `json:"messages"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("Failed to parse json response: %v", err)
		}

		if res.TotalItems < 1 {
			t.Fatalf("Expected at least 1 match for query 'Alpha', got %d", res.TotalItems)
		}
		if res.Messages[0].Subject != "Project Alpha Q4 Strategic Review" {
			t.Errorf("Expected subject 'Project Alpha Q4 Strategic Review', got '%s'", res.Messages[0].Subject)
		}
	})

	// 2. Test Server-side fragment HTML rendering for HTMX/DOM replacement
	t.Run("ServerSearch_HTML_Fragment", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/webmail/messages/fragment?folder=INBOX&q=Weekly", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		body := w.Body.String()
		if len(body) == 0 {
			t.Errorf("Expected non-empty HTML fragment response")
		}
		// Periksa konten yang di-escape oleh engine HTML (& -> &amp;)
		if !contains(body, "Weekly Go &amp; Cloud Security Digest") && !contains(body, "Weekly Go & Cloud Security Digest") {
			t.Errorf("Expected fragment HTML to contain subject 'Weekly Go & Cloud Security Digest', got: %s", body)
		}
	})

	// 3. Test Full Webmail render with search query parameter ?q=Alpha
	t.Run("RenderWebmail_With_SearchQuery", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/mail?folder=INBOX&q=Alpha", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
		body := w.Body.String()
		if !contains(body, "Project Alpha Q4 Strategic Review") {
			t.Errorf("Expected rendered webmail to contain filtered item 'Project Alpha Q4 Strategic Review'")
		}
	})
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 || (len(str) > 0 && len(substr) > 0 && stringContains(str, substr)))
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && fmt.Sprintf("%v", s) != "" && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
