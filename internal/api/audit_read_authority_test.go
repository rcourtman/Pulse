package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"testing"
)

func TestAuditReadCurrentAuthorityAndPrefix(t *testing.T) {
	responseCurrentAuthorityAndPrefix(t, config.ScopeAuditRead, responseMediaAudit)
}

func TestAuditReadKeepsOrganizationBoundAuthority(t *testing.T) {
	responseKeepsOrganizationBoundAuthority(t, config.ScopeAuditRead, responseMediaAudit)
}
