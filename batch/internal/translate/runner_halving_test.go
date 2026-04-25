package translate

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// flakyTranslator returns the wrong number of strings for batches above
// minOK and behaves correctly for smaller batches. Models do this in
// practice: the bigger the batch, the more likely they collapse two
// near-identical items into one.
type flakyTranslator struct {
	mu      sync.Mutex
	calls   int
	minOK   int // batches of size <= minOK return the right count
	dropOne bool
}

func (f *flakyTranslator) Name() string { return "flaky" }

func (f *flakyTranslator) Translate(_ context.Context, srcs []string) ([]string, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if len(srcs) > f.minOK && f.dropOne {
		// Return one fewer translation than asked.
		out := make([]string, 0, len(srcs)-1)
		for i := 0; i < len(srcs)-1; i++ {
			out = append(out, "ja:"+srcs[i])
		}
		return out, nil
	}
	out := make([]string, len(srcs))
	for i, s := range srcs {
		out[i] = "ja:" + s
	}
	return out, nil
}

// On length mismatch the runner halves the batch and retries each half.
// Eventually all items either translate or are dropped one at a time.
func TestRunner_HalvingRecoversFromLengthMismatch(t *testing.T) {
	ft := &flakyTranslator{minOK: 4, dropOne: true}
	r := &Runner{
		Cache:           &Cache{Dir: t.TempDir()},
		Translator:      ft,
		BatchSize:       8,
		HalveOnMismatch: true,
	}
	srcs := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

	stats, err := r.Run(context.Background(), srcs, 0)
	if err != nil {
		t.Fatal(err)
	}
	// All 8 should eventually succeed via halving (8 → 4+4, both ≤ minOK).
	if stats.Translated != 8 {
		t.Fatalf("Translated = %d, want 8 (halving should recover)", stats.Translated)
	}
}

// HalveOnMismatch=false retains the legacy behaviour: one mismatch fails
// the whole batch. This guards the older test cases that rely on the
// strict mode.
func TestRunner_HalvingDisabledKeepsLegacyBehaviour(t *testing.T) {
	ft := &flakyTranslator{minOK: 4, dropOne: true}
	r := &Runner{
		Cache:           &Cache{Dir: t.TempDir()},
		Translator:      ft,
		BatchSize:       8,
		HalveOnMismatch: false,
	}
	stats, _ := r.Run(context.Background(), []string{"a", "b", "c", "d", "e", "f", "g", "h"}, 0)
	if stats.Failed != 8 {
		t.Fatalf("Failed = %d, want 8 (whole batch dropped)", stats.Failed)
	}
}

// Halving must terminate even when single-item batches still fail.
func TestRunner_HalvingTerminatesOnSingleItemFailure(t *testing.T) {
	r := &Runner{
		Cache:      &Cache{Dir: t.TempDir()},
		Translator: alwaysShortByOne{},
		// BatchSize=2 → on mismatch halve to 1+1; each single-item call
		// returns 0 strings (still mismatch), and the runner must drop
		// the item rather than recurse forever.
		BatchSize:       2,
		HalveOnMismatch: true,
	}
	stats, err := r.Run(context.Background(), []string{"a", "b"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Failed != 2 {
		t.Fatalf("Failed = %d, want 2 (single-item failures must drop)", stats.Failed)
	}
	if stats.Translated != 0 {
		t.Fatalf("Translated = %d, want 0", stats.Translated)
	}
}

// Hard errors (not length mismatch) still abort the batch immediately —
// halving is for malformed responses, not for retrying network failures.
func TestRunner_HalvingDoesNotRetryHardErrors(t *testing.T) {
	r := &Runner{
		Cache:           &Cache{Dir: t.TempDir()},
		Translator:      &MockTranslator{ForceErr: errors.New("api down")},
		BatchSize:       4,
		HalveOnMismatch: true,
	}
	stats, _ := r.Run(context.Background(), []string{"a", "b", "c", "d"}, 0)
	if stats.Failed != 4 {
		t.Fatalf("Failed = %d, want 4 (hard error → drop, no retry)", stats.Failed)
	}
}

type alwaysShortByOne struct{}

func (alwaysShortByOne) Name() string { return "short" }
func (alwaysShortByOne) Translate(_ context.Context, srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	return srcs[:len(srcs)-1], nil
}
