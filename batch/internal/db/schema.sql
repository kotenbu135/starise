CREATE TABLE IF NOT EXISTS repositories (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    github_id     TEXT    NOT NULL UNIQUE,
    owner         TEXT    NOT NULL,
    name          TEXT    NOT NULL,
    description   TEXT    NOT NULL DEFAULT '',
    url           TEXT    NOT NULL DEFAULT '',
    homepage_url  TEXT    NOT NULL DEFAULT '',
    language      TEXT    NOT NULL DEFAULT '',
    license       TEXT    NOT NULL DEFAULT '',
    topics        TEXT    NOT NULL DEFAULT '[]',
    is_archived   INTEGER NOT NULL DEFAULT 0,
    is_fork       INTEGER NOT NULL DEFAULT 0,
    fork_count    INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT    NOT NULL DEFAULT '',
    updated_at    TEXT    NOT NULL DEFAULT '',
    pushed_at     TEXT    NOT NULL DEFAULT '',
    fetched_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    UNIQUE(owner, name)
);

CREATE TABLE IF NOT EXISTS daily_stars (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id       INTEGER NOT NULL REFERENCES repositories(id),
    recorded_date TEXT    NOT NULL,
    star_count    INTEGER NOT NULL,
    UNIQUE(repo_id, recorded_date)
);

CREATE TABLE IF NOT EXISTS rankings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id       INTEGER NOT NULL REFERENCES repositories(id),
    period        TEXT    NOT NULL,
    computed_date TEXT    NOT NULL,
    star_start    INTEGER NOT NULL,
    star_end      INTEGER NOT NULL,
    star_delta    INTEGER NOT NULL,
    growth_rate   REAL    NOT NULL,
    rank          INTEGER NOT NULL,
    UNIQUE(repo_id, period, computed_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_stars_repo_date ON daily_stars(repo_id, recorded_date);
CREATE INDEX IF NOT EXISTS idx_rankings_period_date  ON rankings(period, computed_date, rank);
