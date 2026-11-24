package api

import (
	"errors"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

var errSSOConfigUnavailable = errors.New("saved SSO configuration is unavailable")

// ssoManagementConfig distinguishes a known empty first run from a failed load.
// Only an explicit management read may retry: public status, login discovery and
// writes must never turn the fallback snapshot into saved provider authority.
func (r *Router) ssoManagementConfig(retry bool) (*config.SSOConfig, error) {
	r.ssoMu.Lock()
	defer r.ssoMu.Unlock()

	wasFailed := r.ssoAuthLoadFailed.Load()
	current := r.ensureSSOConfigLocked()
	if !r.ssoAuthLoadFailed.Load() {
		return current, nil
	}
	if !retry || !wasFailed || r.persistence == nil {
		return nil, errSSOConfigUnavailable
	}

	loaded, err := r.persistence.LoadPersistedSSOConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to reload SSO config from persistence")
		return nil, errSSOConfigUnavailable
	}
	// Missing storage after a failure is not evidence of a new installation.
	// In particular it must not release writes before a lost mount returns.
	if loaded == nil {
		return nil, errSSOConfigUnavailable
	}

	r.publishSSOConfigLocked(loaded)
	// Reapply environment authority and publish the auth snapshot BEFORE
	// clearing unknown, which also protects unauthenticated config transfer.
	current = r.ensureSSOConfigLocked()
	r.ssoAuthLoadFailed.Store(false)
	return current, nil
}

// reloadSSOConfigAfterImport publishes the SSO configuration a configuration
// import has just saved. Sign-in is served from the published snapshot, never
// from storage: without this a provider the archive removed or changed keeps
// its previous policy until a restart, and the next SSO save writes that stale
// snapshot back over the archive. An import always writes the file, so one
// that is missing or unreadable leaves saved authority unknown, exactly as a
// failed first load does.
func (r *Router) reloadSSOConfigAfterImport() error {
	r.ssoMu.Lock()
	defer r.ssoMu.Unlock()

	if r.persistence == nil {
		return nil
	}
	loaded, err := r.persistence.LoadPersistedSSOConfig()
	if err == nil && loaded == nil {
		err = errSSOConfigUnavailable
	}
	if err != nil {
		r.ssoAuthLoadFailed.Store(true)
		return err
	}

	previous := r.ssoConfig
	// Publication ends callbacks in flight for every provider whose policy
	// the archive changed. As in ssoManagementConfig, reapply environment
	// authority and publish the auth snapshot before clearing unknown.
	r.publishSSOConfigLocked(loaded)
	current := r.ensureSSOConfigLocked()
	r.ssoAuthLoadFailed.Store(false)

	// Withdraw the live services of providers the archive removed, disabled
	// or changed, as the delete and update handlers do. Login and callback
	// initialize a restored provider again from the published snapshot.
	if previous != nil {
		for i := range previous.Providers {
			old := &previous.Providers[i]
			if restored := current.GetProvider(old.ID); restored != nil && ssoPresentationOnlyUpdate(*old, *restored) {
				continue
			}
			switch old.Type {
			case config.SSOProviderTypeSAML:
				if r.samlManager != nil {
					r.samlManager.RemoveProvider(old.ID)
				}
			case config.SSOProviderTypeOIDC:
				r.oidcManager.RemoveService(old.ID)
			}
		}
	}
	return nil
}

func writeSSOConfigUnavailable(w http.ResponseWriter, req *http.Request) {
	status := http.StatusConflict // A refused write, not an uncertain save.
	if req.Method == http.MethodGet {
		status = http.StatusServiceUnavailable
	}
	writeErrorResponse(w, status, "sso_config_unavailable", "Saved SSO configuration could not be read. Check saved providers before making changes.", nil)
}

// Called after the route's existing authentication, permission, license and
// scope checks, before a bound endpoint can read a snapshot or start services.
func (r *Router) requireSSOManagementConfig(w http.ResponseWriter, req *http.Request) (*config.SSOConfig, bool) {
	cfg, err := r.ssoManagementConfig(req.Method == http.MethodGet)
	if err != nil {
		writeSSOConfigUnavailable(w, req)
		return nil, false
	}
	return cfg, true
}

// authorizeSSOManagement is route-only: the usual permission and scope checks
// have already run. A request admitted while saved authority was unknown must
// carry real authentication even if another request has since recovered it.
func (r *Router) authorizeSSOManagement(w http.ResponseWriter, req *http.Request) bool {
	r.ensureSSOConfig()
	admission, _ := req.Context().Value(ssoManagementAdmissionKey{}).(*ssoManagementAdmission)
	if r.ssoAuthLoadFailed.Load() || admission != nil && admission.unknown {
		if admission == nil || !admission.authenticated {
			writeErrorResponse(w, http.StatusUnauthorized, "authentication_required", "Authentication required to check saved SSO configuration", nil)
			return false
		}
		if session := sessionReadRevalidator(req); session != nil && !session() {
			writeAuthenticationRequired(w, req)
			return false
		}
	}
	_, ok := r.requireSSOManagementConfig(w, req)
	return ok
}
