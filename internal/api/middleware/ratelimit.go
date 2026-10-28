package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// clientLimiter mencatat rate limiter dan waktu aktivitas terakhir per IP / key
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter mengelola token bucket rate limiters per client identifier (IP / Email)
type IPRateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*clientLimiter
	r       rate.Limit
	b       int
}

// NewIPRateLimiter membuat rate limiter baru
// r: frekuensi penambahan token per detik (misal: rate.Every(time.Minute / 5))
// b: kapasitas maksimum burst token
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients: make(map[string]*clientLimiter),
		r:       r,
		b:       b,
	}

	// Goroutine background untuk membersihkan entri client yang sudah tidak aktif (> 1 jam)
	go limiter.cleanupStaleClients()

	return limiter
}

func (i *IPRateLimiter) getLimiter(key string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	client, exists := i.clients[key]
	if !exists {
		client = &clientLimiter{
			limiter:  rate.NewLimiter(i.r, i.b),
			lastSeen: time.Now(),
		}
		i.clients[key] = client
		return client.limiter
	}

	client.lastSeen = time.Now()
	return client.limiter
}

func (i *IPRateLimiter) cleanupStaleClients() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		i.mu.Lock()
		for key, client := range i.clients {
			if time.Since(client.lastSeen) > 1*time.Hour {
				delete(i.clients, key)
			}
		}
		i.mu.Unlock()
	}
}

// Global Limiters:
var (
	// AuthLimiter: Maksimal 5 percobaan per menit (burst 5) per IP
	AuthLimiter = NewIPRateLimiter(rate.Every(12*time.Second), 5)

	// RegisterLimiter: Maksimal 3 registrasi per jam (burst 3) per IP
	RegisterLimiter = NewIPRateLimiter(rate.Every(20*time.Minute), 3)

	// OutboundSendLimiter: Maksimal 30 pengiriman per menit (burst 10) per user
	OutboundSendLimiter = NewIPRateLimiter(rate.Every(2*time.Second), 10)
)

// RateLimitMiddleware membuat Gin middleware untuk rate limiting berbasis IP
func RateLimitMiddleware(limiter *IPRateLimiter, errorMsg string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		l := limiter.getLimiter(ip)

		if !l.Allow() {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   errorMsg,
				"status":  429,
				"message": "Terlalu banyak permintaan. Silakan tunggu beberapa saat.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserRateLimitMiddleware membatasi aksi pengiriman berdasarkan User ID
func UserRateLimitMiddleware(limiter *IPRateLimiter, errorMsg string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("user_id")
		key := c.ClientIP()
		if exists {
			key = c.ClientIP() + ":" + string(rune('0'+userIDVal.(uint)))
		}

		l := limiter.getLimiter(key)
		if !l.Allow() {
			c.Header("Retry-After", "30")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   errorMsg,
				"status":  429,
				"message": "Batas pengiriman terlampaui. Silakan tunggu sebentar sebelum mengirim email berikutnya.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
