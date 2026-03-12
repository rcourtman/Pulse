package api

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func apiTokenBoundUser(record *config.APITokenRecord) string {
	if record == nil {
		return ""
	}
	ownerUserID := strings.TrimSpace(apiTokenOwnerUserID(*record))
	if ownerUserID == "" || strings.HasPrefix(ownerUserID, "token:") {
		return ""
	}
	return ownerUserID
}

func apiTokenAuthenticatedUser(record *config.APITokenRecord) string {
	if ownerUserID := apiTokenBoundUser(record); ownerUserID != "" {
		return ownerUserID
	}
	if record == nil || strings.TrimSpace(record.ID) == "" {
		return ""
	}
	return fmt.Sprintf("token:%s", strings.TrimSpace(record.ID))
}
