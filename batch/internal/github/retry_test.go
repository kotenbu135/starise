package github

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// recorderRT records how many times it is called and drives a scripted
// sequence of responses for deterministic retry tests.
type scriptedRT struct {
	responses []*http.Response
	calls     int64
	lastBody  string
}

func (s *scriptedRT) RoundTrip(r *http.Request) (*http.Response, error) {
	idx := atomic.AddInt64(&s.calls, 1) - 1
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		s.lastBody = string(b)
		r.Body.Close()
	}
	if int(idx) >= len(s.responses) {
		return s.responses[len(s.responses)-1], nil
	}
	return s.responses[idx], nil
}

func makeResp(status int, body string, headers map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// recordSleeps captures sleep durations instead of actually sleeping.
type recordSleeps struct{ durs []time.Duration }

func (r *recordSleeps) sleep(d time.Duration) { r.durs = append(r.durs, d) }

func TestRetryTransport_SuccessNoRetry(t *testing.T) {
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(200, `{"data":{}}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 3, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{"query":"x"}`))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status=%d, want 200", resp.StatusCode)
	}
	if inner.calls != 1 {
		t.Errorf("calls=%d, want 1", inner.calls)
	}
	if len(rec.durs) != 0 {
		t.Errorf("unexpected sleeps: %v", rec.durs)
	}
}

func TestRetryTransport_SecondaryRateLimit_RetriesThenSucceeds(t *testing.T) {
	secondaryBody := `{"message":"You have exceeded a secondary rate limit.","documentation_url":"https://docs.github.com/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api#secondary-rate-limits"}`
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(403, secondaryBody, map[string]string{"Retry-After": "1"}),
		makeResp(200, `{"data":{}}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 3, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{"query":"x"}`))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status=%d, want 200", resp.StatusCode)
	}
	if inner.calls != 2 {
		t.Errorf("calls=%d, want 2 (retry once)", inner.calls)
	}
	if len(rec.durs) != 1 {
		t.Fatalf("sleeps=%d, want 1", len(rec.durs))
	}
	if rec.durs[0] < time.Second {
		t.Errorf("sleep[0]=%v, want >= 1s from Retry-After", rec.durs[0])
	}
}

func TestRetryTransport_429_HonorsRetryAfter(t *testing.T) {
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(429, `too many requests`, map[string]string{"Retry-After": "2"}),
		makeResp(200, `{"data":{}}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 3, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{}`))
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Errorf("calls=%d, want 2", inner.calls)
	}
	if len(rec.durs) != 1 || rec.durs[0] < 2*time.Second {
		t.Errorf("durs=%v, want [>=2s]", rec.durs)
	}
}

func TestRetryTransport_MaxRetriesExceeded_ReturnsLastResp(t *testing.T) {
	body := `{"message":"You have exceeded a secondary rate limit."}`
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(403, body, map[string]string{"Retry-After": "1"}),
		makeResp(403, body, map[string]string{"Retry-After": "1"}),
		makeResp(403, body, map[string]string{"Retry-After": "1"}),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 2, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{}`))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("status=%d, want 403 (final failure surfaces)", resp.StatusCode)
	}
	// 1 initial + 2 retries = 3 calls
	if inner.calls != 3 {
		t.Errorf("calls=%d, want 3 (1 initial + 2 retries)", inner.calls)
	}
	// sleeps only between retries = 2
	if len(rec.durs) != 2 {
		t.Errorf("sleeps=%d, want 2", len(rec.durs))
	}
}

func TestRetryTransport_Non403Non429_NoRetry(t *testing.T) {
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(401, `{"message":"Bad credentials"}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 3, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{}`))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("status=%d, want 401", resp.StatusCode)
	}
	if inner.calls != 1 {
		t.Errorf("calls=%d, want 1 (no retry for non rate-limit)", inner.calls)
	}
	if len(rec.durs) != 0 {
		t.Errorf("unexpected sleeps: %v", rec.durs)
	}
}

func TestRetryTransport_403_NonRateLimit_NoRetry(t *testing.T) {
	// Plain 403 without "rate limit" body — auth/permission error. Must not retry.
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(403, `{"message":"Resource not accessible by integration"}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 3, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{}`))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Errorf("calls=%d, want 1 (plain 403 must not retry)", inner.calls)
	}
	if resp.StatusCode != 403 {
		t.Errorf("status=%d, want 403", resp.StatusCode)
	}
}

func TestRetryTransport_RetryAfterMissing_FallsBackToBackoff(t *testing.T) {
	body := `{"message":"You have exceeded a secondary rate limit."}`
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(403, body, nil), // no Retry-After
		makeResp(200, `{"data":{}}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 3, sleep: rec.sleep, now: time.Now}

	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(`{}`))
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.durs) != 1 {
		t.Fatalf("sleeps=%d, want 1", len(rec.durs))
	}
	if rec.durs[0] <= 0 {
		t.Errorf("sleep[0]=%v, want positive backoff fallback", rec.durs[0])
	}
}

func TestRetryTransport_BodyRewoundOnRetry(t *testing.T) {
	// GraphQL POST body must be replayed on retry — else the second call
	// hits the server with an empty body and fails with a different error.
	body := `{"message":"You have exceeded a secondary rate limit."}`
	inner := &scriptedRT{responses: []*http.Response{
		makeResp(403, body, map[string]string{"Retry-After": "1"}),
		makeResp(200, `{"data":{}}`, nil),
	}}
	rec := &recordSleeps{}
	rt := &retryTransport{inner: inner, maxRetries: 2, sleep: rec.sleep, now: time.Now}

	reqBody := `{"query":"{ viewer { login } }"}`
	req, _ := http.NewRequest("POST", "http://example/graphql", strings.NewReader(reqBody))
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if inner.lastBody != reqBody {
		t.Errorf("lastBody=%q, want %q (body not rewound)", inner.lastBody, reqBody)
	}
}

// Integration-style sanity check using httptest — proves the transport plugs
// into http.Client like a normal RoundTripper.
func TestRetryTransport_IntegratesWithHTTPClient(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&hits, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`too many`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	rec := &recordSleeps{}
	client := &http.Client{Transport: &retryTransport{
		inner: http.DefaultTransport, maxRetries: 2, sleep: rec.sleep, now: time.Now,
	}}
	resp, err := client.Post(srv.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status=%d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt64(&hits) != 2 {
		t.Errorf("hits=%d, want 2", hits)
	}
}
