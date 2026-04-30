import type { SearchIndexEntry, SearchResult } from "./types";

const SCORE_OWNER_NAME_EXACT = 1000;
const SCORE_OWNER_NAME_PREFIX = 500;
const SCORE_OWNER_NAME_SUBSTRING = 200;
const SCORE_DESCRIPTION_SUBSTRING = 50;

/**
 * Score one entry against a lower-cased query. Returns 0 when the query does
 * not match any field — the caller filters those out.
 *
 * Rules (highest first; we keep the highest match per field, not the sum):
 *   1000 — owner or name == query
 *    500 — owner or name starts with query
 *    200 — owner or name contains query
 *     50 — description contains query
 */
export function scoreEntry(e: SearchIndexEntry, q: string): number {
  if (!q) return 0;
  const owner = e.o.toLowerCase();
  const name = e.n.toLowerCase();
  const desc = (e.d ?? "").toLowerCase();

  if (owner === q || name === q) return SCORE_OWNER_NAME_EXACT;
  if (owner.startsWith(q) || name.startsWith(q)) return SCORE_OWNER_NAME_PREFIX;
  if (owner.includes(q) || name.includes(q)) return SCORE_OWNER_NAME_SUBSTRING;
  if (desc.includes(q)) return SCORE_DESCRIPTION_SUBSTRING;
  return 0;
}

/**
 * Substring search across the slim repo index. Empty query returns []. Case
 * insensitive. Ties broken by star count (desc).
 */
export function searchRepos(
  index: SearchIndexEntry[],
  query: string,
  limit = 8,
): SearchResult[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];

  const results: SearchResult[] = [];
  for (const e of index) {
    const score = scoreEntry(e, q);
    if (score > 0) results.push({ entry: e, score });
  }
  results.sort((a, b) => {
    if (a.score !== b.score) return b.score - a.score;
    return b.entry.s - a.entry.s;
  });
  return results.slice(0, limit);
}
