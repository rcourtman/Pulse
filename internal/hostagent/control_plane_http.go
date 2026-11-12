package hostagent

import (
	"errors"
	"fmt"
	"net/http"
)

// controlPlaneRedirectError contains only local text and a numeric status. A
// reverse proxy's Location can echo credentials or private endpoint details.
type controlPlaneRedirectError struct {
	statusCode int
}

func (e *controlPlaneRedirectError) Error() string {
	if e.statusCode == 0 {
		return "server returned a redirect - use the final Pulse URL explicitly"
	}
	return fmt.Sprintf("server returned HTTP %d redirect - use the final Pulse URL explicitly", e.statusCode)
}

// doControlPlaneRequest preserves the supplied client's TLS, proxy, timeout and
// cookie handling without allowing redirects. Reject before net/http parses a
// Location: even a parse failure or a fixed CheckRedirect error can otherwise
// put the untrusted destination in an error that runtime callers log.
func doControlPlaneRequest(client *http.Client, request *http.Request) (*http.Response, error) {
	guarded := *client
	transport := guarded.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	guarded.Transport = controlPlaneNoRedirectTransport{base: transport}
	response, err := guarded.Do(request)
	var redirect *controlPlaneRedirectError
	if errors.As(err, &redirect) {
		// net/http wraps RoundTrip errors in a URL error. Release only our
		// fixed diagnostic, not that wrapper or any upstream text.
		return nil, redirect
	}
	return response, err
}

type controlPlaneNoRedirectTransport struct {
	base http.RoundTripper
}

func (t controlPlaneNoRedirectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil || response == nil {
		return response, err
	}
	switch response.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		if response.Header.Get("Location") != "" {
			// No destination parsing, response body reading or raw close-error
			// propagation. The caller receives no open rejected response.
			if response.Body != nil {
				_ = response.Body.Close()
			}
			return nil, &controlPlaneRedirectError{statusCode: response.StatusCode}
		}
	}
	return response, nil
}
