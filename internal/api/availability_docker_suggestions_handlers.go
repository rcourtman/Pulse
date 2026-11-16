package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

const (
	// maxDockerPortChecksPerRequest bounds one "add checks" request. The rest
	// stay offered and a second request adds them.
	maxDockerPortChecksPerRequest = 100
	maxDockerPortSelection        = 500
	dockerPortProbeConcurrency    = 6
	dockerPortProbeTimeout        = 3 * time.Second
)

type dockerPortProbeFunc func(ctx context.Context, target config.AvailabilityTarget) (availabilityprobe.ProbeResult, error)

type dockerPortSuggestionsResponse struct {
	Suggestions []monitoring.DockerPortCheckCandidate `json:"suggestions"`
}

type dockerPortUnansweredResponse struct {
	monitoring.DockerPortCheckCandidate
	Error string `json:"error"`
}

type dockerPortApplyResponse struct {
	Created []availabilityTargetResponse `json:"created"`
	// Unanswered ports did not answer from this Pulse server, so no check was
	// added: a check that fails from its first run only makes noise.
	Unanswered []dockerPortUnansweredResponse `json:"unanswered"`
	// Skipped ports cannot be checked from outside their host.
	Skipped []monitoring.DockerPortCheckCandidate `json:"skipped"`
	// Remaining counts ports left for another request once the per-request
	// limit is reached.
	Remaining int `json:"remaining"`
}

// HandleDockerPortSuggestions lists published container ports that have no
// availability check.
func (h *AvailabilityHandlers) HandleDockerPortSuggestions(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeJSON(w, http.StatusOK, dockerPortSuggestionsResponse{Suggestions: []monitoring.DockerPortCheckCandidate{}})
		return
	}
	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	candidates := monitoring.DockerPortCheckCandidates(h.dockerHostsForRequest(r.Context()), targets)
	if candidates == nil {
		candidates = []monitoring.DockerPortCheckCandidate{}
	}
	writeJSON(w, http.StatusOK, dockerPortSuggestionsResponse{Suggestions: candidates})
}

