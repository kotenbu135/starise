import { useState, useRef, useEffect } from "react";
import { ChevronDown, SlidersHorizontal } from "lucide-react";
import { cn } from "../lib/utils";
import type { AgeFilter } from "../lib/types";

const TOP_COUNT = 6;

interface Props {
  languages: string[];
  languageCounts: Map<string, number>;
  selected: string;
  onChange: (lang: string) => void;
  ageFilter: AgeFilter;
  onAgeChange: (age: AgeFilter) => void;
}

const AGE_OPTIONS: { value: AgeFilter; label: string }[] = [
  { value: "all", label: "全期間" },
  { value: "30d", label: "30日以内" },
  { value: "90d", label: "90日以内" },
  { value: "1y", label: "1年以内" },
];

export function FilterBar({ languages, languageCounts, selected, onChange, ageFilter, onAgeChange }: Props) {
  const topLangs = languages.slice(0, TOP_COUNT);
  const moreLangs = languages.slice(TOP_COUNT);
  const [open, setOpen] = useState(false);
  const [mobileExpanded, setMobileExpanded] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const pillClass = (active: boolean) =>
    cn(
      "px-3 py-1 text-xs font-medium rounded-full border transition-colors duration-150",
      active
        ? "bg-brand text-white border-brand"
        : "bg-white text-text-secondary border-border hover:border-brand hover:text-brand"
    );

  const selectedInMore = moreLangs.includes(selected);
  const hasActiveFilter = selected !== "" || ageFilter !== "all";

  const filterContent = (
    <div className="space-y-3">
      {/* Language filter */}
      <div className="flex flex-wrap items-center gap-2">
        <button onClick={() => onChange("")} className={pillClass(selected === "")}>
          すべて
        </button>
        {topLangs.map((lang) => (
          <button key={lang} onClick={() => onChange(lang)} className={pillClass(selected === lang)}>
            {lang}
            <span className="ml-1 opacity-60">({languageCounts.get(lang) ?? 0})</span>
          </button>
        ))}
        {moreLangs.length > 0 && (
          <div className="relative" ref={dropdownRef}>
            <button
              onClick={() => setOpen((o) => !o)}
              className={cn(
                pillClass(selectedInMore),
                "inline-flex items-center gap-1"
              )}
            >
              {selectedInMore ? selected : `その他 (${moreLangs.length})`}
              <ChevronDown className="w-3 h-3" />
            </button>
            {open && (
              <div className="absolute top-full left-0 mt-1 z-50 bg-white border border-border rounded-lg shadow-md max-h-60 overflow-y-auto min-w-[160px]">
                {moreLangs.map((lang) => (
                  <button
                    key={lang}
                    onClick={() => { onChange(lang); setOpen(false); }}
                    className={cn(
                      "block w-full text-left px-3 py-2 text-xs hover:bg-surface transition-colors duration-150",
                      selected === lang ? "text-brand font-medium" : "text-text-secondary"
                    )}
                  >
                    {lang}
                    <span className="ml-1 opacity-60">({languageCounts.get(lang) ?? 0})</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Age filter */}
      <div className="flex flex-wrap gap-2">
        <span className="text-xs text-text-muted leading-6">リポジトリ作成日:</span>
        {AGE_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => onAgeChange(opt.value)}
            className={pillClass(ageFilter === opt.value)}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );

  return (
    <>
      {/* Mobile: collapsible */}
      <div className="sm:hidden">
        <button
          onClick={() => setMobileExpanded((e) => !e)}
          className="flex items-center gap-2 text-sm text-text-secondary hover:text-brand transition-colors duration-150"
        >
          <SlidersHorizontal className="w-4 h-4" />
          フィルタ
          {hasActiveFilter && (
            <span className="w-2 h-2 rounded-full bg-brand" />
          )}
          <ChevronDown className={cn("w-4 h-4 transition-transform duration-150", mobileExpanded && "rotate-180")} />
        </button>
        {mobileExpanded && <div className="mt-3">{filterContent}</div>}
      </div>

      {/* Desktop: always visible */}
      <div className="hidden sm:block">
        {filterContent}
      </div>
    </>
  );
}
