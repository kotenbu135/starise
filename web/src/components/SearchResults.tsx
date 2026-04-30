import { useState, useEffect, useMemo } from "react";
import { Star, Search } from "lucide-react";
import { searchRepos } from "../lib/search";
import { cn, formatNumber } from "../lib/utils";
import type {
  SearchIndex,
  SearchIndexEntry,
  SearchResult,
  RankSlotKey,
  RankLookup,
  RankingsData,
} from "../lib/types";
import { RANK_SLOT_KEYS } from "../lib/types";
import { Pagination } from "./Pagination";

const PER_PAGE = 20;
const MAX_RESULTS = 1000;

interface Props {
  initialQuery: string;
  basePath?: string;
}

const SLOT_LABELS: Record<RankSlotKey, string> = {
  "1d_breakout": "1日Breakout",
  "1d_trending": "1日Trending",
  "7d_breakout": "7日Breakout",
  "7d_trending": "7日Trending",
  "30d_breakout": "30日Breakout",
  "30d_trending": "30日Trending",
};

function buildRankLookup(rk: RankingsData): RankLookup {
  const out: RankLookup = new Map();
  for (const slot of RANK_SLOT_KEYS) {
    const entries = rk.rankings[slot] ?? [];
    for (const e of entries) {
      const key = `${e.owner}/${e.name}`.toLowerCase();
      const cur = out.get(key) ?? {};
      cur[slot] = e.rank;
      out.set(key, cur);
    }
  }
  return out;
}

export function SearchResults({ initialQuery, basePath = "" }: Props) {
  const [query, setQuery] = useState(initialQuery);
  const [index, setIndex] = useState<SearchIndexEntry[] | null>(null);
  const [ranks, setRanks] = useState<RankLookup | null>(null);
  const [page, setPage] = useState(1);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const root = basePath.replace(/\/$/, "");
    Promise.all([
      fetch(`${root}/data/search-index.json`).then((r) => r.json() as Promise<SearchIndex>),
      fetch(`${root}/data/rankings.json`).then((r) => r.json() as Promise<RankingsData>),
    ])
      .then(([si, rk]) => {
        setIndex(si.repos ?? []);
        setRanks(buildRankLookup(rk));
      })
      .catch((err) => {
        console.error("search resources load failed", err);
        setError("検索データの読み込みに失敗しました");
      });
  }, [basePath]);

  useEffect(() => {
    setPage(1);
  }, [query]);

  const results: SearchResult[] = useMemo(() => {
    if (!index || !query.trim()) return [];
    return searchRepos(index, query, MAX_RESULTS);
  }, [index, query]);

  const total = results.length;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));
  const pageItems = results.slice((page - 1) * PER_PAGE, page * PER_PAGE);

  const root = basePath.replace(/\/$/, "");

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="owner / name / 説明文を検索..."
            className="w-full pl-9 pr-3 py-2.5 text-sm border border-border rounded-md bg-white text-text-primary placeholder:text-text-muted focus:outline-none focus:border-brand transition-colors duration-150"
          />
        </div>
        <p className="text-xs text-text-muted tabular-nums">
          {error
            ? error
            : !index || !ranks
            ? "読み込み中..."
            : query.trim() === ""
            ? "クエリを入力してください"
            : `${formatNumber(total)} 件 ${total >= MAX_RESULTS ? `(上位${MAX_RESULTS}件まで表示)` : ""}`}
        </p>
      </div>

      {pageItems.length > 0 && (
        <>
          <div className="overflow-x-auto hidden sm:block">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-text-muted text-xs uppercase tracking-wide border-b border-border">
                  <th className="py-3 px-2">リポジトリ</th>
                  <th className="py-3 px-2 w-24">言語</th>
                  <th className="py-3 px-2 w-24 text-right">スター</th>
                  <th className="py-3 px-2 w-48">ランキング</th>
                </tr>
              </thead>
              <tbody>
                {pageItems.map((r) => (
                  <SearchRow key={`${r.entry.o}/${r.entry.n}`} entry={r.entry} ranks={ranks} root={root} />
                ))}
              </tbody>
            </table>
          </div>

          <div className="sm:hidden space-y-3">
            {pageItems.map((r) => (
              <SearchCard key={`m-${r.entry.o}/${r.entry.n}`} entry={r.entry} ranks={ranks} root={root} />
            ))}
          </div>

          {totalPages > 1 && (
            <Pagination page={page} totalPages={totalPages} onChange={setPage} />
          )}
        </>
      )}

      {index && ranks && query.trim() !== "" && pageItems.length === 0 && !error && (
        <div className="text-center py-16">
          <p className="text-text-muted text-sm">該当するリポジトリがありません</p>
        </div>
      )}
    </div>
  );
}

