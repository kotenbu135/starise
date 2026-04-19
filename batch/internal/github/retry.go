package github

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryTransport wraps an http.RoundTripper and retries requests that hit
// GitHub's secondary rate limit (HTTP 403 with "secondary rate limit" in the
// body) or HTTP 429. Retry-After is honored when present; otherwise falls
// back to exponential backoff. Non-rate-limit responses (including plain 403
// auth errors) pass through untouched.
//
// The transport buffers POST bodies so they can be replayed on retry — the
// shurcooL/graphql client sends every query as POST, so this is mandatory.
type retryTransport struct {
	inner      http.RoundTripper
	maxRetries int
	sleep      func(time.Duration) // injectable for tests
	now        func() time.Time    // injectable for tests
}

func newRetryTransport(inner http.RoundTripper, maxRetries int) *retryTransport {
	return &retryTransport{
		inner:      inner,
		maxRetries: maxRetries,
		sleep:      time.Sleep,
		now:        time.Now,
	}
}

func (rt *retryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Buffer body once so we can rewind it for each retry attempt.
	var body []byte
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		_ = r.Body.Close()
		body = b
	}

	var resp *http.Response
	var err error
	for attempt := 0; attempt <= rt.maxRetries; attempt++ {
		if body != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err = rt.inner.RoundTrip(r)
		if err != nil {
			return resp, err
		}
		if !shouldRetryResponse(resp) || attempt == rt.maxRetries {
			return resp, nil
		}
		wait := parseRetryAfter(resp.Header.Get("Retry-After"), rt.now())
		if wait <= 0 {
			wait = backoffDuration(attempt)
		}
		// Drain + close so the underlying connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		rt.sleep(wait)
	}
	return resp, nil
}

// shouldRetryResponse returns true for GitHub secondary rate-limit (403 with
// a "rate limit" body) and for 429 Too Many Requests. It reads and restores
// the response body so callers still see the full payload.
func shouldRetryResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode != http.StatusForbidden {
		return false
	}
	// Peek the body to distinguish rate-limit 403 from auth 403. Restore it
	// so the real consumer still reads the same bytes.
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return false
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	lower := strings.ToLower(string(b))
	return strings.Contains(lower, "rate limit")
}

// parseRetryAfter accepts either a delay in seconds (RFC 7231 §7.1.3) or an
// HTTP-date. Returns 0 if the header is missing/unparseable.
func parseRetryAfter(h string, now time.Time) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// backoffDuration returns exponential backoff with a reasonable cap. attempt
// is 0-indexed (0 = first retry). Pattern: 2s, 4s, 8s, 16s, capped at 30s.
func backoffDuration(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt+1)) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}
