package licensing

import (
	"testing"
	"time"
)

func TestOverflowBonus_FreeTierWithinWindow(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-24 * time.Hour).Unix() // granted 1 day ago

	bonus := OverflowBonus(TierFree, &grantedAt, now)
	if bonus != 1 {
		t.Fatalf("expected bonus=1 within 14-day window, got %d", bonus)
	}
}

func TestOverflowBonus_FreeTierExpired(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-15 * 24 * time.Hour).Unix() // granted 15 days ago

	bonus := OverflowBonus(TierFree, &grantedAt, now)
	if bonus != 0 {
		t.Fatalf("expected bonus=0 after 14-day window, got %d", bonus)
	}
}

func TestOverflowBonus_FreeTierExactExpiry(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-14 * 24 * time.Hour).Unix() // exactly 14 days ago

	bonus := OverflowBonus(TierFree, &grantedAt, now)
	if bonus != 0 {
		t.Fatalf("expected bonus=0 at exactly 14 days, got %d", bonus)
	}
}

func TestOverflowBonus_NonFreeTier(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-1 * time.Hour).Unix()

	for _, tier := range []Tier{TierRelay, TierPro, TierProPlus, TierCloud, TierMSP, TierEnterprise} {
		bonus := OverflowBonus(tier, &grantedAt, now)
		if bonus != 0 {
			t.Errorf("tier=%s: expected bonus=0 for non-free tier, got %d", tier, bonus)
		}
	}
}

func TestOverflowBonus_NilGrantedAt(t *testing.T) {
	now := time.Now()

	bonus := OverflowBonus(TierFree, nil, now)
	if bonus != 0 {
		t.Fatalf("expected bonus=0 when OverflowGrantedAt is nil, got %d", bonus)
	}
}

func TestOverflowDaysRemaining_Active(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-2 * 24 * time.Hour).Unix() // 2 days ago

	days := OverflowDaysRemaining(TierFree, &grantedAt, now)
	if days < 11 || days > 13 {
		t.Fatalf("expected ~12 days remaining, got %d", days)
	}
}

func TestOverflowDaysRemaining_Expired(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-15 * 24 * time.Hour).Unix()

	days := OverflowDaysRemaining(TierFree, &grantedAt, now)
	if days != 0 {
		t.Fatalf("expected 0 days remaining after expiry, got %d", days)
	}
}

func TestOverflowDaysRemaining_NonFree(t *testing.T) {
	now := time.Now()
	grantedAt := now.Add(-1 * time.Hour).Unix()

	days := OverflowDaysRemaining(TierPro, &grantedAt, now)
	if days != 0 {
		t.Fatalf("expected 0 days remaining for non-free tier, got %d", days)
	}
}

func TestOverflowDaysRemaining_LastDay(t *testing.T) {
	now := time.Now()
	// Grant 13 days and 23 hours ago — should have 1 day remaining (ceiling).
	grantedAt := now.Add(-(13*24 + 23) * time.Hour).Unix()

	days := OverflowDaysRemaining(TierFree, &grantedAt, now)
	if days != 1 {
		t.Fatalf("expected 1 day remaining on last partial day, got %d", days)
	}
}

func TestOverflowBonus_FutureTimestamp(t *testing.T) {
	now := time.Now()
	futureGrantedAt := now.Add(1 * time.Hour).Unix() // granted in the future

	bonus := OverflowBonus(TierFree, &futureGrantedAt, now)
	if bonus != 0 {
		t.Fatalf("expected bonus=0 for future-dated grant, got %d", bonus)
	}
}

func TestOverflowSetOnceSemantics(t *testing.T) {
	// Verify that OverflowGrantedAt is a simple timestamp that doesn't change.
	// The set-once semantics are enforced by the handler, but we verify
	// that the bonus calculation is stable given a fixed grant timestamp.
	now := time.Now()
	grantedAt := now.Add(-5 * 24 * time.Hour).Unix()

	// Simulating "time passes" — check at different points.
	bonus1 := OverflowBonus(TierFree, &grantedAt, now)
	bonus2 := OverflowBonus(TierFree, &grantedAt, now.Add(8*24*time.Hour))  // day 13
	bonus3 := OverflowBonus(TierFree, &grantedAt, now.Add(10*24*time.Hour)) // day 15

	if bonus1 != 1 {
		t.Fatalf("day 5: expected bonus=1, got %d", bonus1)
	}
	if bonus2 != 1 {
		t.Fatalf("day 13: expected bonus=1, got %d", bonus2)
	}
	if bonus3 != 0 {
		t.Fatalf("day 15: expected bonus=0, got %d", bonus3)
	}
}
