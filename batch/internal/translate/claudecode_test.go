package translate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClaudeCodeTranslatorName(t *testing.T) {
	c := &ClaudeCodeTranslator{}
	if c.Name() != "claude-code" {
		t.Fatalf("Name = %q", c.Name())
	}
}

func TestClaudeCodeTranslator_StubReturnsTranslations(t *testing.T) {
	var seenArgs []string
	c := &ClaudeCodeTranslator{
		Run: func(_ context.Context, args []string) ([]byte, error) {
			seenArgs = args
			return []byte(`{"type":"result","subtype":"success","is_error":false,"result":"[\"こんにちは\",\"世界\"]"}`), nil
		},
	}
	out, err := c.Translate(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(out) != 2 || out[0] != "こんにちは" || out[1] != "世界" {
		t.Fatalf("output = %v", out)
	}

	// args MUST contain -p, --output-format json, --append-system-prompt.
	joined := strings.Join(seenArgs, " ")
	for _, want := range []string{"-p", "--output-format", "json", "--append-system-prompt"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, seenArgs)
		}
	}
}

func TestClaudeCodeTranslator_PassesModelFlag(t *testing.T) {
	var seenArgs []string
	c := &ClaudeCodeTranslator{
		Model: "sonnet",
		Run: func(_ context.Context, args []string) ([]byte, error) {
			seenArgs = args
			return []byte(`{"is_error":false,"result":"[\"x\"]"}`), nil
		},
	}
	if _, err := c.Translate(context.Background(), []string{"x"}); err != nil {
		t.Fatal(err)
	}
	hasModel := false
	for i, a := range seenArgs {
		if a == "--model" && i+1 < len(seenArgs) && seenArgs[i+1] == "sonnet" {
			hasModel = true
		}
	}
	if !hasModel {
		t.Errorf("--model sonnet not passed: %v", seenArgs)
	}
}

func TestClaudeCodeTranslator_RunErrorPropagates(t *testing.T) {
	c := &ClaudeCodeTranslator{
		Run: func(_ context.Context, _ []string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
	}
	_, err := c.Translate(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("error should propagate: %v", err)
	}
}

func TestClaudeCodeTranslator_IsErrorTrueIsFailure(t *testing.T) {
	c := &ClaudeCodeTranslator{
		Run: func(_ context.Context, _ []string) ([]byte, error) {
			return []byte(`{"is_error":true,"result":"rate_limit_error"}`), nil
		},
	}
	_, err := c.Translate(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error when is_error=true")
	}
}

func TestClaudeCodeTranslator_LengthMismatch(t *testing.T) {
	c := &ClaudeCodeTranslator{
		Run: func(_ context.Context, _ []string) ([]byte, error) {
			return []byte(`{"is_error":false,"result":"[\"only\"]"}`), nil
		},
	}
	_, err := c.Translate(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected length-mismatch error")
	}
}

func TestClaudeCodeTranslator_EmptyBatch(t *testing.T) {
	c := &ClaudeCodeTranslator{
		Run: func(_ context.Context, _ []string) ([]byte, error) {
			t.Fatal("Run must not be called for empty batch")
			return nil, nil
		},
	}
	out, err := c.Translate(context.Background(), nil)
	if err != nil || len(out) != 0 {
		t.Fatalf("empty: out=%v err=%v", out, err)
	}
}

func TestClaudeCodeTranslator_MalformedResultJSON(t *testing.T) {
	c := &ClaudeCodeTranslator{
		Run: func(_ context.Context, _ []string) ([]byte, error) {
			return []byte(`{"is_error":false,"result":"not a JSON array"}`), nil
		},
	}
	_, err := c.Translate(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected parse error")
	}
}
