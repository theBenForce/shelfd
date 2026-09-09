-- SQLite Migration 0006: OAuth 2.0 and Dynamic Client Registration Tables

CREATE TABLE IF NOT EXISTS oauth_clients (
    id TEXT PRIMARY KEY,
    client_secret TEXT,
    client_name TEXT NOT NULL,
    redirect_uris TEXT NOT NULL,
    grant_types TEXT NOT NULL DEFAULT 'authorization_code',
    response_types TEXT NOT NULL DEFAULT 'code',
    scope TEXT DEFAULT 'mcp',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS oauth_codes (
    code TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    code_challenge TEXT,
    code_challenge_method TEXT,
    scope TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_oauth_codes_expires ON oauth_codes(expires_at);
