package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// "Claude 独享 Pro" is sold as one catalog product (the template group) backed
// by a pool of seats. A seat is a subscription group named
// Claude-ProNative-Seat-* that holds exactly one upstream account.
const (
	ProNativeTemplateGroupName = "Claude-Monthly-ProNative"
	ProNativeSeatGroupPrefix   = "Claude-ProNative-Seat-"
)

// ProNativeSeatAllocationMu serializes every seat sale (operator assignment and
// redeem codes). The application runs as a single process; holders must keep
// the lock until the allocating transaction has committed.
var ProNativeSeatAllocationMu sync.Mutex

var (
	ErrRedeemSeatCodeInvalid  = infraerrors.BadRequest("REDEEM_CODE_SEAT_GROUP", "独享席位不能直接生成或兑换兑换码，请使用“Claude 独享 Pro”商品")
	ErrRedeemActiveSharedPlan = infraerrors.Conflict("ACTIVE_SHARED_PLAN", "你已有其他生效中的同类套餐，不能叠加兑换。请联系管理员处理换档，兑换码未被使用")
	ErrRedeemNoFreeSeat       = infraerrors.Conflict("NO_FREE_PRONATIVE_SEAT", "Claude 独享 Pro 暂无空闲席位，请联系管理员补位，兑换码未被使用")
	ErrRedeemNoSeatToReduce   = infraerrors.NotFound("PRONATIVE_SEAT_NOT_FOUND", "没有生效中的 Claude 独享 Pro 可扣减")
)

func IsProNativeTemplateGroup(g *Group) bool {
	return g != nil && g.Name == ProNativeTemplateGroupName
}

func IsProNativeSeatGroup(g *Group) bool {
	return g != nil && strings.HasPrefix(g.Name, ProNativeSeatGroupPrefix) && len(g.Name) > len(ProNativeSeatGroupPrefix)
}

// IsCustomerPlanGroup reports whether a group is an explicitly approved
// customer-facing product. Historical private upstream groups are excluded.
func IsCustomerPlanGroup(g *Group) bool {
	if g == nil || g.Status != StatusActive {
		return false
	}
	if IsProNativeSeatGroup(g) {
		return g.Platform == PlatformAnthropic
	}
	switch g.Name {
	case "Claude-PAYG", "Claude-Monthly-Basic", "Claude-Monthly-Standard", "Claude-Monthly-Advanced", ProNativeTemplateGroupName:
		return g.Platform == PlatformAnthropic
	case "GPT-Monthly-Basic", "GPT-Monthly-Standard", "GPT-Monthly-Advanced":
		return g.Platform == PlatformOpenAI
	case "Grok-Monthly-Basic", "Grok-Monthly-Standard", "Grok-Monthly-Advanced":
		return g.Platform == PlatformGrok
	case "Gemini-Monthly-Basic", "Gemini-Monthly-Standard", "Gemini-Monthly-Advanced":
		return g.Platform == PlatformGemini
	}
	return false
}

// CustomerPlanConcurrency is the per-user concurrency a customer plan includes.
func CustomerPlanConcurrency(g *Group) int {
	if g != nil && strings.HasSuffix(g.Name, "-Advanced") {
		return 2
	}
	return 1
}

// redeemSubscriptionTarget is the group a subscription code really applies to.
type redeemSubscriptionTarget struct {
	GroupID     int64
	Concurrency int // minimum user concurrency to guarantee; 0 = leave unchanged
}

func (s *RedeemService) redeemGroup(ctx context.Context, groupID int64) (*Group, error) {
	if s.subscriptionService == nil || s.subscriptionService.groupRepo == nil {
		return nil, nil
	}
	g, err := s.subscriptionService.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get redeem group: %w", err)
	}
	return g, nil
}

// activeCustomerPlans lists the user's active subscriptions with their groups.
func (s *RedeemService) activeCustomerPlans(ctx context.Context, userID int64) ([]*Group, error) {
	subs, err := s.subscriptionService.userSubRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}
	out := []*Group{}
	for i := range subs {
		if !subs[i].IsActive() {
			continue
		}
		g := subs[i].Group
		if g == nil || g.Name == "" {
			if g, err = s.subscriptionService.groupRepo.GetByID(ctx, subs[i].GroupID); err != nil {
				return nil, fmt.Errorf("get subscription group: %w", err)
			}
		}
		if IsCustomerPlanGroup(g) {
			out = append(out, g)
		}
	}
	return out, nil
}

