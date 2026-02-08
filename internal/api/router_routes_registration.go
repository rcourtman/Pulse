package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

func (r *Router) registerPublicAndAuthRoutes() {
	r.registerAuthSecurityInstallRoutes()
}

func (r *Router) registerMonitoringRoutes(
	guestMetadataHandler *GuestMetadataHandler,
	dockerMetadataHandler *DockerMetadataHandler,
	hostMetadataHandler *HostMetadataHandler,
	infraUpdateHandlers *UpdateDetectionHandlers,
) {
	r.mux.HandleFunc("/api/monitoring/scheduler/health", RequireAuth(r.config, r.handleSchedulerHealth))
	r.mux.HandleFunc("/api/storage/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleStorage)))
	r.mux.HandleFunc("/api/storage-charts", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleStorageCharts)))
	r.mux.HandleFunc("/api/charts", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleCharts)))
	r.mux.HandleFunc("/api/charts/workloads", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleWorkloadCharts)))
	r.mux.HandleFunc("/api/charts/infrastructure", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleInfrastructureCharts)))
	r.mux.HandleFunc("/api/charts/infrastructure-summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleInfrastructureSummaryCharts)))
	r.mux.HandleFunc("/api/charts/workloads-summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleWorkloadsSummaryCharts)))
	r.mux.HandleFunc("/api/metrics-store/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleMetricsStoreStats)))
	r.mux.HandleFunc("/api/metrics-store/history", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleMetricsHistory)))
	r.mux.HandleFunc("/api/backups", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackups)))
	r.mux.HandleFunc("/api/backups/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackups)))
	r.mux.HandleFunc("/api/backups/unified", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackups)))
	r.mux.HandleFunc("/api/backups/pve", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackupsPVE)))
	r.mux.HandleFunc("/api/backups/pbs", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackupsPBS)))
	r.mux.HandleFunc("/api/snapshots", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleSnapshots)))

	// Unified resources API (Phase 1 of unified resource architecture)
	r.mux.HandleFunc("/api/resources", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleGetResources)))
	r.mux.HandleFunc("/api/resources/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleGetResourceStats)))
	r.mux.HandleFunc("/api/resources/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleGetResource)))

	// Unified resources API v2 (merge layer)
	r.mux.HandleFunc("/api/v2/resources", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceV2Handlers.HandleListResources)))
	r.mux.HandleFunc("/api/v2/resources/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceV2Handlers.HandleStats)))
	r.mux.HandleFunc("/api/v2/resources/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceV2Handlers.HandleResourceRoutes)))
	// Guest metadata routes
	r.mux.HandleFunc("/api/guests/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, guestMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/guests/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			guestMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			guestMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			guestMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Docker metadata routes
	r.mux.HandleFunc("/api/docker/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, dockerMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/docker/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			dockerMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Docker host metadata routes (for managing Docker host custom URLs, e.g., Portainer links)
	r.mux.HandleFunc("/api/docker/hosts/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, dockerMetadataHandler.HandleGetHostMetadata)))
	r.mux.HandleFunc("/api/docker/hosts/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			dockerMetadataHandler.HandleGetHostMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleUpdateHostMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleDeleteHostMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Host metadata routes
	r.mux.HandleFunc("/api/hosts/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, hostMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/hosts/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			hostMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			hostMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			hostMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Infrastructure update detection routes (Docker containers, packages, etc.)
	r.mux.HandleFunc("/api/infra-updates", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, infraUpdateHandlers.HandleGetInfraUpdates)))
	r.mux.HandleFunc("/api/infra-updates/summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, infraUpdateHandlers.HandleGetInfraUpdatesSummary)))
	r.mux.HandleFunc("/api/infra-updates/check", RequireAuth(r.config, RequireScope(config.ScopeMonitoringWrite, infraUpdateHandlers.HandleTriggerInfraUpdateCheck)))
	r.mux.HandleFunc("/api/infra-updates/host/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, req *http.Request) {
		// Extract host ID from path: /api/infra-updates/host/{hostId}
		hostID := strings.TrimPrefix(req.URL.Path, "/api/infra-updates/host/")
		hostID = strings.TrimSuffix(hostID, "/")
		if hostID == "" {
			writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
			return
		}
		infraUpdateHandlers.HandleGetInfraUpdatesForHost(w, req, hostID)
	})))
	r.mux.HandleFunc("/api/infra-updates/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, req *http.Request) {
		// Extract resource ID from path: /api/infra-updates/{resourceId}
		resourceID := strings.TrimPrefix(req.URL.Path, "/api/infra-updates/")
		resourceID = strings.TrimSuffix(resourceID, "/")
		if resourceID == "" || resourceID == "summary" || resourceID == "check" || strings.HasPrefix(resourceID, "host/") {
			// Let specific handlers deal with these
			http.NotFound(w, req)
			return
		}
		infraUpdateHandlers.HandleGetInfraUpdateForResource(w, req, resourceID)
	})))
	r.mux.HandleFunc("/api/discover", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleDiscoverServers)))
	// Alert routes - require monitoring:read scope to view alerts
	r.mux.HandleFunc("/api/alerts/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.alertHandlers.HandleAlerts)))

	// Notification routes
	r.mux.HandleFunc("/api/notifications/", RequireAdmin(r.config, r.notificationHandlers.HandleNotifications))

	// Notification queue/DLQ routes
	// Security tokens are handled later in the setup with RBAC
	// SECURITY: DLQ endpoints require settings:read/write scope because DLQ entries may contain
	// notification configs with webhook URLs, SMTP credentials, or other sensitive data
	r.mux.HandleFunc("/api/notifications/dlq", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			r.notificationQueueHandlers.GetDLQ(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/notifications/queue/stats", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			r.notificationQueueHandlers.GetQueueStats(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/notifications/dlq/retry", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			r.notificationQueueHandlers.RetryDLQItem(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/notifications/dlq/delete", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost || req.Method == http.MethodDelete {
			r.notificationQueueHandlers.DeleteDLQItem(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	// AI-powered infrastructure discovery endpoints
	r.mux.HandleFunc("/api/discovery", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.discoveryHandlers.HandleListDiscoveries)))
	r.mux.HandleFunc("/api/discovery/status", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.discoveryHandlers.HandleGetStatus)))
	r.mux.HandleFunc("/api/discovery/settings", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, r.discoveryHandlers.HandleUpdateSettings)))
	r.mux.HandleFunc("/api/discovery/info/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.discoveryHandlers.HandleGetInfo)))
	r.mux.HandleFunc("/api/discovery/type/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.discoveryHandlers.HandleListByType)))
	r.mux.HandleFunc("/api/discovery/host/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		// Route based on method and path depth:
		// GET /api/discovery/host/{hostId} → list discoveries for host
		// GET /api/discovery/host/{hostId}/{resourceId} → get specific discovery
		// GET /api/discovery/host/{hostId}/{resourceId}/progress → get scan progress
		// POST /api/discovery/host/{hostId}/{resourceId} → trigger discovery
		// PUT /api/discovery/host/{hostId}/{resourceId}/notes → update notes
		// DELETE /api/discovery/host/{hostId}/{resourceId} → delete discovery
		path := strings.TrimPrefix(req.URL.Path, "/api/discovery/host/")
		pathParts := strings.Split(strings.TrimSuffix(path, "/"), "/")

		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			if len(pathParts) == 1 && pathParts[0] != "" {
				// GET /api/discovery/host/{hostId} → list by host
				r.discoveryHandlers.HandleListByHost(w, req)
			} else if len(pathParts) >= 2 {
				if strings.HasSuffix(req.URL.Path, "/progress") {
					r.discoveryHandlers.HandleGetProgress(w, req)
				} else {
					// GET /api/discovery/host/{hostId}/{resourceId} → get specific discovery
					r.discoveryHandlers.HandleGetDiscovery(w, req)
				}
			} else {
				http.Error(w, "Invalid path", http.StatusBadRequest)
			}
		case http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			// POST /api/discovery/host/{hostId}/{resourceId} → trigger discovery
			r.discoveryHandlers.HandleTriggerDiscovery(w, req)
		case http.MethodPut:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			if strings.HasSuffix(req.URL.Path, "/notes") {
				r.discoveryHandlers.HandleUpdateNotes(w, req)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			r.discoveryHandlers.HandleDeleteDiscovery(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/discovery/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			if strings.HasSuffix(path, "/progress") {
				r.discoveryHandlers.HandleGetProgress(w, req)
			} else {
				r.discoveryHandlers.HandleGetDiscovery(w, req)
			}
		case http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			r.discoveryHandlers.HandleTriggerDiscovery(w, req)
		case http.MethodPut:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			if strings.HasSuffix(path, "/notes") {
				r.discoveryHandlers.HandleUpdateNotes(w, req)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			r.discoveryHandlers.HandleDeleteDiscovery(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}

func (r *Router) registerConfigSystemRoutes(updateHandlers *UpdateHandlers) {
	// Log management routes
	r.mux.HandleFunc("/api/logs/stream", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.logHandlers.HandleStreamLogs)))
	r.mux.HandleFunc("/api/logs/download", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.logHandlers.HandleDownloadBundle)))
	r.mux.HandleFunc("/api/logs/level", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.logHandlers.HandleGetLevel))(w, req)
		case http.MethodPost:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.logHandlers.HandleSetLevel))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.mux.HandleFunc("/api/agents/docker/report", RequireAuth(r.config, RequireScope(config.ScopeDockerReport, r.dockerAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/kubernetes/report", RequireAuth(r.config, RequireScope(config.ScopeKubernetesReport, r.kubernetesAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/host/report", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/host/lookup", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleLookup)))
	r.mux.HandleFunc("/api/agents/host/uninstall", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleUninstall)))
	// SECURITY: Use settings:write (not just host_manage) to prevent compromised host tokens from manipulating other hosts
	r.mux.HandleFunc("/api/agents/host/unlink", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.hostAgentHandlers.HandleUnlink)))
	r.mux.HandleFunc("/api/agents/host/link", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.hostAgentHandlers.HandleLink)))
	// Host agent management routes - config endpoint is accessible by agents (GET) and admins (PATCH)
	r.mux.HandleFunc("/api/agents/host/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		// Route /api/agents/host/{id}/config to HandleConfig
		if strings.HasSuffix(req.URL.Path, "/config") {
			// GET is for agents to fetch config (host config scope)
			// PATCH is for UI to update config (host_manage scope, admin only)
			if req.Method == http.MethodPatch {
				RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
					if !ensureScope(w, req, config.ScopeHostManage) {
						return
					}
					r.hostAgentHandlers.HandleConfig(w, req)
				})(w, req)
				return
			}
			r.hostAgentHandlers.HandleConfig(w, req)
			return
		}
		// Route DELETE /api/agents/host/{id} to HandleDeleteHost
		// SECURITY: Require settings:write (not just host_manage) to prevent compromised host tokens from deleting other hosts
		if req.Method == http.MethodDelete {
			RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
				if !ensureScope(w, req, config.ScopeSettingsWrite) {
					return
				}
				r.hostAgentHandlers.HandleDeleteHost(w, req)
			})(w, req)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))
	r.mux.HandleFunc("/api/agents/docker/commands/", RequireAuth(r.config, RequireScope(config.ScopeDockerReport, r.dockerAgentHandlers.HandleCommandAck)))
	r.mux.HandleFunc("/api/agents/docker/hosts/", RequireAdmin(r.config, RequireScope(config.ScopeDockerManage, r.dockerAgentHandlers.HandleDockerHostActions)))
	r.mux.HandleFunc("/api/agents/docker/containers/update", RequireAdmin(r.config, RequireScope(config.ScopeDockerManage, r.dockerAgentHandlers.HandleContainerUpdate)))
	r.mux.HandleFunc("/api/agents/kubernetes/clusters/", RequireAdmin(r.config, RequireScope(config.ScopeKubernetesManage, r.kubernetesAgentHandlers.HandleClusterActions)))
	r.mux.HandleFunc("/api/diagnostics", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.handleDiagnostics)))
	r.mux.HandleFunc("/api/diagnostics/docker/prepare-token", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.handleDiagnosticsDockerPrepareToken)))
	r.mux.HandleFunc("/api/config", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleConfig)))
	// Update routes
	r.mux.HandleFunc("/api/updates/check", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleCheckUpdates)))
	r.mux.HandleFunc("/api/updates/apply", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, updateHandlers.HandleApplyUpdate)))
	r.mux.HandleFunc("/api/updates/status", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleUpdateStatus)))
	r.mux.HandleFunc("/api/updates/stream", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleUpdateStream)))
	r.mux.HandleFunc("/api/updates/plan", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleGetUpdatePlan)))
	r.mux.HandleFunc("/api/updates/history", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleListUpdateHistory)))
	r.mux.HandleFunc("/api/updates/history/entry", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleGetUpdateHistoryEntry)))
	// Config management routes
	r.mux.HandleFunc("/api/config/nodes", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.configHandlers.HandleGetNodes))(w, req)
		case http.MethodPost:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleAddNode))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Test node configuration endpoint (for new nodes)
	r.mux.HandleFunc("/api/config/nodes/test-config", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleTestNodeConfig))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Test connection endpoint
	r.mux.HandleFunc("/api/config/nodes/test-connection", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleTestConnection))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.mux.HandleFunc("/api/config/nodes/", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleUpdateNode))(w, req)
		case http.MethodDelete:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleDeleteNode))(w, req)
		case http.MethodPost:
			// Handle test endpoint and refresh-cluster endpoint
			if strings.HasSuffix(req.URL.Path, "/test") {
				RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleTestNode))(w, req)
			} else if strings.HasSuffix(req.URL.Path, "/refresh-cluster") {
				RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleRefreshClusterNodes))(w, req)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Config Profile Routes - Protected by Admin Auth, Settings Scope, and Pro License
	// SECURITY: Require settings:write scope to prevent low-privilege tokens from modifying agent profiles
	// r.configProfileHandler.ServeHTTP implements http.Handler, so we wrap it
	r.mux.Handle("/api/admin/profiles/", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, RequireLicenseFeature(r.licenseHandlers, license.FeatureAgentProfiles, func(w http.ResponseWriter, req *http.Request) {
		http.StripPrefix("/api/admin/profiles", r.configProfileHandler).ServeHTTP(w, req)
	}))))

	// System settings routes
	r.mux.HandleFunc("/api/config/system", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			handler := r.configHandlers.HandleGetSystemSettings
			if r.systemSettingsHandler != nil {
				handler = r.systemSettingsHandler.HandleGetSystemSettings
			}
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, handler))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Mock mode toggle routes
	r.mux.HandleFunc("/api/system/mock-mode", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.configHandlers.HandleGetMockMode))(w, req)
		case http.MethodPost, http.MethodPut:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleUpdateMockMode))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Config export/import routes (requires authentication)
	r.mux.HandleFunc("/api/config/export", r.exportLimiter.Middleware(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			// Check proxy auth first
			hasValidProxyAuth := false
			proxyAuthIsAdmin := false
			if r.config.ProxyAuthSecret != "" {
				if valid, _, isAdmin := CheckProxyAuth(r.config, req); valid {
					hasValidProxyAuth = true
					proxyAuthIsAdmin = isAdmin
				}
			}

			// Check authentication - accept proxy auth, session auth or API token
			hasValidSession := false
			if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
				hasValidSession = ValidateSession(cookie.Value)
			}

			validateAPIToken := func(token string) bool {
				if token == "" || !r.config.HasAPITokens() {
					return false
				}
				_, ok := r.config.ValidateAPIToken(token)
				return ok
			}

			token := req.Header.Get("X-API-Token")
			if token == "" {
				if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			hasValidAPIToken := validateAPIToken(token)

			// Check if any valid auth method is present
			hasValidAuth := hasValidProxyAuth || hasValidSession || hasValidAPIToken

			// Determine if auth is required
			authRequired := r.config.AuthUser != "" && r.config.AuthPass != "" ||
				r.config.HasAPITokens() ||
				r.config.ProxyAuthSecret != ""

			// Check admin privileges for proxy auth users
			if hasValidProxyAuth && !proxyAuthIsAdmin {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Msg("Non-admin proxy auth user attempted export/import")
				http.Error(w, "Admin privileges required for export/import", http.StatusForbidden)
				return
			}

			if authRequired && !hasValidAuth {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Bool("proxyAuth", hasValidProxyAuth).
					Bool("session", hasValidSession).
					Bool("apiToken", hasValidAPIToken).
					Msg("Unauthorized export attempt")
				http.Error(w, "Unauthorized - please log in or provide API token", http.StatusUnauthorized)
				return
			} else if !authRequired {
				// No auth configured - check if this is a homelab/private network
				clientIP := GetClientIP(req)

				isPrivate := isPrivateIP(clientIP)
				allowUnprotected := os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true"

				if !isPrivate && !allowUnprotected {
					// Public network access without auth - definitely block
					log.Warn().
						Str("ip", req.RemoteAddr).
						Bool("private_network", isPrivate).
						Msg("Export blocked - public network requires authentication")
					http.Error(w, "Export requires authentication on public networks", http.StatusForbidden)
					return
				} else if isPrivate && !allowUnprotected {
					// Private network but ALLOW_UNPROTECTED_EXPORT not set - show helpful message
					log.Info().
						Str("ip", req.RemoteAddr).
						Msg("Export allowed - private network with no auth")
					// Continue - allow export on private networks for homelab users
				}
			}

			// SECURITY: Check settings:read scope for API token auth
			if hasValidAPIToken && token != "" {
				record, _ := r.config.ValidateAPIToken(token)
				if record != nil && !record.HasScope(config.ScopeSettingsRead) {
					log.Warn().
						Str("ip", req.RemoteAddr).
						Str("path", req.URL.Path).
						Str("token_id", record.ID).
						Msg("API token missing settings:read scope for export")
					http.Error(w, "API token missing required scope: settings:read", http.StatusForbidden)
					return
				}
			}

			// Log successful export attempt
			log.Info().
				Str("ip", req.RemoteAddr).
				Bool("proxy_auth", hasValidProxyAuth).
				Bool("session_auth", hasValidSession).
				Bool("api_token_auth", hasValidAPIToken).
				Msg("Configuration export initiated")

			r.configHandlers.HandleExportConfig(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	r.mux.HandleFunc("/api/config/import", r.exportLimiter.Middleware(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			// Check proxy auth first
			hasValidProxyAuth := false
			proxyAuthIsAdmin := false
			if r.config.ProxyAuthSecret != "" {
				if valid, _, isAdmin := CheckProxyAuth(r.config, req); valid {
					hasValidProxyAuth = true
					proxyAuthIsAdmin = isAdmin
				}
			}

			// Check authentication - accept proxy auth, session auth or API token
			hasValidSession := false
			if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
				hasValidSession = ValidateSession(cookie.Value)
			}

			validateAPIToken := func(token string) bool {
				if token == "" || !r.config.HasAPITokens() {
					return false
				}
				_, ok := r.config.ValidateAPIToken(token)
				return ok
			}

			token := req.Header.Get("X-API-Token")
			if token == "" {
				if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			hasValidAPIToken := validateAPIToken(token)

			// Check if any valid auth method is present
			hasValidAuth := hasValidProxyAuth || hasValidSession || hasValidAPIToken

			// Determine if auth is required
			authRequired := r.config.AuthUser != "" && r.config.AuthPass != "" ||
				r.config.HasAPITokens() ||
				r.config.ProxyAuthSecret != ""

			// Check admin privileges for proxy auth users
			if hasValidProxyAuth && !proxyAuthIsAdmin {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Msg("Non-admin proxy auth user attempted export/import")
				http.Error(w, "Admin privileges required for export/import", http.StatusForbidden)
				return
			}

			if authRequired && !hasValidAuth {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Bool("proxyAuth", hasValidProxyAuth).
					Bool("session", hasValidSession).
					Bool("apiToken", hasValidAPIToken).
					Msg("Unauthorized import attempt")
				http.Error(w, "Unauthorized - please log in or provide API token", http.StatusUnauthorized)
				return
			} else if !authRequired {
				// No auth configured - check if this is a homelab/private network
				clientIP := GetClientIP(req)

				isPrivate := isPrivateIP(clientIP)
				allowUnprotected := os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true"

				if !isPrivate && !allowUnprotected {
					// Public network access without auth - definitely block
					log.Warn().
						Str("ip", req.RemoteAddr).
						Bool("private_network", isPrivate).
						Msg("Import blocked - public network requires authentication")
					http.Error(w, "Import requires authentication on public networks", http.StatusForbidden)
					return
				} else if isPrivate && !allowUnprotected {
					// Private network but ALLOW_UNPROTECTED_EXPORT not set - show helpful message
					log.Info().
						Str("ip", req.RemoteAddr).
						Msg("Import allowed - private network with no auth")
					// Continue - allow import on private networks for homelab users
				}
			}

			// SECURITY: Check settings:write scope for API token auth
			if hasValidAPIToken && token != "" {
				record, _ := r.config.ValidateAPIToken(token)
				if record != nil && !record.HasScope(config.ScopeSettingsWrite) {
					log.Warn().
						Str("ip", req.RemoteAddr).
						Str("path", req.URL.Path).
						Str("token_id", record.ID).
						Msg("API token missing settings:write scope for import")
					http.Error(w, "API token missing required scope: settings:write", http.StatusForbidden)
					return
				}
			}

			// Log successful import attempt
			log.Info().
				Str("ip", req.RemoteAddr).
				Bool("session_auth", hasValidSession).
				Bool("api_token_auth", hasValidAPIToken).
				Msg("Configuration import initiated")

			r.configHandlers.HandleImportConfig(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Discovery route

	// Setup script route
	r.mux.HandleFunc("/api/setup-script", r.configHandlers.HandleSetupScript)

	// Generate setup script URL with temporary token (for authenticated users)
	r.mux.HandleFunc("/api/setup-script-url", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleSetupScriptURL)))

	// Generate agent install command with API token (for authenticated users)
	r.mux.HandleFunc("/api/agent-install-command", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleAgentInstallCommand)))

	// Auto-register route for setup scripts
	r.mux.HandleFunc("/api/auto-register", r.configHandlers.HandleAutoRegister)
	// Discovery endpoint
	// Test endpoint for WebSocket notifications
	// SECURITY: Require settings:write scope for test notifications to prevent unauthenticated broadcasting
	r.mux.HandleFunc("/api/test-notification", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Send a test auto-registration notification
		r.wsHub.BroadcastMessage(websocket.Message{
			Type: "node_auto_registered",
			Data: map[string]interface{}{
				"type":     "pve",
				"host":     "test-node.example.com",
				"name":     "Test Node",
				"tokenId":  "test-token",
				"hasToken": true,
			},
			Timestamp: time.Now().Format(time.RFC3339),
		})

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "notification sent"})
	})))
	r.mux.HandleFunc("/api/system/settings", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.systemSettingsHandler.HandleGetSystemSettings)))
	r.mux.HandleFunc("/api/system/settings/update", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.systemSettingsHandler.HandleUpdateSystemSettings)))
	r.mux.HandleFunc("/api/system/ssh-config", r.handleSSHConfig)
	r.mux.HandleFunc("/api/system/verify-temperature-ssh", r.handleVerifyTemperatureSSH)
}

