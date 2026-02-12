package account

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

type memberResponse struct {
	UserID    string              `json:"user_id"`
	Email     string              `json:"email"`
	Role      registry.MemberRole `json:"role"`
	CreatedAt time.Time           `json:"created_at"`
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

		memberships, err := reg.ListMembersByAccount(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if memberships == nil {
			memberships = []*registry.AccountMembership{}
		}

		resp := make([]memberResponse, 0, len(memberships))
		for _, m := range memberships {
			u, err := reg.GetUser(m.UserID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if u == nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			resp = append(resp, memberResponse{
				UserID:    m.UserID,
				Email:     u.Email,
				Role:      m.Role,
				CreatedAt: m.CreatedAt,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

type inviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
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

		var req inviteMemberRequest
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}

		email := normalizeEmail(req.Email)
		if email == "" {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}

		role, ok := parseMemberRole(req.Role)
		if !ok {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}

		u, err := reg.GetUserByEmail(email)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if u == nil {
			userID, err := registry.GenerateUserID()
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			u = &registry.User{
				ID:    userID,
				Email: email,
			}
			if err := reg.CreateUser(u); err != nil {
				// If a concurrent request created the user, fall back to lookup.
				u2, gerr := reg.GetUserByEmail(email)
				if gerr != nil || u2 == nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				u = u2
			}
		}

		if err := reg.CreateMembership(&registry.AccountMembership{
			AccountID: accountID,
			UserID:    u.ID,
			Role:      role,
		}); err != nil {
			if isUniqueViolation(err) {
				http.Error(w, "membership already exists", http.StatusConflict)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
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
			http.Error(w, "missing account_id or user_id", http.StatusBadRequest)
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

		var req updateMemberRoleRequest
		if err := decodeJSON(w, r, &req); err != nil {
			return
		}

		role, ok := parseMemberRole(req.Role)
		if !ok {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}

		if err := reg.UpdateMembershipRole(accountID, userID, role); err != nil {
			if isNotFoundErr(err) {
				http.Error(w, "membership not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

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
			http.Error(w, "missing account_id or user_id", http.StatusBadRequest)
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

		m, err := reg.GetMembership(accountID, userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if m == nil {
			http.Error(w, "membership not found", http.StatusNotFound)
			return
		}

		if m.Role == registry.MemberRoleOwner {
			memberships, err := reg.ListMembersByAccount(accountID)
			if err != nil {
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
				http.Error(w, "cannot remove last owner", http.StatusConflict)
				return
			}
		}

		if err := reg.DeleteMembership(accountID, userID); err != nil {
			if isNotFoundErr(err) {
				http.Error(w, "membership not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

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

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
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
