package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGuestMetadataHandler_Reload(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	handler := NewGuestMetadataHandler(mtp)

	if err := handler.Reload(); err != nil {
		t.Fatalf("Reload error: %v", err)
	}
}
