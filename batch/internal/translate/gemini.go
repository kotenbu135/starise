package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GeminiTranslator uses Google's Generative Language API. The free tier
// (as of 2026-04) allows ~1,500 requests/day on gemini-2.0-flash, which
// covers the daily incremental translation load of starise (a few
// hundred new descriptions per cron run after the initial Claude seed).
type GeminiTranslator struct {
	APIKey  string
	Model   string // e.g. "gemini-2.0-flash"
	BaseURL string // override for tests; defaults to https://generativelanguage.googleapis.com
	HTTP    *http.Client
}

func (g *GeminiTranslator) Name() string { return "gemini" }

func (g *GeminiTranslator) Translate(ctx context.Context, srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	base := g.BaseURL
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	model := g.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent",
		strings.TrimRight(base, "/"), model)
	q := url.Values{}
	q.Set("key", g.APIKey)
	endpoint += "?" + q.Encode()

	prompt := translationSystemPrompt + "\n\n" + buildTranslationPrompt(srcs)
	reqBody := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"temperature":      0.2,
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpc := g.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini: HTTP %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty candidates in response")
	}
	text := parsed.Candidates[0].Content.Parts[0].Text

	out, err := parseJSONArray(text)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}
	if len(out) != len(srcs) {
		return nil, fmt.Errorf("gemini: expected %d translations, got %d", len(srcs), len(out))
	}
	return out, nil
}