// dockerPortCheckSelection binds one reviewed offer to its exact container and
// endpoint. The saved DockerPort source deliberately has a different lifetime:
// its name-based key reattaches an existing check after container replacement.
// It must not be used as consent to probe a newly published endpoint.
type dockerPortCheckSelection struct {
	HostID        string `json:"hostId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	PrivatePort   int    `json:"privatePort"`
	PublicPort    int    `json:"publicPort"`
	Address       string `json:"address"`
}

const dockerPortsChangedMessage = "Published container ports changed. Refresh the page to review them before adding checks. No checks were added."

// HandleApplyDockerPortSuggestions adds a check only for the exact ports the
// operator reviewed. An optional hostId further limits it to one Docker host.
func (h *AvailabilityHandlers) HandleApplyDockerPortSuggestions(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}
	var request struct {
		HostID string `json:"hostId"`
		// Ports is required: no request may silently select the latest inventory.
		Ports []dockerPortCheckSelection `json:"ports"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	defer r.Body.Close()
	if body, err := io.ReadAll(r.Body); err != nil || (len(strings.TrimSpace(string(body))) > 0 && json.Unmarshal(body, &request) != nil) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body", nil)
		return
	}
	hostID := strings.TrimSpace(request.HostID)
	if len(request.Ports) == 0 || len(request.Ports) > maxDockerPortSelection {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Select between 1 and %d listed container ports, including their container IDs, addresses and published ports. Refresh the page to review them first.", maxDockerPortSelection), nil)
		return
	}
	selected := make(map[dockerPortCheckSelection]struct{}, len(request.Ports))
	for _, port := range request.Ports {
		if strings.TrimSpace(port.HostID) == "" || strings.TrimSpace(port.ContainerID) == "" ||
			strings.TrimSpace(port.ContainerName) == "" || port.PrivatePort < 1 || port.PrivatePort > 65535 ||
			port.PublicPort < 1 || port.PublicPort > 65535 ||
			port.HostID != strings.TrimSpace(port.HostID) || port.ContainerID != strings.TrimSpace(port.ContainerID) ||
			port.ContainerName != strings.TrimPrefix(strings.TrimSpace(port.ContainerName), "/") ||
			port.Address != strings.TrimSpace(port.Address) || (hostID != "" && port.HostID != hostID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Select the listed container IDs, addresses and published ports. Refresh the page to review them first.", nil)
			return
		}
		selected[port] = struct{}{}
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}

	hosts := h.dockerHostsForRequest(r.Context())
	if !dockerPortSelectionsCurrent(hosts, selected, hostID) {
		writeErrorResponse(w, http.StatusConflict, "docker_ports_changed", dockerPortsChangedMessage, nil)
		return
	}
	response := dockerPortApplyResponse{
		Created:    []availabilityTargetResponse{},
		Unanswered: []dockerPortUnansweredResponse{},
		Skipped:    []monitoring.DockerPortCheckCandidate{},
	}
	var checkable []monitoring.DockerPortCheckCandidate
	for _, candidate := range monitoring.DockerPortCheckCandidates(hosts, targets) {
		if hostID != "" && candidate.HostID != hostID {
			continue
		}
		if _, ok := selected[dockerPortCandidateKey(candidate)]; !ok {
			continue
		}
		switch {
		case candidate.SkipReason != "":
			response.Skipped = append(response.Skipped, candidate)
		case len(checkable) >= maxDockerPortChecksPerRequest:
			response.Remaining++
		default:
			checkable = append(checkable, candidate)
		}
	}

	resourceIDs := h.dockerContainerResourceIDs(r.Context())
	built := make([]*config.AvailabilityTarget, len(checkable))
	failures := make([]string, len(checkable))
	probe := h.dockerPortProbe
	if probe == nil {
		probe = monitoring.ProbeAvailabilityTargetDetailedResult
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, dockerPortProbeConcurrency)
	for i, candidate := range checkable {
		wg.Add(1)
		go func(i int, candidate monitoring.DockerPortCheckCandidate) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			target, err := detectDockerPortCheck(r.Context(), probe, candidate)
			if err != nil {
				failures[i] = err.Error()
				return
			}
			target.LinkedResourceID = resourceIDs[dockerContainerResourceKey(candidate.HostID, candidate.ContainerID)]
			built[i] = &target
		}(i, candidate)
	}
	wg.Wait()

	// Probing took a while. Load the saved checks again so a check an
	// operator added, edited or removed meanwhile is neither overwritten nor
	// duplicated.
	targets, err = persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	// Inventory can also change while probing. Never save a check against a
	// replacement or a no-longer-published endpoint, even when it answered.
	hosts = h.dockerHostsForRequest(r.Context())
	if !dockerPortSelectionsCurrent(hosts, selected, hostID) {
		writeErrorResponse(w, http.StatusConflict, "docker_ports_changed", dockerPortsChangedMessage, nil)
		return
	}
	stillUncovered := make(map[dockerPortCheckSelection]struct{})
	for _, candidate := range monitoring.DockerPortCheckCandidates(hosts, targets) {
		stillUncovered[dockerPortCandidateKey(candidate)] = struct{}{}
	}
	for i, candidate := range checkable {
		if built[i] == nil {
			response.Unanswered = append(response.Unanswered, dockerPortUnansweredResponse{DockerPortCheckCandidate: candidate, Error: failures[i]})
			continue
		}
		if _, ok := stillUncovered[dockerPortCandidateKey(candidate)]; !ok {
			continue
		}
		targets = append(targets, *built[i])
		response.Created = append(response.Created, availabilityTargetAPIResponse(*built[i]))
	}
	if len(response.Created) > 0 {
		if err := persistence.SaveAvailabilityTargets(targets); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "availability_save_failed", "Failed to save availability targets", map[string]string{"error": err.Error()})
			return
		}
		h.refreshMonitor(r.Context())
	}
	writeJSON(w, http.StatusOK, response)
}

