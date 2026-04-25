// Package translate provides English→Japanese translation of repository
// descriptions, with a content-addressed on-disk cache so re-runs are free
// and the bulk of repos can be seeded once and reused indefinitely.
//
// Cache layout (under data/translations/):
//
//	<sha256[:2]>/<sha256>.json
//
// Sharding by the first two hex chars keeps directory entry counts under
// a few hundred each at our scale (~60k repos).
package translate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CacheEntry is the on-disk JSON shape for one translation.
type CacheEntry struct {
	Src          string `json:"src"`
	JA           string `json:"ja"`
	Provider     string `json:"provider"`      // "claude" | "gemini" | "mock"
	TranslatedAt string `json:"translated_at"` // RFC3339
}

// Cache is a content-addressed translation cache rooted at Dir.
//
// Concurrency: Put is safe across goroutines because writes go through a
// per-file tmp+rename. Get/Has do not mutate state.
type Cache struct {
	Dir string
}

// Hash returns the lowercase hex SHA-256 of src. Stable forever — changing
// the algorithm would invalidate every cached file in data/translations/.
func Hash(src string) string {
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])
}

// Path returns the on-disk path for a given hash. Hash must be at least
// two chars (sharding requires it).
func (c *Cache) Path(hash string) string {
	if len(hash) < 2 {
		panic(fmt.Sprintf("translate: hash too short: %q", hash))
	}
	return filepath.Join(c.Dir, hash[:2], hash+".json")
}

// Has reports whether a translation for src exists on disk.
func (c *Cache) Has(src string) (bool, error) {
	_, err := os.Stat(c.Path(Hash(src)))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Get returns the cached entry for src. ok=false means not cached; err is
// reserved for actual I/O / decode failures.
func (c *Cache) Get(src string) (CacheEntry, bool, error) {
	path := c.Path(Hash(src))
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CacheEntry{}, false, nil
		}
		return CacheEntry{}, false, err
	}
	var e CacheEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		return CacheEntry{}, false, fmt.Errorf("decode %s: %w", path, err)
	}
	return e, true, nil
}

// Put writes the entry. Atomic via tmp+rename so partial files never
// appear in the working tree (and thus never in git).
func (c *Cache) Put(e CacheEntry) error {
	path := c.Path(Hash(e.Src))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
