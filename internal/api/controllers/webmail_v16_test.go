package controllers

import (
	"bytes"
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

func TestConversationThreadingAndQuickReply(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository.InitDB()

	// Inisialisasi RHUI Engine
	rhuiEngine, err := web.GetRHUIEngine()
	if err != nil {
		t.Fatalf("Gagal memuat RHUI Engine: %v", err)
	}
	web.SetGlobalRHUIEngine(rhuiEngine)

	// Pastikan Domain dan Users terbuat
	var dom domain.Domain
	if err := repository.DB.Where("name = ?", "thread.local").First(&dom).Error; err != nil {
		dom = domain.Domain{
			Name:     "thread.local",
			IsActive: true,
		}
		repository.DB.Create(&dom)
	}

	var alice domain.User
	if err := repository.DB.Where("email = ?", "alice@thread.local").First(&alice).Error; err != nil {
		alice = domain.User{
			DomainID:   dom.ID,
			Email:      "alice@thread.local",
			QuotaBytes: 1024 * 1024 * 100,
			IsActive:   true,
		}
		repository.DB.Create(&alice)
	}

	var bob domain.User
	if err := repository.DB.Where("email = ?", "bob@thread.local").First(&bob).Error; err != nil {
		bob = domain.User{
			DomainID:   dom.ID,
			Email:      "bob@thread.local",
			QuotaBytes: 1024 * 1024 * 100,
			IsActive:   true,
		}
		repository.DB.Create(&bob)
	}

	// 1. Buat Email Awal (Root Message) dari Bob ke Alice
	rootMsgID := "<root-discussion-123@thread.local>"
	rootMsg := domain.Message{
		UserID:      alice.ID,
		Mailbox:     "INBOX",
		MessageID:   rootMsgID,
		ThreadID:    rootMsgID,
		From:        bob.Email,
		To:          alice.Email,
		Subject:     "Product Roadmap 2027 Discussion",
		Snippet:     "Hi Alice, here are the strategic proposals for 2027 roadmap.",
		ReceivedAt:  time.Now().Add(-2 * time.Hour),
		StoragePath: "",
	}
	repository.DB.Create(&rootMsg)

	// 2. Buat Balasan 1 (Reply) dari Alice ke Bob
	reply1ID := "<reply-1-456@thread.local>"
	reply1 := domain.Message{
		UserID:      alice.ID,
		Mailbox:     "INBOX",
		MessageID:   reply1ID,
		InReplyTo:   rootMsgID,
		References:  rootMsgID,
		ThreadID:    rootMsgID,
		From:        alice.Email,
		To:          bob.Email,
		Subject:     "Re: Product Roadmap 2027 Discussion",
		Snippet:     "Thanks Bob, looks very comprehensive. Let's focus on RHUI SSR.",
		ReceivedAt:  time.Now().Add(-1 * time.Hour),
		StoragePath: "",
	}
	repository.DB.Create(&reply1)

	// 3. Buat Balasan 2 (Reply) dari Bob ke Alice
	reply2ID := "<reply-2-789@thread.local>"
	reply2 := domain.Message{
		UserID:      alice.ID,
		Mailbox:     "INBOX",
		MessageID:   reply2ID,
		InReplyTo:   reply1ID,
		References:  fmt.Sprintf("%s %s", rootMsgID, reply1ID),
		ThreadID:    rootMsgID,
		From:        bob.Email,
		To:          alice.Email,
		Subject:     "Re: Product Roadmap 2027 Discussion",
		Snippet:     "Agreed! Pure Go and zero Node.js runtime makes it blazingly fast.",
		ReceivedAt:  time.Now(),
		StoragePath: "",
	}
	repository.DB.Create(&reply2)

	mailCtrl := NewMailController()
	r := gin.New()
	r.HTMLRender = rhuigin.New(rhuiEngine)

	r.Use(func(c *gin.Context) {
		c.Set("user", &alice)
		c.Next()
	})

	r.GET("/api/v1/webmail/messages/:id", mailCtrl.GetMessageDetail)
	r.POST("/api/v1/webmail/send", mailCtrl.ComposeAndSendEmail)

	// Test 1: Verifikasi GetMessageDetail menyertakan conversation thread aggregation
	t.Run("ThreadAggregation_JSON", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/webmail/messages/%d", rootMsg.ID), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var res struct {
			ThreadCount    int              `json:"thread_count"`
			ThreadMessages []domain.Message `json:"thread_messages"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("Failed to parse json response: %v", err)
		}

		if res.ThreadCount < 3 {
			t.Fatalf("Expected thread_count >= 3, got %d", res.ThreadCount)
		}
	})

	// Test 2: Verifikasi rendering Fragment Thread Tree SSR via RHUI
	t.Run("ThreadTree_HTML_Fragment", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/webmail/messages/%d?format=fragment", rootMsg.ID), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		body := w.Body.String()
		if len(body) == 0 {
			t.Fatalf("Expected non-empty HTML fragment")
		}
		if !contains(body, "Riwayat Percakapan") {
			t.Errorf("Expected fragment to contain 'Riwayat Percakapan'")
		}
		if !contains(body, "Pure Go and zero Node.js runtime") {
			t.Errorf("Expected fragment to contain reply snippet")
		}
	})

	// Test 3: Verifikasi Quick Reply mengirimkan header In-Reply-To dan References yang tepat
	t.Run("QuickReply_SendWithThreadHeaders", func(t *testing.T) {
		payload := map[string]interface{}{
			"to":          bob.Email,
			"subject":     "Re: Product Roadmap 2027 Discussion",
			"body_text":   "Awesome, ready for production rollout!",
			"in_reply_to": reply2ID,
			"references":  fmt.Sprintf("%s %s %s", rootMsgID, reply1ID, reply2ID),
			"thread_id":   rootMsgID,
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "/api/v1/webmail/send", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verifikasi pesan tersimpan di Sent dengan ThreadID yang sesuai
		var latestSent domain.Message
		err := repository.DB.Where("user_id = ? AND in_reply_to = ?", alice.ID, reply2ID).Order("id desc").First(&latestSent).Error
		if err != nil {
			t.Fatalf("Expected quick reply message saved with InReplyTo: %v", err)
		}
		if latestSent.ThreadID != rootMsgID {
			t.Errorf("Expected ThreadID %s, got %s", rootMsgID, latestSent.ThreadID)
		}
	})
}
