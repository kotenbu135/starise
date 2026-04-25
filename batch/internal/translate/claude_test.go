package translate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeTranslatorName(t *testing.T) {
	c := &ClaudeTranslator{}
	if c.Name() != "claude" {
		t.Fatalf("Name = %q, want %q", c.Name(), "claude")
	}
}

func TestClaudeTranslator_RequestShape(t *testing.T) {
	var captured struct {
		Method     string
		Path       string
		APIKey     string
		AnthVer    string
		ContentTyp string
		Body       map[string]any
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.APIKey = r.Header.Get("x-api-key")
		captured.AnthVer = r.Header.Get("anthropic-version")
		captured.ContentTyp = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured.Body)
		writeAnthropicJSONReply(w, []string{"こんにちは", "世界"})
	}))
	defer srv.Close()

	c := &ClaudeTranslator{APIKey: "sk-test", Model: "claude-haiku-4-5-20251001", BaseURL: srv.URL}
	out, err := c.Translate(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(out) != 2 || out[0] != "こんにちは" || out[1] != "世界" {
		t.Fatalf("output = %v", out)
	}

	if captured.Method != http.MethodPost {
		t.Errorf("Method = %s, want POST", captured.Method)
	}
	if !strings.HasSuffix(captured.Path, "/v1/messages") {
		t.Errorf("Path = %s, want suffix /v1/messages", captured.Path)
	}
	if captured.APIKey != "sk-test" {
		t.Errorf("x-api-key = %q", captured.APIKey)
	}
	if captured.AnthVer == "" {
		t.Errorf("missing anthropic-version header")
	}
	if !strings.Contains(captured.ContentTyp, "application/json") {
		t.Errorf("Content-Type = %q", captured.ContentTyp)
	}
	if captured.Body["model"] != "claude-haiku-4-5-20251001" {
		t.Errorf("model in body = %v", captured.Body["model"])
	}
}

func TestClaudeTranslator_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit","message":"slow down"}}`))
	}))
	defer srv.Close()

	c := &ClaudeTranslator{APIKey: "k", Model: "m", BaseURL: srv.URL}
	_, err := c.Translate(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "rate") {
		t.Fatalf("error should mention status: %v", err)
	}
}

func TestClaudeTranslator_LengthMismatchIsError(t *testing.T) {
	// Provider returned fewer items than asked — must surface as error so
	// Runner counts it as Failed and skips cache write.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeAnthropicJSONReply(w, []string{"only one"})
	}))
	defer srv.Close()

	c := &ClaudeTranslator{APIKey: "k", Model: "m", BaseURL: srv.URL}
	_, err := c.Translate(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected length-mismatch error")
	}
}

func TestClaudeTranslator_EmptyBatch(t *testing.T) {
	c := &ClaudeTranslator{APIKey: "k", Model: "m", BaseURL: "http://unused"}
	out, err := c.Translate(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty batch: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("len = %d, want 0", len(out))
	}
}

func TestClaudeTranslator_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := &ClaudeTranslator{APIKey: "k", Model: "m", BaseURL: srv.URL}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Translate(ctx, []string{"x"})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

// writeAnthropicJSONReply emits a Messages API response whose first text
// block contains the JSON-encoded slice of translations.
func writeAnthropicJSONReply(w http.ResponseWriter, translations []string) {
	body, _ := json.Marshal(translations)
	resp := map[string]any{
		"id":    "msg_test",
		"type":  "message",
		"role":  "assistant",
		"model": "claude-haiku-4-5-20251001",
		"content": []map[string]any{
			{"type": "text", "text": string(body)},
		},
		"stop_reason": "end_turn",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
