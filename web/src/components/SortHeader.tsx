import { ArrowUp, ArrowDown, ArrowUpDown } from "lucide-react";
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
      className={`py-3 px-2 text-right cursor-pointer select-none hover:text-brand transition-colors duration-150 ${className ?? ""}`}
      onClick={() => onSort(sortKey)}
    >
      <span className="inline-flex items-center justify-end gap-1">
        {label}
        {active ? (
          direction === "desc" ? (
            <ArrowDown className="w-3 h-3" />
          ) : (
            <ArrowUp className="w-3 h-3" />
          )
        ) : (
          <ArrowUpDown className="w-3 h-3 opacity-40" />
        )}
      </span>
    </th>
  );
}
