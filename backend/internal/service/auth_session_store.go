package service

import (
	"context"
	"time"
)

// AuthSessionStore is durable; missing sessions are rejected, including tokens
// issued before this security release. API keys are unaffected.
type AuthSessionStore interface {
	CreateAuthSession(context.Context, string, int64, time.Time) error
	AuthSessionActive(context.Context, string, int64) (bool, error)
	RevokeAuthSession(context.Context, string) error
	RevokeUserAuthSessions(context.Context, int64) error
}

func (s *AuthService) createAuthSession(ctx context.Context, id string, userID int64, ttl time.Duration) error {
	if store, ok := s.refreshTokenCache.(AuthSessionStore); ok {
		return store.CreateAuthSession(ctx, id, userID, time.Now().Add(ttl))
	}
	// Lightweight unit-test doubles do not provide a database. Production is
	// always constructed with the durable cache provider.
	if s.entClient != nil {
		return ErrServiceUnavailable
	}
	return nil
}

func (s *AuthService) checkAuthSession(ctx context.Context, id string, userID int64) error {
	if store, ok := s.refreshTokenCache.(AuthSessionStore); ok {
		if id == "" {
			return ErrTokenRevoked
		}
		active, err := store.AuthSessionActive(ctx, id, userID)
		if err != nil {
			return ErrServiceUnavailable
		}
		if !active {
			return ErrTokenRevoked
		}
	} else if s.entClient != nil {
		return ErrServiceUnavailable
	}
	return nil
}

// LogoutAccessToken revokes the authenticated session without affecting other
// devices. Already invalid credentials are harmless on this idempotent endpoint.
func (s *AuthService) LogoutAccessToken(ctx context.Context, token string) error {
	claims, err := s.ValidateToken(token)
	if err != nil {
		if err == ErrServiceUnavailable {
			return err
		}
		return nil
	}
	return s.RevokeSessionFamily(ctx, claims.SessionID)
}
