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

func TestGeminiTranslatorName(t *testing.T) {
	g := &GeminiTranslator{}
	if g.Name() != "gemini" {
		t.Fatalf("Name = %q", g.Name())
	}
}

func TestGeminiTranslator_RequestShapeAndAuth(t *testing.T) {
	var captured struct {
		Method string
		Path   string
		Query  string
		Body   map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Query = r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured.Body)
		writeGeminiJSONReply(w, []string{"こんにちは", "世界"})
	}))
	defer srv.Close()

	g := &GeminiTranslator{APIKey: "GKEY", Model: "gemini-2.0-flash", BaseURL: srv.URL}
	out, err := g.Translate(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(out) != 2 || out[0] != "こんにちは" {
		t.Fatalf("output = %v", out)
	}

	if captured.Method != http.MethodPost {
		t.Errorf("Method = %s", captured.Method)
	}
	if !strings.Contains(captured.Path, "gemini-2.0-flash:generateContent") {
		t.Errorf("Path = %q", captured.Path)
	}
	if !strings.Contains(captured.Query, "key=GKEY") {
		t.Errorf("API key not in query: %q", captured.Query)
	}
	// Must request structured JSON output to keep parsing robust.
	gc, _ := captured.Body["generationConfig"].(map[string]any)
	if gc == nil {
		t.Fatalf("missing generationConfig")
	}
	if gc["responseMimeType"] != "application/json" {
		t.Errorf("responseMimeType = %v", gc["responseMimeType"])
	}
}

func TestGeminiTranslator_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403}}`))
	}))
	defer srv.Close()

	g := &GeminiTranslator{APIKey: "k", Model: "gemini-2.0-flash", BaseURL: srv.URL}
	_, err := g.Translate(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error should mention status: %v", err)
	}
}

func TestGeminiTranslator_LengthMismatchIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeGeminiJSONReply(w, []string{"only one"})
	}))
	defer srv.Close()

	g := &GeminiTranslator{APIKey: "k", Model: "gemini-2.0-flash", BaseURL: srv.URL}
	_, err := g.Translate(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected length-mismatch error")
	}
}

func TestGeminiTranslator_EmptyBatch(t *testing.T) {
	g := &GeminiTranslator{APIKey: "k", Model: "m", BaseURL: "http://unused"}
	out, err := g.Translate(context.Background(), nil)
	if err != nil || len(out) != 0 {
		t.Fatalf("empty batch: out=%v err=%v", out, err)
	}
}

func writeGeminiJSONReply(w http.ResponseWriter, translations []string) {
	body, _ := json.Marshal(translations)
	resp := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]any{
						{"text": string(body)},
					},
				},
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
