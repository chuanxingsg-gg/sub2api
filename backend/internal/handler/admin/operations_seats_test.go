package admin

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func seat(id int64, suffix string, status string) service.Group {
	return service.Group{ID: id, Name: proNativeSeatPrefix + suffix, Platform: service.PlatformAnthropic, Status: status, SubscriptionType: service.SubscriptionTypeSubscription}
}

func TestProNativeGroupsAreWhitelisted(t *testing.T) {
	active := service.StatusActive
	for _, test := range []struct {
		name, platform string
		allowed        bool
	}{
		{proNativeTemplate, service.PlatformAnthropic, true},
		{proNativeSeatPrefix + "1", service.PlatformAnthropic, true},
		{proNativeSeatPrefix + "acct-7", service.PlatformAnthropic, true},
		{proNativeTemplate, service.PlatformOpenAI, false},
		{proNativeSeatPrefix + "1", service.PlatformOpenAI, false},
		{proNativeSeatPrefix, service.PlatformAnthropic, false}, // prefix alone is not a seat
		{"Claude-ProNative-Seat", service.PlatformAnthropic, false},
		{"AWS-Claude", service.PlatformAnthropic, false}, // private legacy group stays closed
	} {
		require.Equal(t, test.allowed, operationsGroup(&service.Group{Name: test.name, Platform: test.platform, Status: active}), test.name+"/"+test.platform)
	}
	require.False(t, operationsGroup(&service.Group{Name: proNativeSeatPrefix + "1", Platform: service.PlatformAnthropic, Status: service.StatusDisabled}))
	require.True(t, isProNativeTemplate(&service.Group{Name: proNativeTemplate}))
	require.False(t, isProNativeSeat(&service.Group{Name: proNativeTemplate}))
	require.False(t, isProNativeSeat(nil))
}

func TestSelectFreeSeatsOneCustomerPerAccount(t *testing.T) {
	groups := []service.Group{
		seat(30, "c", service.StatusActive),
		seat(10, "a", service.StatusActive),
		seat(20, "b", service.StatusActive),
		{ID: 99, Name: "Claude-Monthly-Basic", Platform: service.PlatformAnthropic, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription},
		{ID: 98, Name: proNativeTemplate, Platform: service.PlatformAnthropic, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription},
	}
	has := func(*service.Group) (bool, error) { return true, nil }
	none := func(int64) (bool, error) { return false, nil }

	free, err := selectFreeSeats(groups, has, none)
	require.NoError(t, err)
	require.Equal(t, []int64{10, 20, 30}, []int64{free[0].ID, free[1].ID, free[2].ID}, "lowest id first; template and shared plans are never seats")

	taken := map[int64]bool{10: true}
	free, err = selectFreeSeats(groups, has, func(id int64) (bool, error) { return taken[id], nil })
	require.NoError(t, err)
	require.Equal(t, int64(20), free[0].ID, "a seat with an active customer is skipped")
	require.Len(t, free, 2)

	taken = map[int64]bool{10: true, 20: true, 30: true}
	free, err = selectFreeSeats(groups, has, func(id int64) (bool, error) { return taken[id], nil })
	require.NoError(t, err)
	require.Empty(t, free, "sold out when every seat is held")
}

func TestSelectFreeSeatsSkipsSeatsWithoutAccountOrDisabled(t *testing.T) {
	groups := []service.Group{
		seat(1, "no-account", service.StatusActive),
		seat(2, "disabled", service.StatusDisabled),
		seat(3, "ok", service.StatusActive),
	}
	free, err := selectFreeSeats(groups,
		func(g *service.Group) (bool, error) { return g.ID != 1, nil },
		func(int64) (bool, error) { return false, nil })
	require.NoError(t, err)
	require.Len(t, free, 1)
	require.Equal(t, int64(3), free[0].ID)
}

func TestSelectFreeSeatsRejectsStandardTypeSeat(t *testing.T) {
	g := seat(5, "std", service.StatusActive)
	g.SubscriptionType = service.SubscriptionTypeStandard
	free, err := selectFreeSeats([]service.Group{g},
		func(*service.Group) (bool, error) { return true, nil },
		func(int64) (bool, error) { return false, nil })
	require.NoError(t, err)
	require.Empty(t, free)
}

func TestSelectFreeSeatsPropagatesErrors(t *testing.T) {
	boom := errors.New("db down")
	groups := []service.Group{seat(1, "a", service.StatusActive)}
	_, err := selectFreeSeats(groups, func(*service.Group) (bool, error) { return false, boom }, func(int64) (bool, error) { return false, nil })
	require.ErrorIs(t, err, boom)
	_, err = selectFreeSeats(groups, func(*service.Group) (bool, error) { return true, nil }, func(int64) (bool, error) { return false, boom })
	require.ErrorIs(t, err, boom)
}
