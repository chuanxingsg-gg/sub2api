package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"time"
)

type durableRefreshTokenCache struct {
	service.RefreshTokenCache
	db  *sql.DB
	rdb *redis.Client
}

func ProvideDurableRefreshTokenCache(rdb *redis.Client, db *sql.DB) service.RefreshTokenCache {
	return &durableRefreshTokenCache{RefreshTokenCache: NewRefreshTokenCache(rdb), db: db, rdb: rdb}
}

// Atomically consume a refresh credential so parallel refresh requests cannot
// both exchange the same bearer secret for new credentials.
func (c *durableRefreshTokenCache) ConsumeRefreshToken(ctx context.Context, hash string) (*service.RefreshTokenData, error) {
	value, err := c.rdb.GetDel(ctx, refreshTokenKey(hash)).Result()
	if err == redis.Nil {
		return nil, service.ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	var data service.RefreshTokenData
	if err = json.Unmarshal([]byte(value), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *durableRefreshTokenCache) CreateAuthSession(ctx context.Context, id string, userID int64, expires time.Time) error {
	_, err := c.db.ExecContext(ctx, `INSERT INTO newflow_auth_sessions(session_id,user_id,expires_at) VALUES($1,$2,$3)`, id, userID, expires)
	return err
}
func (c *durableRefreshTokenCache) AuthSessionActive(ctx context.Context, id string, userID int64) (bool, error) {
	var active bool
	err := c.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM newflow_auth_sessions WHERE session_id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>CURRENT_TIMESTAMP)`, id, userID).Scan(&active)
	return active, err
}
func (c *durableRefreshTokenCache) RevokeAuthSession(ctx context.Context, id string) error {
	_, err := c.db.ExecContext(ctx, `UPDATE newflow_auth_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE session_id=$1 AND revoked_at IS NULL`, id)
	return err
}
func (c *durableRefreshTokenCache) RevokeUserAuthSessions(ctx context.Context, userID int64) error {
	_, err := c.db.ExecContext(ctx, `UPDATE newflow_auth_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return err
}

// Existing revocation callers, including refresh rejection and binding changes,
// must revoke access tokens even when they call the cache directly.
func (c *durableRefreshTokenCache) DeleteTokenFamily(ctx context.Context, id string) error {
	if err := c.RevokeAuthSession(ctx, id); err != nil {
		return err
	}
	return c.RefreshTokenCache.DeleteTokenFamily(ctx, id)
}
func (c *durableRefreshTokenCache) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	if err := c.RevokeUserAuthSessions(ctx, userID); err != nil {
		return err
	}
	return c.RefreshTokenCache.DeleteUserRefreshTokens(ctx, userID)
}
