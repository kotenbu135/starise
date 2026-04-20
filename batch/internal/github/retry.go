package github

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryTransport wraps an http.RoundTripper and retries requests that hit a
// GitHub rate limit. Three cases are handled:
//
//   - HTTP 429 / HTTP 403 "secondary rate limit" body: honor Retry-After or
//     fall back to short exponential backoff (seconds).
//   - HTTP 200 with GraphQL errors[{type:"RATE_LIMITED"}] — primary-limit
//     exhaustion. Parse data.rateLimit.resetAt and sleep until then. If
//     resetAt is absent, fall back to a minute-long backoff (short retries
//     are wasted — the budget resets hourly).
//   - HTTP 200 with GraphQL errors[{type:"MAX_NODE_LIMIT_EXCEEDED"}] — query
//     too large. Do NOT retry; propagate so callers can shrink the batch.
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

// primaryLimitFallback is the sleep we apply when a primary-limit response is
// seen without a parseable resetAt. One minute is long enough that short
// retry churn doesn't burn attempts uselessly, short enough that callers
// don't block forever if the resetAt was just missing from a partial body.
const primaryLimitFallback = 60 * time.Second

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
		wait, retry := retryDecision(resp, attempt, rt.now())
		if !retry || attempt == rt.maxRetries {
			return resp, nil
		}
		// Drain + close so the underlying connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		rt.sleep(wait)
	}
	return resp, nil
}

// retryDecision inspects the response and returns (sleep, shouldRetry). It
// restores resp.Body so a caller that ultimately receives this response still
// reads the full payload (shurcooL/graphql parses 200 bodies into Go errors).
func retryDecision(resp *http.Response, attempt int, now time.Time) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		d := parseRetryAfter(resp.Header.Get("Retry-After"), now)
		if d <= 0 {
			d = backoffDuration(attempt)
		}
		return d, true
	case http.StatusForbidden:
		b := peekBody(resp)
		if !strings.Contains(strings.ToLower(string(b)), "rate limit") {
			return 0, false // plain auth 403 — do not retry
		}
		d := parseRetryAfter(resp.Header.Get("Retry-After"), now)
		if d <= 0 {
			d = backoffDuration(attempt)
		}
		return d, true
	case http.StatusOK:
		b := peekBody(resp)
		kind, resetAt := classifyGraphQLError(b)
		switch kind {
		case gqlErrRateLimited:
			if d := resetAtDelay(resetAt, now); d > 0 {
				return d, true
			}
			return primaryLimitFallback, true
		case gqlErrMaxNodeLimit:
			return 0, false
		}
		return 0, false
	}
	return 0, false
}

// peekBody reads the response body fully and replaces it with a fresh
// io.NopCloser so downstream consumers still see the same bytes. Returns
// empty on error; callers treat unreadable bodies as non-retryable.
func peekBody(resp *http.Response) []byte {
	if resp.Body == nil {
		return nil
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return b
}

type gqlErrKind int

const (
	gqlErrNone gqlErrKind = iota
	gqlErrRateLimited
	gqlErrMaxNodeLimit
)

// classifyGraphQLError parses a 200 body looking for GitHub GraphQL error
// markers we care about. Returns the detected kind and data.rateLimit.resetAt
// (RFC3339) when present. A missing/malformed body yields gqlErrNone.
func classifyGraphQLError(body []byte) (gqlErrKind, string) {
	if len(body) == 0 {
		return gqlErrNone, ""
	}
	var parsed struct {
		Data struct {
			RateLimit struct {
				ResetAt string `json:"resetAt"`
			} `json:"rateLimit"`
		} `json:"data"`
		Errors []struct {
			Type string `json:"type"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return gqlErrNone, ""
	}
	for _, e := range parsed.Errors {
		switch e.Type {
		case "RATE_LIMITED":
			return gqlErrRateLimited, parsed.Data.RateLimit.ResetAt
		case "MAX_NODE_LIMIT_EXCEEDED":
			return gqlErrMaxNodeLimit, ""
		}
	}
	return gqlErrNone, ""
}

// resetAtDelay returns the duration from now until resetAt plus a 5s buffer,
// or 0 when resetAt is empty/past/unparseable.
func resetAtDelay(resetAt string, now time.Time) time.Duration {
	if resetAt == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, resetAt)
	if err != nil {
		return 0
	}
	if !t.After(now) {
		return 0
	}
	return t.Sub(now) + 5*time.Second
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
