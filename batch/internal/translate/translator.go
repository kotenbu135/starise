package translate

import (
	"context"
	"fmt"
)

// Translator turns a batch of English strings into Japanese, in the same
// order. Implementations: claude, gemini, mock.
//
// Contract: len(out) MUST equal len(srcs). Any failure is signalled via
// non-nil error; callers must not partially apply on error.
type Translator interface {
	Name() string
	Translate(ctx context.Context, srcs []string) ([]string, error)
}

// MockTranslator is a deterministic, no-network implementation used by
// tests. Look up by exact string match against Responses; ForceErr short-
// circuits all calls.
type MockTranslator struct {
	Responses map[string]string
	ForceErr  error
	Calls     [][]string
}

func (m *MockTranslator) Name() string { return "mock" }

func (m *MockTranslator) Translate(_ context.Context, srcs []string) ([]string, error) {
	m.Calls = append(m.Calls, append([]string(nil), srcs...))
	if m.ForceErr != nil {
		return nil, m.ForceErr
	}
	out := make([]string, len(srcs))
	for i, s := range srcs {
		v, ok := m.Responses[s]
		if !ok {
			return nil, fmt.Errorf("MockTranslator: no response for %q", s)
		}
		out[i] = v
	}
	return out, nil
}
