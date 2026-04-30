# Plan: Repo Search by owner / name / description

## Context

starise は現在ランキング表示のみで、ユーザーが任意のリポジトリ名・ユーザー名・説明文で検索する手段がない。`data/repos/` には 60,223 件のリポジトリがあるがランキング外の repo は事実上発見できない。
日本語説明 (`description_ja`) も翻訳済みなのに検索面では活用されていない。

ヘッダー右上に検索ボックスを置き、全 60k リポジトリを対象にインクリメンタル検索 (owner / name / description / description_ja) で結果ドロップダウン表示、Enter で `/search` ページへ遷移する。

## 全体アーキテクチャ

```
Go batch (export step)
  └─ 新規: data/search-index.json (slim, ~1.5MB gzipped)
       │
       ▼
Astro frontend
  └─ Layout.astro header-right slot
       └─ SearchBox.tsx (React island, client:idle)
            ├─ on focus: fetch /data/search-index.json (lazy, once)
            ├─ on input: substring match → top 8 ドロップダウン
            └─ on Enter: navigate /search?q=...

  └─ /search.astro
       └─ SearchResults.tsx (client:load) — ページネーション付き全件結果
```

## 1. Go batch — search index 生成

### 新規ファイル
- `batch/internal/export/search.go` — index ビルダー
- `batch/internal/export/search_test.go` — TDD (先に Red を書く)

### スキーマ (`SearchIndexEntry`, `SearchIndex`)
極小化のため短キー + description は ja → en フォールバック単一フィールド + 80 文字 truncate。

```go
type SearchIndexEntry struct {
  O string `json:"o"`           // owner
  N string `json:"n"`           // name
  L string `json:"l,omitempty"` // language
  S int    `json:"s"`           // star_count (結果ソート用)
  D string `json:"d,omitempty"` // description_ja || description, 80 chars
}

type SearchIndex struct {
  GeneratedAt string             `json:"generated_at"`
  Repos       []SearchIndexEntry `json:"repos"`
}
```

### 出力
- `data/search-index.json`
- ソース: `db.ListActiveRepositories(d)` (deleted_at IS NULL のみ — 削除 repo は検索対象外)
- 翻訳キャッシュから `description_ja` を取得、空なら `description`、80 chars で切る (rune 単位、日本語マルチバイト保護)
- ソート: `(owner, name)` で I13 互換
- meta.json は無変更 (search-index は副次的アーティファクト)

### 既存ファイル更新
- `batch/internal/export/json.go` — `Export()` 末尾で search-index を書き出すヘルパーを呼ぶ
- 失敗時の挙動: rankings/repos と同様にエラー伝播

### サイズ試算
- 平均 owner+name 30B、description_ja 80 chars ≒ 240B、language 10B、star_count 数値、JSON オーバーヘッド
- ~110 bytes/entry × 60k ≒ **6.6MB raw / ~1.5MB gzipped** (GitHub Pages は gzip 配信)

### 不変条件
- 決定性 (I13): ソート済み、JSON MarshalIndent、UpdatedAt 等の可変フィールドなし
- 既存 invariants は壊さない (repos / rankings / meta は無変更)

## 2. Frontend — SearchBox (header)

### 新規ファイル
- `web/src/components/SearchBox.tsx` — header の React island
- `web/src/lib/search.ts` — 純粋関数: `searchRepos(index, query, limit)` (Vitest 単体テスト容易)

### 動作仕様
- **lazy load**: 初回フォーカスまたは初回入力時に `${BASE_URL}/data/search-index.json` を fetch、メモリ保持。`AbortController` で重複防止
- **マッチング**: 大文字小文字無視の substring。スコア順位付け
  - owner / name 完全一致 = 1000
  - owner / name 前方一致 = 500
  - owner / name 部分一致 = 200
  - description 部分一致 = 50
  - 同点時は `star_count desc`
- 上位 **8 件** をドロップダウン表示 (owner/name + 言語バッジ + description 抜粋)
- キーボード: ↑↓ で選択、Enter で個別 repo へ遷移、何も選択していなければ `/search?q=...` へ
- Esc / 外クリックで閉じる
- "もっと見る" 行 → `/search?q=...`

### 配置
- `web/src/layouts/Layout.astro` — `<slot name="header-right" />` に統合 (現状空 slot)
- 全ページに表示するため、Layout 内に直接 island を埋め込む (各 page で slot を埋める方式より重複なし)
- `client:idle` で初期 JS 負荷最小化、検索インデックス自体は focus まで触らない

### スタイル
- 既存 FilterBar.tsx の pill/border パターンを踏襲 (`border-border`, `text-text-secondary`, `text-brand` など `global.css` のデザイントークンを再利用)
- shadcn 不使用なので Tailwind 直書き + lucide `Search` icon

## 3. Frontend — /search ページ (全件結果)

### 新規ファイル
- `web/src/pages/search.astro` — query 取得 + island ホスト
- `web/src/components/SearchResults.tsx` — フィルタ・ページネーション付きリスト

