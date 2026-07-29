package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTraceIDUsesIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceID())
	router.GET("/ping", func(c *gin.Context) {
		if got := TraceIDFromContext(c); got != "trace-fixed" {
			t.Fatalf("TraceIDFromContext = %q, want trace-fixed", got)
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Trace-ID", "trace-fixed")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-Trace-ID"); got != "trace-fixed" {
		t.Fatalf("response trace id = %q, want trace-fixed", got)
	}
}

func TestTraceIDGeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceID())
	router.GET("/ping", func(c *gin.Context) {
		if got := TraceIDFromContext(c); got == "" {
			t.Fatal("TraceIDFromContext is empty")
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Trace-ID"); got == "" {
		t.Fatal("response trace id is empty")
	}
}
