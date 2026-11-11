package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// nextProNativeSeatNumber returns max(existing seat suffix)+1 so seat names stay
// unique and increasing even when seats were deleted or disabled.
func nextProNativeSeatNumber(groups []service.Group) int {
	next := 1
	for i := range groups {
		if !service.IsProNativeSeatGroup(&groups[i]) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(groups[i].Name, service.ProNativeSeatGroupPrefix))
		if err == nil && n >= next {
			next = n + 1
		}
	}
	return next
}

// proNativeSeatSource picks the group whose settings a new seat inherits: the
// oldest active seat (so seats stay identical), else the catalog template.
func proNativeSeatSource(groups []service.Group) *service.Group {
	var seat, tpl *service.Group
	for i := range groups {
		g := &groups[i]
		switch {
		case service.IsProNativeSeatGroup(g) && g.Status == service.StatusActive:
			if seat == nil || g.ID < seat.ID {
				seat = g
			}
		case service.IsProNativeTemplateGroup(g):
			tpl = g
		}
	}
	if seat != nil {
		return seat
	}
	return tpl
}

func proNativeSeatInput(src *service.Group, number int) *service.CreateGroupInput {
	peak := src.PeakRateMultiplier
	return &service.CreateGroupInput{
		Name:                        fmt.Sprintf("%s%d", service.ProNativeSeatGroupPrefix, number),
		Description:                 src.Description,
		Platform:                    service.PlatformAnthropic,
		RateMultiplier:              src.RateMultiplier,
		IsExclusive:                 src.IsExclusive,
		SubscriptionType:            service.SubscriptionTypeSubscription,
		DailyLimitUSD:               src.DailyLimitUSD,
		WeeklyLimitUSD:              src.WeeklyLimitUSD,
		MonthlyLimitUSD:             src.MonthlyLimitUSD,
		LongContextPricingEnabled:   src.LongContextPricingEnabled,
		ModelPricing:                src.ModelPricing,
		PeakRateEnabled:             src.PeakRateEnabled,
		PeakStart:                   src.PeakStart,
		PeakEnd:                     src.PeakEnd,
		PeakRateMultiplier:          &peak,
		ClaudeCodeOnly:              src.ClaudeCodeOnly,
		FallbackGroupID:             src.FallbackGroupID,
		RPMLimit:                    src.RPMLimit,
		MaxReasoningEffort:          src.MaxReasoningEffort,
		MaxReasoningEffortOverLimit: src.MaxReasoningEffortOverLimit,
		ReasoningEffortMappings:     src.ReasoningEffortMappings,
	}
}

// createProNativeSeatGroup creates the next Claude-ProNative-Seat-N group.
// It serializes on the seat allocation lock so concurrent imports never reuse a number.
func (h *AccountHandler) createProNativeSeatGroup(ctx context.Context) (*service.Group, error) {
	service.ProNativeSeatAllocationMu.Lock()
	defer service.ProNativeSeatAllocationMu.Unlock()
	groups, err := h.adminService.GetAllGroupsIncludingInactive(ctx)
	if err != nil {
		return nil, err
	}
	src := proNativeSeatSource(groups)
	if src == nil {
		return nil, infraerrors.BadRequest("PRONATIVE_TEMPLATE_MISSING", "未找到独享 Pro 模板分组 "+service.ProNativeTemplateGroupName+"，无法自动创建席位")
	}
	return h.adminService.CreateGroup(ctx, proNativeSeatInput(src, nextProNativeSeatNumber(groups)))
}

// validateAutoProNativeSeat limits the option to Claude subscription-login accounts.
func validateAutoProNativeSeat(platform, accountType string) error {
	if platform != service.PlatformAnthropic || (accountType != "oauth" && accountType != "setup-token") {
		return infraerrors.BadRequest("PRONATIVE_AUTO_SEAT_UNSUPPORTED", "自动创建独享席位仅支持 Claude 的 OAuth / Setup Token 账号")
	}
	return nil
}
