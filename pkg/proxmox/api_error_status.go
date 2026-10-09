package proxmox

import "errors"

// APIErrorStatus returns the observed HTTP status, never a number quoted in a
// provider's error body. Unknown/local errors have no HTTP-status evidence.
func APIErrorStatus(err error) (int, bool) {
	var response *apiResponseError
	if errors.As(err, &response) && response != nil {
		return response.statusCode, true
	}
	var auth *authHTTPError
	if errors.As(err, &auth) && auth != nil {
		return auth.status, true
	}
	return 0, false
}
