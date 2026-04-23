package securityutil

import (
	"context"
	"io"
	"net/http"
	"net/url"

	pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

type PulseURLValidationOptions = pubsec.PulseURLValidationOptions

func NormalizeAbsoluteHTTPURL(raw string) (*url.URL, error) {
	return pubsec.NormalizeAbsoluteHTTPURL(raw)
}

func NormalizeHTTPBaseURL(raw string, defaultScheme string) (*url.URL, error) {
	return pubsec.NormalizeHTTPBaseURL(raw, defaultScheme)
}

func NormalizeSecureHTTPBaseURL(raw string) (*url.URL, error) {
	return pubsec.NormalizeSecureHTTPBaseURL(raw)
}

func IsLoopbackHost(host string) bool {
	return pubsec.IsLoopbackHost(host)
}

func NormalizePulseHTTPBaseURL(raw string) (*url.URL, error) {
	return pubsec.NormalizePulseHTTPBaseURL(raw)
}

func NormalizePulseHTTPBaseURLWithOptions(raw string, opts PulseURLValidationOptions) (*url.URL, error) {
	return pubsec.NormalizePulseHTTPBaseURLWithOptions(raw, opts)
}

func NormalizePulseWebSocketBaseURL(raw string) (*url.URL, error) {
	return pubsec.NormalizePulseWebSocketBaseURL(raw)
}

func NormalizePulseWebSocketBaseURLWithOptions(raw string, opts PulseURLValidationOptions) (*url.URL, error) {
	return pubsec.NormalizePulseWebSocketBaseURLWithOptions(raw, opts)
}

func AppendURLPath(base *url.URL, segments ...string) *url.URL {
	return pubsec.AppendURLPath(base, segments...)
}

func ResolveRelativeURL(base *url.URL, relativePath string) (*url.URL, error) {
	return pubsec.ResolveRelativeURL(base, relativePath)
}

func NewValidatedRequestWithContext(ctx context.Context, method string, target *url.URL, body io.Reader) (*http.Request, error) {
	return pubsec.NewValidatedRequestWithContext(ctx, method, target, body)
}

func NewRelativeRequestWithContext(ctx context.Context, method string, base *url.URL, relativePath string, body io.Reader) (*http.Request, error) {
	return pubsec.NewRelativeRequestWithContext(ctx, method, base, relativePath, body)
}
