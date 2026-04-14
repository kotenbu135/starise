import type { RankingEntry, SortKey, SortDirection, AgeFilter } from "./types";

const AGE_DAYS: Record<Exclude<AgeFilter, "all">, number> = {
  "30d": 30,
  "90d": 90,
  "1y": 365,
};

export function filterByAge(entries: RankingEntry[], filter: AgeFilter): RankingEntry[] {
  if (filter === "all") return entries;
  const maxDays = AGE_DAYS[filter];
  const now = Date.now();
  return entries.filter((e) => {
    if (!e.created_at) return true;
    const created = new Date(e.created_at).getTime();
    if (isNaN(created)) return true;
    return (now - created) / 86_400_000 <= maxDays;
  });
}

export function sortEntries(
  entries: RankingEntry[],
  key: SortKey,
  dir: SortDirection,
): RankingEntry[] {
  return [...entries].sort((a, b) => {
    const diff = a[key] - b[key];
    return dir === "desc" ? -diff : diff;
  });
}

export function paginate<T>(
  items: T[],
  page: number,
  perPage: number,
): { items: T[]; totalPages: number; total: number } {
  const total = items.length;
  const totalPages = Math.max(1, Math.ceil(total / perPage));
  const safePage = Math.min(Math.max(1, page), totalPages);
  return {
    items: items.slice((safePage - 1) * perPage, safePage * perPage),
    totalPages,
    total,
  };
}
