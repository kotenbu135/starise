import { useState, useMemo } from "react";
import { ArrowUp, ArrowDown, Star } from "lucide-react";
import { cn, formatNumber, formatGrowthRate } from "../lib/utils";
import type { RankingEntry, Period, SortKey, SortDirection, AgeFilter } from "../lib/types";
import { filterByAge, sortEntries, paginate } from "../lib/ranking";
import { PeriodToggle } from "./PeriodToggle";
import { FilterBar } from "./FilterBar";
import { SortHeader } from "./SortHeader";
import { Pagination } from "./Pagination";

const PER_PAGE = 20;

interface Props {
  rankings: {
    "7d": RankingEntry[];
    "30d": RankingEntry[];
  };
  updatedAt: string;
  basePath?: string;
}

export function RankingTable({ rankings, updatedAt, basePath = "" }: Props) {
  const [period, setPeriod] = useState<Period>("7d");
  const [langFilter, setLangFilter] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("growth_rate");
  const [sortDir, setSortDir] = useState<SortDirection>("desc");
  const [ageFilter, setAgeFilter] = useState<AgeFilter>("all");
  const [page, setPage] = useState(1);

  const entries = rankings[period] ?? [];

  const languages = useMemo(() => {
    const set = new Set<string>();
    for (const e of entries) {
      if (e.language) set.add(e.language);
    }
    return Array.from(set).sort();
  }, [entries]);

  // Data pipeline: lang filter → age filter → sort → paginate
  const processed = useMemo(() => {
    let result = entries;
    if (langFilter) result = result.filter((e) => e.language === langFilter);
    result = filterByAge(result, ageFilter);
    result = sortEntries(result, sortKey, sortDir);
    return result;
  }, [entries, langFilter, ageFilter, sortKey, sortDir]);

  const { items: pageItems, totalPages, total } = useMemo(
    () => paginate(processed, page, PER_PAGE),
    [processed, page],
  );

  const handleSort = (key: SortKey) => {
    if (key === sortKey) {
      setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
    setPage(1);
  };

  const handleLangChange = (lang: string) => {
    setLangFilter(lang);
    setPage(1);
  };

  const handleAgeChange = (age: AgeFilter) => {
    setAgeFilter(age);
    setPage(1);
  };

  const handlePeriodChange = (p: Period) => {
    setPeriod(p);
    setPage(1);
  };

  // Use display index instead of rank when custom sort is active
  const isDefaultSort = sortKey === "growth_rate" && sortDir === "desc";

  const updatedDate = new Date(updatedAt).toLocaleDateString("ja-JP", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <div className="space-y-6">
      {/* Controls */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <PeriodToggle period={period} onChange={handlePeriodChange} />
        <p className="text-xs text-text-muted">
          最終更新: {updatedDate}
        </p>
      </div>

      {languages.length > 0 && (
        <FilterBar
          languages={languages}
          selected={langFilter}
          onChange={handleLangChange}
          ageFilter={ageFilter}
          onAgeChange={handleAgeChange}
        />
      )}

      {/* Desktop table */}
      <div className="hidden sm:block overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left text-xs text-text-muted">
              <th className="py-3 pr-2 w-12">#</th>
              <th className="py-3 px-2">リポジトリ</th>
              <th className="py-3 px-2 w-24">言語</th>
              <SortHeader
                label="スター"
                sortKey="star_count"
                currentKey={sortKey}
                direction={sortDir}
                onSort={handleSort}
                className="w-28"
              />
              <SortHeader
                label="増減"
                sortKey="star_delta"
                currentKey={sortKey}
                direction={sortDir}
                onSort={handleSort}
                className="w-28"
              />
              <SortHeader
                label="増加率"
                sortKey="growth_rate"
                currentKey={sortKey}
                direction={sortDir}
                onSort={handleSort}
                className="w-28 pl-2"
              />
            </tr>
          </thead>
          <tbody>
            {pageItems.map((entry, idx) => (
              <tr
                key={`${entry.owner}/${entry.name}`}
                className="border-b border-border/50 hover:bg-surface transition-colors duration-150"
              >
                <td className="py-3 pr-2">
                  <span className="text-base font-bold tabular-nums text-text-secondary">
                    {isDefaultSort ? entry.rank : (page - 1) * PER_PAGE + idx + 1}
                  </span>
                </td>
                <td className="py-3 px-2">
                  <div>
                    <a
                      href={`${basePath}/repo/${entry.owner}/${entry.name}`}
                      className="font-mono text-[13px] font-medium text-brand hover:text-brand-hover transition-colors duration-150"
                    >
                      {entry.owner}/{entry.name}
                    </a>
                    <p className="text-text-secondary text-xs mt-0.5 truncate max-w-[400px]">
                      {entry.description}
                    </p>
                  </div>
                </td>
                <td className="py-3 px-2">
                  {entry.language && (
                    <span className="text-xs font-medium text-text-secondary">
                      {entry.language}
                    </span>
                  )}
                </td>
                <td className="py-3 px-2 text-right">
                  <span className="tabular-nums flex items-center justify-end gap-1">
                    <Star className="w-3.5 h-3.5 text-accent" />
                    {formatNumber(entry.star_count)}
                  </span>
                </td>
                <td className="py-3 px-2 text-right">
                  <DeltaCell delta={entry.star_delta} />
                </td>
                <td className="py-3 pl-2 text-right">
                  <GrowthCell rate={entry.growth_rate} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Mobile cards */}
      <div className="sm:hidden space-y-3">
        {pageItems.map((entry, idx) => (
          <a
            key={`${entry.owner}/${entry.name}`}
            href={`${basePath}/repo/${entry.owner}/${entry.name}`}
            className="block border border-border rounded-lg p-4 hover:border-brand/30 hover:bg-surface transition-colors duration-150"
          >
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-base font-bold tabular-nums text-text-secondary shrink-0">
                    #{isDefaultSort ? entry.rank : (page - 1) * PER_PAGE + idx + 1}
                  </span>
                  <span className="font-mono text-[13px] font-medium text-brand truncate">
                    {entry.owner}/{entry.name}
                  </span>
                </div>
                <p className="text-text-secondary text-xs mt-1 line-clamp-2">
                  {entry.description}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-4 mt-3 text-xs">
              {entry.language && (
                <span className="text-text-secondary">{entry.language}</span>
              )}
              <span className="tabular-nums flex items-center gap-1">
                <Star className="w-3 h-3 text-accent" />
                {formatNumber(entry.star_count)}
              </span>
              <DeltaCell delta={entry.star_delta} />
              <GrowthCell rate={entry.growth_rate} />
            </div>
          </a>
        ))}
      </div>

      {pageItems.length === 0 && (
        <p className="text-center py-12 text-text-muted text-sm">
          該当するリポジトリがありません
        </p>
      )}

      <Pagination
        currentPage={page}
        totalPages={totalPages}
        total={total}
        perPage={PER_PAGE}
        onPageChange={setPage}
      />
    </div>
  );
}

function DeltaCell({ delta }: { delta: number }) {
  if (delta === 0) return <span className="tabular-nums text-text-muted">0</span>;
  const positive = delta > 0;
  return (
    <span
      className={cn(
        "tabular-nums inline-flex items-center gap-0.5",
        positive ? "text-success" : "text-danger"
      )}
    >
      {positive ? (
        <ArrowUp className="w-3 h-3" />
      ) : (
        <ArrowDown className="w-3 h-3" />
      )}
      {positive ? "+" : ""}
      {formatNumber(Math.abs(delta))}
    </span>
  );
}

function GrowthCell({ rate }: { rate: number }) {
  if (rate === 0) return <span className="tabular-nums text-text-muted">0%</span>;
  const positive = rate > 0;
  return (
    <span
      className={cn(
        "tabular-nums font-medium",
        positive ? "text-success" : "text-danger"
      )}
    >
      {formatGrowthRate(rate)}
    </span>
  );
}
