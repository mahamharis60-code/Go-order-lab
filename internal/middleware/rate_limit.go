package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go-order-lab/internal/metrics"
	"go-order-lab/internal/response"
)

type TokenBucketConfig struct {
	RPS   float64
	Burst int
}

type TokenBucketLimiter struct {
	mu       sync.Mutex
	rps      float64
	burst    float64
	tokens   float64
	lastFill time.Time
}

func NewTokenBucketLimiter(cfg TokenBucketConfig) *TokenBucketLimiter {
	if cfg.RPS <= 0 {
		cfg.RPS = 20
	}
	if cfg.Burst <= 0 {
		cfg.Burst = int(cfg.RPS)
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}
	return &TokenBucketLimiter{
		rps:      cfg.RPS,
		burst:    float64(cfg.Burst),
		tokens:   float64(cfg.Burst),
		lastFill: time.Now(),
	}
}

func (l *TokenBucketLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(time.Now()) {
			metrics.IncBusinessEvent("rate_limit", "blocked")
			response.Error(c, http.StatusTooManyRequests, "too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (l *TokenBucketLimiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	elapsed := now.Sub(l.lastFill).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.rps
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.lastFill = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}
