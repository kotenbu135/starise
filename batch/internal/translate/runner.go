package translate

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// RunStats summarises one Runner.Run invocation.
type RunStats struct {
	Total      int // unique non-blank inputs
	CacheHits  int // already in cache before this Run
	Translated int // newly written to cache by this Run
	Failed     int // sources we tried to translate but couldn't
	Skipped    int // empty / whitespace-only sources
}

// Runner glues a Translator to a Cache. It batches missing sources,
// translates them, and writes results to the cache. Cached sources are
// counted but never re-translated.
type Runner struct {
	Cache      *Cache
	Translator Translator
	BatchSize  int
	// Concurrency caps the number of in-flight Translate calls. <=1
	// runs sequentially (default). For the claude-code subprocess
	// provider on a Max plan, 5 is a sensible value: each batch ~60s,
	// so 5 parallel cuts wallclock by ~5x without saturating the
	// account's message rate budget.
	Concurrency int
	// HalveOnMismatch enables a recursive recovery path: when a provider
	// returns the wrong number of translations (LLMs occasionally collapse
	// near-identical items into one) the batch is split in half and each
	// half retried. Single-item batches that still mismatch are dropped.
	// Hard errors (network, auth) are NOT retried — only length mismatch.
	HalveOnMismatch bool
	// Now is injectable so tests get a stable TranslatedAt timestamp.
	Now func() time.Time
	// ErrorLog, when non-nil, receives one line per failed batch with the
	// actual provider error. Default: nil (silent), so unit tests stay
	// quiet. CLI wires this to os.Stderr for live diagnosis.
	ErrorLog io.Writer
}

// Run translates each non-blank source in srcs that isn't already cached.
//
// limit > 0 caps the number of *new* translations performed in this call;
// useful for time-boxing the daily CI step against free-tier quotas.
//
// A translator failure (network, malformed response, bad length) is
// counted in Failed but does not abort the run — the next caller can
// retry, and meanwhile the export step will fall back to the English
// description for any source still missing from the cache.
func (r *Runner) Run(ctx context.Context, srcs []string, limit int) (RunStats, error) {
	stats := RunStats{}
	if r.BatchSize <= 0 {
		r.BatchSize = 32
	}
	now := r.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	// Dedup + skip blanks. Order is preserved for deterministic batching.
	seen := map[string]bool{}
	uniques := make([]string, 0, len(srcs))
	for _, s := range srcs {
		if strings.TrimSpace(s) == "" {
			stats.Skipped++
			continue
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		uniques = append(uniques, s)
	}
	stats.Total = len(uniques)

	// Partition into hits / misses.
	misses := make([]string, 0, len(uniques))
	for _, s := range uniques {
		has, err := r.Cache.Has(s)
		if err != nil {
			return stats, err
		}
		if has {
			stats.CacheHits++
			continue
		}
		misses = append(misses, s)
	}

	if limit > 0 && len(misses) > limit {
		misses = misses[:limit]
	}

	// Slice the misses into batches up front so concurrency = "process
	// these N batches with at most C workers".
	batches := make([][]string, 0, (len(misses)+r.BatchSize-1)/r.BatchSize)
	for start := 0; start < len(misses); start += r.BatchSize {
		end := start + r.BatchSize
		if end > len(misses) {
			end = len(misses)
		}
		batches = append(batches, misses[start:end])
	}

	concurrency := r.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	var statsMu sync.Mutex

	var processBatch func(batch []string)
	processBatch = func(batch []string) {
		out, err := r.Translator.Translate(ctx, batch)
		if err != nil {
			statsMu.Lock()
			stats.Failed += len(batch)
			statsMu.Unlock()
			if r.ErrorLog != nil {
				fmt.Fprintf(r.ErrorLog, "translate batch failed (n=%d): %v\n", len(batch), err)
			}
			return
		}
		if len(out) != len(batch) {
			// Length mismatch — provider dropped or merged items. With
			// HalveOnMismatch we split and recurse; smaller prompts are
			// more reliable, and we eventually shrink to single-item
			// batches that either succeed or are dropped.
			if r.HalveOnMismatch && len(batch) > 1 {
				if r.ErrorLog != nil {
					fmt.Fprintf(r.ErrorLog, "translate batch length mismatch (n=%d, got=%d) — halving\n",
						len(batch), len(out))
				}
				mid := len(batch) / 2
				processBatch(batch[:mid])
				processBatch(batch[mid:])
				return
			}
			statsMu.Lock()
			stats.Failed += len(batch)
			statsMu.Unlock()
			if r.ErrorLog != nil {
				fmt.Fprintf(r.ErrorLog, "translate batch length mismatch: want %d got %d\n", len(batch), len(out))
			}
			return
		}

		ts := now().UTC().Format(time.RFC3339)
		for i, s := range batch {
			entry := CacheEntry{
				Src:          s,
				JA:           out[i],
				Provider:     r.Translator.Name(),
				TranslatedAt: ts,
			}
			if err := r.Cache.Put(entry); err != nil {
				statsMu.Lock()
				stats.Failed++
				statsMu.Unlock()
				continue
			}
			statsMu.Lock()
			stats.Translated++
			statsMu.Unlock()
		}
	}

	if concurrency == 1 {
		for _, b := range batches {
			processBatch(b)
		}
		return stats, nil
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, b := range batches {
		sem <- struct{}{}
		wg.Add(1)
		go func(batch []string) {
			defer wg.Done()
			defer func() { <-sem }()
			processBatch(batch)
		}(b)
	}
	wg.Wait()

	return stats, nil
}