// detectDockerPortCheck returns the HTTPS or HTTP check to save for one
// published port. It refuses a port that does not answer over either now.
func detectDockerPortCheck(ctx context.Context, probe dockerPortProbeFunc, candidate monitoring.DockerPortCheckCandidate) (config.AvailabilityTarget, error) {
	base := config.NewAvailabilityTarget()
	base.Name = fmt.Sprintf("%s:%d", candidate.ContainerName, candidate.PublicPort)
	base.TargetKind = config.AvailabilityTargetService
	base.Address = candidate.Address
	base.Port = candidate.PublicPort
	source := candidate.Source()
	base.DockerPort = &source

	attempt := func(protocol config.AvailabilityProbeProtocol) (config.AvailabilityTarget, availabilityprobe.ProbeResult, error) {
		target := base
		target.Protocol = protocol
		if protocol != config.AvailabilityProbeTCP {
			// An HTTP address is parsed as a URL host, where a bare IPv6
			// literal is malformed.
			if ip := net.ParseIP(target.Address); ip != nil && ip.To4() == nil {
				target.Address = "[" + ip.String() + "]"
			}
		}
		target = config.NormalizeAvailabilityTarget(target)
		if err := target.Validate(); err != nil {
			return target, availabilityprobe.ProbeResult{}, err
		}
		destination := target.ProbeAddress()
		if u, err := target.HTTPURL(); err == nil && protocol != config.AvailabilityProbeTCP {
			destination = u.String()
		}
		if err := securityutil.CheckTenantOutboundDestination(ctx, destination); err != nil {
			return target, availabilityprobe.ProbeResult{}, fmt.Errorf("%s", securityutil.TenantOutboundRefusalMessage(ctx, err))
		}
		probeCtx, cancel := context.WithTimeout(ctx, dockerPortProbeTimeout)
		defer cancel()
		result, err := probe(probeCtx, target)
		return target, result, err
	}

	// A refused or filtered port fails here once, without waiting on two
	// HTTP attempts.
	if _, _, err := attempt(config.AvailabilityProbeTCP); err != nil {
		return config.AvailabilityTarget{}, err
	}
	var lastErr error
	for _, protocol := range []config.AvailabilityProbeProtocol{config.AvailabilityProbeHTTPS, config.AvailabilityProbeHTTP} {
		target, result, err := attempt(protocol)
		if err == nil {
			return target, nil
		}
		if result.TransportOutcome == availabilityprobe.OutcomeReachable {
			// It speaks HTTP and is failing right now: say what it answered.
			return config.AvailabilityTarget{}, err
		}
		lastErr = err
	}
	// It accepted a connection and answered neither HTTPS nor HTTP. Pulse
	// saves no plain connection check here: Docker's proxy accepts for a
	// dead service when Pulse shares the host, so the check would pass for
	// good.
	return config.AvailabilityTarget{}, fmt.Errorf("it accepted a connection but did not answer over HTTP or HTTPS (%v)", lastErr)
}

func (h *AvailabilityHandlers) dockerHostsForRequest(ctx context.Context) []models.DockerHost {
	if h != nil && h.dockerHosts != nil {
		return h.dockerHosts(ctx)
	}
	if monitor := h.monitorForRequest(ctx); monitor != nil {
		return monitor.GetDockerHosts()
	}
	return nil
}

// dockerContainerResourceIDs maps a Docker host and container ID to the
// container's resource, so a new check is attached to it from the start.
func (h *AvailabilityHandlers) dockerContainerResourceIDs(ctx context.Context) map[string]string {
	ids := map[string]string{}
	if h != nil && h.dockerContainerResources != nil {
		return h.dockerContainerResources(ctx)
	}
	monitor := h.monitorForRequest(ctx)
	if monitor == nil {
		return ids
	}
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return ids
	}
	for _, container := range readState.DockerContainers() {
		if container == nil {
			continue
		}
		if key := dockerContainerResourceKey(container.HostSourceID(), container.ContainerID()); key != "" {
			ids[key] = container.ID()
		}
	}
	return ids
}

func dockerPortCandidateKey(candidate monitoring.DockerPortCheckCandidate) dockerPortCheckSelection {
	return dockerPortCheckSelection{
		HostID: candidate.HostID, ContainerID: candidate.ContainerID, ContainerName: candidate.ContainerName,
		PrivatePort: candidate.PrivatePort, PublicPort: candidate.PublicPort, Address: candidate.Address,
	}
}

// Validate against inventory, not just uncovered suggestions: an exact offer
// another request has already covered is an idempotent no-op, not a conflict.
func dockerPortSelectionsCurrent(hosts []models.DockerHost, selected map[dockerPortCheckSelection]struct{}, hostID string) bool {
	current := make(map[dockerPortCheckSelection]struct{})
	for _, candidate := range monitoring.DockerPortCheckCandidates(hosts, nil) {
		if hostID == "" || candidate.HostID == hostID {
			current[dockerPortCandidateKey(candidate)] = struct{}{}
		}
	}
	for selection := range selected {
		if _, ok := current[selection]; !ok {
			return false
		}
	}
	return true
}

func dockerContainerResourceKey(hostID, containerID string) string {
	hostID, containerID = strings.TrimSpace(hostID), strings.TrimSpace(containerID)
	if hostID == "" || containerID == "" {
		return ""
	}
	return hostID + "\x00" + containerID
}
