import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from "recharts";
import { formatNumber } from "../lib/utils";

interface Props {
  data: { date: string; stars: number }[];
}

export function StarChart({ data }: Props) {
  if (data.length < 2) {
    return (
      <p className="text-center py-8 text-text-muted text-sm">
        グラフ表示には2日以上のデータが必要です
      </p>
    );
  }

  return (
    <div className="w-full h-[300px]">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#E2E8F0" />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 12, fill: "#64748B" }}
            tickFormatter={(v: string) => {
              const d = new Date(v);
              return `${d.getMonth() + 1}/${d.getDate()}`;
            }}
          />
          <YAxis
            tick={{ fontSize: 12, fill: "#64748B" }}
            tickFormatter={(v: number) => formatNumber(v)}
            width={60}
          />
          <Tooltip
            contentStyle={{
              border: "1px solid #E2E8F0",
              borderRadius: "8px",
              fontSize: "13px",
              boxShadow: "0 4px 12px rgba(0,0,0,0.08)",
            }}
            formatter={(value: number) => [
              value.toLocaleString("ja-JP"),
              "スター数",
            ]}
            labelFormatter={(label: string) => {
              const d = new Date(label);
              return d.toLocaleDateString("ja-JP", {
                year: "numeric",
                month: "long",
                day: "numeric",
              });
            }}
          />
          <Line
            type="monotone"
            dataKey="stars"
            stroke="#2563EB"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, fill: "#2563EB" }}
            animationDuration={500}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
