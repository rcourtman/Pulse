package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func (r *Router) registerMonitoringResourceRoutes(
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
	r.mux.HandleFunc("/api/charts/workloads-summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleWorkloadsSummaryCharts)))
	r.mux.HandleFunc("/api/metrics-store/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleMetricsStoreStats)))
	r.mux.HandleFunc("/api/metrics-store/history", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleMetricsHistory)))
	r.mux.HandleFunc("/api/recovery/points", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.recoveryHandlers.HandleListPoints)))
	r.mux.HandleFunc("/api/recovery/series", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.recoveryHandlers.HandleListSeries)))
	r.mux.HandleFunc("/api/recovery/facets", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.recoveryHandlers.HandleListFacets)))
	r.mux.HandleFunc("/api/recovery/rollups", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.recoveryHandlers.HandleListRollups)))

	// Unified resources API
	r.mux.HandleFunc("/api/resources", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleListResources)))
	r.mux.HandleFunc("/api/resources/storage-incidents", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleStorageIncidents)))
	r.mux.HandleFunc("/api/resources/storage-summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleStorageSummary)))
	r.mux.HandleFunc("/api/resources/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleStats)))
	r.mux.HandleFunc("/api/resources/k8s/namespaces", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleK8sNamespaces)))
	r.mux.HandleFunc("/api/resources/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleResourceRoutes)))
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

	// Docker runtime metadata routes (for custom display names, URLs, notes).
	r.mux.HandleFunc("/api/docker/runtimes/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, dockerMetadataHandler.HandleGetRuntimeMetadata)))
	r.mux.HandleFunc("/api/docker/runtimes/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			dockerMetadataHandler.HandleGetRuntimeMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleUpdateRuntimeMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleDeleteRuntimeMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Agent metadata routes (v6 canonical naming).
	handleAgentMetadataWriteRoute := func(w http.ResponseWriter, req *http.Request) {
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
	}
	r.mux.HandleFunc("/api/agents/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, hostMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/agents/metadata/", RequireAuth(r.config, handleAgentMetadataWriteRoute))

	// Infrastructure update detection routes (Docker containers, packages, etc.)
	r.mux.HandleFunc("/api/infra-updates", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, infraUpdateHandlers.HandleGetInfraUpdates)))
	r.mux.HandleFunc("/api/infra-updates/summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, infraUpdateHandlers.HandleGetInfraUpdatesSummary)))
	r.mux.HandleFunc("/api/infra-updates/check", RequireAuth(r.config, RequireScope(config.ScopeMonitoringWrite, infraUpdateHandlers.HandleTriggerInfraUpdateCheck)))
	r.mux.HandleFunc("/api/infra-updates/agent/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, req *http.Request) {
		// Extract agent ID from path: /api/infra-updates/agent/{agentId}
		agentID := strings.TrimPrefix(req.URL.Path, "/api/infra-updates/agent/")
		agentID = strings.TrimSuffix(agentID, "/")
		if agentID == "" {
			writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
			return
		}
		infraUpdateHandlers.HandleGetInfraUpdatesForAgent(w, req, agentID)
	})))
	r.mux.HandleFunc("/api/infra-updates/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, req *http.Request) {
		// Extract resource ID from path: /api/infra-updates/{resourceId}
		resourceID := strings.TrimPrefix(req.URL.Path, "/api/infra-updates/")
		resourceID = strings.TrimSuffix(resourceID, "/")
		if resourceID == "" || resourceID == "summary" || resourceID == "check" || strings.HasPrefix(resourceID, "agent/") {
			// Let specific handlers deal with these
			http.NotFound(w, req)
			return
		}
		infraUpdateHandlers.HandleGetInfraUpdateForResource(w, req, resourceID)
	})))
	r.mux.HandleFunc("/api/discover", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleDiscoverServers)))
	// Alert routes enforce read/write scopes inside HandleAlerts per endpoint method.
	r.mux.HandleFunc("/api/alerts/", RequireAuth(r.config, r.alertHandlers.HandleAlerts))

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
	handleDiscoveryAgentRoute := func(pathPrefix string) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			// Route based on method and path depth:
			// GET /api/discovery/agent/{agentId} → list discoveries for an agent
			// GET /api/discovery/agent/{agentId}/{resourceId} → get specific discovery
			// GET /api/discovery/agent/{agentId}/{resourceId}/progress → get scan progress
			// POST /api/discovery/agent/{agentId}/{resourceId} → trigger discovery
			// PUT /api/discovery/agent/{agentId}/{resourceId}/notes → update notes
			// DELETE /api/discovery/agent/{agentId}/{resourceId} → delete discovery
			path := strings.TrimPrefix(req.URL.Path, pathPrefix)
			pathParts := strings.Split(strings.TrimSuffix(path, "/"), "/")

			switch req.Method {
			case http.MethodGet:
				if !ensureScope(w, req, config.ScopeMonitoringRead) {
					return
				}
				if len(pathParts) == 1 && pathParts[0] != "" {
					// GET /api/discovery/agent/{id} → list by agent
					r.discoveryHandlers.HandleListByAgent(w, req)
				} else if len(pathParts) >= 2 {
					if strings.HasSuffix(req.URL.Path, "/progress") {
						r.discoveryHandlers.HandleGetProgress(w, req)
					} else {
						// GET /api/discovery/agent/{agentId}/{resourceId} → get specific discovery
						r.discoveryHandlers.HandleGetDiscovery(w, req)
					}
				} else {
					http.Error(w, "Invalid path", http.StatusBadRequest)
				}
			case http.MethodPost:
				if !ensureScope(w, req, config.ScopeMonitoringWrite) {
					return
				}
				// POST /api/discovery/agent/{agentId}/{resourceId} → trigger discovery
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
		}
	}
	r.mux.HandleFunc("/api/discovery/agent/", RequireAuth(r.config, handleDiscoveryAgentRoute("/api/discovery/agent/")))
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
