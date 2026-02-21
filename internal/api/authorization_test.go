package api

import (
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockOrgLoader struct {
	mock.Mock
}

func (m *mockOrgLoader) GetOrganization(orgID string) (*models.Organization, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func TestDefaultAuthorizationChecker_TokenCanAccessOrg(t *testing.T) {
	checker := NewAuthorizationChecker(nil)

	t.Run("nil token", func(t *testing.T) {
		assert.True(t, checker.TokenCanAccessOrg(nil, "any"))
	})

	t.Run("valid access single", func(t *testing.T) {
		token := &config.APITokenRecord{OrgID: "acme"}
		assert.True(t, checker.TokenCanAccessOrg(token, "acme"))
	})

	t.Run("valid access multi", func(t *testing.T) {
		token := &config.APITokenRecord{OrgIDs: []string{"acme", "other"}}
		assert.True(t, checker.TokenCanAccessOrg(token, "acme"))
	})

	t.Run("denied access", func(t *testing.T) {
		token := &config.APITokenRecord{OrgID: "other"}
		assert.False(t, checker.TokenCanAccessOrg(token, "acme"))
	})

	t.Run("wildcard legacy access", func(t *testing.T) {
		token := &config.APITokenRecord{} // empty orgs = legacy
		assert.True(t, checker.TokenCanAccessOrg(token, "tenant1"))
	})
}

func TestDefaultAuthorizationChecker_UserCanAccessOrg(t *testing.T) {
	ml := new(mockOrgLoader)
	checker := NewAuthorizationChecker(ml)

	t.Run("default org legacy fallback when metadata missing", func(t *testing.T) {
		ml.On("GetOrganization", "default").Return(nil, nil).Once()
		assert.True(t, checker.UserCanAccessOrg("user1", "default"))
	})

	t.Run("default org enforces membership when metadata configured", func(t *testing.T) {
		org := &models.Organization{
			ID: "default",
			Members: []models.OrganizationMember{
				{UserID: "owner", Role: models.OrgRoleOwner},
				{UserID: "user1", Role: models.OrgRoleMember},
			},
		}
		ml.On("GetOrganization", "default").Return(org, nil).Twice()
		assert.True(t, checker.UserCanAccessOrg("user1", "default"))
		assert.False(t, checker.UserCanAccessOrg("other", "default"))
	})

	t.Run("missing loader", func(t *testing.T) {
		badChecker := NewAuthorizationChecker(nil)
		assert.True(t, badChecker.UserCanAccessOrg("user1", "default"))
		assert.False(t, badChecker.UserCanAccessOrg("user1", "acme"))
	})

	t.Run("authorized member", func(t *testing.T) {
		org := &models.Organization{
			ID: "acme",
			Members: []models.OrganizationMember{
				{UserID: "user1", Role: models.OrgRoleAdmin},
			},
		}
		ml.On("GetOrganization", "acme").Return(org, nil).Once()
		assert.True(t, checker.UserCanAccessOrg("user1", "acme"))
	})

	t.Run("unauthorized user", func(t *testing.T) {
		org := &models.Organization{
			ID: "acme",
			Members: []models.OrganizationMember{
				{UserID: "other", Role: models.OrgRoleMember},
			},
		}
		ml.On("GetOrganization", "acme").Return(org, nil).Once()
		assert.False(t, checker.UserCanAccessOrg("user1", "acme"))
	})

	t.Run("loader error", func(t *testing.T) {
		ml.On("GetOrganization", "fail").Return(nil, errors.New("db error")).Once()
		assert.False(t, checker.UserCanAccessOrg("user1", "fail"))
	})

	t.Run("not found", func(t *testing.T) {
		ml.On("GetOrganization", "missing").Return(nil, nil).Once()
		assert.False(t, checker.UserCanAccessOrg("user1", "missing"))
	})
}

func TestDefaultAuthorizationChecker_CheckAccess(t *testing.T) {
	ml := new(mockOrgLoader)
	checker := NewAuthorizationChecker(ml)

	t.Run("token takes precedence", func(t *testing.T) {
		token := &config.APITokenRecord{OrgID: "acme"}
		res := checker.CheckAccess(token, "user1", "acme")
		assert.True(t, res.Allowed)
		assert.False(t, res.IsLegacyToken)

		tokenLegacy := &config.APITokenRecord{OrgID: ""} // Wildcard
		res = checker.CheckAccess(tokenLegacy, "user1", "acme")
		assert.True(t, res.Allowed)
		assert.True(t, res.IsLegacyToken)

		tokenDenied := &config.APITokenRecord{OrgID: "other"}
		res = checker.CheckAccess(tokenDenied, "user1", "acme")
		assert.False(t, res.Allowed)
		assert.Equal(t, "Token is not authorized for this organization", res.Reason)
	})

	t.Run("user fallback", func(t *testing.T) {
		org := &models.Organization{
			ID: "acme",
			Members: []models.OrganizationMember{
				{UserID: "user1", Role: models.OrgRoleAdmin},
			},
		}
		ml.On("GetOrganization", "acme").Return(org, nil).Once()
		res := checker.CheckAccess(nil, "user1", "acme")
		assert.True(t, res.Allowed)
		assert.Equal(t, "User is a member of the organization", res.Reason)
	})

	t.Run("no context", func(t *testing.T) {
		res := checker.CheckAccess(nil, "", "acme")
		assert.False(t, res.Allowed)
		assert.Equal(t, "No authentication context provided", res.Reason)
	})
}
