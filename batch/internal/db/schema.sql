CREATE TABLE IF NOT EXISTS repositories (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    github_id    TEXT    NOT NULL UNIQUE,
    owner        TEXT    NOT NULL,
    name         TEXT    NOT NULL,
    description  TEXT    NOT NULL DEFAULT '',
    url          TEXT    NOT NULL DEFAULT '',
    homepage_url TEXT    NOT NULL DEFAULT '',
    language     TEXT    NOT NULL DEFAULT '',
    license      TEXT    NOT NULL DEFAULT '',
    topics       TEXT    NOT NULL DEFAULT '[]',
    is_archived  INTEGER NOT NULL DEFAULT 0,
    is_fork      INTEGER NOT NULL DEFAULT 0,
    fork_count   INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL DEFAULT '',
    updated_at   TEXT    NOT NULL DEFAULT '',
    pushed_at    TEXT    NOT NULL DEFAULT '',
    deleted_at   TEXT    NOT NULL DEFAULT '',
    UNIQUE (owner, name)
);

CREATE TABLE IF NOT EXISTS daily_stars (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id       INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    recorded_date TEXT    NOT NULL,
    star_count    INTEGER NOT NULL,
    UNIQUE (repo_id, recorded_date)
);
CREATE INDEX IF NOT EXISTS idx_daily_stars_repo_date ON daily_stars (repo_id, recorded_date);

CREATE TABLE IF NOT EXISTS rankings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id       INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    period        TEXT    NOT NULL,
    rank_type     TEXT    NOT NULL,
    computed_date TEXT    NOT NULL,
    start_stars   INTEGER NOT NULL,
    end_stars     INTEGER NOT NULL,
    star_delta    INTEGER NOT NULL,
    growth_pct    REAL    NOT NULL,
    rank          INTEGER NOT NULL,
    UNIQUE (repo_id, period, rank_type, computed_date)
);
CREATE INDEX IF NOT EXISTS idx_rankings_key ON rankings (period, rank_type, computed_date, rank);