### 動作
- `Astro.url.searchParams.get('q')` を初期 prop 渡し
- SearchResults は SearchBox と同じ index を fetch (両者で SWR 的に共有しても可、初期は単純に独立 fetch)
- 既存 `Pagination`, `FilterBar` (言語のみ) を再利用、表形式は RankingTable のセルレイアウトを継承
- 結果なし時は `該当するリポジトリがありません` メッセージ (RankingTable の既存パターン)

## 4. テスト

### Go (TDD 必須 — Red → Green → Refactor)
- `batch/internal/export/search_test.go`:
  - `TestBuildSearchIndex_Sorted` — owner/name 順
  - `TestBuildSearchIndex_DescriptionFallback` — ja 空 → en フォールバック
  - `TestBuildSearchIndex_TruncatesUTF8` — 日本語 80 文字以内、文字化けなし
  - `TestBuildSearchIndex_ExcludesDeleted` — soft-delete 除外
  - `TestBuildSearchIndex_Deterministic` — 同一 DB から 2 回実行で byte-identical
- DB セットアップ: 既存テストと同じ `:memory:` パターン

### Frontend
- Vitest 未導入なので `web/src/lib/search.test.ts` 追加 (`pnpm add -D vitest @testing-library/dom`):
  - `searchRepos` の関連性スコアリング検証
  - 大文字小文字無視確認
  - 空クエリ時の挙動 (空配列を返す)
- React コンポーネント単体テストは v1 ではスキップ。CLAUDE.md の TDD 例外手続きに従い PR 説明に明記
  - 理由: テストランナー新規導入は本変更のスコープ外。pure 関数層のみテスト

### 手動検証
- `cd batch && go test ./... -race -cover`
- `cd batch && go run . export --out-dir ../data` → `ls data/search-index.json` 存在確認
- `jq '.repos | length' data/search-index.json` → 60177 (active 数と一致)
- `cd web && cp -r ../data/ public/data/ && pnpm dev`
- ブラウザで `http://localhost:4321/`:
  - ヘッダー検索ボックスが出る
  - "react" と入力 → ドロップダウン表示、上位に有名 react リポジトリ
  - "react native" と入力 → スペース含む複数語マッチ
  - 日本語 "ゲーム" と入力 → description_ja マッチで game 系 repo
  - Enter → `/search?q=...` 遷移、結果一覧表示
  - 個別結果クリック → `/repo/{owner}/{name}` 遷移
- DevTools Network: 初回 focus で `search-index.json` が一度だけ fetch される

## 5. CI / pipeline 影響

- `batch/internal/pipeline/run.go` の `Export` 呼び出しは無変更 (Export 内部で search-index も書く)
- `.github/workflows/batch.yml` 変更不要 (export 出力ファイルが 1 つ増えるだけ)
- 既存 invariant tests (`batch/internal/pipeline/invariant_*_test.go`) は無関係なので green 維持

## 6. 修正/新規ファイル一覧

**新規:**
- `batch/internal/export/search.go`
- `batch/internal/export/search_test.go`
- `web/src/components/SearchBox.tsx`
- `web/src/components/SearchResults.tsx`
- `web/src/lib/search.ts`
- `web/src/lib/search.test.ts`
- `web/src/pages/search.astro`

**修正:**
- `batch/internal/export/json.go` — `Export()` 末尾で search-index 書き出し
- `batch/internal/export/schema.go` — `SearchIndex`, `SearchIndexEntry` 型追加 (search.go に置いても可)
- `web/src/layouts/Layout.astro` — header-right に SearchBox 直接埋め込み (`<SearchBox client:idle />`)
- `web/src/lib/types.ts` — `SearchIndexEntry` 型をフロント側にも定義 (Go と独立、JSON 短キー対応)
- `web/package.json` — vitest devDep 追加

## 7. 実装順序 (TDD)

1. Go: `search_test.go` Red → `search.go` Green → `json.go` 統合 → `go test ./... -race`
2. Frontend pure: `search.test.ts` Red → `search.ts` Green → vitest 起動確認
3. UI: `SearchBox.tsx` 実装 → Layout 配置 → 手動確認
4. UI: `/search.astro` + `SearchResults.tsx` → 手動確認
5. ローカル `pnpm build` で静的生成成功確認

## 8. リスク / 留意点

- **ペイロードサイズ**: 60k × ~110B = 6.6MB raw。GitHub Pages の gzip 配信頼みで ~1.5MB。低速回線で気になるなら将来シャーディング (頭文字別 26 ファイル) で対応可能。v1 は単一ファイルで開始
- **lazy load タイミング**: focus 時 fetch だと初回タイプにラグ。`client:idle` 後に prefetch する案もあるが、JS バンドル肥大を避けるため focus トリガーで開始
- **検索精度**: substring のみ。typo 耐性が必要なら fuse.js (約 30KB) を後から追加可能
- **base path**: `/starise` (GitHub Pages サブパス)。fetch URL は `${import.meta.env.BASE_URL}/data/search-index.json` で組み立て
- **SSR 描画**: SearchBox は client only (window/fetch 必要)。Layout から呼ぶ際に `client:idle` 必須
