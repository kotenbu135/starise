# starise

日本語で使える GitHub Star 発見サイト。直近 N 日間のスター増加率で OSS をランキング表示する。

## 仕組み

```
GitHub Actions (daily cron)
  └─ Go batch → SQLite → 静的 JSON → Astro → GitHub Pages
```

毎日 GitHub GraphQL API からスター数を取得し、7日/30日間の増加率でランキングを計算。結果を静的 JSON として出力し、Astro でビルドして GitHub Pages にデプロイする。外部サービス不要、GitHub だけで完結。

## セットアップ

### 前提

- Go 1.23+
- Node.js 22+ / pnpm 10+
- GitHub Token（`gh auth token` or PAT）

### バッチ実行

```bash
cd batch
export GITHUB_TOKEN=$(gh auth token)

# 個別実行
go run . fetch --seed-file seeds.txt    # リポジトリ情報 + スター数取得
go run . compute                         # 7d/30d 増加率ランキング計算
go run . export --out-dir ../data        # 静的 JSON 出力

# 一括実行
go run . run --seed-file seeds.txt --out-dir ../data
```

### フロントエンド

```bash
cd web
cp -r ../data/ public/data/
pnpm install
pnpm dev       # http://localhost:4321
pnpm build     # 静的サイト出力
```

### CI/CD

GitHub Actions が日次で自動実行する。

| Workflow | トリガー | 内容 |
|----------|---------|------|
| Daily Batch | 毎日 00:00 JST + 手動 | Go batch → JSON → commit |
| Deploy to GitHub Pages | data/web 変更時 + batch 完了後 | Astro build → Pages deploy |

## 技術スタック

| Layer | Technology |
|-------|-----------|
| Batch | Go, Cobra, modernc.org/sqlite |
| API | GitHub GraphQL API |
| DB | SQLite（Actions cache で永続化） |
| Frontend | Astro, React islands, Tailwind CSS, Recharts |
| Hosting | GitHub Pages |

## ディレクトリ構成

```
batch/          Go バッチ処理 CLI
  cmd/          Cobra コマンド (fetch, compute, export, run)
  internal/     内部パッケージ (db, github, ranking, export)
  seeds.txt     監視対象リポジトリリスト
web/            Astro フロントエンド
  src/pages/    ランキング一覧 + リポジトリ詳細
  src/components/  React islands (RankingTable, StarChart, etc.)
data/           生成された静的 JSON
docs/           プロジェクトドキュメント
```

## ライセンス

MIT
