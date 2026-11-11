package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"time"
)

// LimitLoginAccount adds an account bucket independent of client IP. Request
// bodies are bounded before buffering and email addresses are never cache keys.
func (r *RateLimiter) LimitLoginAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"code": 413, "message": "Login request too large"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		var req struct {
			Email string `json:"email"`
		}
		if json.Unmarshal(body, &req) != nil {
			c.Next()
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))
		if email == "" || len(email) > 254 {
			c.Next()
			return
		}
		digest := sha256.Sum256([]byte(email))
		result, err := r.Allow(c.Request.Context(), "auth-login-account:"+hex.EncodeToString(digest[:]), 10, time.Minute)
		if err != nil || !result.Allowed {
			abortRateLimit(c, time.Minute)
			return
		}
		c.Next()
	}
}
