# GitHub GraphQL API リファレンス（starise 用）

本プロジェクトで使用する GitHub GraphQL API のエンドポイント・クエリ・型情報をまとめる。

## エンドポイント

```
POST https://api.github.com/graphql
Authorization: bearer <GITHUB_TOKEN>
```

## 認証

- `GITHUB_TOKEN` 環境変数（GitHub Actions ビルトインまたは PAT）
- Header: `Authorization: bearer <token>`

---

## レート制限

| 認証方式 | ポイント/時 |
|----------|------------|
| PAT（個人トークン） | 5,000 |
| GitHub Actions `GITHUB_TOKEN` | 1,000（リポジトリ単位） |
| GitHub App | 5,000〜12,500 |

### ポイント計算

- 各接続（connection）の `first`/`last` 引数の積を合算 → 100 で割る → 切り上げ
- 最小 1 ポイント
- ノード上限: 1 リクエストあたり 500,000 ノード

### 残量確認クエリ

```graphql
query {
  rateLimit {
    limit
    remaining
    used
    resetAt
  }
}
```

レスポンスヘッダでも確認可能: `x-ratelimit-remaining`, `x-ratelimit-limit`, `x-ratelimit-used`, `x-ratelimit-reset`

---

## クエリルートフィールド

### `repository(owner: String!, name: String!): Repository`

単一リポジトリ取得。

```graphql
query {
  repository(owner: "golang", name: "go") {
    id
    name
    owner { login }
    description
    stargazerCount
    primaryLanguage { name }
    repositoryTopics(first: 10) {
      nodes { topic { name } }
    }
  }
}
```

### `search(query: String!, type: SearchType!, first: Int, after: String): SearchResultItemConnection!`

検索（最大 1,000 件）。`type: REPOSITORY` でリポジトリ検索。

```graphql
query {
  search(query: "stars:>1000 language:go", type: REPOSITORY, first: 10) {
    repositoryCount
    nodes {
      ... on Repository {
        nameWithOwner
        stargazerCount
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
```

---

## Repository オブジェクト（使用フィールド）

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `id` | `ID!` | GraphQL ノード ID（グローバル一意） |
| `databaseId` | `Int` | GitHub 内部 DB ID |
| `name` | `String!` | リポジトリ名 |
| `nameWithOwner` | `String!` | `owner/name` 形式 |
| `owner` | `RepositoryOwner!` | `.login` でオーナー名取得 |
| `description` | `String` | 説明文 |
| `url` | `URI!` | リポジトリ URL |
| `homepageUrl` | `URI` | ホームページ URL |
| `stargazerCount` | `Int!` | スター総数 |
| `forkCount` | `Int!` | フォーク数 |
| `primaryLanguage` | `Language` | `.name` で言語名取得 |
| `repositoryTopics` | `RepositoryTopicConnection!` | トピック一覧（`first` 引数必須） |
| `licenseInfo` | `License` | `.spdxId`, `.name` |
| `isArchived` | `Boolean!` | アーカイブ済みか |
| `isFork` | `Boolean!` | フォークか |
| `createdAt` | `DateTime!` | 作成日時 |
| `updatedAt` | `DateTime!` | 更新日時 |
| `pushedAt` | `DateTime` | 最終プッシュ日時 |

### RepositoryTopicConnection

```graphql
repositoryTopics(first: 10) {
  nodes {
    topic {
      name    # String! — トピック名
    }
  }
}
```

### Stargazers Connection（参考: 個別スターユーザ取得）

```graphql
stargazers(first: 100, after: $cursor, orderBy: {field: STARRED_AT, direction: DESC}) {
  totalCount
  edges {
    starredAt   # DateTime!
    node {
      login     # スターしたユーザ
    }
  }
  pageInfo {
    hasNextPage
    endCursor
  }
}
```

> **注意**: stargazers connection は 1 リポジトリあたり最大 100 件/リクエスト。全スターユーザ取得はレート制限的に非現実的。本プロジェクトでは `stargazerCount` のみ使用。

---

## starise で使用するクエリ

### fetch コマンド: リポジトリ情報 + スター数取得

```graphql
query ($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    id
    databaseId
    name
    nameWithOwner
    owner { login }
    description
    url
    homepageUrl
    stargazerCount
    forkCount
    primaryLanguage { name }
    repositoryTopics(first: 20) {
      nodes {
        topic { name }
      }
    }
    licenseInfo {
      spdxId
      name
    }
    isArchived
    isFork
    createdAt
    updatedAt
    pushedAt
  }
  rateLimit {
    remaining
    resetAt
  }
}
```

**変数例:**
```json
{
  "owner": "denoland",
  "name": "deno"
}
```

### レスポンス例

```json
{
  "data": {
    "repository": {
      "id": "MDEwOlJlcG9zaXRvcnkxMzMwMDI4OTE=",
      "databaseId": 133002891,
      "name": "deno",
      "nameWithOwner": "denoland/deno",
      "owner": { "login": "denoland" },
      "description": "A modern runtime for JavaScript and TypeScript.",
      "url": "https://github.com/denoland/deno",
      "homepageUrl": "https://deno.land",
      "stargazerCount": 98000,
      "forkCount": 5400,
      "primaryLanguage": { "name": "Rust" },
      "repositoryTopics": {
        "nodes": [
          { "topic": { "name": "typescript" } },
          { "topic": { "name": "javascript" } },
          { "topic": { "name": "runtime" } }
        ]
      },
      "licenseInfo": { "spdxId": "MIT", "name": "MIT License" },
      "isArchived": false,
      "isFork": false,
      "createdAt": "2018-05-14T00:00:00Z",
      "updatedAt": "2024-01-15T00:00:00Z",
      "pushedAt": "2024-01-15T00:00:00Z"
    },
    "rateLimit": {
      "remaining": 4998,
      "resetAt": "2024-01-15T01:00:00Z"
    }
  }
}
```

---

## レート制限の実装指針

1. **各リクエスト後に `rateLimit.remaining` を確認**
2. `remaining < 100` で一時停止、`resetAt` まで待機
3. **並行数**: goroutine 5 並列（semaphore）で十分
4. **リトライ**: 403/429 レスポンス時は `Retry-After` ヘッダ or 指数バックオフ
5. **バッチ最適化**: 将来的に複数リポジトリを 1 クエリにまとめる（alias 使用）

### エイリアスによるバッチクエリ（将来最適化）

```graphql
query {
  repo0: repository(owner: "golang", name: "go") {
    ...RepoFields
  }
  repo1: repository(owner: "rust-lang", name: "rust") {
    ...RepoFields
  }
  rateLimit { remaining resetAt }
}

fragment RepoFields on Repository {
  id databaseId name nameWithOwner
  owner { login }
  description stargazerCount
  primaryLanguage { name }
  repositoryTopics(first: 20) {
    nodes { topic { name } }
  }
}
```

> **注意**: 1 クエリあたりのノード上限 500,000 に注意。alias 20〜30 程度が安全。

---

## 参考リンク

- [GraphQL API Reference](https://docs.github.com/en/graphql/reference)
- [Repository Object](https://docs.github.com/en/graphql/reference/objects#repository)
- [Rate Limits](https://docs.github.com/en/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api)
- [GraphQL Explorer](https://docs.github.com/en/graphql/overview/explorer) — ブラウザでクエリ試行可能
