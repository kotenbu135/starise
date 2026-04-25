package translate

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newRunner(t *testing.T, tr Translator) *Runner {
	t.Helper()
	return &Runner{
		Cache:      &Cache{Dir: t.TempDir()},
		Translator: tr,
		BatchSize:  3,
		Now:        func() time.Time { return time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC) },
	}
}

func TestRunnerEmptyInput(t *testing.T) {
	r := newRunner(t, &MockTranslator{})
	stats, err := r.Run(context.Background(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (RunStats{}) {
		t.Fatalf("expected zero stats, got %+v", stats)
	}
}

func TestRunnerAllCached_NoTranslatorCalls(t *testing.T) {
	m := &MockTranslator{Responses: map[string]string{"a": "あ"}}
	r := newRunner(t, m)
	if err := r.Cache.Put(CacheEntry{Src: "a", JA: "pre", Provider: "claude", TranslatedAt: "earlier"}); err != nil {
		t.Fatal(err)
	}
	stats, err := r.Run(context.Background(), []string{"a"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CacheHits != 1 || stats.Translated != 0 || len(m.Calls) != 0 {
		t.Fatalf("expected pure cache hit, stats=%+v calls=%v", stats, m.Calls)
	}
}

func TestRunnerTranslatesMissingOnly(t *testing.T) {
	m := &MockTranslator{Responses: map[string]string{"miss": "見逃し"}}
	r := newRunner(t, m)
	if err := r.Cache.Put(CacheEntry{Src: "hit", JA: "ヒット", Provider: "claude"}); err != nil {
		t.Fatal(err)
	}
	stats, err := r.Run(context.Background(), []string{"hit", "miss"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CacheHits != 1 || stats.Translated != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	if len(m.Calls) != 1 || len(m.Calls[0]) != 1 || m.Calls[0][0] != "miss" {
		t.Fatalf("translator should have been called only with [miss], got %v", m.Calls)
	}

	// And the missing one is now in cache.
	got, ok, err := r.Cache.Get("miss")
	if err != nil || !ok {
		t.Fatalf("not cached after Run: ok=%v err=%v", ok, err)
	}
	if got.JA != "見逃し" || got.Provider != "mock" {
		t.Fatalf("cached entry wrong: %+v", got)
	}
	if got.TranslatedAt != "2026-04-26T00:00:00Z" {
		t.Fatalf("TranslatedAt not stamped: %q", got.TranslatedAt)
	}
}

func TestRunnerSkipsBlankSrc(t *testing.T) {
	m := &MockTranslator{}
	r := newRunner(t, m)
	stats, err := r.Run(context.Background(), []string{"", "   ", "\n"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Skipped != 3 || stats.Translated != 0 || len(m.Calls) != 0 {
		t.Fatalf("blanks should be skipped without calling translator: %+v calls=%v", stats, m.Calls)
	}
}

func TestRunnerDedupDuplicateSrcs(t *testing.T) {
	m := &MockTranslator{Responses: map[string]string{"dup": "重複"}}
	r := newRunner(t, m)
	stats, err := r.Run(context.Background(), []string{"dup", "dup", "dup"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// One unique miss → one translated, two cache hits (after the first
	// is written to cache).
	if stats.Translated != 1 {
		t.Fatalf("Translated = %d, want 1 (dedup)", stats.Translated)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("translator called %d times, want 1", len(m.Calls))
	}
	if len(m.Calls[0]) != 1 {
		t.Fatalf("batch contained dup entries: %v", m.Calls[0])
	}
}

func TestRunnerBatchSizeRespected(t *testing.T) {
	resp := map[string]string{}
	srcs := []string{}
	for i := 0; i < 7; i++ {
		s := string(rune('a' + i))
		srcs = append(srcs, s)
		resp[s] = s + "!"
	}
	m := &MockTranslator{Responses: resp}
	r := newRunner(t, m)
	r.BatchSize = 3
	if _, err := r.Run(context.Background(), srcs, 0); err != nil {
		t.Fatal(err)
	}
	// 7 items, batch=3 → 3,3,1 → 3 calls
	if len(m.Calls) != 3 {
		t.Fatalf("expected 3 batches, got %d (%v)", len(m.Calls), m.Calls)
	}
	if len(m.Calls[0]) != 3 || len(m.Calls[1]) != 3 || len(m.Calls[2]) != 1 {
		t.Fatalf("batch sizes wrong: %v", m.Calls)
	}
}

func TestRunnerLimitCapsTranslated(t *testing.T) {
	resp := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}
	m := &MockTranslator{Responses: resp}
	r := newRunner(t, m)
	r.BatchSize = 2

	stats, err := r.Run(context.Background(), []string{"a", "b", "c", "d"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Translated != 2 {
		t.Fatalf("Translated = %d, want 2 (limit)", stats.Translated)
	}
}

func TestRunnerTranslatorErrorTallied(t *testing.T) {
	m := &MockTranslator{ForceErr: errors.New("api down")}
	r := newRunner(t, m)
	stats, err := r.Run(context.Background(), []string{"x", "y"}, 0)
	if err != nil {
		t.Fatalf("translator errors should NOT abort Run: %v", err)
	}
	if stats.Failed == 0 {
		t.Fatalf("expected Failed > 0, got %+v", stats)
	}
	if stats.Translated != 0 {
		t.Fatalf("nothing should be translated when API errors: %+v", stats)
	}
}

func TestRunnerLengthMismatchIsHardError(t *testing.T) {
	// A buggy translator that returns the wrong number of strings would
	// silently corrupt the cache; treat it as failure, not as success.
	m := &shortReturnTranslator{}
	r := newRunner(t, m)
	stats, err := r.Run(context.Background(), []string{"a", "b"}, 0)
	if err != nil {
		t.Fatalf("Run itself shouldn't error, but should mark Failed: %v", err)
	}
	if stats.Failed == 0 {
		t.Fatalf("expected Failed > 0 on length mismatch, got %+v", stats)
	}
	if has, _ := r.Cache.Has("a"); has {
		t.Fatal("must not write cache when translator output is malformed")
	}
}

type shortReturnTranslator struct{}

func (shortReturnTranslator) Name() string { return "short" }
func (shortReturnTranslator) Translate(_ context.Context, srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	return srcs[:len(srcs)-1], nil // one short
}
