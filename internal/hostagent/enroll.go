package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	enrollEndpoint   = "/api/agents/agent/enroll"
	runtimeTokenFile = "runtime.token"
)

// Enrollment retry parameters. Variables (not constants) so tests can override.
var (
	enrollInitialDelay = 5 * time.Second
	enrollMaxDelay     = 5 * time.Minute
	enrollMultiplier   = 2.0
)

// enrollPayload is the JSON body sent to the enrollment endpoint.
type enrollPayload struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	AgentVersion    string `json:"agentVersion"`
	CommandsEnabled bool   `json:"commandsEnabled,omitempty"`
}

// enrollResponse is the JSON body returned by the enrollment endpoint.
type enrollResponse struct {
	AgentID        string `json:"agentId"`
	RuntimeToken   string `json:"runtimeToken"`
	RuntimeTokenID string `json:"runtimeTokenId"`
	ReportInterval string `json:"reportInterval"`
}

// runEnrollmentLoop checks for an existing runtime token or performs enrollment
// with exponential backoff. On success, updates a.cfg.APIToken in memory and
// persists the runtime token to disk.
//
// Behavior on specific HTTP status codes:
//   - 200: enrollment succeeded, runtime token returned
//   - 403: token is not a bootstrap token (e.g. manual install) — skip enrollment
//   - 401: token expired or invalid — permanent failure, no retry
//   - 409: token already consumed — permanent failure, no retry
//   - 400/404: invalid request or target not found — permanent failure, no retry
//   - 5xx/network errors: retry with exponential backoff
func (a *Agent) runEnrollmentLoop(ctx context.Context) error {
	tokenPath := filepath.Join(a.stateDir, runtimeTokenFile)

	// Check if we already have a persisted runtime token from a previous enrollment.
	if data, err := a.collector.ReadFile(tokenPath); err == nil {
		token := string(bytes.TrimSpace(data))
		if token != "" {
			a.logger.Info().Msg("Found existing runtime token, skipping enrollment")
			a.cfg.APIToken = token
			return nil
		}
	}

	a.logger.Info().Msg("Starting enrollment to exchange bootstrap token for runtime token")

	delay := enrollInitialDelay
	for {
		result, err := a.enroll(ctx)
		if err == nil {
			// Success — persist and use the runtime token.
			a.cfg.APIToken = result.RuntimeToken
			a.persistRuntimeToken(result.RuntimeToken)
			canonicalID := strings.TrimSpace(result.AgentID)
			if canonicalID != "" {
				a.persistAgentID(canonicalID)
			}
			a.logger.Info().
				Str("agentId", canonicalID).
				Msg("Enrollment succeeded, using runtime token")
			return nil
		}

		// Check for non-retryable errors.
		var statusErr *enrollStatusError
		if errors.As(err, &statusErr) {
			switch statusErr.StatusCode {
			case http.StatusForbidden:
				// 403 = not a bootstrap token (manual install) — skip enrollment gracefully.
				a.logger.Info().
					Msg("Token is not a bootstrap token (403), skipping enrollment")
				return nil
			case http.StatusUnauthorized:
				// 401 = token expired or invalid — permanent failure.
				return fmt.Errorf("bootstrap token rejected (401 Unauthorized): %w", err)
			case http.StatusConflict:
				// 409 = token already consumed — permanent failure.
				return fmt.Errorf("bootstrap token already consumed (409 Conflict): %w", err)
			case http.StatusBadRequest, http.StatusNotFound:
				// 400/404 = invalid request or target not found — permanent failure.
				return fmt.Errorf("enrollment permanently rejected (HTTP %d): %w", statusErr.StatusCode, err)
			}
		}

		// Context cancelled — stop retrying.
		if ctx.Err() != nil {
			return fmt.Errorf("enrollment cancelled: %w", ctx.Err())
		}

		a.logger.Warn().
			Err(err).
			Str("next_retry", delay.String()).
			Msg("Enrollment failed, retrying")

		// Wait with backoff.
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("enrollment cancelled: %w", ctx.Err())
		case <-timer.C:
		}

		delay = time.Duration(float64(delay) * enrollMultiplier)
		if delay > enrollMaxDelay {
			delay = enrollMaxDelay
		}
	}
}

// enrollStatusError represents an HTTP error from the enrollment endpoint.
type enrollStatusError struct {
	StatusCode int
	Body       string
}

func (e *enrollStatusError) Error() string {
	return fmt.Sprintf("enrollment returned HTTP %d: %s", e.StatusCode, e.Body)
}

// enroll performs a single enrollment attempt by POSTing to the server.
func (a *Agent) enroll(ctx context.Context) (*enrollResponse, error) {
	payload := enrollPayload{
		Hostname:        a.hostname,
		OS:              a.osName,
		Arch:            a.architecture,
		AgentVersion:    a.agentVersion,
		CommandsEnabled: a.cfg.EnableCommands,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal enrollment request: %w", err)
	}

	url := fmt.Sprintf("%s%s", a.trimmedPulseURL, enrollEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create enrollment request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", a.cfg.APIToken)
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-agent/"+a.agentVersion)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("enrollment request failed: %w", err)
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	status := resp.StatusCode
	resp.Body.Close()

	if status != http.StatusOK {
		return nil, &enrollStatusError{
			StatusCode: status,
			Body:       string(respBody),
		}
	}

	var result enrollResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode enrollment response: %w", err)
	}

	if result.RuntimeToken == "" {
		return nil, fmt.Errorf("enrollment response missing runtimeToken")
	}

	return &result, nil
}

// persistRuntimeToken writes the runtime token to the state directory.
func (a *Agent) persistRuntimeToken(token string) {
	if a.stateDir == "" {
		return
	}
	if err := a.collector.MkdirAll(a.stateDir, 0700); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to create state directory for runtime token")
		return
	}
	tokenPath := filepath.Join(a.stateDir, runtimeTokenFile)
	if err := a.collector.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to persist runtime token")
		return
	}
	if err := a.collector.Chmod(tokenPath, 0600); err != nil {
		a.logger.Debug().Err(err).Msg("Failed to enforce runtime token file permissions")
	}
}
