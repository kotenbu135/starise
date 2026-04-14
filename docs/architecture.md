# アーキテクチャ

## データフロー

```
GitHub Actions (daily cron)
  └─ Go batch (batch/)
       ├─ fetch: GitHub GraphQL API → SQLite
       ├─ compute: growth rate ranking
       └─ export: SQLite → static JSON (data/)
                          ↓
Astro frontend (web/) ← reads JSON at build time → GitHub Pages
```

**設計原則: GitHub だけで完結（外部サービスアカウント不要・月額コストゼロ）**

## ディレクトリ構成

```
starise/
├── batch/                     # Go module (CLI batch processor)
│   ├── main.go                # Cobra CLI entry point
│   ├── cmd/
│   │   ├── root.go            # Root command + global flags
│   │   ├── fetch.go           # fetch --seed-file seeds.txt
│   │   ├── compute.go         # compute (7d/30d growth rate)
│   │   ├── export.go          # export --out-dir ../data
│   │   └── run.go             # fetch + compute + export orchestration
│   ├── internal/
│   │   ├── github/
│   │   │   └── client.go      # GraphQL client + rate limiting
│   │   ├── db/
│   │   │   ├── schema.sql     # DDL (go:embed)
│   │   │   ├── schema.go      # Migrate()
│   │   │   ├── repo.go        # repositories CRUD
│   │   │   └── stars.go       # daily_stars operations
│   │   ├── ranking/
│   │   │   └── compute.go     # Growth rate calculation
│   │   └── export/
│   │       └── json.go        # JSON file generation
│   └── seeds.txt              # Initial repo watch list
├── web/                       # Astro frontend (static export)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── index.astro
│   │   │   └── repo/[owner]/[name].astro
│   │   └── components/
│   │       ├── RankingTable.tsx   # React island (shadcn/ui)
│   │       ├── StarChart.tsx      # React island (Recharts)
│   │       ├── FilterBar.tsx
│   │       └── PeriodToggle.tsx
│   └── public/data/           # JSON placement (copied from data/)
├── data/                      # Generated JSON (git-committed)
│   ├── rankings.json
│   ├── meta.json
│   └── repos/{owner}__{name}.json
├── .github/workflows/
│   ├── batch.yml              # Daily cron: Go batch → JSON → commit
│   └── pages.yml              # Astro build → GitHub Pages
├── docs/                      # Project documentation
└── DESIGN.md                  # UI design contract (jp-ui-contracts)
```

## 技術スタック

| Layer | Technology | Notes |
|-------|-----------|-------|
| Batch | Go + Cobra CLI | `modernc.org/sqlite` (pure Go, no CGO) |
| API | GitHub GraphQL API | `GITHUB_TOKEN` env var |
| DB | SQLite | Runner-local, cached via Actions cache |
| Frontend | Astro + React islands | shadcn/ui, Tailwind CSS, Recharts |
| Hosting | GitHub Pages | Zero cost |

## DB スキーマ (SQLite)

3 テーブル: `repositories`, `daily_stars`, `rankings`

- `repositories`: GitHub リポジトリメタデータ (github_id UNIQUE, owner+name UNIQUE)
- `daily_stars`: 日次スター数スナップショット (repo_id + recorded_date UNIQUE)
- `rankings`: 期間別増加率ランキング (repo_id + period + computed_date UNIQUE)

Growth rate: `star_delta / star_start * 100` (star_start = 0 → delta をそのまま使用)

## JSON 出力スキーマ

- `rankings.json`: `{ updated_at, rankings: { "7d": [...], "30d": [...] } }`
- `repos/{owner}__{name}.json`: リポジトリ詳細 + `star_history: [{ date, stars }]`
- `meta.json`: `{ generated_at, total_repos, periods }`

## 設計判断

- **SQLite over PostgreSQL**: ランナー内完結、外部 DB 不要。Actions cache で永続化
- **Static JSON over API**: 静的サイト。バッチ失敗時も最後の正常データで表示継続
- **Astro over Next.js**: デフォルトゼロ JS。Islands Architecture で必要部分だけ React
- **modernc.org/sqlite over mattn/go-sqlite3**: CGO 不要。GitHub Actions で追加設定なし
- **GraphQL over REST**: スター数 + リポジトリ情報を 1 リクエストで取得可能

## 実装フェーズ

1. **Phase 1 (Go batch)**: DB schema → fetch → compute → export → run → Actions workflow
2. **Phase 2 (Frontend)**: Astro scaffold → ranking table → filters → detail page → star chart
3. **Phase 3 (Deploy)**: pages.yml → GitHub Pages → README
