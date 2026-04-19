package github

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestMockClientParallelAccessIsRaceFree exercises MockClient from many
// goroutines at once. Run under `go test -race` to catch unsynchronized
// map access. Covers BulkRefresh + SearchRepos + Add interleavings so
// regressions in mutex coverage fail loudly in CI.
func TestMockClientParallelAccessIsRaceFree(t *testing.T) {
	c := NewMockClient()
	for i := 0; i < 50; i++ {
		c.Add(RepoData{GitHubID: fmt.Sprintf("G%d", i), Owner: "x", Name: fmt.Sprintf("r%d", i)})
	}
	c.SearchResult = []RepoData{{GitHubID: "G0", Owner: "x", Name: "r0"}}

	ctx := context.Background()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ids := make([]string, 20)
			for i := range ids {
				ids[i] = fmt.Sprintf("G%d", i)
			}
			for i := 0; i < 20; i++ {
				c.BulkRefresh(ctx, ids)
				c.SearchRepos(ctx, SearchOptions{Query: fmt.Sprintf("q%d", g)})
				c.Add(RepoData{
					GitHubID: fmt.Sprintf("G%d-%d", g, i),
					Owner:    "x",
					Name:     fmt.Sprintf("new%d-%d", g, i),
				})
			}
		}(g)
	}
	wg.Wait()
}
