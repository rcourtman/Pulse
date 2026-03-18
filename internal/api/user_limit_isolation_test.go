package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestUserLimitIsolation_IndependentPerOrg(t *testing.T) {
	setMaxUsersLimitForTests(t, map[string]int64{
		maxUsersLicenseGateKey: 5,
	})

	orgA := &models.Organization{
		ID:      "org-a",
		Members: makeMembersForIsolationTest(5),
	}
	orgB := &models.Organization{
		ID:      "org-b",
		Members: makeMembersForIsolationTest(2),
	}

	recA := httptest.NewRecorder()
	blockedA := enforceUserLimitForMemberAdd(recA, context.Background(), orgA)
	if !blockedA {
		t.Fatalf("expected org A add to be blocked at limit")
	}
	if recA.Code != http.StatusPaymentRequired {
		t.Fatalf("expected org A status %d, got %d", http.StatusPaymentRequired, recA.Code)
	}

	recB := httptest.NewRecorder()
	blockedB := enforceUserLimitForMemberAdd(recB, context.Background(), orgB)
	if blockedB {
		t.Fatalf("expected org B add to be allowed under limit")
	}
	if recB.Code != http.StatusOK {
		t.Fatalf("expected org B status %d, got %d", http.StatusOK, recB.Code)
	}
}

func TestUserLimitIsolation_NoLimitAllowsUnlimited(t *testing.T) {
	SetLicenseServiceProvider(nil)
	t.Cleanup(func() {
		SetLicenseServiceProvider(nil)
	})

	org := &models.Organization{
		ID:      "org-unlimited",
		Members: makeMembersForIsolationTest(100),
	}

	rec := httptest.NewRecorder()
	blocked := enforceUserLimitForMemberAdd(rec, context.Background(), org)
	if blocked {
		t.Fatalf("expected add to be allowed without max_users limit")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestUserLimitIsolation_ExistingMemberUpdateNotBlocked(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	setMaxUsersLimitForTests(t, map[string]int64{
		maxUsersLicenseGateKey: 5,
	})

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	for _, userID := range []string{"bob", "charlie", "dave", "erin"} {
		inviteReq := withUser(
			httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"`+userID+`","role":"viewer"}`)),
			"alice",
		)
		inviteReq.SetPathValue("id", "acme")
		inviteRec := httptest.NewRecorder()
		h.HandleInviteMember(inviteRec, inviteReq)
		if inviteRec.Code != http.StatusOK {
			t.Fatalf("invite %s failed: %d %s", userID, inviteRec.Code, inviteRec.Body.String())
		}
	}

	orgBeforeUpdate, err := persistence.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load org before update failed: %v", err)
	}
	if got := currentUserCount(orgBeforeUpdate); got != 5 {
		t.Fatalf("expected currentUserCount=5 before role update, got %d", got)
	}

	updateReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"admin"}`)),
		"alice",
	)
	updateReq.SetPathValue("id", "acme")
	updateRec := httptest.NewRecorder()
	h.HandleInviteMember(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected existing member update to bypass add-limit check, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	orgAfterUpdate, err := persistence.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load org after update failed: %v", err)
	}
	if got := currentUserCount(orgAfterUpdate); got != 5 {
		t.Fatalf("expected member count to remain 5 after role update, got %d", got)
	}

	member, ok := findMemberForIsolationTest(orgAfterUpdate, "bob")
	if !ok {
		t.Fatalf("expected bob to remain a member after role update")
	}
	if member.Role != models.OrgRoleAdmin {
		t.Fatalf("expected bob role %q, got %q", models.OrgRoleAdmin, member.Role)
	}
}

func makeMembersForIsolationTest(count int) []models.OrganizationMember {
	members := make([]models.OrganizationMember, count)
	for i := 0; i < count; i++ {
		members[i] = models.OrganizationMember{
			UserID: "user-" + strconv.Itoa(i+1),
			Role:   models.OrgRoleViewer,
		}
	}
	return members
}

func findMemberForIsolationTest(org *models.Organization, userID string) (models.OrganizationMember, bool) {
	for _, member := range org.Members {
		if member.UserID == userID {
			return member, true
		}
	}
	return models.OrganizationMember{}, false
}
