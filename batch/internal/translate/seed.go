package translate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadDescriptionsFromRepoDir scans dir for *.json files and returns the
// deduplicated, non-blank "description" field of each. Used by the
// translate CLI's --source-dir mode (initial Claude seed) where the DB
// is not yet populated with the full data/ tree.
//
// The returned order is the lexical order of filenames so callers get
// deterministic batching across runs.
func ReadDescriptionsFromRepoDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var doc struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode %s: %w", e.Name(), err)
		}
		desc := doc.Description
		if strings.TrimSpace(desc) == "" {
			continue
		}
		if seen[desc] {
			continue
		}
		seen[desc] = true
		out = append(out, desc)
	}
	return out, nil
}
