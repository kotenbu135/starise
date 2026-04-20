# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

starise — 日本語で使える GitHub Star 発見サイト。直近 N 日間のスター増加率で OSS をランキング。  
Go バッチ → SQLite → 静的 JSON → Astro フロントエンド。GitHub だけで完結、外部サービス不要。

## Architecture

```
GitHub Actions (daily cron)
  └─ Go batch (batch/) → SQLite → static JSON (data/) → Astro (web/) → GitHub Pages
```

詳細: [docs/architecture.md](docs/architecture.md)

## Commands

### Go batch (v3, invariant-driven)
```bash
cd batch
export GITHUB_TOKEN=$(gh auth token)

go build ./...                                                  # Build
go test ./... -cover                                            # All tests + invariants

go run . restore --in-dir ../data                               # Rebuild DB from data/
go run . fetch --seed-file seeds.txt                            # Fetch seed repos + today snapshot
go run . discover --query "stars:>10 sort:stars-desc"           # Discover via Search API (single query)
go run . refresh                                                # Sequential bulk refresh (nodes(ids:) ×100 per batch)
go run . compute --top-n 2000                                   # Compute breakout + trending × 1d/7d/30d
go run . export --out-dir ../data                               # Write JSON tree

# All-in-one — matches CI exactly:
go run . run --seed-file seeds.txt --out-dir ../data --restore-from ../data \
  --preset --discover-concurrency 1 --allow-empty-rankings

# Single test (working dir = batch/):
go test -run TestName ./internal/ranking
go test ./internal/pipeline/ -run Invariant                     # Invariants only
go test ./... -race                                             # Race detector (required before merge)
```

### Frontend
```bash
cd web
cp -r ../data/ public/data/
pnpm install
pnpm dev          # http://localhost:4321
pnpm build        # Static export
```

### Verification
```bash
sqlite3 batch/starise.db "SELECT COUNT(*) FROM repositories;"
jq . data/meta.json
```

## Key References

| Doc | Purpose |
|-----|---------|
| [docs/architecture.md](docs/architecture.md) | ディレクトリ構成、技術スタック、DB スキーマ、設計判断 |
| [docs/github-graphql-api.md](docs/github-graphql-api.md) | GraphQL クエリ、型情報、レート制限、実装指針 |
| [DESIGN.md](DESIGN.md) | 日本語 UI デザイン契約（jp-ui-contracts ベース） |

## Tech Stack Quick Ref

- **Go**: `modernc.org/sqlite` (pure Go, no CGO), `spf13/cobra`, `shurcooL/graphql`
- **Frontend**: Astro + React islands, shadcn/ui, Tailwind CSS, Recharts
- **DB**: SQLite 3 tables: `repositories` (+ `deleted_at` soft delete), `daily_stars`, `rankings` (+ `rank_type`)
- **Ranking**: 2-axis (breakout for `1<=start<100`, trending for `start>=100`) × 3 periods (1d/7d/30d)
- **CI**: GitHub Actions cron (daily) + GitHub Pages deploy
- **Discovery scale**: `--preset` fans out ~64 queries (star bands × 15 langs × 7 topics) via `discover.BuildQuerySet`, dedup by GitHubID → v1-scale ~30k repos. Single-query mode is for debugging only
- **Execution model**: 並列は廃止。`refresh` と preset `discover` は **sequential (concurrency=1)** で実行。理由: 並列化すると GitHub の secondary rate limit に頻繁に引っかかり、retry sleep で結局 wallclock が延びるため。sequential でも primary budget (5000 pts/hr) 内に余裕で収まる試算 (~1500 pts/run)。`runBulkRefreshParallel` 内部実装は残してあり、将来 cross-shard rate-limit awareness を追加すれば再度並列化可能
- **Rate limit defense** (`batch/internal/github/retry.go`):
  - HTTP 429 / 403+"rate limit" (secondary) → Retry-After or exponential backoff (数秒)
  - HTTP 200 + `errors[{type:"RATE_LIMITED"}]` (primary 枯渇) → `data.rateLimit.resetAt` まで sleep (最大1時間、5s buffer 付き)
  - HTTP 200 + `errors[{type:"MAX_NODE_LIMIT_EXCEEDED"}]` → retry せずエラー伝播
  - HTTP Client.Timeout は 180s (retry ループ全体を覆う)。primary 枯渇時の長 sleep は timeout 超過で abort する想定 — pipeline の次回実行が resetAt 後になるよう cron 設計で担保

## Invariants (issue #2)

Ranking + export correctness is enforced by 13 invariant tests under
`batch/internal/pipeline/invariant_iN_test.go`. All MUST stay green:

- I1 completeness, I2 export↔restore round-trip, I3 multi-day history,
  I4 refresh failure tolerance (>30% missing aborts), I5a-d ranking math,
  I6 no NaN/Inf, I7 contiguous 1..N ranks, I8 6-key rankings.json,
  I9 JSON round-trip, I10 idempotent migrate, I11 data/ source-of-truth,
  I12 macro emptiness aborts, I13 deterministic export.

Run only the invariants: `go test ./batch/internal/pipeline/ -run Invariant`

## TDD 強制 (MUST)

全コード変更で TDD を厳守。例外なし。

**Red → Green → Refactor サイクル:**
1. **Red**: 失敗するテストを先に書く。実装コードより前にコミット可能な状態にする
2. **Green**: テストを通す最小限の実装。過剰設計禁止
3. **Refactor**: テスト緑維持のままリファクタ

**Go (`batch/`)**:
- 対象パッケージと同ディレクトリに `*_test.go`。table-driven test を基本形とする
- `go test ./... -cover` で実行。カバレッジ 80%+ 目標
- 単一テスト: `go test -run TestName ./batch/internal/ranking`
- DB 関連は `modernc.org/sqlite` の `:memory:` で独立セットアップ
- GitHub GraphQL 呼び出しは interface 化しモック。本物の API を叩くテスト禁止

**Frontend (`web/`)**:
- テストランナー未導入。新規追加時は Vitest + @testing-library/react を選択
- React island コンポーネントは `*.test.tsx` 同置配置
- データ読み込み層はピュア関数に切り出してユニットテスト可能にする

**禁止事項**:
- テストなしで実装のみ追加するコミット
- 「後で書く」「動作確認済みだから省略」での TDD スキップ
- テスト削除によるテスト失敗の解消（バグなら実装を直す）

**例外手続き**: TDD が物理的に困難な箇所（マイグレーション DDL、config ファイル、純粋な型定義）は PR 説明で理由を明示。

関連: `/ecc:go-test` (Go TDD 自動化)、`/ecc:tdd` (汎用 TDD ワークフロー)
