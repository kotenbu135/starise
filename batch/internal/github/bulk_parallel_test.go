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

func TestRunBulkRefreshParallelAggregatesCostAsSum(t *testing.T) {
	// Post-2026-04-20 observability: the aggregate RateLimitInfo returned
	// from runBulkRefreshParallel must sum Cost across all successful
	// batches so refresh.Run can report the true budget consumption of a
	// single run. Verifies the documented semantic shift — the per-call
	// Cost is a point cost, the aggregated Cost is a sum.
	ids := make([]string, 300) // 3 batches of 100
	for i := range ids {
		ids[i] = fmt.Sprintf("G%d", i)
	}
	var batchIdx int64
	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		idx := atomic.AddInt64(&batchIdx, 1)
		// Per-batch costs: 10, 15, 20. Sum=45, Max=20.
		return nil, batch, RateLimitInfo{
			Remaining: 5000 - int(idx)*10,
			Cost:      5 + int(idx)*5,
			ResetAt:   "2026-04-19T00:00:00Z",
		}, nil
	}
	_, _, limit, err := runBulkRefreshParallel(context.Background(), ids, 100, 1, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if limit.Cost != 45 {
		t.Errorf("aggregated Cost=%d, want 45 (sum of 10+15+20)", limit.Cost)
	}
	if limit.MaxBatchCost != 20 {
		t.Errorf("MaxBatchCost=%d, want 20", limit.MaxBatchCost)
	}
}

func TestExtractMissingNodeID_HappyPath(t *testing.T) {
	// GitHub GraphQL reports a deleted/private node inside a nodes(ids:[])
	// batch with the exact phrase below. shurcooL surfaces this as a Go
	// error — we extract the id so the caller can drop it and retry.
	err := errors.New("Could not resolve to a node with the global id of 'R_kgDOSIhctg'.")
	id, ok := extractMissingNodeID(err)
	if !ok {
		t.Fatal("want ok=true for canonical message")
	}
	if id != "R_kgDOSIhctg" {
		t.Errorf("id=%q, want R_kgDOSIhctg", id)
	}
}

func TestExtractMissingNodeID_WrappedError(t *testing.T) {
	// runBulkRefreshParallel wraps with fmt.Errorf("batch %d (%d ids): %w").
	// The extractor must still find the id inside the wrapper.
	wrapped := fmt.Errorf("batch 52 (100 ids): %w",
		errors.New("Could not resolve to a node with the global id of 'R_xyz'."))
	id, ok := extractMissingNodeID(wrapped)
	if !ok || id != "R_xyz" {
		t.Errorf("got (%q, %v), want (R_xyz, true)", id, ok)
	}
}

func TestExtractMissingNodeID_NoMatch(t *testing.T) {
	for _, in := range []error{
		nil,
		errors.New("some other error"),
		errors.New("rate limit exceeded"),
	} {
		id, ok := extractMissingNodeID(in)
		if ok || id != "" {
			t.Errorf("in=%v → got (%q, %v), want (\"\", false)", in, id, ok)
		}
	}
}

func TestRecoverBatchFromMissing_DropsOffendingIDThenSucceeds(t *testing.T) {
	// The 2026-04-22 production failure: 1 deleted repo inside a batch of
	// 100 caused the entire batch to abort. recoverBatchFromMissing must
	// strip the bad id, mark it missing, and re-query the remainder.
	calls := 0
	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		calls++
		// First attempt: 100 ids → one bad. Second attempt: 99 ids → success.
		for _, id := range batch {
			if id == "R_BAD" {
				return nil, nil, RateLimitInfo{},
					errors.New("Could not resolve to a node with the global id of 'R_BAD'.")
			}
		}
		found := make([]RepoData, 0, len(batch))
		for _, id := range batch {
			found = append(found, RepoData{GitHubID: id})
		}
		return found, nil, RateLimitInfo{Remaining: 4000, Cost: 1}, nil
	}

	batch := []string{"R_A", "R_BAD", "R_B", "R_C"}
	found, missing, limit, err := recoverBatchFromMissing(context.Background(), batch, fetch, 10)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls=%d, want 2 (one failed attempt, one retry)", calls)
	}
	if len(found) != 3 {
		t.Errorf("found=%d, want 3", len(found))
	}
	if len(missing) != 1 || missing[0] != "R_BAD" {
		t.Errorf("missing=%v, want [R_BAD]", missing)
	}
	if limit.Remaining != 4000 {
		t.Errorf("limit not propagated from final successful call")
	}
}

func TestRecoverBatchFromMissing_MultipleBadIDs(t *testing.T) {
	// Several dead repos in one batch: the helper must peel them off
	// iteratively. Each attempt exposes at most one bad id (GitHub returns
	// on the first failure).
	bad := map[string]bool{"R_X": true, "R_Y": true, "R_Z": true}
	fetch := func(_ context.Context, batch []string) ([]RepoData, []string, RateLimitInfo, error) {
		for _, id := range batch {
			if bad[id] {
				return nil, nil, RateLimitInfo{},
					fmt.Errorf("Could not resolve to a node with the global id of '%s'.", id)
			}
		}
		found := make([]RepoData, 0, len(batch))
		for _, id := range batch {
			found = append(found, RepoData{GitHubID: id})
		}
		return found, nil, RateLimitInfo{Remaining: 3000, Cost: 1}, nil
	}

	batch := []string{"R_A", "R_X", "R_B", "R_Y", "R_C", "R_Z"}
	found, missing, _, err := recoverBatchFromMissing(context.Background(), batch, fetch, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Errorf("found=%d, want 3 (good ids)", len(found))
	}
	if len(missing) != 3 {
		t.Errorf("missing=%d, want 3 (bad ids)", len(missing))
	}
}

func TestRecoverBatchFromMissing_UnrelatedErrorSurfaces(t *testing.T) {
	sentinel := errors.New("network down")
	fetch := func(_ context.Context, _ []string) ([]RepoData, []string, RateLimitInfo, error) {
		return nil, nil, RateLimitInfo{}, sentinel
	}
	_, _, _, err := recoverBatchFromMissing(context.Background(), []string{"a", "b"}, fetch, 10)
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v, want sentinel (non-NOT_FOUND errors must propagate)", err)
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
