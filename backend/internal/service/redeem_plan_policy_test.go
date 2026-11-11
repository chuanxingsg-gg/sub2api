package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type policyGroupRepo struct {
	GroupRepository
	groups   map[int64]*Group
	accounts map[int64]int64
}

func (r *policyGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	return r.groups[id], nil
}

func (r *policyGroupRepo) ListActive(context.Context) ([]Group, error) {
	out := []Group{}
	for _, g := range r.groups {
		if g.Status == StatusActive {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (r *policyGroupRepo) GetAccountCount(_ context.Context, id int64) (int64, int64, error) {
	return r.accounts[id], r.accounts[id], nil
}

type policySubRepo struct {
	UserSubscriptionRepository
	subs []UserSubscription
}

func (r *policySubRepo) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	out := []UserSubscription{}
	for _, s := range r.subs {
		if s.UserID == userID && s.IsActive() {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *policySubRepo) ListByGroupID(_ context.Context, groupID int64, _ pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	out := []UserSubscription{}
	for _, s := range r.subs {
		if s.GroupID == groupID {
			out = append(out, s)
		}
	}
	return out, nil, nil
}

func policyFixture(subs ...UserSubscription) *RedeemService {
	g := func(id int64, name, platform string) *Group {
		return &Group{ID: id, Name: name, Platform: platform, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}
	}
	groups := map[int64]*Group{
		12: g(12, "Claude-Monthly-Basic", PlatformAnthropic),
		13: g(13, "Claude-Monthly-Standard", PlatformAnthropic),
		14: g(14, "Claude-Monthly-Advanced", PlatformAnthropic),
		20: g(20, ProNativeTemplateGroupName, PlatformAnthropic),
		21: g(21, ProNativeSeatGroupPrefix+"1", PlatformAnthropic),
		22: g(22, ProNativeSeatGroupPrefix+"2", PlatformAnthropic),
		23: g(23, ProNativeSeatGroupPrefix+"3", PlatformAnthropic),
		31: g(31, "GPT-Monthly-Basic", PlatformOpenAI),
		90: g(90, "AWS-Claude", PlatformAnthropic),
	}
	repo := &policySubRepo{subs: subs}
	return &RedeemService{subscriptionService: &SubscriptionService{
		groupRepo:   &policyGroupRepo{groups: groups, accounts: map[int64]int64{21: 1, 22: 1}}, // seat 3 has no account
		userSubRepo: repo,
	}}
}

func activeSub(userID, groupID int64) UserSubscription {
	return UserSubscription{ID: userID*100 + groupID, UserID: userID, GroupID: groupID, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(72 * time.Hour)}
}

func resolve(t *testing.T, svc *RedeemService, userID, groupID int64, days int) (*redeemSubscriptionTarget, error) {
	t.Helper()
	g := svc.subscriptionService.groupRepo.(*policyGroupRepo).groups[groupID]
	return svc.resolveSubscriptionRedeemTarget(context.Background(), userID, g, days)
}

func TestRedeemPolicySharedPlans(t *testing.T) {
	svc := policyFixture(activeSub(1, 13), activeSub(2, 12))

	target, err := resolve(t, svc, 9, 12, 30)
	require.NoError(t, err)
	require.Equal(t, int64(12), target.GroupID)
	require.Equal(t, 1, target.Concurrency)

	target, err = resolve(t, svc, 9, 14, 30)
	require.NoError(t, err)
	require.Equal(t, 2, target.Concurrency, "advanced plan includes two concurrent requests")

	_, err = resolve(t, svc, 1, 12, 30)
	require.ErrorIs(t, err, ErrRedeemActiveSharedPlan, "different Claude plan must not stack")

	target, err = resolve(t, svc, 2, 12, 30)
	require.NoError(t, err, "same plan renews")
	require.Equal(t, int64(12), target.GroupID)

	target, err = resolve(t, svc, 2, 31, 30)
	require.NoError(t, err, "other platform is independent")
	require.Equal(t, int64(31), target.GroupID)

	target, err = resolve(t, svc, 1, 90, 30)
	require.NoError(t, err, "historical groups keep original behaviour")
	require.Equal(t, int64(90), target.GroupID)
	require.Equal(t, 0, target.Concurrency)
}

func TestRedeemPolicyProNativeSeats(t *testing.T) {
	svc := policyFixture(activeSub(1, 21), activeSub(3, 12))

	target, err := resolve(t, svc, 2, 20, 30)
	require.NoError(t, err)
	require.Equal(t, int64(22), target.GroupID, "first free seat with an upstream account")

	target, err = resolve(t, svc, 1, 20, 30)
	require.NoError(t, err)
	require.Equal(t, int64(21), target.GroupID, "renewal keeps the held seat")

	_, err = resolve(t, svc, 3, 20, 30)
	require.ErrorIs(t, err, ErrRedeemActiveSharedPlan)

	_, err = resolve(t, svc, 1, 12, 30)
	require.ErrorIs(t, err, ErrRedeemActiveSharedPlan, "seat holder cannot stack a shared plan")

	_, err = resolve(t, svc, 2, 21, 30)
	require.ErrorIs(t, err, ErrRedeemSeatCodeInvalid)

	target, err = resolve(t, svc, 1, 20, -5)
	require.NoError(t, err)
	require.Equal(t, int64(21), target.GroupID, "deduction applies to the held seat")

	_, err = resolve(t, svc, 2, 20, -5)
	require.ErrorIs(t, err, ErrRedeemNoSeatToReduce)

	full := policyFixture(activeSub(1, 21), activeSub(4, 22))
	_, err = resolve(t, full, 2, 20, 30)
	require.ErrorIs(t, err, ErrRedeemNoFreeSeat)
}