function rankBadges(entry: SearchIndexEntry, ranks: RankLookup | null) {
  if (!ranks) return null;
  const key = `${entry.o}/${entry.n}`.toLowerCase();
  const hits = ranks.get(key);
  if (!hits || Object.keys(hits).length === 0) {
    return <span className="text-xs text-text-muted">圏外</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {RANK_SLOT_KEYS.map((slot) => {
        const r = hits[slot];
        if (r == null) return null;
        return (
          <span
            key={slot}
            title={SLOT_LABELS[slot]}
            className="inline-flex items-center gap-1 text-[10px] font-medium bg-surface border border-border rounded px-1.5 py-0.5 text-text-secondary"
          >
            <span className="text-brand">{SLOT_LABELS[slot]}</span>
            <span className="tabular-nums">#{r}</span>
          </span>
        );
      })}
    </div>
  );
}

function SearchRow({
  entry,
  ranks,
  root,
}: {
  entry: SearchIndexEntry;
  ranks: RankLookup | null;
  root: string;
}) {
  return (
    <tr className="border-b border-border/50 hover:bg-surface transition-colors duration-150">
      <td className="py-3 px-2 max-w-0">
        <a
          href={`${root}/repo/${entry.o}/${entry.n}`}
          className="font-mono text-[13px] font-medium text-brand hover:text-brand-hover transition-colors duration-150 block truncate"
        >
          {entry.o}/{entry.n}
        </a>
        {entry.d && (
          <p className="text-xs text-text-secondary mt-1 line-clamp-2 leading-relaxed">{entry.d}</p>
        )}
      </td>
      <td className="py-3 px-2">
        {entry.l && <span className="text-xs font-medium text-text-secondary">{entry.l}</span>}
      </td>
      <td className="py-3 px-2 text-right">
        <span className="tabular-nums inline-flex items-center justify-end gap-1">
          <Star className="w-3.5 h-3.5 text-accent" />
          {formatNumber(entry.s)}
        </span>
      </td>
      <td className="py-3 px-2">{rankBadges(entry, ranks)}</td>
    </tr>
  );
}

function SearchCard({
  entry,
  ranks,
  root,
}: {
  entry: SearchIndexEntry;
  ranks: RankLookup | null;
  root: string;
}) {
  return (
    <a
      href={`${root}/repo/${entry.o}/${entry.n}`}
      className={cn(
        "block border border-border rounded-lg p-4 transition-colors duration-150",
        "hover:border-brand/30 hover:bg-surface",
      )}
    >
      <div className="font-mono text-[13px] font-medium text-brand truncate">
        {entry.o}/{entry.n}
      </div>
      {entry.d && (
        <p className="text-xs text-text-secondary mt-2 line-clamp-2 leading-relaxed">{entry.d}</p>
      )}
      <div className="flex items-center gap-3 mt-3 text-xs flex-wrap">
        {entry.l && <span className="text-text-secondary">{entry.l}</span>}
        <span className="tabular-nums inline-flex items-center gap-1">
          <Star className="w-3 h-3 text-accent" />
          {formatNumber(entry.s)}
        </span>
      </div>
      <div className="mt-2">{rankBadges(entry, ranks)}</div>
    </a>
  );
}
