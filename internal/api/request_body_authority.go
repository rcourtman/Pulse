package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type requestBodyAuthorityKey struct{}

// errRequestAuthorityWithdrawn is deliberately not io.EOF: a body that outlived
// its credential is neither a complete document nor an omitted one.
var errRequestAuthorityWithdrawn = errors.New("credential withdrawn before the request body arrived")

// requestBodyAuthority binds a request body to the credential that admitted
// its headers. Authentication runs when the headers arrive, but the server
// reads the body with no deadline: a client can hold back the last byte until
// its session has been logged out or its API token deleted, and still have
// the request carried out. Every body read therefore rechecks the admitted
// credential, and once it is withdrawn the handler receives no further input.
type requestBodyAuthority struct {
	mu        sync.Mutex
	allowed   func() bool
	withdrawn bool
}

// withRequestBodyAuthority lets the outermost response writer share a body's
// authority, so a withdrawn credential gets the same generic 401 as a
// withheld protected read, whatever the handler makes of its failed read.
// Bodyless requests cannot be held open.
func withRequestBodyAuthority(r *http.Request) (*http.Request, *requestBodyAuthority) {
	if r.Body == nil || r.Body == http.NoBody {
		return r, nil
	}
	authority := &requestBodyAuthority{}
	return r.WithContext(context.WithValue(r.Context(), requestBodyAuthorityKey{}, authority)), authority
}

// retainRequestBodyAuthority runs at credential admission and retains only
// the validity of the credential selected there: the exact local session, or
// the API token's live identity. It does not extend expiry or rerun
// authentication. A scope or organization edit leaves a token valid and stays
// the route's decision (403), and proxy, Basic and bypass admission stay with
// the route as they do for protected reads.
func retainRequestBodyAuthority(cfg *config.Config, r *http.Request) {
	if r.Body == nil || r.Body == http.NoBody {
		return
	}
	allowed := admittedTokenRevalidator(cfg, r)
	if allowed == nil {
		allowed = sessionReadRevalidator(r)
	}
	authority, _ := r.Context().Value(requestBodyAuthorityKey{}).(*requestBodyAuthority)
	if authority == nil {
		if allowed == nil {
			return
		}
		// Not every caller runs inside ErrorHandler. The body is still bound;
		// only the handler's own answer to the failed read is left as it is.
		authority = &requestBodyAuthority{}
		*r = *r.WithContext(context.WithValue(r.Context(), requestBodyAuthorityKey{}, authority))
	}
	// Only the credential selected by the latest admission owns the body. A
	// later proxy or Basic admission must not inherit an earlier one's check.
	authority.admit(allowed)
	if allowed == nil {
		return
	}
	// Middleware may authenticate a request more than once.
	if _, bound := r.Body.(*authorityBoundBody); !bound {
		r.Body = &authorityBoundBody{ReadCloser: r.Body, authority: authority}
	}
}

// admittedTokenRevalidator reports whether the admitted API token is still the
// same live credential in the inventory that issued it. It updates no token
// metadata and never falls back to another inventory once a tenant owner is
// withdrawn.
func admittedTokenRevalidator(cfg *config.Config, req *http.Request) func() bool {
	admitted := getAPITokenRecordFromRequest(req)
	if admitted == nil {
		return nil
	}
	source, tenantInventory := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
	tenantInventory = tenantInventory && source.orgID != "" && source.orgID != "default"
	return func() bool {
		config.Mu.RLock()
		defer config.Mu.RUnlock()
		live := cfg
		if tenantInventory {
			live = tenantTokenConfigForRequest(req)
		}
		return live != nil && admittedTokenLocked(live, admitted) != nil
	}
}

func (a *requestBodyAuthority) admit(allowed func() bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Withdrawal is final for this request, whatever is admitted afterwards.
	if !a.withdrawn {
		a.allowed = allowed
	}
}

func (a *requestBodyAuthority) permit() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.withdrawn && a.allowed != nil && !a.allowed() {
		a.withdrawn = true
	}
	return !a.withdrawn
}

// refused reports whether a body read has found the admitted credential
// withdrawn. It never rechecks: a response is withheld only from a request
// that was denied its input, not from one that completed and then logged out.
func (a *requestBodyAuthority) refused() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.withdrawn
}

// authorityBoundBody deliberately exposes no Unwrap or WriterTo escape around
// the check. Bytes are checked as they arrive, not only at EOF: a JSON decoder
// can finish a document without ever reaching the end of the body.
type authorityBoundBody struct {
	io.ReadCloser
	authority *requestBodyAuthority
}

func (b *authorityBoundBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if (n > 0 || err == io.EOF) && !b.authority.permit() {
		return 0, errRequestAuthorityWithdrawn
	}
	return n, err
}
