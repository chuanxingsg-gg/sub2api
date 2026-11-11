package service

import (
	"context"
	"encoding/json"
	"math"
	"time"
)

// ProNativeSeatQuotaRows returns the 5h / weekly upstream windows of the Claude
// account bound to a ProNative seat group, in the row format that CC Switch's
// Claude subscription card expects. It never exposes the upstream e-mail.
// ok=false means no bound account could report usage.
func (s *AccountUsageService) ProNativeSeatQuotaRows(ctx context.Context, groupID int64) ([]map[string]any, bool) {
	accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, false
	}
	for i := range accounts {
		acc := &accounts[i]
		if acc.Platform != PlatformAnthropic || !acc.IsSchedulable() || !acc.CanGetUsage() {
			continue
		}
		info, err := s.getUsageForAccount(ctx, acc, false)
		if err != nil || info == nil || info.FiveHour == nil {
			continue
		}
		updated := time.Now()
		if info.UpdatedAt != nil {
			updated = *info.UpdatedAt
		}
		rows := []map[string]any{
			proNativeQuotaRow("5h", "5小时", info.FiveHour, updated),
		}
		if info.SevenDay != nil {
			rows = append(rows, proNativeQuotaRow("weekly", "每周", info.SevenDay, updated))
		} else {
			return nil, false
		}
		return rows, true
	}
	return nil, false
}

func proNativeQuotaRow(window, label string, p *UsageProgress, updated time.Time) map[string]any {
	used := math.Round(math.Min(100, math.Max(0, p.Utilization))*100) / 100
	meta := map[string]any{
		"kind":      "claude_oauth_quota_v1",
		"window":    window,
		"account":   map[string]any{"plan": "Claude Pro"},
		"updatedAt": updated.UTC().Format(time.RFC3339Nano),
		"resetsAt":  nil,
	}
	if p.ResetsAt != nil {
		meta["resetsAt"] = p.ResetsAt.UTC().Format(time.RFC3339Nano)
	}
	extra, _ := json.Marshal(meta)
	return map[string]any{
		"planName":  label,
		"isValid":   true,
		"unit":      "%",
		"total":     100,
		"used":      used,
		"remaining": math.Round((100-used)*100) / 100,
		"extra":     string(extra),
	}
}
