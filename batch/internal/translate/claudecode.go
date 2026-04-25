package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// ClaudeCodeTranslator shells out to the `claude` CLI in print mode
// (`claude -p`). The CLI uses the user's Claude subscription, so this
// provider works without a separate Anthropic API credit balance.
//
// Trade-off: subscription rate limits are tighter than API tier limits,
// so seeding the full ~60k descriptions takes considerably longer than
// the API path. The cache is content-addressed and persisted, so partial
// runs accumulate and the seed can be completed across many sessions.
type ClaudeCodeTranslator struct {
	// Binary names the executable. Default: "claude".
	Binary string
	// Model passes through `--model` (e.g. "sonnet", "haiku", "opus"
	// or a full model id). Empty leaves the CLI default.
	Model string
	// Run is the command-execution hook used by tests to stub out the
	// real subprocess. Production callers leave it nil.
	Run func(ctx context.Context, args []string) ([]byte, error)
}

func (c *ClaudeCodeTranslator) Name() string { return "claude-code" }

func (c *ClaudeCodeTranslator) Translate(ctx context.Context, srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	binary := c.Binary
	if binary == "" {
		binary = "claude"
	}

	args := []string{
		"-p",
		"--output-format", "json",
		"--append-system-prompt", translationSystemPrompt,
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	args = append(args, buildTranslationPrompt(srcs))

	runFn := c.Run
	if runFn == nil {
		runFn = func(ctx context.Context, a []string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, binary, a...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("%w (stderr: %s)", err, truncate(stderr.String(), 256))
			}
			return out, nil
		}
	}

	raw, err := runFn(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("claude-code: %w", err)
	}

	var resp struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("claude-code: decode response: %w", err)
	}
	if resp.IsError {
		return nil, fmt.Errorf("claude-code: returned error: %s", truncate(resp.Result, 256))
	}

	out, err := parseJSONArray(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("claude-code: %w", err)
	}
	if len(out) != len(srcs) {
		return nil, fmt.Errorf("claude-code: expected %d translations, got %d", len(srcs), len(out))
	}
	return out, nil
}