// freeProNativeSeats returns sellable seats: active seat groups with a
// schedulable upstream account and no active customer, lowest id first.
func (s *RedeemService) freeProNativeSeats(ctx context.Context) ([]Group, error) {
	groups, err := s.subscriptionService.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	free := []Group{}
	for i := range groups {
		g := &groups[i]
		if !IsProNativeSeatGroup(g) || !IsCustomerPlanGroup(g) || !g.IsSubscriptionType() {
			continue
		}
		_, active, err := s.subscriptionService.groupRepo.GetAccountCount(ctx, g.ID)
		if err != nil {
			return nil, fmt.Errorf("count seat accounts: %w", err)
		}
		if active == 0 {
			continue
		}
		subs, _, err := s.subscriptionService.userSubRepo.ListByGroupID(ctx, g.ID, pagination.PaginationParams{Page: 1, PageSize: 100})
		if err != nil {
			return nil, fmt.Errorf("list seat subscriptions: %w", err)
		}
		taken := false
		for j := range subs {
			if subs[j].IsActive() {
				taken = true
				break
			}
		}
		if !taken {
			free = append(free, *g)
		}
	}
	sort.Slice(free, func(a, b int) bool { return free[a].ID < free[b].ID })
	return free, nil
}

// resolveSubscriptionRedeemTarget applies the same sales rules operators follow:
//   - a customer keeps at most one active customer plan per platform; the same
//     plan may be renewed, a different one is rejected (no stacking);
//   - Claude 独享 Pro codes renew the seat the customer already holds, or
//     allocate a free seat; seats are never redeemed directly;
//   - plans that include extra concurrency raise the user's concurrency.
//
// Errors returned here roll back the transaction, so the code stays unused.
func (s *RedeemService) resolveSubscriptionRedeemTarget(ctx context.Context, userID int64, g *Group, validityDays int) (*redeemSubscriptionTarget, error) {
	target := &redeemSubscriptionTarget{GroupID: g.ID}
	if IsProNativeSeatGroup(g) {
		return nil, ErrRedeemSeatCodeInvalid
	}
	if !IsCustomerPlanGroup(g) {
		return target, nil // historical/private groups keep the original behaviour
	}

	plans, err := s.activeCustomerPlans(ctx, userID)
	if err != nil {
		return nil, err
	}
	var heldSeat *Group
	for _, other := range plans {
		if IsProNativeSeatGroup(other) {
			heldSeat = other
		}
	}

	if validityDays < 0 {
		if IsProNativeTemplateGroup(g) {
			if heldSeat == nil {
				return nil, ErrRedeemNoSeatToReduce
			}
			target.GroupID = heldSeat.ID
		}
		return target, nil
	}

	if IsProNativeTemplateGroup(g) {
		if heldSeat != nil {
			target.GroupID = heldSeat.ID // renewal keeps the same upstream account
		}
	}
	for _, other := range plans {
		if other.ID == target.GroupID || other.Platform != g.Platform {
			continue
		}
		return nil, ErrRedeemActiveSharedPlan
	}
	if IsProNativeTemplateGroup(g) && heldSeat == nil {
		seats, err := s.freeProNativeSeats(ctx)
		if err != nil {
			return nil, err
		}
		if len(seats) == 0 {
			return nil, ErrRedeemNoFreeSeat
		}
		target.GroupID = seats[0].ID
	}
	target.Concurrency = CustomerPlanConcurrency(g)
	return target, nil
}

// ensureRedeemConcurrency raises (never lowers) the user's concurrency.
func (s *RedeemService) ensureRedeemConcurrency(ctx context.Context, userID int64, minimum int) error {
	if minimum <= 0 {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user.Concurrency >= minimum {
		return nil
	}
	return s.userRepo.UpdateConcurrency(ctx, userID, minimum-user.Concurrency)
}

// ValidateRedeemSubscriptionGroup rejects codes that would bypass seat allocation.
func ValidateRedeemSubscriptionGroup(g *Group) error {
	if IsProNativeSeatGroup(g) {
		return ErrRedeemSeatCodeInvalid
	}
	return nil
}