func (r *Router) registerAIRelayRoutes() {
	r.mux.HandleFunc("/api/settings/ai", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceSettings, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleGetAISettings)))
	r.mux.HandleFunc("/api/settings/ai/update", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleUpdateAISettings)))
	r.mux.HandleFunc("/api/ai/test", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestAIConnection)))
	r.mux.HandleFunc("/api/ai/test/{provider}", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestProvider)))
	// AI models list - require ai:chat scope (needed to select a model for chat)
	r.mux.HandleFunc("/api/ai/models", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleListModels)))
	r.mux.HandleFunc("/api/ai/execute", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecute)))
	r.mux.HandleFunc("/api/ai/execute/stream", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecuteStream)))
	r.mux.HandleFunc("/api/ai/kubernetes/analyze", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, license.FeatureKubernetesAI, r.aiSettingsHandler.HandleAnalyzeKubernetesCluster))))
	r.mux.HandleFunc("/api/ai/investigate-alert", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIAlerts, r.aiSettingsHandler.HandleInvestigateAlert))))

	r.mux.HandleFunc("/api/ai/run-command", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleRunCommand)))
	// SECURITY: AI Knowledge endpoints require ai:chat scope to prevent arbitrary guest data access
	r.mux.HandleFunc("/api/ai/knowledge", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleGetGuestKnowledge)))
	r.mux.HandleFunc("/api/ai/knowledge/save", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleSaveGuestNote)))
	r.mux.HandleFunc("/api/ai/knowledge/delete", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleDeleteGuestNote)))
	r.mux.HandleFunc("/api/ai/knowledge/export", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleExportGuestKnowledge)))
	r.mux.HandleFunc("/api/ai/knowledge/import", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleImportGuestKnowledge)))
	r.mux.HandleFunc("/api/ai/knowledge/clear", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleClearGuestKnowledge)))
	// SECURITY: Debug context leaks system prompt and infra details - require settings:read scope
	r.mux.HandleFunc("/api/ai/debug/context", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleDebugContext)))
	// SECURITY: Connected agents list could reveal fleet topology - require ai:execute scope
	r.mux.HandleFunc("/api/ai/agents", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetConnectedAgents)))
	// SECURITY: Cost summary could reveal usage patterns - require settings:read scope
	r.mux.HandleFunc("/api/ai/cost/summary", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleGetAICostSummary)))
	r.mux.HandleFunc("/api/ai/cost/reset", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleResetAICostHistory)))
	r.mux.HandleFunc("/api/ai/cost/export", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleExportAICostHistory)))
	// OAuth endpoints for Claude Pro/Max subscription authentication
	// Require settings:write scope to prevent low-privilege tokens from modifying OAuth credentials
	r.mux.HandleFunc("/api/ai/oauth/start", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleOAuthStart)))
	r.mux.HandleFunc("/api/ai/oauth/exchange", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleOAuthExchange))) // Manual code input
	r.mux.HandleFunc("/api/ai/oauth/callback", r.aiSettingsHandler.HandleOAuthCallback)                                                                  // Public - receives redirect from Anthropic
	r.mux.HandleFunc("/api/ai/oauth/disconnect", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleOAuthDisconnect)))

	// Relay routes for mobile remote access
	r.mux.HandleFunc("GET /api/settings/relay", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, RequireLicenseFeature(r.licenseHandlers, license.FeatureRelay, r.handleGetRelayConfig))))
	r.mux.HandleFunc("PUT /api/settings/relay", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, RequireLicenseFeature(r.licenseHandlers, license.FeatureRelay, r.handleUpdateRelayConfig))))
	r.mux.HandleFunc("GET /api/settings/relay/status", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, RequireLicenseFeature(r.licenseHandlers, license.FeatureRelay, r.handleGetRelayStatus))))
	r.mux.HandleFunc("GET /api/onboarding/qr", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, r.handleGetOnboardingQR)))
	r.mux.HandleFunc("POST /api/onboarding/validate", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, r.handleValidateOnboardingConnection)))
	r.mux.HandleFunc("GET /api/onboarding/deep-link", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, r.handleGetOnboardingDeepLink)))

	// AI Patrol routes for background monitoring
	// Note: Status remains accessible so UI can show license/upgrade state
	// Read endpoints (findings, history, runs) return redacted preview data when unlicensed
	// Mutation endpoints (run, acknowledge, dismiss, etc.) return 402 to prevent unauthorized actions
	// SECURITY: Patrol status and stream require ai:execute scope to access findings
	r.mux.HandleFunc("/api/ai/patrol/status", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatrolStatus)))
	r.mux.HandleFunc("/api/ai/patrol/stream", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandlePatrolStream)))
	r.mux.HandleFunc("/api/ai/patrol/findings", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetPatrolFindings(w, req)
		case http.MethodDelete:
			// Clear all findings - doesn't require Pro license so users can clean up accumulated findings
			r.aiSettingsHandler.HandleClearAllFindings(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	// SECURITY: AI Patrol read endpoints - require ai:execute scope
	r.mux.HandleFunc("/api/ai/patrol/history", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetFindingsHistory)))
	r.mux.HandleFunc("/api/ai/patrol/run", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleForcePatrol)))
	// SECURITY: AI Patrol mutation endpoints - require ai:execute scope to prevent low-privilege tokens from
	// dismissing, suppressing, or otherwise hiding findings. This prevents attackers from blinding AI Patrol.
	r.mux.HandleFunc("/api/ai/patrol/acknowledge", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleAcknowledgeFinding)))
	// Dismiss and resolve don't require Pro license - users should be able to clear findings they can see
	// This is especially important for users who accumulated findings before fixing the patrol-without-AI bug
	r.mux.HandleFunc("/api/ai/patrol/dismiss", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleDismissFinding)))
	r.mux.HandleFunc("/api/ai/patrol/findings/note", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleSetFindingNote)))
	r.mux.HandleFunc("/api/ai/patrol/suppress", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleSuppressFinding)))
	r.mux.HandleFunc("/api/ai/patrol/snooze", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleSnoozeFinding)))
	r.mux.HandleFunc("/api/ai/patrol/resolve", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleResolveFinding)))
	r.mux.HandleFunc("/api/ai/patrol/runs", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatrolRunHistory)))
	// Suppression rules management - require scope to prevent low-privilege tokens from creating suppression rules
	r.mux.HandleFunc("/api/ai/patrol/suppressions", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetSuppressionRules(w, req)
		case http.MethodPost:
			r.aiSettingsHandler.HandleAddSuppressionRule(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/ai/patrol/suppressions/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleDeleteSuppressionRule)))
	r.mux.HandleFunc("/api/ai/patrol/dismissed", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetDismissedFindings)))

	// Patrol Autonomy - monitor/approval free, assisted/full require Pro (enforced in handlers)
	r.mux.HandleFunc("/api/ai/patrol/autonomy", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetPatrolAutonomy(w, req)
		case http.MethodPut:
			r.aiSettingsHandler.HandleUpdatePatrolAutonomy(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Investigation endpoints - viewing and reinvestigation are free, fix execution (reapprove) requires Pro
	// SECURITY: Require ai:execute scope to prevent low-privilege tokens from reading investigation details
	r.mux.HandleFunc("/api/ai/findings/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		switch {
		case strings.HasSuffix(path, "/investigation/messages"):
			r.aiSettingsHandler.HandleGetInvestigationMessages(w, req)
		case strings.HasSuffix(path, "/investigation"):
			r.aiSettingsHandler.HandleGetInvestigation(w, req)
		case strings.HasSuffix(path, "/reinvestigate"):
			r.aiSettingsHandler.HandleReinvestigateFinding(w, req)
		case strings.HasSuffix(path, "/reapprove"):
			// Fix execution requires Pro license
			RequireLicenseFeature(r.licenseHandlers, license.FeatureAIAutoFix, r.aiSettingsHandler.HandleReapproveInvestigationFix)(w, req)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})))

	// AI Intelligence endpoints - expose learned patterns, correlations, and predictions
	// SECURITY: Require ai:execute scope to prevent low-privilege tokens from reading sensitive intelligence
	// Unified intelligence endpoint - aggregates all AI subsystems into a single view
	r.mux.HandleFunc("/api/ai/intelligence", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetIntelligence)))
	// Individual sub-endpoints for specific intelligence layers
	r.mux.HandleFunc("/api/ai/intelligence/patterns", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPatterns)))
	r.mux.HandleFunc("/api/ai/intelligence/predictions", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetPredictions)))
	r.mux.HandleFunc("/api/ai/intelligence/correlations", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetCorrelations)))
	r.mux.HandleFunc("/api/ai/intelligence/changes", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRecentChanges)))
	r.mux.HandleFunc("/api/ai/intelligence/baselines", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetBaselines)))
	r.mux.HandleFunc("/api/ai/intelligence/remediations", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRemediations)))
	r.mux.HandleFunc("/api/ai/intelligence/anomalies", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetAnomalies)))
	r.mux.HandleFunc("/api/ai/intelligence/learning", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetLearningStatus)))
	// Unified findings endpoint (alerts + AI findings)
	r.mux.HandleFunc("/api/ai/unified/findings", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetUnifiedFindings)))

	// Phase 6: AI Intelligence Services
	r.mux.HandleFunc("/api/ai/forecast", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetForecast)))
	r.mux.HandleFunc("/api/ai/forecasts/overview", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetForecastOverview)))
	r.mux.HandleFunc("/api/ai/learning/preferences", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetLearningPreferences)))
	r.mux.HandleFunc("/api/ai/proxmox/events", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetProxmoxEvents)))
	r.mux.HandleFunc("/api/ai/proxmox/correlations", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetProxmoxCorrelations)))
	// SECURITY: Remediation endpoints require ai:execute scope to prevent unauthorized access to remediation plans
	r.mux.HandleFunc("/api/ai/remediation/plans", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetRemediationPlans(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/ai/remediation/plan", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRemediationPlan)))
	// Approving a remediation plan is a mutation - keep with ai:execute scope
	r.mux.HandleFunc("/api/ai/remediation/approve", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleApproveRemediationPlan)))
	r.mux.HandleFunc("/api/ai/remediation/execute", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleExecuteRemediationPlan)))
	r.mux.HandleFunc("/api/ai/remediation/rollback", RequireAdmin(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleRollbackRemediationPlan)))
	// SECURITY: Circuit breaker status could reveal operational state - require ai:execute scope
	r.mux.HandleFunc("/api/ai/circuit/status", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetCircuitBreakerStatus)))

	// Phase 7: Incident Recording API - require ai:execute scope to protect incident data
	r.mux.HandleFunc("/api/ai/incidents", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetRecentIncidents)))
	r.mux.HandleFunc("/api/ai/incidents/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleGetIncidentData)))

	// AI Chat Sessions - sync across devices (legacy endpoints)
	r.mux.HandleFunc("/api/ai/chat/sessions", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiSettingsHandler.HandleListAIChatSessions)))
	r.mux.HandleFunc("/api/ai/chat/sessions/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetAIChatSession(w, req)
		case http.MethodPut:
			r.aiSettingsHandler.HandleSaveAIChatSession(w, req)
		case http.MethodDelete:
			r.aiSettingsHandler.HandleDeleteAIChatSession(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// AI chat endpoints
	r.mux.HandleFunc("/api/ai/status", RequireAuth(r.config, r.aiHandler.HandleStatus))
	r.mux.HandleFunc("/api/ai/chat", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.aiHandler.HandleChat)))
	r.mux.HandleFunc("/api/ai/sessions", RequireAuth(r.config, RequireScope(config.ScopeAIChat, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiHandler.HandleSessions(w, req)
		case http.MethodPost:
			r.aiHandler.HandleCreateSession(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	r.mux.HandleFunc("/api/ai/sessions/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.routeAISessions)))

	// AI approval endpoints - for command approval workflow
	// Require ai:execute scope to prevent low-privilege tokens from enumerating or denying approvals
	r.mux.HandleFunc("/api/ai/approvals", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.aiSettingsHandler.HandleListApprovals)))
	r.mux.HandleFunc("/api/ai/approvals/", RequireAuth(r.config, RequireScope(config.ScopeAIExecute, r.routeApprovals)))

	// AI question endpoints - require ai:chat scope for interactive AI features
	r.mux.HandleFunc("/api/ai/question/", RequireAuth(r.config, RequireScope(config.ScopeAIChat, r.routeQuestions)))
}

func (r *Router) registerOrgLicenseRoutes(orgHandlers *OrgHandlers, rbacHandlers *RBACHandlers, auditHandlers *AuditHandlers) {
	// License routes (Pulse Pro)
	r.mux.HandleFunc("/api/license/status", RequireAdmin(r.config, r.licenseHandlers.HandleLicenseStatus))
	r.mux.HandleFunc("/api/license/features", RequireAuth(r.config, r.licenseHandlers.HandleLicenseFeatures))
	r.mux.HandleFunc("/api/license/activate", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleActivateLicense)))
	r.mux.HandleFunc("/api/license/clear", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleClearLicense)))

	// Organization routes (multi-tenant foundation)
	r.mux.HandleFunc("GET /api/orgs", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListOrgs)))
	r.mux.HandleFunc("POST /api/orgs", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleCreateOrg)))
	r.mux.HandleFunc("GET /api/orgs/{id}", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleGetOrg)))
	r.mux.HandleFunc("PUT /api/orgs/{id}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleUpdateOrg)))
	r.mux.HandleFunc("DELETE /api/orgs/{id}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleDeleteOrg)))
	r.mux.HandleFunc("GET /api/orgs/{id}/members", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListMembers)))
	r.mux.HandleFunc("POST /api/orgs/{id}/members", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleInviteMember)))
	r.mux.HandleFunc("DELETE /api/orgs/{id}/members/{userId}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleRemoveMember)))
	r.mux.HandleFunc("GET /api/orgs/{id}/shares", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListShares)))
	r.mux.HandleFunc("GET /api/orgs/{id}/shares/incoming", RequireAuth(r.config, RequireScope(config.ScopeSettingsRead, orgHandlers.HandleListIncomingShares)))
	r.mux.HandleFunc("POST /api/orgs/{id}/shares", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleCreateShare)))
	r.mux.HandleFunc("DELETE /api/orgs/{id}/shares/{shareId}", RequireAuth(r.config, RequireScope(config.ScopeSettingsWrite, orgHandlers.HandleDeleteShare)))

	// Audit log routes (Enterprise feature)
	r.mux.HandleFunc("GET /api/audit", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))))
	r.mux.HandleFunc("GET /api/audit/", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))))
	r.mux.HandleFunc("GET /api/audit/{id}/verify", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleVerifyAuditEvent))))

	// RBAC routes (Phase 2 - Enterprise feature)
	r.mux.HandleFunc("/api/admin/roles", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleRoles)))
	r.mux.HandleFunc("/api/admin/roles/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleRoles)))
	r.mux.HandleFunc("/api/admin/users", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleGetUsers)))
	r.mux.HandleFunc("/api/admin/users/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleUserRoleActions)))

	// Advanced Reporting routes
	r.mux.HandleFunc("/api/admin/reports/generate", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, RequireLicenseFeature(r.licenseHandlers, license.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateReport))))
	r.mux.HandleFunc("/api/admin/reports/generate-multi", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, RequireLicenseFeature(r.licenseHandlers, license.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateMultiReport))))

	// Audit Webhook routes
	r.mux.HandleFunc("/api/admin/webhooks/audit", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			RequireScope(config.ScopeSettingsRead, auditHandlers.HandleGetWebhooks)(w, req)
		} else {
			RequireScope(config.ScopeSettingsWrite, auditHandlers.HandleUpdateWebhooks)(w, req)
		}
	})))
}
