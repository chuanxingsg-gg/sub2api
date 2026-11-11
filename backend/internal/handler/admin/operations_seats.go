package admin

import (
	"context"
	"sort"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// "Claude 独享 Pro" is sold as one catalog product backed by a pool of seats.
// A seat is a subscription group named Claude-ProNative-Seat-* that holds exactly
// one upstream account. The product's template group carries no accounts and is
// only used as the stable catalog/price anchor.
const (
	proNativeTemplate   = service.ProNativeTemplateGroupName
	proNativeSeatPrefix = service.ProNativeSeatGroupPrefix
)

// Seat allocation is check-then-assign. Operator sales and redeem codes share
// one process-wide lock so two sales can never take the same seat.
var seatAllocationMu = &service.ProNativeSeatAllocationMu

func isProNativeTemplate(g *service.Group) bool { return service.IsProNativeTemplateGroup(g) }

func isProNativeSeat(g *service.Group) bool { return service.IsProNativeSeatGroup(g) }

// seatHasActiveSubscription reports whether any customer currently holds the seat.
func (h *OperationsHandler) seatHasActiveSubscription(ctx context.Context, seatID int64) (bool, error) {
	subs, _, err := h.subscriptions.List(ctx, 1, 100, nil, &seatID, service.SubscriptionStatusActive, "", "created_at", "desc")
	if err != nil {
		return false, err
	}
	for i := range subs {
		if subs[i].IsActive() {
			return true, nil
		}
	}
	return false, nil
}

// selectFreeSeats is the pure allocation rule: a seat is sellable when its group
// is an active subscription seat, has a working upstream account and no active
// customer. Lowest id first so allocation is predictable.
func selectFreeSeats(groups []service.Group, hasAccount func(*service.Group) (bool, error), taken func(int64) (bool, error)) ([]service.Group, error) {
	free := []service.Group{}
	for i := range groups {
		g := &groups[i]
		if !isProNativeSeat(g) || !operationsGroup(g) || g.SubscriptionType != service.SubscriptionTypeSubscription {
			continue
		}
		ok, err := hasAccount(g)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		busy, err := taken(g.ID)
		if err != nil {
			return nil, err
		}
		if !busy {
			free = append(free, *g)
		}
	}
	sort.Slice(free, func(a, b int) bool { return free[a].ID < free[b].ID })
	return free, nil
}

func (h *OperationsHandler) freeSeats(ctx context.Context) ([]service.Group, error) {
	groups, err := h.users.GetAllGroups(ctx)
	if err != nil {
		return nil, err
	}
	return selectFreeSeats(groups,
		func(g *service.Group) (bool, error) { return h.catalogReady(ctx, g) },
		func(id int64) (bool, error) { return h.seatHasActiveSubscription(ctx, id) })
}

// customerSeat returns the seat group the customer currently holds, if any.
func (h *OperationsHandler) customerSeat(ctx context.Context, userID int64) (*service.Group, error) {
	subs, _, err := h.subscriptions.List(ctx, 1, 100, &userID, nil, service.SubscriptionStatusActive, "", "created_at", "desc")
	if err != nil {
		return nil, err
	}
	for i := range subs {
		if !subs[i].IsActive() {
			continue
		}
		g, err := h.users.GetGroup(ctx, subs[i].GroupID)
		if err != nil {
			return nil, err
		}
		if isProNativeSeat(g) {
			return g, nil
		}
	}
	return nil, nil
}

// assignProNative sells one seat. It must be called with seatAllocationMu held.
func (h *OperationsHandler) assignProNative(ctx context.Context, userID int64, in service.AssignSubscriptionInput) (*service.UserSubscription, error) {
	if held, err := h.customerSeat(ctx, userID); err != nil {
		return nil, err
	} else if held != nil {
		return nil, infraerrors.Conflict("ACTIVE_PRONATIVE_SEAT", "客户已有有效的 Claude 独享 Pro，请使用“续期”，不要重复开通")
	}
	seats, err := h.freeSeats(ctx)
	if err != nil {
		return nil, err
	}
	if len(seats) == 0 {
		return nil, infraerrors.Conflict("NO_FREE_PRONATIVE_SEAT", "Claude 独享 Pro 席位已满，请先接入新的 Pro 账号并建立席位")
	}
	in.GroupID = seats[0].ID
	return h.subscriptions.AssignSubscription(ctx, &in)
}
