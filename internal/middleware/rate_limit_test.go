package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTokenBucketLimiterRejectsOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewTokenBucketLimiter(TokenBucketConfig{RPS: 1, Burst: 2})
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/limited", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	statuses := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		statuses = append(statuses, rec.Code)
	}

	if statuses[0] != http.StatusOK || statuses[1] != http.StatusOK || statuses[2] != http.StatusTooManyRequests {
		t.Fatalf("statuses = %v, want [200 200 429]", statuses)
	}
}

func TestTokenBucketLimiterRefills(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewTokenBucketLimiter(TokenBucketConfig{RPS: 50, Burst: 1})
	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/limited", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/limited", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("overflow status = %d, want 429", rec.Code)
	}

	time.Sleep(25 * time.Millisecond)
	req = httptest.NewRequest(http.MethodGet, "/limited", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refilled status = %d, want 200", rec.Code)
	}
}
