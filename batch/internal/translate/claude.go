package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClaudeTranslator uses the Anthropic Messages API to translate batches of
// English strings to Japanese.
//
// Used only for the one-time bulk seed of the existing ~60k repository
// descriptions. Once the cache is populated and committed, ongoing
// increments run through Gemini and the Claude provider can be retired
// without affecting the site.
type ClaudeTranslator struct {
	APIKey  string
	Model   string // e.g. "claude-haiku-4-5-20251001"
	BaseURL string // override for tests; defaults to https://api.anthropic.com
	HTTP    *http.Client
}

func (c *ClaudeTranslator) Name() string { return "claude" }

func (c *ClaudeTranslator) Translate(ctx context.Context, srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	model := c.Model
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	prompt := buildTranslationPrompt(srcs)
	reqBody := map[string]any{
		"model":      model,
		"max_tokens": 8192,
		"system":     translationSystemPrompt,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(base, "/")+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	httpc := c.HTTP
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
		return nil, fmt.Errorf("claude: HTTP %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("claude: decode response: %w", err)
	}

	var text string
	for _, b := range parsed.Content {
		if b.Type == "text" {
			text = b.Text
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("claude: empty content in response")
	}

	out, err := parseJSONArray(text)
	if err != nil {
		return nil, fmt.Errorf("claude: %w", err)
	}
	if len(out) != len(srcs) {
		return nil, fmt.Errorf("claude: expected %d translations, got %d", len(srcs), len(out))
	}
	return out, nil
}

const translationSystemPrompt = `You translate English GitHub repository descriptions into natural, concise Japanese for a Japanese-speaking developer audience.

Rules:
- Output ONLY a JSON array of strings, in the same order as the inputs.
- The output array MUST contain EXACTLY the same number of strings as the input. Do not deduplicate. Do not merge similar items. Do not skip any input. Even if two inputs are identical, output two separate translations.
- Keep proper nouns, project names, brand names, and technology names (React, GraphQL, etc.) in their original form.
- Preserve technical accuracy. Do not paraphrase ambiguously.
- Do not add commentary, headers, or markdown fences. Output the JSON array and nothing else.`

func buildTranslationPrompt(srcs []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Translate each of the %d English GitHub repository descriptions below to Japanese. Output a JSON array of EXACTLY %d strings — one Japanese translation per numbered input, in the same order.\n\n", len(srcs), len(srcs))
	for i, s := range srcs {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, s)
	}
	return b.String()
}

// parseJSONArray extracts the first JSON array of strings from text. Models
// occasionally wrap the array in markdown fences; tolerate that.
func parseJSONArray(text string) ([]string, error) {
	t := strings.TrimSpace(text)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	t = strings.TrimSpace(t)

	// Find the first '[' — anything before it is a stray preamble.
	if i := strings.Index(t, "["); i > 0 {
		t = t[i:]
	}
	// And the last ']'.
	if j := strings.LastIndex(t, "]"); j >= 0 && j < len(t)-1 {
		t = t[:j+1]
	}

	var out []string
	if err := json.Unmarshal([]byte(t), &out); err != nil {
		return nil, fmt.Errorf("not a JSON array of strings: %w", err)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
