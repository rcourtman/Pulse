package securityutil

import pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"

func NormalizeWebSocketOriginHost(host string) string {
	return pubsec.NormalizeWebSocketOriginHost(host)
}

func SameHostWebSocketOrigin(origin string, requestHost string) bool {
	return pubsec.SameHostWebSocketOrigin(origin, requestHost)
}

func HTTPOriginForWebSocketBaseURL(raw string) (string, error) {
	return pubsec.HTTPOriginForWebSocketBaseURL(raw)
}

func HTTPOriginForWebSocketBaseURLWithOptions(raw string, opts PulseURLValidationOptions) (string, error) {
	return pubsec.HTTPOriginForWebSocketBaseURLWithOptions(raw, opts)
}
