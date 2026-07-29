package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestTimeoutAddsDeadlineAndReturnsGatewayTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestTimeout(20 * time.Millisecond))

	done := make(chan error, 1)
	router.GET("/slow", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); !ok {
			c.String(http.StatusInternalServerError, "missing deadline")
			return
		}
		<-c.Request.Context().Done()
		done <- c.Request.Context().Err()
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("context error = %v, want deadline exceeded", err)
		}
	default:
		t.Fatal("handler did not observe context cancellation")
	}
}
