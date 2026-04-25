package translate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashIsDeterministicHexSha256(t *testing.T) {
	h1 := Hash("hello")
	h2 := Hash("hello")
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got %d (%q)", len(h1), h1)
	}
	if Hash("hello") == Hash("world") {
		t.Fatal("different inputs must hash differently")
	}
}

func TestHashSpecVector(t *testing.T) {
	// Lock the algorithm to prevent silent breakage that would invalidate
	// every cache entry across runs.
	got := Hash("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("Hash(\"hello\") = %q, want %q", got, want)
	}
}

func TestCachePathShardsByFirstTwoHexChars(t *testing.T) {
	c := &Cache{Dir: "/tmp/xyz"}
	p := c.Path("ab12cd34")
	want := filepath.Join("/tmp/xyz", "ab", "ab12cd34.json")
	if p != want {
		t.Fatalf("Path = %q, want %q", p, want)
	}
}

func TestCachePathRejectsShortHash(t *testing.T) {
	c := &Cache{Dir: "/tmp/xyz"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on short hash")
		}
	}()
	_ = c.Path("a")
}

func TestCachePutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Dir: dir}

	entry := CacheEntry{
		Src:          "Hello world",
		JA:           "こんにちは世界",
		Provider:     "claude",
		TranslatedAt: "2026-04-26T00:00:00Z",
	}
	if err := c.Put(entry); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok, err := c.Get("Hello world")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get returned ok=false after Put")
	}
	if got != entry {
		t.Fatalf("Get mismatch:\n got  %+v\n want %+v", got, entry)
	}
}

func TestCacheHasMissAndHit(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Dir: dir}

	if has, err := c.Has("never seen"); err != nil || has {
		t.Fatalf("Has(missing) = %v, %v; want false, nil", has, err)
	}

	if err := c.Put(CacheEntry{Src: "seen", JA: "見た", Provider: "mock", TranslatedAt: "now"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if has, err := c.Has("seen"); err != nil || !has {
		t.Fatalf("Has(seen) = %v, %v; want true, nil", has, err)
	}
}

func TestCacheGetMissReturnsFalse(t *testing.T) {
	c := &Cache{Dir: t.TempDir()}
	_, ok, err := c.Get("nope")
	if err != nil {
		t.Fatalf("Get on miss should not error: %v", err)
	}
	if ok {
		t.Fatal("Get on miss should return ok=false")
	}
}

func TestCachePutIsAtomic(t *testing.T) {
	// Atomicity: the json file must never appear partially written. We
	// approximate this by checking that Put leaves no .tmp residue.
	dir := t.TempDir()
	c := &Cache{Dir: dir}
	if err := c.Put(CacheEntry{Src: "x", JA: "x", Provider: "mock", TranslatedAt: "now"}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	var tmpFound bool
	_ = filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tmp") {
			tmpFound = true
		}
		return nil
	})
	if tmpFound {
		t.Fatal("Put left a .tmp file behind")
	}
}

func TestCacheFileFormatIsCanonicalJSON(t *testing.T) {
	// Lock the on-disk format so we can git-commit it and review diffs.
	dir := t.TempDir()
	c := &Cache{Dir: dir}
	entry := CacheEntry{
		Src:          "test",
		JA:           "テスト",
		Provider:     "claude",
		TranslatedAt: "2026-04-26T00:00:00Z",
	}
	if err := c.Put(entry); err != nil {
		t.Fatalf("Put: %v", err)
	}

	path := c.Path(Hash("test"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Fatal("cache file must end with newline (POSIX)")
	}

	var decoded CacheEntry
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, raw)
	}
	if decoded != entry {
		t.Fatalf("round-trip mismatch")
	}
}

func TestCachePutCreatesShardDir(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Dir: dir}
	if err := c.Put(CacheEntry{Src: "abc", JA: "あ", Provider: "mock", TranslatedAt: "now"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	expectedShard := filepath.Join(dir, Hash("abc")[:2])
	st, err := os.Stat(expectedShard)
	if err != nil {
		t.Fatalf("shard dir not created: %v", err)
	}
	if !st.IsDir() {
		t.Fatal("shard path is not a directory")
	}
}
