import { cn } from "../lib/utils";
import type { Period } from "../lib/types";

interface Props {
  period: Period;
  onChange: (period: Period) => void;
  count?: number;
}

const periods: { value: Period; label: string }[] = [
  { value: "1d", label: "1日間" },
  { value: "7d", label: "7日間" },
  { value: "30d", label: "30日間" },
];

export function PeriodToggle({ period, onChange, count }: Props) {
  return (
    <div className="inline-flex rounded-lg border border-border bg-surface p-1">
      {periods.map((p) => (
        <button
          key={p.value}
          onClick={() => onChange(p.value)}
          className={cn(
            "px-5 py-2 text-sm font-medium rounded-md transition-colors duration-150",
            period === p.value
              ? "bg-white text-text-primary shadow-sm"
              : "text-text-secondary hover:text-text-primary"
          )}
        >
          {p.label}
          {period === p.value && count != null && (
            <span className="ml-1.5 text-xs text-text-muted tabular-nums">({count}件)</span>
          )}
        </button>
      ))}
    </div>
  );
}
