import { ArrowUp, ArrowDown, ArrowUpDown } from "lucide-react";
import { cn } from "../lib/utils";
import type { SortKey, SortDirection } from "../lib/types";

interface Props {
  label: string;
  sortKey: SortKey;
  currentKey: SortKey;
  direction: SortDirection;
  onSort: (key: SortKey) => void;
  className?: string;
}

export function SortHeader({ label, sortKey, currentKey, direction, onSort, className }: Props) {
  const active = sortKey === currentKey;
  return (
    <th
      className={cn(
        "py-3 px-2 text-right cursor-pointer select-none transition-colors duration-150",
        active ? "text-brand border-b-2 border-brand" : "hover:text-brand",
        className,
      )}
      onClick={() => onSort(sortKey)}
    >
      <span className="inline-flex items-center justify-end gap-1">
        {label}
        {active ? (
          direction === "desc" ? (
            <ArrowDown className="w-4 h-4" />
          ) : (
            <ArrowUp className="w-4 h-4" />
          )
        ) : (
          <ArrowUpDown className="w-3.5 h-3.5 opacity-40" />
        )}
      </span>
    </th>
  );
}
