package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mail-server/internal/api/middleware"
	"mail-server/internal/domain"
	"mail-server/internal/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestPhase2Flow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	repository.InitDB()
	mailCtrl := NewMailController()

	mailAPI := r.Group("/api/v1/webmail", middleware.UserAuthMiddleware())
	{
		mailAPI.GET("/contacts", mailCtrl.ListContacts)
		mailAPI.POST("/contacts", mailCtrl.CreateContact)
		mailAPI.DELETE("/contacts/:id", mailCtrl.DeleteContact)

		mailAPI.GET("/auto-responder", mailCtrl.GetAutoResponder)
		mailAPI.POST("/auto-responder", mailCtrl.SaveAutoResponder)

		mailAPI.GET("/templates", mailCtrl.ListCannedResponses)
		mailAPI.POST("/templates", mailCtrl.CreateCannedResponse)
		mailAPI.DELETE("/templates/:id", mailCtrl.DeleteCannedResponse)
	}

	// 1. Buat user test
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	uniqueEmail := fmt.Sprintf("phase2-%d@example.com", time.Now().UnixNano())
	testUser := domain.User{
		DomainID:     1,
		Email:        uniqueEmail,
		PasswordHash: string(hashedPwd),
		IsActive:     true,
	}
	if err := repository.DB.Create(&testUser).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionCookie := http.Cookie{
		Name:  "rahamail_session",
		Value: middleware.GenerateSessionToken(testUser.ID),
	}

	// 2. Test Create Contact
	contactPayload, _ := json.Marshal(map[string]string{
		"name":    "Alice Smith",
		"email":   "alice@example.com",
		"company": "Acme Corp",
		"notes":   "VIP Client",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webmail/contacts", bytes.NewBuffer(contactPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for create contact, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Test List Contacts with search
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/webmail/contacts?q=Alice", nil)
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for list contacts, got %d", w.Code)
	}
	var contacts []domain.Contact
	json.Unmarshal(w.Body.Bytes(), &contacts)
	if len(contacts) == 0 || contacts[0].Email != "alice@example.com" {
		t.Fatalf("Expected contact alice@example.com, got %v", contacts)
	}

	// 4. Test Save Auto-Responder (Vacation Notice)
	now := time.Now()
	later := now.Add(48 * time.Hour)
	arPayload, _ := json.Marshal(map[string]interface{}{
		"enabled":    true,
		"subject":    "Vacation Out of Office",
		"body_text":  "I am currently traveling.",
		"start_time": now,
		"end_time":   later,
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/webmail/auto-responder", bytes.NewBuffer(arPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for save auto responder, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Test Get Auto-Responder
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/webmail/auto-responder", nil)
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for get auto responder, got %d", w.Code)
	}
	var ar domain.AutoResponder
	json.Unmarshal(w.Body.Bytes(), &ar)
	if !ar.Enabled || ar.Subject != "Vacation Out of Office" {
		t.Fatalf("Expected active auto responder, got %v", ar)
	}

	// 6. Test Create Canned Response Template
	tplPayload, _ := json.Marshal(map[string]string{
		"title":   "Order Confirmation",
		"content": "Thank you for your order! It is being processed.",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/webmail/templates", bytes.NewBuffer(tplPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for create template, got %d: %s", w.Code, w.Body.String())
	}

	// 7. Test List Canned Templates
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/webmail/templates", nil)
	req.AddCookie(&sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for list templates, got %d", w.Code)
	}
	var tpls []domain.CannedResponse
	json.Unmarshal(w.Body.Bytes(), &tpls)
	if len(tpls) == 0 || tpls[0].Title != "Order Confirmation" {
		t.Fatalf("Expected template Order Confirmation, got %v", tpls)
	}
}
