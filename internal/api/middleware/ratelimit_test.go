package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestRateLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Buat limiter uji coba: burst 2, rate 1 per detik
	testLimiter := NewIPRateLimiter(rate.Every(1*time.Second), 2)
	r.GET("/test-limit", RateLimitMiddleware(testLimiter, "Rate limit reached"), func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Request 1: harus OK
	req1, _ := http.NewRequest("GET", "/test-limit", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w1.Code)
	}

	// Request 2: harus OK (burst 2)
	req2, _ := http.NewRequest("GET", "/test-limit", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w2.Code)
	}

	// Request 3: harus 429 Too Many Requests
	req3, _ := http.NewRequest("GET", "/test-limit", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w3.Code)
	}
}
