package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"unicode/utf8"
)

const HomeModelCardsKey = "home_model_cards_v1"

type HomeModelCard struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Logo string  `json:"logo"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}
type HomeModelCards struct {
	Mode  string          `json:"mode"`
	Cards []HomeModelCard `json:"cards"`
}

func DefaultHomeModelCards() HomeModelCards {
	return HomeModelCards{Mode: "random", Cards: []HomeModelCard{
		{"gpt", "GPT", "builtin:GPT", .22, .15}, {"claude", "Claude", "builtin:Claude", .76, .27},
		{"gemini", "Gemini", "builtin:Gemini", .33, .48}, {"deepseek", "DeepSeek", "builtin:DeepSeek", .72, .59},
		{"qwen", "Qwen", "builtin:Qwen", .19, .8}, {"more", "更多模型", "", .79, .87},
	}}
}
func ValidateHomeModelCards(v HomeModelCards) error {
	if v.Mode != "random" && v.Mode != "manual" {
		return fmt.Errorf("请选择随机或手动排列")
	}
	if len(v.Cards) < 2 || len(v.Cards) > 10 {
		return fmt.Errorf("卡片数量须为 2 至 10 张（包含更多模型）")
	}
	ids := map[string]bool{}
	more := false
	total := 0
	for _, c := range v.Cards {
		if len(c.ID) < 1 || len(c.ID) > 48 || ids[c.ID] {
			return fmt.Errorf("卡片编号无效或重复")
		}
		for _, r := range c.ID {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return fmt.Errorf("卡片编号无效")
			}
		}
		ids[c.ID] = true
		if c.ID == "more" {
			more = true
		}
		if strings.TrimSpace(c.Name) == "" || utf8.RuneCountInString(c.Name) > 24 {
			return fmt.Errorf("模型名称须为 1 至 24 个字")
		}
		if math.IsNaN(c.X) || math.IsNaN(c.Y) || math.IsInf(c.X, 0) || math.IsInf(c.Y, 0) || c.X < 0 || c.X > 1 || c.Y < 0 || c.Y > 1 {
			return fmt.Errorf("卡片位置须在展示区域内")
		}
		if err := validateHomeLogo(c.Logo); err != nil {
			return err
		}
		total += len(c.Logo)
	}
	if !more {
		return fmt.Errorf("请保留更多模型卡片")
	}
	if total > 1500000 {
		return fmt.Errorf("Logo 总大小过大，请压缩后重试")
	}
	return nil
}
func validateHomeLogo(s string) error {
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "builtin:") {
		for _, n := range []string{"GPT", "Claude", "Gemini", "DeepSeek", "Qwen", "Grok"} {
			if s == "builtin:"+n {
				return nil
			}
		}
		return fmt.Errorf("内置 Logo 无效")
	}
	if strings.HasPrefix(s, "https://") {
		u, e := url.Parse(s)
		if e == nil && u.Hostname() != "" && u.User == nil && len(s) <= 2048 {
			return nil
		}
	}
	for _, mime := range []string{"png", "jpeg", "webp", "gif"} {
		prefix := "data:image/" + mime + ";base64,"
		if strings.HasPrefix(s, prefix) {
			b, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
			if e == nil && len(b) > 0 && len(b) <= 150000 {
				return nil
			}
		}
	}
	return fmt.Errorf("Logo 请使用内置图标、HTTPS 图片地址或 150KB 以内的 PNG/JPG/WebP/GIF 图片")
}
func (s *SettingService) GetHomeModelCards(ctx context.Context) (HomeModelCards, error) {
	values, e := s.settingRepo.GetMultiple(ctx, []string{HomeModelCardsKey})
	if e != nil {
		return HomeModelCards{}, e
	}
	raw := values[HomeModelCardsKey]
	if raw == "" {
		return DefaultHomeModelCards(), nil
	}
	var v HomeModelCards
	if e = json.Unmarshal([]byte(raw), &v); e != nil {
		return v, e
	}
	if e = ValidateHomeModelCards(v); e != nil {
		return v, e
	}
	return v, nil
}
func (s *SettingService) SaveHomeModelCards(ctx context.Context, v HomeModelCards) error {
	if e := ValidateHomeModelCards(v); e != nil {
		return e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return s.settingRepo.Set(ctx, HomeModelCardsKey, string(b))
}
