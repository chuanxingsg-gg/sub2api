//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type homeRepo struct{ settingPublicRepoStub }

func (r *homeRepo) Set(_ context.Context, key, value string) error {
	if r.err != nil {
		return r.err
	}
	r.values[key] = value
	return nil
}
func TestHomeModelCardsPersistenceAndIsolation(t *testing.T) {
	repo := &homeRepo{settingPublicRepoStub{values: map[string]string{"site_name": "Existing"}}}
	s := NewSettingService(repo, nil)
	ctx := context.Background()
	v, e := s.GetHomeModelCards(ctx)
	require.NoError(t, e)
	require.Len(t, v.Cards, 6)
	v.Mode = "manual"
	v.Cards[0].Name = "自定义模型"
	v.Cards[0].Logo = "https://example.com/logo.png"
	v.Cards[0].X = .42
	require.NoError(t, s.SaveHomeModelCards(ctx, v))
	got, e := s.GetHomeModelCards(ctx)
	require.NoError(t, e)
	require.Equal(t, v, got)
	require.Equal(t, "Existing", repo.values["site_name"])
	require.Contains(t, repo.values[HomeModelCardsKey], "自定义模型")
	repo.err = errors.New("database unavailable")
	_, e = s.GetHomeModelCards(ctx)
	require.Error(t, e)
}
func TestHomeModelCardsValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*HomeModelCards)
	}{
		{"missing more", func(v *HomeModelCards) { v.Cards = v.Cards[:5] }},
		{"duplicate IDs", func(v *HomeModelCards) { v.Cards[0].ID = "more" }},
		{"invalid mode", func(v *HomeModelCards) { v.Mode = "other" }},
		{"empty name", func(v *HomeModelCards) { v.Cards[0].Name = "  " }},
		{"offscreen position", func(v *HomeModelCards) { v.Cards[0].X = 1.1 }},
		{"javascript logo", func(v *HomeModelCards) { v.Cards[0].Logo = "javascript:alert(1)" }},
		{"SVG upload", func(v *HomeModelCards) { v.Cards[0].Logo = "data:image/svg+xml;base64,PHN2Zz4=" }},
		{"invalid builtin", func(v *HomeModelCards) { v.Cards[0].Logo = "builtin:unknown" }},
		{"insecure URL", func(v *HomeModelCards) { v.Cards[0].Logo = "http://example.com/logo.png" }},
		{"too many", func(v *HomeModelCards) {
			for i := 0; i < 10; i++ {
				v.Cards = append(v.Cards, v.Cards[0])
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := DefaultHomeModelCards()
			tc.change(&v)
			require.Error(t, ValidateHomeModelCards(v))
		})
	}
}
func TestHomeModelCardsCorruptConfigNotSilentlyReplaced(t *testing.T) {
	r := &homeRepo{settingPublicRepoStub{values: map[string]string{HomeModelCardsKey: "bad-json"}}}
	s := NewSettingService(r, nil)
	_, e := s.GetHomeModelCards(context.Background())
	require.Error(t, e)
	require.Equal(t, "bad-json", r.values[HomeModelCardsKey])
	v := DefaultHomeModelCards()
	b, e := json.Marshal(v)
	require.NoError(t, e)
	r.values[HomeModelCardsKey] = string(b)
	_, e = s.GetHomeModelCards(context.Background())
	require.NoError(t, e)
}
