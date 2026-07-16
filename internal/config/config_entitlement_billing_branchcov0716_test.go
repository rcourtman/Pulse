package config

import (
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/assert"
)

// This file adds branch coverage for the entitlement-billing helpers in
// entitlement_billing_state.go: NormalizeEntitlementBillingOrgID and
// billingStateHasEntitlementAuthority. Test functions use the BranchCov prefix
// so they can be selected with `-run BranchCov`.

// TestBranchCovNormalizeEntitlementBillingOrgID covers both branches of
// NormalizeEntitlementBillingOrgID: the empty-after-trim arm that returns the
// "default" sentinel, and the non-empty arm that returns the trimmed value
// verbatim. It also exercises whitespace-only input (mixing spaces, tabs and
// newlines), the literal "default" id, and the fact that internal whitespace is
// preserved (only leading/trailing whitespace is trimmed).
func TestBranchCovNormalizeEntitlementBillingOrgID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		// --- Empty-after-trim branch: returns entitlementBillingDefaultOrgID. ---
		{
			name: "empty string falls back to default",
			raw:  "",
			want: "default",
		},
		{
			name: "single space falls back to default",
			raw:  " ",
			want: "default",
		},
		{
			name: "mixed whitespace only falls back to default",
			raw:  "  \t\n\r\t ",
			want: "default",
		},

		// --- Non-empty branch: returns the trimmed value verbatim. ---
		{
			name: "plain id returned as-is",
			raw:  "org-123",
			want: "org-123",
		},
		{
			name: "leading and trailing whitespace trimmed",
			raw:  "  org-123\t\n",
			want: "org-123",
		},
		{
			name: "internal whitespace is preserved only outer trimmed",
			raw:  "  org 456  ",
			want: "org 456",
		},
		{
			name: "literal default id returned as default",
			raw:  "default",
			want: "default",
		},
		{
			name: "literal default id with surrounding whitespace trimmed to default",
			raw:  "   default   ",
			want: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NormalizeEntitlementBillingOrgID(tc.raw))
		})
	}
}

// TestBranchCovBillingStateHasEntitlementAuthority covers every branch of
// billingStateHasEntitlementAuthority:
//   - the nil-state early return (false)
//   - each of the three OR operands independently granting authority when set
//     (SubscriptionState, EntitlementJWT, EntitlementRefreshToken)
//   - each operand being whitespace-only so its TrimSpace collapses to "" and
//     it does NOT grant authority
//   - a fully empty/zero BillingState (all three operands empty) returning false
//   - a fully populated BillingState returning true
//
// Because billingStateHasEntitlementAuthority is an unexported helper in this
// package, the test lives in-package (`package config`) and invokes it
// directly rather than driving it through LoadEffectiveEntitlementBillingState.
func TestBranchCovBillingStateHasEntitlementAuthority(t *testing.T) {
	tests := []struct {
		name  string
		state *pkglicensing.BillingState
		want  bool
	}{
		// --- nil guard branch. ---
		{
			name:  "nil state returns false",
			state: nil,
			want:  false,
		},

		// --- Empty / zero BillingState: all three operands empty. ---
		{
			name:  "empty billing state returns false",
			state: &pkglicensing.BillingState{},
			want:  false,
		},

		// --- Each operand alone grants authority (non-whitespace value). ---
		{
			name: "subscription state alone grants authority",
			state: &pkglicensing.BillingState{
				SubscriptionState: pkglicensing.SubStateActive,
			},
			want: true,
		},
		{
			name: "subscription state trial grants authority",
			state: &pkglicensing.BillingState{
				SubscriptionState: pkglicensing.SubStateTrial,
			},
			want: true,
		},
		{
			name: "entitlement jwt alone grants authority",
			state: &pkglicensing.BillingState{
				EntitlementJWT: "eyJhbGciOiJIUzI1NiJ9.payload.sig",
			},
			want: true,
		},
		{
			name: "entitlement refresh token alone grants authority",
			state: &pkglicensing.BillingState{
				EntitlementRefreshToken: "refresh-token-abc",
			},
			want: true,
		},

		// --- Each operand whitespace-only does NOT grant authority. ---
		{
			name: "whitespace only subscription state does not grant authority",
			state: &pkglicensing.BillingState{
				SubscriptionState: pkglicensing.SubscriptionState("   "),
			},
			want: false,
		},
		{
			name: "whitespace only entitlement jwt does not grant authority",
			state: &pkglicensing.BillingState{
				EntitlementJWT: "\t\n ",
			},
			want: false,
		},
		{
			name: "whitespace only refresh token does not grant authority",
			state: &pkglicensing.BillingState{
				EntitlementRefreshToken: " \r\n\t ",
			},
			want: false,
		},

		// --- All three operands whitespace-only exercises every OR arm false. ---
		{
			name: "all three whitespace only returns false",
			state: &pkglicensing.BillingState{
				SubscriptionState:       pkglicensing.SubscriptionState(" "),
				EntitlementJWT:          " ",
				EntitlementRefreshToken: " ",
			},
			want: false,
		},

		// --- All three populated returns true. ---
		{
			name: "all three populated returns true",
			state: &pkglicensing.BillingState{
				SubscriptionState:       pkglicensing.SubStateGrace,
				EntitlementJWT:          "jwt",
				EntitlementRefreshToken: "refresh",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, billingStateHasEntitlementAuthority(tc.state))
		})
	}
}
