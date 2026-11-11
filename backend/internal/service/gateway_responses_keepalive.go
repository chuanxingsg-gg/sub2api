package service

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func (s *GatewayService) responsesKeepaliveInterval() time.Duration {
	if s.cfg == nil {
		return 10 * time.Second
	}
	return time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
}
func responsesKeepaliveTicker(interval time.Duration) (*time.Ticker, <-chan time.Time) {
	if interval <= 0 {
		return nil, nil
	}
	t := time.NewTicker(interval)
	return t, t.C
}

// Only the request goroutine writes to Gin. The upstream worker never touches c.
func writeResponsesKeepalive(c *gin.Context) error {
	if !c.Writer.Written() {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
	}
	if _, err := fmt.Fprint(c.Writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n"); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}
func (s *GatewayService) doResponsesUpstreamWithKeepalive(c *gin.Context, stream bool, req *http.Request, call func(*http.Request) (*http.Response, error)) (*http.Response, error, context.CancelFunc) {
	noop := func() {}
	if !stream || s.responsesKeepaliveInterval() <= 0 {
		r, e := call(req)
		return r, e, noop
	}
	ctx, cancel := context.WithCancel(req.Context())
	type result struct {
		response *http.Response
		err      error
	}
	done := make(chan result)
	go func() {
		r, e := call(req.WithContext(ctx))
		select {
		case done <- result{r, e}:
		case <-ctx.Done():
			if r != nil && r.Body != nil {
				r.Body.Close()
			}
		}
	}()
	ticker, _ := responsesKeepaliveTicker(s.responsesKeepaliveInterval())
	defer ticker.Stop()
	for {
		select {
		case r := <-done:
			return r.response, r.err, cancel
		case <-c.Request.Context().Done():
			cancel()
			return nil, c.Request.Context().Err(), noop
		case <-ticker.C:
			if err := writeResponsesKeepalive(c); err != nil {
				cancel()
				return nil, err, noop
			}
		}
	}
}
