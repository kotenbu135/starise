# アーキテクチャ

## データフロー

```
GitHub Actions (daily cron)
  └─ Go batch (batch/) — v3 invariant-driven pipeline
       ├─ restore : data/repos/*.json → SQLite (source of truth, I11)
       ├─ fetch   : seeds.txt + GraphQL FetchRepo → today snapshot
       ├─ discover: Search API → new repos + today snapshot
       ├─ refresh : bulk nodes() → today snapshot for all non-deleted (I4)
       ├─ compute : 2-axis ranking — breakout + trending × 1d/7d/30d (I5/I6/I7)
       └─ export  : SQLite → JSON tree (data/) — deterministic (I13)
                          ↓
Astro frontend (web/) ← reads JSON at build time → GitHub Pages
```

**設計原則: GitHub だけで完結（外部サービスアカウント不要・月額コストゼロ）**

**正当性は invariant で担保**: 13 個の不変条件 (I1-I13) が
`batch/internal/pipeline/invariant_iN_test.go` に常駐し、CI で全 GREEN 必須。

## ディレクトリ構成

```
starise/
├── batch/                     # Go module (CLI batch processor, v3)
│   ├── main.go                # Cobra CLI entry point
│   ├── cmd/                   # subcommand wiring (root/fetch/discover/refresh/compute/export/restore/run)
│   ├── internal/
│   │   ├── github/            # Client interface + MockClient + GraphQLClient (FetchRepo, SearchRepos, BulkRefresh)
│   │   ├── db/                # schema.sql + Migrate + repo/stars/rankings CRUD (deleted_at, rank_type)
│   │   ├── ranking/           # 2-axis: breakout (delta) + trending (%), Compute, Validate, MacroValidate
│   │   ├── fetch/             # seed-by-seed FetchRepo + today snapshot
│   │   ├── discover/          # SearchRepos + insert/refresh today snapshot
│   │   ├── refresh/           # bulk nodes() refresh + soft-delete on 404 + I4 failure threshold
│   │   ├── restore/           # data/repos/*.json → DB (source of truth)
│   │   ├── export/            # schema.go (RepoDetail/Rankings/Meta), json.go (deterministic), cleanup.go (orphan + 90d hard delete)
│   │   └── pipeline/          # RunAll orchestrator + RunSimulationDay + invariant_iN_test.go (I1-I13)
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
  - 新カラム `deleted_at TEXT`: 空 = active、`YYYY-MM-DD` = soft delete (90 日後に hard delete)
- `daily_stars`: 日次スター数スナップショット (repo_id + recorded_date UNIQUE)
- `rankings`: 期間別増加率ランキング (repo_id + period + **rank_type** + computed_date UNIQUE)
  - `rank_type`: `'breakout'` (新興発見) または `'trending'` (成長継続)

## ランキング計算 (2-axis, v3 確定仕様)

| Axis | 条件 | 指標 | Tie-break |
|------|------|------|-----------|
| **Breakout** | `1 <= start_stars < 100 AND delta > 0` | `star_delta` 降順 | `repo_id` 昇順 |
| **Trending** | `start_stars >= 100 AND growth_pct > 0` | `growth_pct = (end-start)/start*100` 降順 | `repo_id` 昇順 |

- 期間: `1d`, `7d`, `30d` の 3 種類 → 計 6 slot を毎日生成
- 除外: archived, fork, snapshot 欠損, delta<=0 / growth<=0
- 新規 repo (期間開始日より後に発見) は `EarliestStarCount` を fallback として使用

## JSON 出力スキーマ

- `rankings.json`: `{ updated_at, rankings: { "1d_breakout":[], "1d_trending":[], "7d_breakout":[], "7d_trending":[], "30d_breakout":[], "30d_trending":[] } }` (常に 6 キー、空配列可)
- `repos/{owner}__{name}.json`: リポジトリ詳細 + `star_history: [{ date, stars }]` + `deleted_at`
- `meta.json`: `{ generated_at, total_repos, total_active, periods, rank_types }`

## 不変条件 (I1-I13)

実装の正当性は `batch/internal/pipeline/invariant_iN_test.go` の 13 テストで担保:

| ID | 内容 |
|---|---|
| I1 | 全 non-deleted repo に JSON 存在、rankings には active のみ |
| I2 | DB → export → restore → DB' で全列一致 |
| I3 | 3 日連続 simulation で履歴喪失なし |
| I4 | refresh 失敗率 30% 超で abort |
| I5 | 2 軸 ranking の filter / 並び / 重複なし |
| I6 | NaN / Inf 一切出現なし |
| I7 | rank が 1..N 連番、欠番/重複なし |
| I8 | rankings.json が 6 キー必ず保持 |
| I9 | 全 schema が JSON Marshal/Unmarshal で等価 |
| I10 | Migrate 冪等 |
| I11 | data/ から restore + compute + export で同一 JSON |
| I12 | 全 slot 空で abort |
| I13 | 同条件再 export で byte 一致 (updated_at/generated_at 除く) |

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
