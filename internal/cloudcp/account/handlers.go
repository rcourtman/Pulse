package account

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

type accessSubjectResponse struct {
	SubjectID string                      `json:"subject_id"`
	UserID    string                      `json:"user_id,omitempty"`
	Email     string                      `json:"email"`
	Role      registry.MemberRole         `json:"role"`
	State     registry.AccountAccessState `json:"state"`
	CreatedAt time.Time                   `json:"created_at"`
}

// HandleListMembers returns an authenticated handler that lists all members of an account.
func HandleListMembers(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		if accountID == "" {
			http.Error(w, "missing account_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		actorRole, _ := requestActorRole(r)
		resp, err := accessSubjectsForActor(reg, accountID, actorRole)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encodeJSON(w, resp)
	}
}

type inviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type inviteMemberResponse struct {
	SubjectID string                      `json:"subject_id"`
	UserID    string                      `json:"user_id,omitempty"`
	State     registry.AccountAccessState `json:"state"`
}

func requestActorRole(r *http.Request) (registry.MemberRole, bool) {
	if r == nil {
		return "", false
	}
	role := registry.MemberRole(strings.TrimSpace(r.Header.Get("X-User-Role")))
	if role == "" {
		return "", false
	}
	return role, true
}

func actorCanManageAccount(role registry.MemberRole) bool {
	return role == registry.MemberRoleOwner || role == registry.MemberRoleAdmin
}

func accessSubjectsForActor(reg *registry.TenantRegistry, accountID string, actorRole registry.MemberRole) ([]accessSubjectResponse, error) {
	subjects, err := reg.ListAccessSubjectsByAccount(accountID)
	if err != nil {
		return nil, err
	}

	includePending := actorCanManageAccount(actorRole)
	resp := make([]accessSubjectResponse, 0, len(subjects))
	for _, subject := range subjects {
		if subject == nil {
			continue
		}
		if subject.State == registry.AccountAccessStatePending && !includePending {
			continue
		}
		resp = append(resp, accessSubjectResponse{
			SubjectID: subject.SubjectID,
			UserID:    subject.UserID,
			Email:     subject.Email,
			Role:      subject.Role,
			State:     subject.State,
			CreatedAt: subject.CreatedAt,
		})
	}
	return resp, nil
}

type accountAccessTarget struct {
	membership *registry.AccountMembership
	invitation *registry.AccountInvitation
}

func loadAccountAccessTarget(reg *registry.TenantRegistry, accountID string, subjectID string) (*accountAccessTarget, error) {
	membership, err := reg.GetMembership(accountID, subjectID)
	if err != nil {
		return nil, err
	}
	if membership != nil {
		return &accountAccessTarget{membership: membership}, nil
	}

	invitation, err := reg.GetInvitation(subjectID)
	if err != nil {
		return nil, err
	}
	if invitation != nil && invitation.AccountID == accountID {
		return &accountAccessTarget{invitation: invitation}, nil
	}
	return &accountAccessTarget{}, nil
}

// HandleInviteMember returns an authenticated handler that invites a user to an account.
func HandleInviteMember(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		if accountID == "" {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("reason", "missing_account_id").
				Msg("Account member invite failed")
			http.Error(w, "missing account_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			auditEvent(r, "cp_account_member_invite", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("reason", "account_lookup_failed").
				Msg("Account member invite failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("account_id", accountID).
				Str("reason", "account_not_found").
				Msg("Account member invite failed")
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		var req inviteMemberRequest
		if err := decodeJSON(w, r, &req); err != nil {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("account_id", accountID).
				Str("reason", "invalid_json_body").
				Msg("Account member invite failed")
			return
		}

		email := normalizeEmail(req.Email)
		if email == "" {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("account_id", accountID).
				Str("reason", "invalid_email").
				Msg("Account member invite failed")
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}

		role, ok := parseMemberRole(req.Role)
		if !ok {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("account_id", accountID).
				Str("email", email).
				Str("reason", "invalid_role").
				Msg("Account member invite failed")
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		actorRole, hasActorRole := requestActorRole(r)
		if !hasActorRole || !actorCanManageAccount(actorRole) {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("account_id", accountID).
				Str("email", email).
				Str("actor_role", string(actorRole)).
				Str("reason", "missing_or_insufficient_role").
				Msg("Account member invite failed")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if role == registry.MemberRoleOwner && actorRole != registry.MemberRoleOwner {
			auditEvent(r, "cp_account_member_invite", "failure").
				Str("account_id", accountID).
				Str("email", email).
				Str("actor_role", string(actorRole)).
				Str("requested_role", string(role)).
				Str("reason", "owner_role_requires_owner_actor").
				Msg("Account member invite failed")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		u, err := reg.GetUserByEmail(email)
		if err != nil {
			auditEvent(r, "cp_account_member_invite", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("email", email).
				Str("reason", "user_lookup_failed").
				Msg("Account member invite failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if u == nil {
			invitation := &registry.AccountInvitation{
				AccountID: accountID,
				Email:     email,
				Role:      role,
			}
			if err := reg.UpsertInvitation(invitation); err != nil {
				auditEvent(r, "cp_account_member_invite", "failure").
					Err(err).
					Str("account_id", accountID).
					Str("email", email).
					Str("role", string(role)).
					Str("reason", "invitation_upsert_failed").
					Msg("Account member invite failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			storedInvitation, err := reg.GetInvitationByAccountAndEmail(accountID, email)
			if err != nil || storedInvitation == nil {
				auditEvent(r, "cp_account_member_invite", "failure").
					Err(err).
					Str("account_id", accountID).
					Str("email", email).
					Str("role", string(role)).
					Str("reason", "invitation_lookup_failed").
					Msg("Account member invite failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			auditEvent(r, "cp_account_member_invite", "success").
				Str("account_id", accountID).
				Str("subject_id", storedInvitation.ID).
				Str("email", email).
				Str("role", string(role)).
				Str("state", string(registry.AccountAccessStatePending)).
				Msg("Account invitation saved")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			encodeJSON(w, inviteMemberResponse{
				SubjectID: storedInvitation.ID,
				State:     registry.AccountAccessStatePending,
			})
			return
		}

		if err := reg.CreateMembership(&registry.AccountMembership{
			AccountID: accountID,
			UserID:    u.ID,
			Role:      role,
		}); err != nil {
			if isUniqueViolation(err) {
				auditEvent(r, "cp_account_member_invite", "failure").
					Str("account_id", accountID).
					Str("user_id", u.ID).
					Str("email", email).
					Str("role", string(role)).
					Str("reason", "membership_already_exists").
					Msg("Account member invite failed")
				http.Error(w, "membership already exists", http.StatusConflict)
				return
			}
			auditEvent(r, "cp_account_member_invite", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("user_id", u.ID).
				Str("email", email).
				Str("role", string(role)).
				Str("reason", "membership_create_failed").
				Msg("Account member invite failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := reg.DeleteInvitationByAccountAndEmail(accountID, email); err != nil {
			auditEvent(r, "cp_account_member_invite", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("user_id", u.ID).
				Str("email", email).
				Str("reason", "invitation_cleanup_failed").
				Msg("Account member invite failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		auditEvent(r, "cp_account_member_invite", "success").
			Str("account_id", accountID).
			Str("subject_id", u.ID).
			Str("email", email).
			Str("role", string(role)).
			Str("state", string(registry.AccountAccessStateActive)).
			Msg("Account member invited")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		encodeJSON(w, inviteMemberResponse{
			SubjectID: u.ID,
			UserID:    u.ID,
			State:     registry.AccountAccessStateActive,
		})
	}
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

// HandleUpdateMemberRole returns an authenticated handler that updates a member's role.
func HandleUpdateMemberRole(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		userID := strings.TrimSpace(r.PathValue("user_id"))
		if accountID == "" || userID == "" {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("reason", "missing_account_id_or_user_id").
				Msg("Account member role update failed")
			http.Error(w, "missing account_id or user_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "account_lookup_failed").
				Msg("Account member role update failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "account_not_found").
				Msg("Account member role update failed")
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}
		actorRole, hasActorRole := requestActorRole(r)
		if !hasActorRole || !actorCanManageAccount(actorRole) {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("actor_role", string(actorRole)).
				Str("reason", "missing_or_insufficient_role").
				Msg("Account member role update failed")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var req updateMemberRoleRequest
		if err := decodeJSON(w, r, &req); err != nil {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "invalid_json_body").
				Msg("Account member role update failed")
			return
		}

		role, ok := parseMemberRole(req.Role)
		if !ok {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "invalid_role").
				Msg("Account member role update failed")
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}

		target, err := loadAccountAccessTarget(reg, accountID, userID)
		if err != nil {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "access_target_lookup_failed").
				Msg("Account member role update failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		currentRole := registry.MemberRole("")
		currentState := registry.AccountAccessStateActive
		switch {
		case target.membership != nil:
			currentRole = target.membership.Role
			currentState = registry.AccountAccessStateActive
		case target.invitation != nil:
			currentRole = target.invitation.Role
			currentState = registry.AccountAccessStatePending
		default:
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "access_target_not_found").
				Msg("Account member role update failed")
			http.Error(w, "access subject not found", http.StatusNotFound)
			return
		}

		if (role == registry.MemberRoleOwner || currentRole == registry.MemberRoleOwner) && actorRole != registry.MemberRoleOwner {
			auditEvent(r, "cp_account_member_role_update", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("actor_role", string(actorRole)).
				Str("target_current_role", string(currentRole)).
				Str("target_new_role", string(role)).
				Str("reason", "owner_role_change_requires_owner_actor").
				Msg("Account member role update failed")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if target.membership != nil && currentRole == registry.MemberRoleOwner && role != registry.MemberRoleOwner {
			memberships, listErr := reg.ListMembersByAccount(accountID)
			if listErr != nil {
				auditEvent(r, "cp_account_member_role_update", "failure").
					Err(listErr).
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("reason", "membership_list_failed").
					Msg("Account member role update failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			owners := 0
			for _, mm := range memberships {
				if mm.Role == registry.MemberRoleOwner {
					owners++
				}
			}
			if owners <= 1 {
				auditEvent(r, "cp_account_member_role_update", "failure").
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("reason", "cannot_demote_last_owner").
					Msg("Account member role update denied")
				http.Error(w, "cannot demote last owner", http.StatusConflict)
				return
			}
		}

		if target.membership != nil {
			if err := reg.UpdateMembershipRole(accountID, userID, role); err != nil {
				if isNotFoundErr(err) {
					auditEvent(r, "cp_account_member_role_update", "failure").
						Err(err).
						Str("account_id", accountID).
						Str("user_id", userID).
						Str("reason", "membership_not_found").
						Msg("Account member role update failed")
					http.Error(w, "membership not found", http.StatusNotFound)
					return
				}
				auditEvent(r, "cp_account_member_role_update", "failure").
					Err(err).
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("reason", "membership_update_failed").
					Msg("Account member role update failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		} else {
			if err := reg.UpdateInvitationRole(userID, role, ""); err != nil {
				if isNotFoundErr(err) {
					auditEvent(r, "cp_account_member_role_update", "failure").
						Err(err).
						Str("account_id", accountID).
						Str("user_id", userID).
						Str("reason", "invitation_not_found").
						Msg("Account member role update failed")
					http.Error(w, "invitation not found", http.StatusNotFound)
					return
				}
				auditEvent(r, "cp_account_member_role_update", "failure").
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("reason", "invitation_update_failed").
					Msg("Account member role update failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		auditEvent(r, "cp_account_member_role_update", "success").
			Str("account_id", accountID).
			Str("user_id", userID).
			Str("state", string(currentState)).
			Str("old_role", string(currentRole)).
			Str("new_role", string(role)).
			Msg("Account member role updated")

		w.WriteHeader(http.StatusOK)
	}
}

// HandleRemoveMember returns an authenticated handler that removes a user from an account.
func HandleRemoveMember(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		accountID := strings.TrimSpace(r.PathValue("account_id"))
		userID := strings.TrimSpace(r.PathValue("user_id"))
		if accountID == "" || userID == "" {
			auditEvent(r, "cp_account_member_remove", "failure").
				Str("reason", "missing_account_id_or_user_id").
				Msg("Account member removal failed")
			http.Error(w, "missing account_id or user_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			auditEvent(r, "cp_account_member_remove", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "account_lookup_failed").
				Msg("Account member removal failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			auditEvent(r, "cp_account_member_remove", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "account_not_found").
				Msg("Account member removal failed")
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}
		actorRole, hasActorRole := requestActorRole(r)
		if !hasActorRole || !actorCanManageAccount(actorRole) {
			auditEvent(r, "cp_account_member_remove", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("actor_role", string(actorRole)).
				Str("reason", "missing_or_insufficient_role").
				Msg("Account member removal failed")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		target, err := loadAccountAccessTarget(reg, accountID, userID)
		if err != nil {
			auditEvent(r, "cp_account_member_remove", "failure").
				Err(err).
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "access_target_lookup_failed").
				Msg("Account member removal failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		currentRole := registry.MemberRole("")
		currentState := registry.AccountAccessStateActive
		switch {
		case target.membership != nil:
			currentRole = target.membership.Role
			currentState = registry.AccountAccessStateActive
		case target.invitation != nil:
			currentRole = target.invitation.Role
			currentState = registry.AccountAccessStatePending
		default:
			auditEvent(r, "cp_account_member_remove", "failure").
				Str("account_id", accountID).
				Str("user_id", userID).
				Str("reason", "access_target_not_found").
				Msg("Account member removal failed")
			http.Error(w, "access subject not found", http.StatusNotFound)
			return
		}

		if currentRole == registry.MemberRoleOwner {
			if actorRole != registry.MemberRoleOwner {
				auditEvent(r, "cp_account_member_remove", "failure").
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("actor_role", string(actorRole)).
					Str("target_role", string(currentRole)).
					Str("reason", "owner_removal_requires_owner_actor").
					Msg("Account member removal failed")
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			if target.membership != nil {
				memberships, err := reg.ListMembersByAccount(accountID)
				if err != nil {
					auditEvent(r, "cp_account_member_remove", "failure").
						Err(err).
						Str("account_id", accountID).
						Str("user_id", userID).
						Str("reason", "membership_list_failed").
						Msg("Account member removal failed")
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				owners := 0
				for _, mm := range memberships {
					if mm.Role == registry.MemberRoleOwner {
						owners++
					}
				}
				if owners <= 1 {
					auditEvent(r, "cp_account_member_remove", "failure").
						Str("account_id", accountID).
						Str("user_id", userID).
						Str("reason", "cannot_remove_last_owner").
						Msg("Account member removal denied")
					http.Error(w, "cannot remove last owner", http.StatusConflict)
					return
				}
			}
		}

		if target.membership != nil {
			if err := reg.DeleteMembership(accountID, userID); err != nil {
				if isNotFoundErr(err) {
					auditEvent(r, "cp_account_member_remove", "failure").
						Err(err).
						Str("account_id", accountID).
						Str("user_id", userID).
						Str("reason", "membership_not_found").
						Msg("Account member removal failed")
					http.Error(w, "membership not found", http.StatusNotFound)
					return
				}
				auditEvent(r, "cp_account_member_remove", "failure").
					Err(err).
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("reason", "membership_delete_failed").
					Msg("Account member removal failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		} else {
			if err := reg.DeleteInvitation(userID); err != nil {
				if isNotFoundErr(err) {
					auditEvent(r, "cp_account_member_remove", "failure").
						Err(err).
						Str("account_id", accountID).
						Str("user_id", userID).
						Str("reason", "invitation_not_found").
						Msg("Account member removal failed")
					http.Error(w, "invitation not found", http.StatusNotFound)
					return
				}
				auditEvent(r, "cp_account_member_remove", "failure").
					Str("account_id", accountID).
					Str("user_id", userID).
					Str("reason", "invitation_delete_failed").
					Msg("Account member removal failed")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		auditEvent(r, "cp_account_member_remove", "success").
			Str("account_id", accountID).
			Str("user_id", userID).
			Str("state", string(currentState)).
			Str("removed_role", string(currentRole)).
			Msg("Account member removed")

		w.WriteHeader(http.StatusNoContent)
	}
}

func normalizeEmail(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// Minimal sanity; deeper validation comes later with session auth flows.
	if s == "" || !strings.Contains(s, "@") {
		return ""
	}
	return s
}

func parseMemberRole(s string) (registry.MemberRole, bool) {
	switch registry.MemberRole(strings.TrimSpace(s)) {
	case registry.MemberRoleOwner:
		return registry.MemberRoleOwner, true
	case registry.MemberRoleAdmin:
		return registry.MemberRoleAdmin, true
	case registry.MemberRoleTech:
		return registry.MemberRoleTech, true
	case registry.MemberRoleReadOnly:
		return registry.MemberRoleReadOnly, true
	default:
		return "", false
	}
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request, dst *T) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return fmt.Errorf("decode request body: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return fmt.Errorf("decode request body: multiple JSON values")
		}
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

func encodeJSON(w http.ResponseWriter, payload any) {
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("cloudcp.account: encode JSON response")
	}
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	// Registry uses fmt.Errorf("... not found") (no sentinel errors yet).
	return strings.Contains(err.Error(), "not found")
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// modernc.org/sqlite returns strings containing "UNIQUE constraint failed".
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed")
}
