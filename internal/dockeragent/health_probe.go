package dockeragent

import (
	"regexp"
	"strconv"
	"strings"

	containertypes "github.com/moby/moby/api/types/container"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// healthProbeOutputLimit bounds how much probe output is read. Docker keeps at
// most 4096 bytes per result; the bound holds for runtimes that keep more.
const healthProbeOutputLimit = 8192

var (
	// "The requested URL returned error: 503" (curl -f), "ERROR 503: Service
	// Unavailable" (GNU wget), "server returned error: HTTP/1.1 500 Internal
	// Server Error" (BusyBox wget), and a bare status line.
	healthProbeHTTPStatusPatterns = []*regexp.Regexp{
		regexp.MustCompile(`returned error:\s*([1-5][0-9]{2})\b`),
		regexp.MustCompile(`\berror\s+([45][0-9]{2})\b`),
		regexp.MustCompile(`\bhttp/[0-9](?:\.[0-9])?\s+([45][0-9]{2})\b`),
	}
	healthProbeTLSPattern = regexp.MustCompile(`\b(ssl|tls|x509|certificate)\b`)
)

// containerHealthProbe reports why an unhealthy container's last health check
// failed. It reads the probe output on the host and sends only what it derives
// from it: probe output can hold credentials, paths and query strings, and it
// must never cross the agent reporting boundary. A container that is not
// unhealthy, or whose runtime kept no failed result, reports nothing.
func containerHealthProbe(health *containertypes.Health) *agentsdocker.ContainerHealthProbe {
	if health == nil || health.Status != containertypes.Unhealthy {
		return nil
	}
	var last *containertypes.HealthcheckResult
	for i := len(health.Log) - 1; i >= 0; i-- {
		if health.Log[i] != nil {
			last = health.Log[i]
			break
		}
	}
	if last == nil || last.ExitCode == 0 {
		return nil
	}
	reason, httpStatus := classifyHealthProbe(last.ExitCode, last.Output)
	streak := health.FailingStreak
	if streak < 0 {
		streak = 0
	}
	return &agentsdocker.ContainerHealthProbe{
		Reason:        reason,
		ExitCode:      last.ExitCode,
		FailingStreak: streak,
		HTTPStatus:    httpStatus,
	}
}

// classifyHealthProbe names the failure only when the output states it. The
// phrases are the ones curl, wget, the Docker engine and the common client
// tools print. Anything else is "other": a wrong reason sends the operator the
// wrong way, and the exit code is still reported.
func classifyHealthProbe(exitCode int, output string) (reason string, httpStatus int) {
	if len(output) > healthProbeOutputLimit {
		output = output[:healthProbeOutputLimit]
	}
	out := strings.ToLower(output)
	containsAny := func(phrases ...string) bool {
		for _, phrase := range phrases {
			if strings.Contains(out, phrase) {
				return true
			}
		}
		return false
	}

	switch {
	// The engine's own verdict when a probe outlives its timeout.
	case strings.Contains(out, "health check exceeded timeout"):
		return agentsdocker.HealthProbeReasonTimeout, 0
	case exitCode == 127, containsAny("executable file not found", "no such file or directory: unknown"):
		return agentsdocker.HealthProbeReasonCommandMissing, 0
	}
	for _, pattern := range healthProbeHTTPStatusPatterns {
		if match := pattern.FindStringSubmatch(out); match != nil {
			if status, err := strconv.Atoi(match[1]); err == nil && status >= 400 && status <= 599 {
				return agentsdocker.HealthProbeReasonHTTPStatus, status
			}
		}
	}
	switch {
	case strings.Contains(out, "connection refused"):
		return agentsdocker.HealthProbeReasonConnectionRefused, 0
	case containsAny("could not resolve host", "couldn't resolve host", "name or service not known",
		"temporary failure in name resolution", "no such host", "bad address", "name does not resolve"):
		return agentsdocker.HealthProbeReasonNameNotResolved, 0
	case healthProbeTLSPattern.MatchString(out):
		return agentsdocker.HealthProbeReasonTLS, 0
	case containsAny("timed out", "timeout was reached", "deadline exceeded"):
		return agentsdocker.HealthProbeReasonTimeout, 0
	default:
		return agentsdocker.HealthProbeReasonOther, 0
	}
}
