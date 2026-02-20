package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlePublicMagicLinkVerifyRejectsInvalidOrgIDInToken(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	store := NewInMemoryMagicLinkStore()
	svc := NewMagicLinkServiceWithKey(key, store, nil, nil)
	t.Cleanup(func() { svc.Stop() })

	token, err := randomOpaqueTokenID()
	if err != nil {
		t.Fatalf("randomOpaqueTokenID: %v", err)
	}
	tokenHash := signHMACSHA256(key, token)
	if err := store.Put(tokenHash, &MagicLinkToken{
		Email:     "alice@example.com",
		OrgID:     "../evil",
		ExpiresAt: time.Now().Add(5 * time.Minute).UTC(),
	}); err != nil {
		t.Fatalf("store.Put: %v", err)
	}

	h := NewMagicLinkHandlers(nil, svc, true, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/public/magic-link/verify?token="+token, nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	h.HandlePublicMagicLinkVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid org ID in token, got %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_session" {
			t.Fatalf("did not expect pulse_session cookie on invalid token org")
		}
	}
}
