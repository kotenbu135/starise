import { describe, it, expect } from "vitest";
import { searchRepos, scoreEntry } from "./search";
import type { SearchIndexEntry } from "./types";

const fixture: SearchIndexEntry[] = [
  { o: "facebook", n: "react", l: "JavaScript", s: 200000, d: "A library for UI" },
  { o: "vuejs", n: "vue", l: "JavaScript", s: 200000, d: "Progressive framework" },
  { o: "preactjs", n: "preact", l: "JavaScript", s: 30000, d: "Fast 3kB react alternative" },
  { o: "evanyou", n: "react-native-utils", l: "TypeScript", s: 1000, d: "react native utility" },
  { o: "alice", n: "game-engine", l: "Rust", s: 5000, d: "ゲーム エンジン" },
  { o: "bob", n: "no-desc", s: 10 },
];

describe("searchRepos", () => {
  it("returns [] for empty query", () => {
    expect(searchRepos(fixture, "")).toEqual([]);
    expect(searchRepos(fixture, "   ")).toEqual([]);
  });

  it("is case-insensitive", () => {
    const upper = searchRepos(fixture, "REACT");
    const lower = searchRepos(fixture, "react");
    expect(upper.map((r) => r.entry.n)).toEqual(lower.map((r) => r.entry.n));
    expect(lower.length).toBeGreaterThan(0);
  });

  it("ranks exact name match above prefix above substring above description", () => {
    const out = searchRepos(fixture, "react");
    // facebook/react: name == "react" → 1000 (exact)
    // evanyou/react-native-utils: name starts with "react-" → 500 (prefix)
    // preactjs/preact: name "preact" contains "react" → 200 (substring on name)
    //   note: also matches description but name match wins by max-not-sum
    const names = out.map((r) => r.entry.n);
    expect(names[0]).toBe("react");
    expect(names.indexOf("react-native-utils")).toBeGreaterThan(0);
    expect(names.indexOf("react-native-utils")).toBeLessThan(names.indexOf("preact"));
  });

  it("breaks score ties by star count desc", () => {
    // facebook/react and vuejs/vue both have only description match for "framework"?
    // Use a query both would match by substring on name length etc — easier: use s field directly
    const tieFixture: SearchIndexEntry[] = [
      { o: "x", n: "match", s: 100 },
      { o: "x", n: "match-clone", s: 9999 },
    ];
    const out = searchRepos(tieFixture, "match");
    // Both name-prefix matches → tie at 500. Higher stars wins.
    // Wait — "match" is exact (1000), "match-clone" is prefix (500). Different scores.
    // Better: use queries equal for both.
    const tieFixture2: SearchIndexEntry[] = [
      { o: "low", n: "abcfoobar", s: 100 },
      { o: "high", n: "xyzfoobar", s: 9999 },
    ];
    const out2 = searchRepos(tieFixture2, "foobar");
    // both substring matches on name → tie at 200
    expect(out2[0].entry.o).toBe("high");
    expect(out2[1].entry.o).toBe("low");
    // smoke check the original assertion
    expect(out.length).toBe(2);
  });

  it("respects limit", () => {
    const many: SearchIndexEntry[] = Array.from({ length: 50 }, (_, i) => ({
      o: "x",
      n: `react-tool-${i}`,
      s: 1,
    }));
    const out = searchRepos(many, "react", 5);
    expect(out.length).toBe(5);
  });

  it("matches Japanese description", () => {
    const out = searchRepos(fixture, "ゲーム");
    expect(out.length).toBe(1);
    expect(out[0].entry.n).toBe("game-engine");
  });

  it("ignores entries with no matching field", () => {
    const out = searchRepos(fixture, "zzzzzznevermatch");
    expect(out).toEqual([]);
  });
});

describe("scoreEntry", () => {
  it("returns 0 for empty query", () => {
    expect(scoreEntry({ o: "x", n: "y", s: 1 }, "")).toBe(0);
  });

  it("scores exact owner match as 1000", () => {
    expect(scoreEntry({ o: "Foo", n: "bar", s: 1 }, "foo")).toBe(1000);
  });

  it("scores prefix match as 500", () => {
    expect(scoreEntry({ o: "facebook", n: "react", s: 1 }, "fac")).toBe(500);
  });

  it("scores description-only match as 50", () => {
    expect(scoreEntry({ o: "x", n: "y", s: 1, d: "deep learning kit" }, "learning")).toBe(50);
  });

  it("returns 0 when no field matches", () => {
    expect(scoreEntry({ o: "x", n: "y", s: 1, d: "" }, "missing")).toBe(0);
  });
});
