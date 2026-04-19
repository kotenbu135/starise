package github

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunBulkRefreshParallelPreservesAllIds(t *testing.T) {
	ids := make([]string, 250)
	for i := range ids {
		ids[i] = fmt.Sprintf("G%d", i)
	}

	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		var found []RepoData
		var missing []string
		for _, id := range batch {
			// even ids found, odd missing — deterministic fixture
			suffix := id[1:]
			var n int
			fmt.Sscanf(suffix, "%d", &n)
			if n%2 == 0 {
				found = append(found, RepoData{GitHubID: id, StarCount: n})
			} else {
				missing = append(missing, id)
			}
		}
		return found, missing, RateLimitInfo{Remaining: 4000, Cost: 1}, nil
	}

	found, missing, limit, err := runBulkRefreshParallel(context.Background(), ids, 100, 5, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(found)+len(missing) != len(ids) {
		t.Fatalf("lost ids: found=%d missing=%d total=%d", len(found), len(missing), len(ids))
	}
	if limit.Remaining == 0 {
		t.Error("limit snapshot not propagated")
	}
	seen := map[string]bool{}
	for _, r := range found {
		seen[r.GitHubID] = true
	}
	for _, id := range missing {
		seen[id] = true
	}
	if len(seen) != len(ids) {
		t.Fatalf("duplicates detected: unique=%d ids=%d", len(seen), len(ids))
	}
}

func TestRunBulkRefreshParallelRespectsConcurrencyCap(t *testing.T) {
	ids := make([]string, 500)
	for i := range ids {
		ids[i] = fmt.Sprintf("G%d", i)
	}

	var active, peak int64
	var mu sync.Mutex

	fetch := func(ctx context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		n := atomic.AddInt64(&active, 1)
		mu.Lock()
		if n > peak {
			peak = n
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt64(&active, -1)
		return nil, batch, RateLimitInfo{}, nil
	}

	concurrency := 3
	_, _, _, err := runBulkRefreshParallel(context.Background(), ids, 100, concurrency, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if peak > int64(concurrency) {
		t.Errorf("peak concurrency %d exceeded cap %d", peak, concurrency)
	}
	if peak < 2 {
		t.Errorf("peak=%d, parallel execution not engaged", peak)
	}
}

func TestRunBulkRefreshParallelPropagatesError(t *testing.T) {
	ids := make([]string, 300)
	for i := range ids {
		ids[i] = fmt.Sprintf("G%d", i)
	}
	sentinel := errors.New("boom")
	var calls int64
	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		if atomic.AddInt64(&calls, 1) == 2 {
			return nil, nil, RateLimitInfo{}, sentinel
		}
		return nil, batch, RateLimitInfo{}, nil
	}
	_, _, _, err := runBulkRefreshParallel(context.Background(), ids, 100, 3, fetch)
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v, want sentinel", err)
	}
}

func TestRunBulkRefreshParallelReturnsPartialDataOnError(t *testing.T) {
	// 5 batches × 100 ids. Batch index 2 errors out; the other 4 succeed.
	// We must see data from the 4 successful batches alongside the error
	// so refresh.Run can persist what came back and compute a meaningful
	// failure rate (I4).
	ids := make([]string, 500)
	for i := range ids {
		ids[i] = fmt.Sprintf("G%d", i)
	}
	sentinel := errors.New("rate limited")
	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		// Deterministic: batch carrying "G200" (start of batch index 2) fails.
		for _, id := range batch {
			if id == "G200" {
				return nil, nil, RateLimitInfo{}, sentinel
			}
		}
		found := make([]RepoData, 0, len(batch))
		for _, id := range batch {
			found = append(found, RepoData{GitHubID: id})
		}
		return found, nil, RateLimitInfo{Remaining: 3000, Cost: 1}, nil
	}
	found, missing, _, err := runBulkRefreshParallel(context.Background(), ids, 100, 1, fetch)
	if !errors.Is(err, sentinel) {
		t.Errorf("err=%v, want sentinel", err)
	}
	// 4 successful batches × 100 = 400 found. 1 failed batch = 0 from it.
	if len(found) != 400 {
		t.Errorf("found=%d, want 400 (partial data from completed shards must survive)", len(found))
	}
	if len(missing) != 0 {
		t.Errorf("missing=%d, want 0", len(missing))
	}
}

func TestRunBulkRefreshParallelReturnsMinRemainingLimit(t *testing.T) {
	ids := make([]string, 300)
	for i := range ids {
		ids[i] = fmt.Sprintf("G%d", i)
	}
	var batchIdx int64
	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		idx := atomic.AddInt64(&batchIdx, 1)
		// Descending remaining per batch: 4000, 3000, 2000. Min = 2000.
		return nil, batch, RateLimitInfo{
			Remaining: 5000 - int(idx)*1000,
			Cost:      1,
			ResetAt:   "2026-04-19T00:00:00Z",
		}, nil
	}
	_, _, limit, err := runBulkRefreshParallel(context.Background(), ids, 100, 3, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if limit.Remaining != 2000 {
		t.Errorf("Remaining=%d, want 2000 (the most conservative across shards)", limit.Remaining)
	}
}

func TestRunBulkRefreshParallelEmptyInput(t *testing.T) {
	fetch := func(_ context.Context, _ []string) ([]RepoData, []string, RateLimitInfo, error) {
		t.Fatal("fetch must not be called for empty input")
		return nil, nil, RateLimitInfo{}, nil
	}
	found, missing, _, err := runBulkRefreshParallel(context.Background(), nil, 100, 3, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 || len(missing) != 0 {
		t.Errorf("expected empty, got found=%d missing=%d", len(found), len(missing))
	}
}
