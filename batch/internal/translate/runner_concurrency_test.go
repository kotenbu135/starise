package translate

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingTranslator records the maximum number of in-flight Translate
// calls so we can assert the worker pool actually parallelises.
type countingTranslator struct {
	mu       sync.Mutex
	inFlight int32
	maxSeen  int32
	delay    time.Duration
}

func (c *countingTranslator) Name() string { return "counting" }

func (c *countingTranslator) Translate(_ context.Context, srcs []string) ([]string, error) {
	now := atomic.AddInt32(&c.inFlight, 1)
	defer atomic.AddInt32(&c.inFlight, -1)

	c.mu.Lock()
	if now > c.maxSeen {
		c.maxSeen = now
	}
	c.mu.Unlock()

	time.Sleep(c.delay) // simulate API latency

	out := make([]string, len(srcs))
	for i, s := range srcs {
		out[i] = "ja:" + s
	}
	return out, nil
}

func TestRunner_ConcurrencyDispatchesParallelBatches(t *testing.T) {
	ct := &countingTranslator{delay: 30 * time.Millisecond}
	r := &Runner{
		Cache:       &Cache{Dir: t.TempDir()},
		Translator:  ct,
		BatchSize:   1,
		Concurrency: 4,
	}
	// 12 unique sources → 12 batches at BatchSize=1.
	srcs := make([]string, 12)
	for i := range srcs {
		srcs[i] = string(rune('a' + i))
	}

	stats, err := r.Run(context.Background(), srcs, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Translated != 12 {
		t.Fatalf("Translated = %d, want 12", stats.Translated)
	}
	if ct.maxSeen < 2 {
		t.Fatalf("expected at least 2 concurrent calls, observed maxSeen=%d", ct.maxSeen)
	}
	if ct.maxSeen > 4 {
		t.Fatalf("maxSeen=%d exceeds Concurrency=4", ct.maxSeen)
	}
}

func TestRunner_ConcurrencyRespectsLimit(t *testing.T) {
	ct := &countingTranslator{delay: 5 * time.Millisecond}
	r := &Runner{
		Cache:       &Cache{Dir: t.TempDir()},
		Translator:  ct,
		BatchSize:   2,
		Concurrency: 4,
	}
	srcs := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

	// limit=4 → only 4 new translations even though we have 8 sources
	// and 4 concurrent workers.
	stats, err := r.Run(context.Background(), srcs, 4)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Translated != 4 {
		t.Fatalf("Translated = %d, want 4 (capped by limit)", stats.Translated)
	}
}

func TestRunner_ConcurrencyZeroOrOneIsSequential(t *testing.T) {
	ct := &countingTranslator{delay: 10 * time.Millisecond}
	r := &Runner{
		Cache:       &Cache{Dir: t.TempDir()},
		Translator:  ct,
		BatchSize:   1,
		Concurrency: 0,
	}
	srcs := []string{"a", "b", "c", "d"}

	if _, err := r.Run(context.Background(), srcs, 0); err != nil {
		t.Fatal(err)
	}
	if ct.maxSeen != 1 {
		t.Fatalf("Concurrency=0 must run sequentially, got maxSeen=%d", ct.maxSeen)
	}
}

func TestRunner_ConcurrencyAccumulatesStatsSafely(t *testing.T) {
	// All batches succeed → Translated count must equal source count
	// regardless of goroutine interleaving.
	ct := &countingTranslator{delay: 1 * time.Millisecond}
	r := &Runner{
		Cache:       &Cache{Dir: t.TempDir()},
		Translator:  ct,
		BatchSize:   3,
		Concurrency: 8,
	}
	srcs := make([]string, 100)
	for i := range srcs {
		srcs[i] = string(rune('A' + i%26)) + string(rune('0'+i/26)) + ":" + string(rune('a'+i%26))
	}

	stats, err := r.Run(context.Background(), srcs, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Translated+stats.Failed != stats.Total {
		t.Fatalf("Translated(%d) + Failed(%d) != Total(%d)",
			stats.Translated, stats.Failed, stats.Total)
	}
	if stats.Translated != 100 {
		t.Fatalf("Translated = %d, want 100", stats.Translated)
	}
}
