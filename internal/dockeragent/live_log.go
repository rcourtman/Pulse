package dockeragent

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// The report response is handled on the collection path with collectMu held.
// Do not retain the read in telemetry, retries, logs or observer reports.
func (a *Agent) returnLogTail(ctx context.Context, target TargetConfig, request agentsdocker.LogTailRequest) {
	if (!target.Authoritative && target.Name != "") || !a.cfg.AllowLiveLogs || a.cfg.HelperInventory != nil || a.helperInventory != nil || a.docker == nil {
		return
	}
	id, err := hex.DecodeString(request.ContainerID)
	if err != nil || len(id) != 32 || len(request.ID) != 36 || len(request.HostID) > 512 || request.HostID == "" {
		return
	}
	outcome := agentsdocker.LogTailResult{LogTailRequest: request, Unavailable: true}
	readCtx, cancel := context.WithTimeout(ctx, failureLogReadTimeout)
	inspect, err := a.docker.ContainerInspect(readCtx, request.ContainerID)
	if err == nil && inspect.ID == request.ContainerID {
		stream, err := a.docker.ContainerLogTail(readCtx, request.ContainerID, agentsdocker.FailureLogMaxLines)
		if err == nil {
			raw, truncated, readErr := readFailureLogStream(stream, inspect.Config != nil && inspect.Config.Tty)
			_ = stream.Close()
			if readErr == nil || len(raw) > 0 {
				lines, cut, _ := prepareFailureLogLines(raw)
				outcome.Lines = lines
				outcome.Truncated = truncated || cut || readErr != nil
				outcome.Unavailable = false
			}
		}
	}
	cancel()
	b, err := json.Marshal(outcome)
	if err != nil {
		return
	}
	sendCtx, done := context.WithTimeout(ctx, 5*time.Second)
	defer done()
	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, target.URL+"/api/agents/docker/log-tail/result", bytes.NewReader(b))
	if err != nil {
		return
	}
	setAgentHeaders(req, target.Token)
	// A result is bound to the reporting destination. Refuse redirects before
	// transmitting the application output to another path or authority.
	client := *a.httpClientFor(target)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}
