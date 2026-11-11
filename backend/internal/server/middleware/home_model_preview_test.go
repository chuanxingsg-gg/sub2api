//go:build unit

package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeModelPreviewFramingIsLimitedToInertPreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/hero-preview/preview.html", "/admin/settings", "/hero-preview/site-assets/preview.js", "/"} {
		r := gin.New()
		r.Use(SecurityHeaders(config.CSPConfig{Enabled: true}, nil))
		r.GET(path, func(c *gin.Context) { c.String(200, "ok") })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		policy := w.Header().Get("Content-Security-Policy")
		if path == "/hero-preview/preview.html" {
			if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || !strings.Contains(policy, "frame-ancestors 'self'") {
				t.Fatal("preview framing policy", w.Header())
			}
		} else {
			if w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(policy, "frame-ancestors 'none'") {
				t.Fatal("non-preview framing unexpectedly allowed", path)
			}
		}
	}
}
