package translate

import (
	"context"
	"errors"
	"testing"
)

func TestMockTranslatorReturnsCannedResponses(t *testing.T) {
	m := &MockTranslator{
		Responses: map[string]string{
			"hello":     "こんにちは",
			"good day":  "こんにちは",
			"foo bar":   "ふー ばー",
		},
	}
	got, err := m.Translate(context.Background(), []string{"hello", "good day", "foo bar"})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	want := []string{"こんにちは", "こんにちは", "ふー ばー"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

func TestMockTranslatorErrorsOnUnknownInput(t *testing.T) {
	m := &MockTranslator{Responses: map[string]string{"a": "あ"}}
	_, err := m.Translate(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for unknown input 'b'")
	}
}

func TestMockTranslatorRespectsForceErr(t *testing.T) {
	want := errors.New("forced")
	m := &MockTranslator{ForceErr: want}
	_, err := m.Translate(context.Background(), []string{"x"})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func TestMockTranslatorRecordsCalls(t *testing.T) {
	m := &MockTranslator{Responses: map[string]string{"a": "あ", "b": "び"}}
	_, err := m.Translate(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Translate(context.Background(), []string{"b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Calls) != 2 {
		t.Fatalf("Calls len = %d, want 2", len(m.Calls))
	}
	if m.Calls[0][0] != "a" || m.Calls[1][0] != "b" {
		t.Fatalf("Calls = %v", m.Calls)
	}
}

func TestMockTranslatorName(t *testing.T) {
	m := &MockTranslator{}
	if m.Name() != "mock" {
		t.Fatalf("Name = %q, want %q", m.Name(), "mock")
	}
}

func TestMockTranslatorEmptyBatchOK(t *testing.T) {
	m := &MockTranslator{}
	out, err := m.Translate(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty batch should be allowed: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("empty in → empty out, got %d", len(out))
	}
}
