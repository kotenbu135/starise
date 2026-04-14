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

### Go batch
```bash
cd batch
export GITHUB_TOKEN=$(gh auth token)

go build ./...                                      # Build
go run . fetch --seed-file seeds.txt                # Fetch repos + stars
go run . compute                                     # Calculate 7d/30d growth
go run . export --out-dir ../data                    # Generate JSON
go run . run --seed-file seeds.txt --out-dir ../data # All-in-one
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
- **DB**: SQLite 3 tables: `repositories`, `daily_stars`, `rankings`
- **CI**: GitHub Actions cron (daily) + GitHub Pages deploy

## Communication Style

- セッション開始時、自動で `/genshijin 極限` モード有効化。全レスポンス極限圧縮で返答
- 解除: ユーザが「原始人やめて」「normal mode」と言った場合のみ
