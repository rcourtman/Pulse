package licensing

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testPurchaseReturnSigningKey() []byte {
	return bytes.Repeat([]byte{0x42}, 32)
}

func TestSignAndVerifyPurchaseReturnToken(t *testing.T) {
	token, err := SignPurchaseReturnToken(testPurchaseReturnSigningKey(), PurchaseReturnClaims{
		OrgID:        "default",
		Feature:      "max_monitored_systems",
		InstanceHost: "pulse.example.com",
		ReturnURL:    "https://pulse.example.com/auth/license-purchase-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignPurchaseReturnToken: %v", err)
	}

	claims, err := VerifyPurchaseReturnToken(token, testPurchaseReturnSigningKey(), "pulse.example.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyPurchaseReturnToken: %v", err)
	}
	if claims.OrgID != "default" {
		t.Fatalf("claims.OrgID=%q, want %q", claims.OrgID, "default")
	}
	if claims.Feature != "max_monitored_systems" {
		t.Fatalf("claims.Feature=%q, want %q", claims.Feature, "max_monitored_systems")
	}
	if claims.InstanceHost != "pulse.example.com" {
		t.Fatalf("claims.InstanceHost=%q, want %q", claims.InstanceHost, "pulse.example.com")
	}
}

func TestVerifyPurchaseReturnToken_HostMismatch(t *testing.T) {
	token, err := SignPurchaseReturnToken(testPurchaseReturnSigningKey(), PurchaseReturnClaims{
		OrgID:        "default",
		InstanceHost: "pulse-a.example.com",
		ReturnURL:    "https://pulse-a.example.com/auth/license-purchase-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignPurchaseReturnToken: %v", err)
	}

	_, err = VerifyPurchaseReturnToken(token, testPurchaseReturnSigningKey(), "pulse-b.example.com", time.Now())
	if !errors.Is(err, ErrPurchaseReturnHostMismatch) {
		t.Fatalf("VerifyPurchaseReturnToken() error=%v, want %v", err, ErrPurchaseReturnHostMismatch)
	}
}

func TestValidatePurchaseReturnURL(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		expectedHost string
		wantHost     string
		wantErr      error
	}{
		{
			name:         "https public host",
			raw:          "https://pulse.example.com/auth/license-purchase-activate",
			expectedHost: "pulse.example.com",
			wantHost:     "pulse.example.com",
		},
		{
			name:     "http localhost allowed",
			raw:      "http://localhost:7655/auth/license-purchase-activate",
			wantHost: "localhost",
		},
		{
			name:    "missing return url",
			raw:     "",
			wantErr: ErrPurchaseReturnReturnURLMissing,
		},
		{
			name:    "http public host rejected",
			raw:     "http://pulse.example.com/auth/license-purchase-activate",
			wantErr: ErrPurchaseReturnReturnURLInvalid,
		},
		{
			name:    "query string rejected",
			raw:     "https://pulse.example.com/auth/license-purchase-activate?token=x",
			wantErr: ErrPurchaseReturnReturnURLInvalid,
		},
		{
			name:         "host mismatch rejected",
			raw:          "https://pulse.example.com/auth/license-purchase-activate",
			expectedHost: "other.example.com",
			wantErr:      ErrPurchaseReturnHostMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, err := ValidatePurchaseReturnURL(tt.raw, tt.expectedHost)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidatePurchaseReturnURL() error=%v, want %v", err, tt.wantErr)
			}
			if gotHost != tt.wantHost {
				t.Fatalf("ValidatePurchaseReturnURL() host=%q, want %q", gotHost, tt.wantHost)
			}
		})
	}
}

func TestSignPurchaseReturnToken_RequiresReturnURL(t *testing.T) {
	_, err := SignPurchaseReturnToken(testPurchaseReturnSigningKey(), PurchaseReturnClaims{
		OrgID:        "default",
		InstanceHost: "pulse.example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
		},
	})
	if !errors.Is(err, ErrPurchaseReturnReturnURLMissing) {
		t.Fatalf("SignPurchaseReturnToken() error=%v, want %v", err, ErrPurchaseReturnReturnURLMissing)
	}
}
