package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	traceIDHeader = "X-Trace-ID"
	traceIDKey    = "trace_id"
)

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(traceIDHeader)
		if traceID == "" {
			traceID = newTraceID()
		}
		c.Set(traceIDKey, traceID)
		c.Header(traceIDHeader, traceID)
		c.Next()
	}
}

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf(
			"trace_id=%s method=%s path=%s status=%d latency=%s client_ip=%s",
			TraceIDFromContext(c),
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
		)
	}
}

func TraceIDFromContext(c *gin.Context) string {
	value, ok := c.Get(traceIDKey)
	if !ok {
		return ""
	}
	traceID, _ := value.(string)
	return traceID
}

func newTraceID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("trace_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("trace_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
}
