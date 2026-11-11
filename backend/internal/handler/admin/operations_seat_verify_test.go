package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func seatAccount(id int64) service.Account {
	return service.Account{ID: id, Name: "acct", Platform: service.PlatformAnthropic, Status: service.StatusActive, Schedulable: true}
}

func okSub(now time.Time) *service.UserSubscription {
	return &service.UserSubscription{Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(24 * time.Hour)}
}

func TestSeatVerifyHealthySeat(t *testing.T) {
	now := time.Now()
	g := seat(28, "1", service.StatusActive)
	checks := evaluateSeatStructure(okSub(now), &g, []service.Account{seatAccount(2)}, []seatKeyView{{ID: 13, Status: service.StatusActive, GroupID: 28}}, now)
	require.Len(t, checks, 4)
	require.True(t, allChecksOK(checks), "%+v", checks)
}

func TestSeatVerifyKeyStillOnTemplate(t *testing.T) {
	now := time.Now()
	g := seat(28, "1", service.StatusActive)
	checks := evaluateSeatStructure(okSub(now), &g, []service.Account{seatAccount(2)}, []seatKeyView{{ID: 13, Status: service.StatusActive, GroupID: 26}}, now)
	require.False(t, allChecksOK(checks))
	require.Equal(t, "key", checks[3].Key)
	require.False(t, checks[3].OK)
	require.Contains(t, checks[3].Detail, "#13")
}

func TestSeatVerifyTemplateGroupAndSharedAccounts(t *testing.T) {
	now := time.Now()
	tpl := service.Group{ID: 26, Name: proNativeTemplate, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription}
	checks := evaluateSeatStructure(okSub(now), &tpl, []service.Account{seatAccount(2), seatAccount(3)}, nil, now)
	require.False(t, checks[1].OK, "template is not a seat")
	require.False(t, checks[2].OK, "two accounts on one seat")
	require.False(t, checks[3].OK, "no keys")
}

func TestSeatVerifyExpiredAndInactiveKey(t *testing.T) {
	now := time.Now()
	g := seat(28, "1", service.StatusActive)
	expired := &service.UserSubscription{Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(-time.Hour)}
	checks := evaluateSeatStructure(expired, &g, []service.Account{seatAccount(2)}, []seatKeyView{{ID: 13, Status: "inactive", GroupID: 28}}, now)
	require.False(t, checks[0].OK)
	require.False(t, checks[3].OK, "inactive keys do not count")
}
