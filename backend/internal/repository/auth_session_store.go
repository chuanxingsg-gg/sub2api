package repository

import (
	"context"
	"database/sql"
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

// ConsumeRefreshToken 使用内嵌缓存的原子 Lua 实现（v0.2.21 上游修复），
// 并发刷新时只有一个请求能兑换同一个 refresh token。

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
