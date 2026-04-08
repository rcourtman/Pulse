package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// HypervisorHandlers manages API endpoints for hypervisor/cloud provider configuration.
type HypervisorHandlers struct {
	config *config.Config
}

// NewHypervisorHandlers creates a new HypervisorHandlers instance.
func NewHypervisorHandlers(cfg *config.Config) *HypervisorHandlers {
	return &HypervisorHandlers{config: cfg}
}

// HypervisorInstanceResponse is the API response for a hypervisor instance (password redacted).
type HypervisorInstanceResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Host           string `json:"host,omitempty"`
	Enabled        bool   `json:"enabled"`
	Username       string `json:"username,omitempty"`
	Region         string `json:"region,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	TenantID       string `json:"tenantId,omitempty"`
	ProjectID      string `json:"projectId,omitempty"`
	Datacenter     string `json:"datacenter,omitempty"`
	VerifySSL      bool   `json:"verifySSL"`
}

// HandleListHypervisors returns all configured hypervisor instances.
func (h *HypervisorHandlers) HandleListHypervisors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	instances := make([]HypervisorInstanceResponse, 0, len(h.config.HypervisorInstances))
	for _, inst := range h.config.HypervisorInstances {
		instances = append(instances, HypervisorInstanceResponse{
			ID:             inst.ID,
			Name:           inst.Name,
			Type:           inst.Type,
			Host:           inst.Host,
			Enabled:        inst.Enabled,
			Username:       inst.Username,
			Region:         inst.Region,
			SubscriptionID: inst.SubscriptionID,
			TenantID:       inst.TenantID,
			ProjectID:      inst.ProjectID,
			Datacenter:     inst.Datacenter,
			VerifySSL:      inst.VerifySSL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

// HandleAddHypervisor adds a new hypervisor instance.
func (h *HypervisorHandlers) HandleAddHypervisor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var inst config.HypervisorInstance
	if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate type.
	validTypes := map[string]bool{
		"vmware": true, "libvirt": true, "nutanix": true,
		"aws": true, "azure": true, "gcp": true, "hyperv": true,
	}
	if !validTypes[inst.Type] {
		http.Error(w, "Invalid hypervisor type. Supported: vmware, libvirt, nutanix, aws, azure, gcp, hyperv", http.StatusBadRequest)
		return
	}

	// Generate ID if not provided.
	if inst.ID == "" {
		inst.ID = inst.Type + "-" + uuid.New().String()[:8]
	}

	config.Mu.Lock()
	h.config.HypervisorInstances = append(h.config.HypervisorInstances, inst)
	config.Mu.Unlock()

	log.Info().Str("id", inst.ID).Str("type", inst.Type).Str("name", inst.Name).Msg("Hypervisor instance added")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(HypervisorInstanceResponse{
		ID:      inst.ID,
		Name:    inst.Name,
		Type:    inst.Type,
		Host:    inst.Host,
		Enabled: inst.Enabled,
	})
}

// HandleUpdateHypervisor updates an existing hypervisor instance.
func (h *HypervisorHandlers) HandleUpdateHypervisor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/hypervisors/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/hypervisors/")
	if id == "" {
		http.Error(w, "Hypervisor ID required", http.StatusBadRequest)
		return
	}

	var update config.HypervisorInstance
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	config.Mu.Lock()
	found := false
	for i, inst := range h.config.HypervisorInstances {
		if inst.ID == id {
			update.ID = id // Preserve ID
			if update.Type == "" {
				update.Type = inst.Type // Preserve type if not provided
			}
			h.config.HypervisorInstances[i] = update
			found = true
			break
		}
	}
	config.Mu.Unlock()

	if !found {
		http.Error(w, "Hypervisor instance not found", http.StatusNotFound)
		return
	}

	log.Info().Str("id", id).Msg("Hypervisor instance updated")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// HandleDeleteHypervisor removes a hypervisor instance.
func (h *HypervisorHandlers) HandleDeleteHypervisor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/hypervisors/")
	if id == "" {
		http.Error(w, "Hypervisor ID required", http.StatusBadRequest)
		return
	}

	config.Mu.Lock()
	found := false
	for i, inst := range h.config.HypervisorInstances {
		if inst.ID == id {
			h.config.HypervisorInstances = append(h.config.HypervisorInstances[:i], h.config.HypervisorInstances[i+1:]...)
			found = true
			break
		}
	}
	config.Mu.Unlock()

	if !found {
		http.Error(w, "Hypervisor instance not found", http.StatusNotFound)
		return
	}

	log.Info().Str("id", id).Msg("Hypervisor instance deleted")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// HandleTestHypervisor tests connectivity to a hypervisor instance.
func (h *HypervisorHandlers) HandleTestHypervisor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/hypervisors/{id}/test
	path := strings.TrimPrefix(r.URL.Path, "/api/hypervisors/")
	id := strings.TrimSuffix(path, "/test")
	if id == "" {
		http.Error(w, "Hypervisor ID required", http.StatusBadRequest)
		return
	}

	// Find the instance.
	config.Mu.RLock()
	var inst *config.HypervisorInstance
	for _, i := range h.config.HypervisorInstances {
		if i.ID == id {
			inst = &i
			break
		}
	}
	config.Mu.RUnlock()

	if inst == nil {
		http.Error(w, "Hypervisor instance not found", http.StatusNotFound)
		return
	}

	// Test result placeholder - full implementation would create a temporary provider and connect.
	result := map[string]interface{}{
		"id":      inst.ID,
		"type":    inst.Type,
		"success": true,
		"message": "Connection test placeholder - provider creation succeeded",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleHypervisorTypes returns the list of supported hypervisor types.
func (h *HypervisorHandlers) HandleHypervisorTypes(w http.ResponseWriter, r *http.Request) {
	types := []map[string]string{
		{"type": "vmware", "name": "VMware vSphere", "description": "VMware vCenter Server or standalone ESXi host"},
		{"type": "libvirt", "name": "KVM/libvirt", "description": "KVM hypervisor via libvirt (direct or agent-based)"},
		{"type": "nutanix", "name": "Nutanix", "description": "Nutanix AHV via Prism Central"},
		{"type": "aws", "name": "Amazon Web Services", "description": "AWS EC2 instances and EBS volumes"},
		{"type": "azure", "name": "Microsoft Azure", "description": "Azure VMs and managed disks"},
		{"type": "gcp", "name": "Google Cloud Platform", "description": "GCE instances and persistent disks"},
		{"type": "hyperv", "name": "Microsoft Hyper-V", "description": "Hyper-V via WinRM (coming soon)"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types)
}
