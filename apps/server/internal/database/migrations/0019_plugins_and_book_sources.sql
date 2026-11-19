CREATE TABLE IF NOT EXISTS plugins_state (
    name TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 1,
    installed_version TEXT NOT NULL DEFAULT '',
    cached_manifest_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS plugin_configs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plugin_name TEXT NOT NULL,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_configs_user_plugin ON plugin_configs(user_id, plugin_name);

CREATE TABLE IF NOT EXISTS book_sources (
    id TEXT PRIMARY KEY,
    plugin_name TEXT NOT NULL,
    external_id TEXT NOT NULL,
    upload_job_id TEXT,
    book_id TEXT REFERENCES books(id) ON DELETE CASCADE,
    metadata_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_book_sources_plugin_ext ON book_sources(plugin_name, external_id);
CREATE INDEX IF NOT EXISTS idx_book_sources_upload_job_id ON book_sources(upload_job_id);
CREATE INDEX IF NOT EXISTS idx_book_sources_book_id ON book_sources(book_id);
