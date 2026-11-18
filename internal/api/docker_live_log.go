package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// Each waiting HTTP request owns one slot. Disconnect, timeout and successful
// reads all remove it. There is no history, broadcast or persistence of lines.
type dockerLiveLogRequests struct {
	mu    sync.Mutex
	slots map[string]*dockerLiveLogSlot
}
type dockerLiveLogSlot struct {
	request    agentsdocker.LogTailRequest
	org, token string
	offered    bool
	result     chan agentsdocker.LogTailResult
}

func exactLogTailContainer(host models.DockerHost, id string) bool {
	if !host.LiveLogsEnabled || host.IdentityConflict != nil || host.TokenID == "" || len(id) != 64 {
		return false
	}
	for _, c := range host.Containers {
		if c.ID == id {
			return true
		}
	}
	return false
}

func (b *dockerLiveLogRequests) start(org string, host models.DockerHost, container string) *dockerLiveLogSlot {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.slots) >= 128 {
		return nil
	}
	count := 0
	for _, slot := range b.slots {
		if slot.org != org {
			continue
		}
		count++
		if slot.request.HostID == host.ID || count >= 16 {
			return nil
		}
	}
	slot := &dockerLiveLogSlot{request: agentsdocker.LogTailRequest{ID: uuid.NewString(), HostID: host.ID, ContainerID: container}, org: org, token: host.TokenID, result: make(chan agentsdocker.LogTailResult, 1)}
	if b.slots == nil {
		b.slots = make(map[string]*dockerLiveLogSlot)
	}
	b.slots[slot.request.ID] = slot
	return slot
}
func (b *dockerLiveLogRequests) remove(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.slots, id)
}
func (b *dockerLiveLogRequests) offer(org string, host models.DockerHost, token string) *agentsdocker.LogTailRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, slot := range b.slots {
		if !slot.offered && slot.org == org && slot.token == token && slot.request.HostID == host.ID && exactLogTailContainer(host, slot.request.ContainerID) {
			slot.offered = true
			request := slot.request
			return &request
		}
	}
	return nil
}
func (b *dockerLiveLogRequests) finish(org, token string, result agentsdocker.LogTailResult) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	slot := b.slots[result.ID]
	if slot == nil || !slot.offered || slot.token != token || slot.org != org || slot.request != result.LogTailRequest {
		return false
	}
	var cut bool
	result.Lines, cut = agentsdocker.BoundFailureLogLines(result.Lines)
	result.Truncated = result.Truncated || cut
	select {
	case slot.result <- result:
		delete(b.slots, result.ID)
		return true
	default:
		return false
	}
}

func (h *DockerAgentHandlers) HandleLogTail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	monitor := h.requireMonitor(w, r)
	if monitor == nil {
		return
	}
	var target struct {
		HostID      string `json:"hostId"`
		ContainerID string `json:"containerId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&target); err != nil {
		http.Error(w, "Invalid container", http.StatusBadRequest)
		return
	}
	host, ok := monitor.GetDockerHost(target.HostID)
	if !ok || host.ID != target.HostID || !exactLogTailContainer(host, target.ContainerID) {
		http.Error(w, "Live logs unavailable for this container", http.StatusConflict)
		return
	}
	slot := h.liveLogs.start(GetOrgID(r.Context()), host, target.ContainerID)
	if slot == nil {
		http.Error(w, "A log read is already in progress. Try again shortly.", http.StatusTooManyRequests)
		return
	}
	defer h.liveLogs.remove(slot.request.ID)
	tokenCheck := tokenStreamSnapshotRevalidator(h.config, r, config.ScopeDockerManage)
	sessionCheck := combineAuthorization(sessionReadRevalidator(r), orgAccessRevalidator(r))
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		if r.Context().Err() == nil {
			http.Error(w, "The runtime did not respond before the log read expired", http.StatusGatewayTimeout)
		}
	case result := <-slot.result:
		if tokenCheck != nil {
			token, valid := tokenCheck()
			if !valid {
				http.Error(w, "Access expired", http.StatusForbidden)
				return
			}
			current := r.Clone(r.Context())
			attachAPITokenRecord(current, token)
			r = current
		} else if sessionCheck != nil && !sessionCheck() {
			http.Error(w, "Access expired", http.StatusForbidden)
			return
		}
		if !ensureOrgAdminSession(h.config, w, r) {
			return
		}
		current, ok := monitor.GetDockerHost(target.HostID)
		if result.Unavailable || !ok || current.ID != host.ID || current.TokenID != slot.token || !exactLogTailContainer(current, target.ContainerID) {
			http.Error(w, "The runtime could not read this container log", http.StatusConflict)
			return
		}
		_ = utils.WriteJSONResponse(w, map[string]any{"lines": result.Lines, "truncated": result.Truncated})
	}
}
func (h *DockerAgentHandlers) HandleLogTailResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	token := getAPITokenRecordFromRequest(r)
	if token == nil || token.ID == "" {
		http.Error(w, "Reporting token required", http.StatusForbidden)
		return
	}
	var result agentsdocker.LogTailResult
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&result); err != nil {
		http.Error(w, "Invalid log result", http.StatusBadRequest)
		return
	}
	if !h.liveLogs.finish(GetOrgID(r.Context()), token.ID, result) {
		http.Error(w, "Log read expired", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
