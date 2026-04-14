import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "../lib/utils";

interface Props {
  currentPage: number;
  totalPages: number;
  total: number;
  perPage: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ currentPage, totalPages, total, perPage, onPageChange }: Props) {
  if (totalPages <= 1) return null;

  const start = (currentPage - 1) * perPage + 1;
  const end = Math.min(currentPage * perPage, total);

  return (
    <div className="flex items-center justify-between pt-4 border-t border-border">
      <p className="text-xs text-text-muted">
        {total}件中 {start}-{end}件
      </p>
      <div className="flex items-center gap-2">
        <button
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage <= 1}
          className={cn(
            "inline-flex items-center justify-center w-9 h-9 rounded-lg border border-border text-sm transition-colors duration-150",
            currentPage <= 1
              ? "opacity-40 cursor-not-allowed"
              : "hover:border-brand hover:text-brand"
          )}
        >
          <ChevronLeft className="w-4 h-4" />
        </button>
        <span className="text-sm tabular-nums text-text-secondary min-w-[4rem] text-center">
          {currentPage} / {totalPages}
        </span>
        <button
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage >= totalPages}
          className={cn(
            "inline-flex items-center justify-center w-9 h-9 rounded-lg border border-border text-sm transition-colors duration-150",
            currentPage >= totalPages
              ? "opacity-40 cursor-not-allowed"
              : "hover:border-brand hover:text-brand"
          )}
        >
          <ChevronRight className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}
