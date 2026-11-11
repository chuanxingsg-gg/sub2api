package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestNextProNativeSeatNumber(t *testing.T) {
	groups := []service.Group{
		{Name: "Claude-Monthly-ProNative"},
		{Name: "Claude-ProNative-Seat-1"},
		{Name: "Claude-ProNative-Seat-7"},
		{Name: "Claude-ProNative-Seat-x"},
		{Name: "Claude-PAYG"},
	}
	if got := nextProNativeSeatNumber(groups); got != 8 {
		t.Fatalf("got %d want 8", got)
	}
	if got := nextProNativeSeatNumber(nil); got != 1 {
		t.Fatalf("got %d want 1", got)
	}
}

func TestProNativeSeatSourcePrefersOldestActiveSeat(t *testing.T) {
	groups := []service.Group{
		{ID: 1, Name: "Claude-Monthly-ProNative", Status: service.StatusActive},
		{ID: 9, Name: "Claude-ProNative-Seat-2", Status: service.StatusActive},
		{ID: 5, Name: "Claude-ProNative-Seat-1", Status: service.StatusActive},
	}
	if src := proNativeSeatSource(groups); src == nil || src.ID != 5 {
		t.Fatalf("unexpected source %+v", src)
	}
	if src := proNativeSeatSource(groups[:1]); src == nil || src.ID != 1 {
		t.Fatalf("template fallback failed: %+v", src)
	}
}

func TestProNativeSeatInputForcesSubscription(t *testing.T) {
	in := proNativeSeatInput(&service.Group{SubscriptionType: "standard", Platform: "openai"}, 3)
	if in.Name != "Claude-ProNative-Seat-3" || in.SubscriptionType != service.SubscriptionTypeSubscription || in.Platform != service.PlatformAnthropic {
		t.Fatalf("unexpected input %+v", in)
	}
}

func TestValidateAutoProNativeSeat(t *testing.T) {
	if validateAutoProNativeSeat("anthropic", "oauth") != nil || validateAutoProNativeSeat("anthropic", "setup-token") != nil {
		t.Fatal("oauth/setup-token must be allowed")
	}
	if validateAutoProNativeSeat("anthropic", "apikey") == nil || validateAutoProNativeSeat("openai", "oauth") == nil {
		t.Fatal("others must be rejected")
	}
}
