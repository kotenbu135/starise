export interface RankingEntry {
  rank: number;
  repo_id: string;
  owner: string;
  name: string;
  full_name: string;
  description?: string;
  description_ja?: string;
  language: string;
  created_at?: string;
  start_stars: number;
  end_stars: number;
  star_delta: number;
  growth_pct: number;
}

export type Period = "1d" | "7d" | "30d";

export interface PeriodRankings {
  "1d": RankingEntry[];
  "7d": RankingEntry[];
  "30d": RankingEntry[];
}

export interface RankingsData {
  updated_at: string;
  rankings: PeriodRankings;
}

export const EMPTY_PERIOD_RANKINGS: PeriodRankings = {
  "1d": [],
  "7d": [],
  "30d": [],
};

export interface RepoDetail {
  owner: string;
  name: string;
  description: string;
  description_ja?: string;
  url: string;
  homepage_url: string;
  language: string;
  license: string;
  topics: string[];
  fork_count: number;
  star_count: number;
  is_archived: boolean;
  star_history: { date: string; stars: number }[];
}

export interface Meta {
  generated_at: string;
  total_repos: number;
  periods: string[];
}

export type SortKey = "end_stars" | "star_delta" | "growth_pct";
export type SortDirection = "asc" | "desc";
export type AgeFilter = "30d" | "90d" | "1y" | "all";

// SearchIndexEntry mirrors the slim JSON shape produced by
// batch/internal/export/search.go. Keys are 1 char to keep gzipped
// payload small (~1.5MB for 60k repos).
export interface SearchIndexEntry {
  o: string;          // owner
  n: string;          // name
  l?: string;         // language
  s: number;          // star_count
  d?: string;         // description (ja preferred, fallback en, ≤80 runes)
}

export interface SearchIndex {
  generated_at: string;
  repos: SearchIndexEntry[];
}

export interface SearchResult {
  entry: SearchIndexEntry;
  score: number;
}

// RankSlotKey is one of the six rankings.json keys.
export type RankSlotKey =
  | "1d_breakout"
  | "1d_trending"
  | "7d_breakout"
  | "7d_trending"
  | "30d_breakout"
  | "30d_trending";

export const RANK_SLOT_KEYS: RankSlotKey[] = [
  "1d_breakout",
  "1d_trending",
  "7d_breakout",
  "7d_trending",
  "30d_breakout",
  "30d_trending",
];

// RankLookup: full_name (lower) -> per-slot rank. Built once from
// rankings.json so /search can label otherwise-out-of-rank repos as 圏外.
export type RankLookup = Map<string, Partial<Record<RankSlotKey, number>>>;
