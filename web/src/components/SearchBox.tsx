import { useState, useRef, useEffect, useCallback } from "react";
import { Search } from "lucide-react";
import { searchRepos } from "../lib/search";
import type { SearchIndex, SearchIndexEntry, SearchResult } from "../lib/types";
import { cn } from "../lib/utils";

interface Props {
  basePath?: string;
}

const DROPDOWN_LIMIT = 8;

let cachedIndex: SearchIndexEntry[] | null = null;
let inflightFetch: Promise<SearchIndexEntry[]> | null = null;

async function loadIndex(basePath: string): Promise<SearchIndexEntry[]> {
  if (cachedIndex) return cachedIndex;
  if (inflightFetch) return inflightFetch;
  const url = `${basePath.replace(/\/$/, "")}/data/search-index.json`;
  inflightFetch = fetch(url)
    .then((r) => {
      if (!r.ok) throw new Error(`search-index ${r.status}`);
      return r.json() as Promise<SearchIndex>;
    })
    .then((data) => {
      cachedIndex = data.repos ?? [];
      return cachedIndex;
    })
    .catch((err) => {
      inflightFetch = null;
      throw err;
    });
  return inflightFetch;
}

export function SearchBox({ basePath = "" }: Props) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [open, setOpen] = useState(false);
  const [highlighted, setHighlighted] = useState(-1);
  const [loaded, setLoaded] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const ensureLoaded = useCallback(async () => {
    if (loaded || cachedIndex) {
      setLoaded(true);
      return;
    }
    try {
      await loadIndex(basePath);
      setLoaded(true);
    } catch (err) {
      console.error("search-index load failed", err);
    }
  }, [basePath, loaded]);

  useEffect(() => {
    if (!cachedIndex) return;
    if (!query.trim()) {
      setResults([]);
      return;
    }
    setResults(searchRepos(cachedIndex, query, DROPDOWN_LIMIT));
    setHighlighted(-1);
  }, [query, loaded]);

  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  const goSearch = (q: string) => {
    if (!q.trim()) return;
    window.location.href = `${basePath.replace(/\/$/, "")}/search?q=${encodeURIComponent(q.trim())}`;
  };

  const goRepo = (e: SearchIndexEntry) => {
    window.location.href = `${basePath.replace(/\/$/, "")}/repo/${e.o}/${e.n}`;
  };

  const onKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlighted((h) => Math.min(h + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlighted((h) => Math.max(h - 1, -1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (highlighted >= 0 && results[highlighted]) {
        goRepo(results[highlighted].entry);
      } else {
        goSearch(query);
      }
    } else if (e.key === "Escape") {
      setOpen(false);
      inputRef.current?.blur();
    }
  };

  return (
    <div ref={containerRef} className="relative w-full sm:w-72">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          placeholder="リポジトリを検索..."
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => {
            setOpen(true);
            ensureLoaded();
          }}
          onKeyDown={onKey}
          className="w-full pl-9 pr-3 py-2 text-sm border border-border rounded-md bg-white text-text-primary placeholder:text-text-muted focus:outline-none focus:border-brand transition-colors duration-150"
        />
      </div>

      {open && query.trim() !== "" && (
        <div className="absolute top-full left-0 right-0 mt-1 z-50 bg-white border border-border rounded-md shadow-md max-h-96 overflow-y-auto">
          {!loaded && <div className="px-3 py-3 text-xs text-text-muted">読み込み中...</div>}
          {loaded && results.length === 0 && (
            <div className="px-3 py-3 text-xs text-text-muted">該当するリポジトリがありません</div>
          )}
          {loaded && results.map((r, i) => (
            <button
              key={`${r.entry.o}/${r.entry.n}`}
              onMouseEnter={() => setHighlighted(i)}
              onClick={() => goRepo(r.entry)}
              className={cn(
                "block w-full text-left px-3 py-2 border-b border-border last:border-0 transition-colors duration-150",
                highlighted === i ? "bg-surface" : "hover:bg-surface",
              )}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium text-text-primary truncate">
                  {r.entry.o}/{r.entry.n}
                </span>
                {r.entry.l && (
                  <span className="text-[10px] text-text-muted shrink-0">{r.entry.l}</span>
                )}
              </div>
              {r.entry.d && (
                <div className="text-xs text-text-secondary truncate mt-0.5">{r.entry.d}</div>
              )}
            </button>
          ))}
          {loaded && results.length > 0 && (
            <button
              onClick={() => goSearch(query)}
              className="block w-full text-left px-3 py-2 text-xs text-brand hover:bg-surface transition-colors duration-150"
            >
              「{query}」の全結果を見る →
            </button>
          )}
        </div>
      )}
    </div>
  );
}
