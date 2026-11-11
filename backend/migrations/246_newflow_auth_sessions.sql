-- Access-token revocation must survive Redis eviction and service restarts.
CREATE TABLE IF NOT EXISTS newflow_auth_sessions (
    session_id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS newflow_auth_sessions_user_idx ON newflow_auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS newflow_auth_sessions_expiry_idx ON newflow_auth_sessions(expires_at);
