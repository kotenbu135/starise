# DESIGN.md — starise

> Japanese UI design contract for AI agents and human reviewers.
> Based on [jp-ui-contracts](https://github.com/hirokaji/jp-ui-contracts)

---

## 0. Contract Metadata

- **Locale**: `ja-JP`
- **Profile**: `dashboard` + `media` hybrid
- **Primary writing mode**: `horizontal-tb`
- **Target surfaces**: `web` (static site, GitHub Pages)
- **Review status**: `draft`
- **Last reviewed at**: `2026-04-14`
- **Reviewer**: `sakis`

---

## 1. Product Intent

- **What this product is**: 日本語で使える GitHub Star 発見サイト。直近 N 日間のスター増加率でランキング表示
- **Primary audience**: 日本のソフトウェアエンジニア（中〜上級）
- **Primary usage context**: デスクトップブラウザ中心、モバイルでも閲覧可能。日常の情報収集として短時間で消化
- **Design stance**: 情報密度を保ちつつ、読みやすさを犠牲にしない
- **Must feel like**: 信頼できるデータダッシュボード。落ち着いた、ノイズの少ないツール
- **Must not feel like**: 派手なニュースサイト、ゲーミフィケーション過剰なランキング

---

## 2. Visual Theme & Brand Signals

- **Keywords**: minimal, functional, trustworthy, developer-friendly
- **Visual temperature**: `calm`
- **Density**: `balanced` — ランキング表はやや compact、詳細ページは airy
- **Tone**: 技術者向け。装飾より情報。データが主役
- **Motion stance**: `minimal` — ページ遷移と hover のみ

---

## 3. Color System

### Brand colors
- **Primary**: `#2563EB` (blue-600) — リンク、アクセント
- **Primary hover**: `#1D4ED8` (blue-700)
- **Accent**: `#F59E0B` (amber-500) — スター関連のハイライト

### Semantic colors
- **Success**: `#22C55E` (green-500) — 増加率プラス
- **Warning**: `#F59E0B` (amber-500)
- **Danger**: `#EF4444` (red-500) — 減少
- **Info**: `#3B82F6` (blue-500)

### Neutral colors
- **Text primary**: `#1E293B` (slate-800)
- **Text secondary**: `#64748B` (slate-500)
- **Text muted**: `#94A3B8` (slate-400)
- **Border**: `#E2E8F0` (slate-200)
- **Background**: `#FFFFFF`
- **Surface**: `#F8FAFC` (slate-50)
- **Surface elevated**: `#FFFFFF` with subtle shadow

### Color usage rules
- 増加率のプラス/マイナスは色+矢印アイコンで表現（色のみに依存しない）
- ランキング順位は数字で明示、色は補助
- ダークモードは MVP スコープ外

---

## 4. Typography System

### 4.1 Japanese fonts
- **Sans**: `"Noto Sans JP"`, `"Hiragino Kaku Gothic ProN"`, `"Hiragino Sans"`, sans-serif
- **Mono**: `"JetBrains Mono"`, `"Source Code Pro"`, monospace

### 4.2 Latin fonts
- **Sans**: `"Inter"`, `"Noto Sans JP"`, sans-serif
- **Mono**: `"JetBrains Mono"`, `"Source Code Pro"`, monospace

### 4.3 Fallback policy
```css
font-family: "Inter", "Noto Sans JP", "Hiragino Kaku Gothic ProN", "Hiragino Sans", sans-serif;
```

- 日本語 fallback 明示必須
- macOS / Windows 両方で安定描画を確認
- ブラウザ既定への丸投げ禁止

### 4.4 Type scale

| Role | Size | Weight | Line Height | Letter Spacing | Notes |
|------|------|--------|-------------|----------------|-------|
| H1 | 28px | 700 | 1.35 | 0 | ページタイトル |
| H2 | 22px | 600 | 1.4 | 0 | セクション見出し |
| H3 | 18px | 600 | 1.45 | 0 | カード・テーブル見出し |
| Body | 15px | 400 | 1.7 | normal | リポジトリ説明文 |
| Body S | 14px | 400 | 1.6 | normal | テーブル内テキスト |
| Caption | 12px | 400 | 1.5 | normal | 最終更新日時等の補助 |
| Label | 12px | 500 | 1.35 | 0.01em | フィルタ・タグ |
| Metric | 24px | 700 | 1.2 | 0 | スター数、増加率の数値強調 |
| Rank | 20px | 700 | 1.2 | 0 | ランキング順位 |
| Mono | 13px | 400 | 1.5 | normal | リポジトリ名 `owner/name` |

### 4.5 Japanese paragraph rules
- リポジトリ説明文の line-height: `1.7`（長文記事ではないため media より詰める）
- letter-spacing: `normal`（既定のまま）
- 説明文が長い場合でも wall-of-text にならないよう段落間隔を確保

### 4.6 Mixed-script rules
- リポジトリ名は英語が主。日本語説明文中に英語技術用語が頻出
- 見出しに英語プロダクト名が入る場合、視覚的衝突を避ける
- `owner/name` 表記はモノスペースで区別
- 数値（スター数、増加率）は Tabular Figures で揃える

### 4.7 OpenType and rendering
```css
font-kerning: auto;
font-feature-settings: normal;
```

- `palt` は見出しのみ検討、本文には適用しない
- 数値表示: `font-variant-numeric: tabular-nums;`

### 4.8 Writing direction
- Default: horizontal
- Vertical writing: `not used`

---

## 5. Line Breaking & Overflow

### Default rules
```css
html:lang(ja) {
  line-break: strict;
  word-break: normal;
  overflow-wrap: anywhere;
  font-kerning: auto;
  font-feature-settings: normal;
}

body {
  text-rendering: optimizeLegibility;
}

p, li, dd {
  line-break: strict;
  word-break: normal;
  overflow-wrap: anywhere;
}
```

### Additional rules
- リポジトリ名（`owner/name`）は `word-break: break-all` 許容（機械文字列のため）
- URL は `overflow-wrap: anywhere` で折り返し
- テーブル内のリポジトリ説明は `text-overflow: ellipsis` で切り詰め可
- 見出しの折り返しはモバイル幅で確認必須

### Mixed-script enhancement
```css
:lang(ja) em,
:lang(ja) strong,
:lang(ja) a,
:lang(ja) .latin,
:lang(ja) .product-name {
  word-break: normal;
  overflow-wrap: anywhere;
}

:lang(ja) h1,
:lang(ja) h2,
:lang(ja) h3 {
  word-break: auto-phrase;
}
```

### Heading enhancement
```css
:lang(ja) h1 { line-height: 1.35; }
:lang(ja) h2 { line-height: 1.4; }
:lang(ja) h3 { line-height: 1.45; }
```

### Experimental
```css
html:lang(ja) {
  text-autospace: normal;
}
```
- Progressive enhancement として適用。フォールバック不要（無視されるだけ）

---

## 6. Layout Principles

- **Container width**: `max-width: 1200px`
- **Reading width**: `max-width: 42em`（リポジトリ説明文）
- **Grid system**: CSS Grid — ランキング=1col、詳細ページ=メイン+サイド
- **Spacing scale**: 4px base (`4, 8, 12, 16, 24, 32, 48, 64`)
- **Section spacing rule**: セクション間 `32px`、カード間 `16px`
- **Whitespace policy**: 情報グループ間はスペースで区切る。ボーダーは補助的に使用

### Layout rules
- ランキングテーブルは全幅使用
- リポジトリ詳細ページの説明文は reading width 制限
- グラフ（Recharts）はコンテナ幅に追従
- フィルタバーは sticky（スクロール追従）

---

## 7. Component Guidelines

### Ranking Table
- 行高は十分確保（最小 48px）
- 順位・リポジトリ名・言語・スター数・増加率を横並び
- 増加率は色+矢印で方向性を示す
- ホバーで行ハイライト
- モバイルでは横スクロール or カード表示に切替

### Filter Bar
- 言語フィルタ: ドロップダウン or タグ選択
- 期間切替: `7d` / `30d` のトグル
- コンパクトに保つ。ラベルは最小限

### Star Chart (Recharts)
- 軸ラベルは日本語
- ツールチップは日付+スター数
- レスポンシブ（コンテナ幅追従）
- アニメーション: `minimal`（初回描画のみ）

### Repository Card (詳細ページ)
- メタ情報（言語、トピック、ライセンス）はタグ表示
- 説明文は reading width 制限
- スター数は Metric サイズで強調

### Navigation
- ヘッダー: サイト名 + 最終更新日時
- シンプル。ナビゲーション項目は最小限（MVP はランキングページのみ）

---

## 8. Depth, Border, and Surface

| Level | Usage | Border | Shadow |
|-------|-------|--------|--------|
| 0 | Page background (`#FFFFFF`) | none | none |
| 1 | Table rows, filter bar (`#F8FAFC`) | bottom border `#E2E8F0` | none |
| 2 | Repository cards | border `#E2E8F0` | `0 1px 3px rgba(0,0,0,0.06)` |
| 3 | ドロップダウン、ツールチップ | border `#E2E8F0` | `0 4px 12px rgba(0,0,0,0.08)` |

### Rules
- シャドウは控えめ。情報の邪魔をしない
- ボーダーは `slate-200` 統一
- 背景色の差で階層を表現

---

## 9. Responsive Behavior

- **Breakpoints**: `640px` (sm), `768px` (md), `1024px` (lg), `1280px` (xl)
- **Mobile reading width**: `100% - 32px` padding
- **Tablet layout stance**: テーブル表示維持、列の省略あり
- **Desktop layout stance**: フル表示

### Responsive rules
- モバイルではランキングテーブルをカード表示に切替検討
- グラフはコンテナ幅 100% で縮小
- フィルタバーはモバイルで折りたたみ
- タッチターゲット最小 44px
- 見出しの折り返しをモバイル幅で必ず確認

---

## 10. Motion & Interaction

- **Animation stance**: `minimal`
- **Transition speed**: `150ms ease`
- **Reduced motion policy**: `prefers-reduced-motion: reduce` 時はすべてのアニメーション無効化

### Rules
- ホバー/フォーカスの遷移のみ
- グラフ初回描画アニメーションは許容
- ページ遷移アニメーションなし（静的サイト）
- ローディング状態は spinner ではなく skeleton

---

## 11. Do's and Don'ts

### Do
- 日本語の読みやすさを最優先
- 数値（スター数、増加率）は tabular-nums で揃える
- リポジトリ名と説明文のフォント処理を分離する
- テーブル密度と説明文の行間を別管理する
- mixed-script（日本語説明+英語技術用語）のバランスを確認する

### Don't
- `word-break: break-all` を全体に適用しない
- body の letter-spacing を理由なく広げない
- ランキングの色だけで増減を表現しない（アイコン併用）
- テーブルに記事用の line-height を適用しない
- スクリーンショットだけで検証完了としない

---

## 12. Agent Prompt Guide

stariseは日本語ランキングサイト。技術者向けの落ち着いたダッシュボード。

UI生成時の優先順位:
1. 日本語テキストの可読性（line-height、overflow）
2. 数値データの視認性（tabular-nums、Metric サイズ）
3. テーブルの情報密度と読みやすさのバランス
4. mixed-script（英語リポジトリ名 × 日本語説明）の自然な共存

不明な場合:
- 本文は控えめに保つ
- 見出しは明瞭に
- 数値は大きく、ラベルは小さく
- テーブルと本文のスタイルを混ぜない

---

## 13. Validation Targets

- [ ] ランキングテーブルの長い日本語説明文の折り返し確認
- [ ] mixed-script 見出し（英語リポジトリ名）の表示確認
- [ ] URL / 長い英単語のオーバーフロー確認
- [ ] モバイル幅でのテーブル / カード表示確認
- [ ] スター数・増加率の数値揃え確認
- [ ] カラーコントラスト確認（WCAG AA）
- [ ] Windows 環境での日本語フォント描画確認

---

## Profile Rationale

**Why dashboard + media hybrid:**
- ランキングページ → dashboard: テーブル密度、数値スキャン性、フィルタ操作
- リポジトリ詳細ページ → media: 説明文の読みやすさ、グラフの視認性
- 純粋な dashboard ほど詰めず、純粋な media ほど広げない中間地点

**Body line-height `1.7`:**
- リポジトリ説明文は1〜3行程度の短文が多い
- 長文記事（media: 1.85）ほどの行間は不要
- ただし dashboard（1.5）では日本語が窮屈

**Body letter-spacing `normal`:**
- 説明文は短いため tracking 不要
- 日本語既定のまま

**Mixed-script の主な出現場所:**
- テーブル: リポジトリ名（英語）+ 説明文（日本語混在）
- 詳細ページ: 見出しに英語プロダクト名
- フィルタ: 言語名（English）

**テーブルと本文の分離:**
- テーブル内: Body S (14px/1.6)、説明文は ellipsis 切り詰め
- 詳細ページ: Body (15px/1.7)、reading width 制限

**モバイル優先削除:**
- フォーク数、ライセンス列を非表示
- 説明文を1行に切り詰め
- フィルタバーを折りたたみ
