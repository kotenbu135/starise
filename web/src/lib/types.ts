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
