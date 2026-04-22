package securityutil

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

type RestrictedOutboundHTTPOptions = pubsec.RestrictedOutboundHTTPOptions

func ValidateOutboundFetchURL(ctx context.Context, raw string, opts RestrictedOutboundHTTPOptions) (*url.URL, error) {
	return pubsec.ValidateOutboundFetchURL(ctx, raw, opts)
}

func NewRestrictedOutboundHTTPClient(timeout time.Duration, opts RestrictedOutboundHTTPOptions) *http.Client {
	return pubsec.NewRestrictedOutboundHTTPClient(timeout, opts)
}

var resolveOutboundFetchIPs = net.DefaultResolver.LookupIPAddr
