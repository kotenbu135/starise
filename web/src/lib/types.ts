export interface RankingEntry {
  rank: number;
  owner: string;
  name: string;
  description: string;
  language: string;
  license: string;
  star_count: number;
  star_delta: number;
  growth_rate: number;
  url: string;
  created_at: string;
}

export interface RankingsData {
  updated_at: string;
  rankings: {
    "1d": RankingEntry[];
    "7d": RankingEntry[];
    "30d": RankingEntry[];
  };
}

export interface RepoDetail {
  owner: string;
  name: string;
  description: string;
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

export type Period = "1d" | "7d" | "30d";

export type SortKey = "star_count" | "star_delta" | "growth_rate";
export type SortDirection = "asc" | "desc";
export type AgeFilter = "30d" | "90d" | "1y" | "all";
