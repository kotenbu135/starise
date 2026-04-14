import { cn } from "../lib/utils";
import type { AgeFilter } from "../lib/types";

interface Props {
  languages: string[];
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

export function FilterBar({ languages, selected, onChange, ageFilter, onAgeChange }: Props) {
  return (
    <div className="space-y-3">
      {/* Language filter */}
      <div className="flex flex-wrap gap-2">
        <button
          onClick={() => onChange("")}
          className={cn(
            "px-3 py-1 text-xs font-medium rounded-full border transition-colors duration-150",
            selected === ""
              ? "bg-brand text-white border-brand"
              : "bg-white text-text-secondary border-border hover:border-brand hover:text-brand"
          )}
        >
          すべて
        </button>
        {languages.map((lang) => (
          <button
            key={lang}
            onClick={() => onChange(lang)}
            className={cn(
              "px-3 py-1 text-xs font-medium rounded-full border transition-colors duration-150",
              selected === lang
                ? "bg-brand text-white border-brand"
                : "bg-white text-text-secondary border-border hover:border-brand hover:text-brand"
            )}
          >
            {lang}
          </button>
        ))}
      </div>

      {/* Age filter */}
      <div className="flex flex-wrap gap-2">
        <span className="text-xs text-text-muted leading-6">作成日:</span>
        {AGE_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => onAgeChange(opt.value)}
            className={cn(
              "px-3 py-1 text-xs font-medium rounded-full border transition-colors duration-150",
              ageFilter === opt.value
                ? "bg-brand text-white border-brand"
                : "bg-white text-text-secondary border-border hover:border-brand hover:text-brand"
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );
}
