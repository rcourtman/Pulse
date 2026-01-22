import { createSignal, onCleanup, batch } from 'solid-js';
import { createStore, produce, reconcile } from 'solid-js/store';
import type {
  State,
  WSMessage,
  Alert,
  ResolvedAlert,
  PVEBackups,
  VM,
  Container,
  DockerHost,
  Host,
  KubernetesCluster,
  RemovedDockerHost,
  RemovedKubernetesCluster,
} from '@/types/api';
import type { ActivationState as ActivationStateType } from '@/types/alerts';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';
import { notificationStore } from './notifications';
import { eventBus } from './events';
import { ALERTS_ACTIVATION_EVENT, isAlertsActivationEnabled } from '@/utils/alertsActivation';
import { pruneMetricsByPrefix } from './metricsHistory';
import { getMetricKeyPrefix } from '@/utils/metricsKeys';
import { syncWithHostCommand } from './containerUpdates';

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  const [connected, setConnected] = createSignal(false);
  const [reconnecting, setReconnecting] = createSignal(false);
  const [initialDataReceived, setInitialDataReceived] = createSignal(false);
  const [state, setState] = createStore<State>({
    nodes: [],
    vms: [],
    containers: [],
    dockerHosts: [],
    removedDockerHosts: [],
    kubernetesClusters: [],
    removedKubernetesClusters: [],
    hosts: [],
    replicationJobs: [],
    storage: [],
    cephClusters: [],
    physicalDisks: [],
    pbs: [],
    pmg: [],
    metrics: [],
    pveBackups: {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: [],
    } as PVEBackups,
    pbsBackups: [],
    pmgBackups: [],
    backups: {
      pve: {
        backupTasks: [],
        storageBackups: [],
        guestSnapshots: [],
      },
      pbs: [],
      pmg: [],
    },
    performance: {
      apiCallDuration: {},
      lastPollDuration: 0,
      pollingStartTime: '',
      totalApiCalls: 0,
      failedApiCalls: 0,
      cacheHits: 0,
      cacheMisses: 0,
    },
    connectionHealth: {},
    stats: {
      startTime: new Date().toISOString(),
      uptime: 0,
      pollingCycles: 0,
      webSocketClients: 0,
      version: '2.0.0',
    },
    activeAlerts: [],
    recentlyResolved: [],
    lastUpdate: '',
    // Unified resources for cross-platform monitoring
    resources: [],
  });
  const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
  const [recentlyResolved, setRecentlyResolved] = createStore<Record<string, ResolvedAlert>>({});
  const [updateProgress, setUpdateProgress] = createSignal<unknown>(null);

  // Track consecutive empty dockerHost payloads so we can tolerate transient
  // blanks without spamming the UI.
  let consecutiveEmptyDockerUpdates = 0;
  let hasReceivedNonEmptyDockerHosts = false;

  // Track consecutive empty hosts payloads (same protection as dockerHosts)
  // This prevents "Host" type badge from disappearing when transient empty
  // hosts arrays are received. See #773.
  let consecutiveEmptyHostUpdates = 0;
  let hasReceivedNonEmptyHosts = false;

  // Track consecutive empty Kubernetes clusters payloads (same protection as dockerHosts/hosts)
  // This prevents clusters from disappearing when transient empty arrays are received.
  let consecutiveEmptyK8sUpdates = 0;
  let hasReceivedNonEmptyK8sClusters = false;

  const mergeDockerHostRevocations = (incomingHosts: DockerHost[]) => {
    if (!Array.isArray(incomingHosts) || incomingHosts.length === 0) {
      return incomingHosts;
    }

    const existingHosts = state.dockerHosts || [];
    if (!Array.isArray(existingHosts) || existingHosts.length === 0) {
      return incomingHosts;
    }

    return incomingHosts.map((host) => {
      const previous = existingHosts.find((entry) => entry.id === host.id);
      if (!previous?.tokenRevokedAt || !previous.revokedTokenId) {
        return host;
      }

      const tokenChanged =
        previous.revokedTokenId &&
        host.tokenId &&
        host.tokenId !== previous.revokedTokenId;
      const tokenUsedAfterRevocation =
        typeof host.tokenLastUsedAt === 'number' &&
        host.tokenLastUsedAt >= previous.tokenRevokedAt;

      if (tokenChanged || tokenUsedAfterRevocation) {
        return host;
      }

      return {
        ...host,
        revokedTokenId: previous.revokedTokenId,
        tokenRevokedAt: previous.tokenRevokedAt,
      };
    });
  };

  const mergeHostRevocations = (incomingHosts: Host[]) => {
    if (!Array.isArray(incomingHosts) || incomingHosts.length === 0) {
      return incomingHosts;
    }

    const existingHosts = state.hosts || [];
    if (!Array.isArray(existingHosts) || existingHosts.length === 0) {
      return incomingHosts;
    }

    return incomingHosts.map((host) => {
      const previous = existingHosts.find((entry) => entry.id === host.id);
      if (!previous?.tokenRevokedAt || !previous.revokedTokenId) {
        return host;
      }

      const tokenChanged =
        previous.revokedTokenId &&
        host.tokenId &&
        host.tokenId !== previous.revokedTokenId;
      const tokenUsedAfterRevocation =
        typeof host.tokenLastUsedAt === 'number' &&
        host.tokenLastUsedAt >= previous.tokenRevokedAt;

      if (tokenChanged || tokenUsedAfterRevocation) {
        return host;
      }

      return {
        ...host,
        revokedTokenId: previous.revokedTokenId,
        tokenRevokedAt: previous.tokenRevokedAt,
      };
    });
  };

  // Track alerts with pending acknowledgment changes to prevent race conditions
  const pendingAckChanges = new Map<string, { ack: boolean; previousAckTime?: string }>();

  let alertsEnabled = isAlertsActivationEnabled();
  let lastActiveAlertsPayload: Record<string, Alert> = {};

  const applyActiveAlerts = (alertsMap: Record<string, Alert>) => {
    // Remove alerts that no longer exist
    const currentAlertIds = Object.keys(activeAlerts);
    currentAlertIds.forEach((id) => {
      if (!alertsMap[id]) {
        setActiveAlerts(id, undefined as unknown as Alert);
      }
    });

    // Add or update alerts with pending acknowledgment safeguards
    Object.entries(alertsMap).forEach(([id, alert]) => {
      if (pendingAckChanges.has(id)) {
        const pending = pendingAckChanges.get(id)!;

        if (pending.ack) {
          if (!alert.acknowledged) {
            logger.debug(
              `Skipping update for alert ${id} - awaiting server acknowledgment confirmation`,
            );
            return;
          }

          const serverAckTime = alert.ackTime || '';
          const previousAckTime = pending.previousAckTime || '';
          if (serverAckTime === previousAckTime) {
            logger.debug(
              `Server ack time for alert ${id} unchanged (${serverAckTime}); treating as confirmed`,
            );
          }
        } else if (alert.acknowledged) {
          logger.debug(
            `Skipping update for alert ${id} - awaiting server unacknowledge confirmation`,
          );
          return;
        }

        pendingAckChanges.delete(id);
      }

      setActiveAlerts(id, alert);
    });

    setState('activeAlerts', Object.values(alertsMap));
  };

  if (typeof window !== 'undefined') {
    window.addEventListener(
      ALERTS_ACTIVATION_EVENT,
      (event: Event) => {
        const detail = (event as CustomEvent<ActivationStateType | null>).detail;
        alertsEnabled = detail === 'active';
        applyActiveAlerts(alertsEnabled ? lastActiveAlertsPayload : {});
      },
      { passive: true },
    );
  }

  let ws: WebSocket | null = null;
  let reconnectTimeout: number;
  let heartbeatInterval: number;
  let reconnectAttempt = 0;
  let isReconnecting = false;
  const maxReconnectDelay = POLLING_INTERVALS.RECONNECT_MAX;
  const initialReconnectDelay = POLLING_INTERVALS.RECONNECT_BASE;
  const heartbeatIntervalMs = 30000; // Send heartbeat every 30 seconds

  const connect = () => {
    try {
      // Close existing connection if any
      if (ws) {
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
          ws.close(1000, 'Reconnecting');
        }
        ws = null;
      }

      // Add a small delay before reconnecting to avoid rapid reconnect loops
      if (reconnectAttempt > 0) {
        const delay = Math.min(100 * reconnectAttempt, 1000);
        setTimeout(() => {
          ws = new WebSocket(url);
          setupWebSocket();
        }, delay);
        return;
      }

      ws = new WebSocket(url);
      setupWebSocket();
    } catch (err) {
      logger.error('Failed to create WebSocket', err);
      handleReconnect();
    }
  };

  const handleReconnect = () => {
    if (isReconnecting) return;

    isReconnecting = true;
    setReconnecting(true);

    // Clear any existing timeout
    if (reconnectTimeout) {
      window.clearTimeout(reconnectTimeout);
      reconnectTimeout = 0;
    }

    // Calculate exponential backoff delay
    const delay = Math.min(
      initialReconnectDelay * Math.pow(2, reconnectAttempt),
      maxReconnectDelay,
    );

    logger.info(`Reconnecting in ${delay}ms (attempt ${reconnectAttempt + 1})`);
    reconnectAttempt++;

    reconnectTimeout = window.setTimeout(() => {
      isReconnecting = false;
      connect();
    }, delay);
  };

  const setupWebSocket = () => {
    if (!ws) return;

    ws.onopen = () => {
      logger.debug('connect');
      const wasReconnecting = reconnectAttempt > 0;
      setConnected(true);
      setReconnecting(false); // Clear reconnecting state
      reconnectAttempt = 0; // Reset reconnect attempts on successful connection
      isReconnecting = false;
      consecutiveEmptyDockerUpdates = 0;
      hasReceivedNonEmptyDockerHosts = false;
      consecutiveEmptyHostUpdates = 0;
      hasReceivedNonEmptyHosts = false;
      consecutiveEmptyK8sUpdates = 0;
      hasReceivedNonEmptyK8sClusters = false;

      // Start heartbeat to keep connection alive
      if (heartbeatInterval) {
        window.clearInterval(heartbeatInterval);
      }
      heartbeatInterval = window.setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping', data: { timestamp: Date.now() } }));
        }
      }, heartbeatIntervalMs);

      // Emit reconnection event so App can refresh alert config
      // This ensures the alert activation state is re-fetched after connection loss
      if (wasReconnecting) {
        logger.info('WebSocket reconnected, emitting event for config refresh');
        eventBus.emit('websocket_reconnected');
      }

      // Alerts will come with the initial state broadcast
    };


    ws.onmessage = (event) => {
      let data;
      try {
        data = JSON.parse(event.data);
      } catch (parseError) {
        logger.error('Failed to parse WebSocket message', parseError);
        return;
      }

      try {
        const message: WSMessage = data;

        if (
          message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE ||
          message.type === WEBSOCKET.MESSAGE_TYPES.RAW_DATA
        ) {
          // Update state properties individually, but batch the whole payload to
          // reduce reactive recomputations and UI thrash on large updates.
          if (message.data) batch(() => {
            // Mark that we've received usable data (initial payload or raw update)
            if (!initialDataReceived()) {
              setInitialDataReceived(true);
            }

            // Only update if we have actual data, don't overwrite with empty arrays
            if (message.data.nodes !== undefined) {
              logger.debug('[WebSocket] Updating nodes', {
                count: message.data.nodes?.length || 0,
              });
              setState('nodes', reconcile(message.data.nodes, { key: 'id' }));

              // Lifecycle cleanup: remove metrics for nodes that disappeared
              const currentIds = new Set(message.data.nodes?.map((n: any) => n.id).filter(Boolean) || []);
              pruneMetricsByPrefix(getMetricKeyPrefix('node'), currentIds);
            }
            if (message.data.vms !== undefined) {
              // Transform tags from comma-separated strings to arrays
              const transformedVMs = message.data.vms.map((vm: VM) => {
                const originalTags = vm.tags;
                let transformedTags: string[];

                if (originalTags && typeof originalTags === 'string' && originalTags.trim()) {
                  // String with content - split into array
                  transformedTags = originalTags
                    .split(',')
                    .map((t: string) => t.trim())
                    .filter((t: string) => t.length > 0);
                } else if (Array.isArray(originalTags)) {
                  // Already an array - filter out empty/whitespace-only tags
                  transformedTags = originalTags.filter(
                    (tag: string) => typeof tag === 'string' && tag.trim().length > 0,
                  );
                } else {
                  // null, undefined, empty string, or other - convert to empty array
                  transformedTags = [];
                }

                return {
                  ...vm,
                  tags: transformedTags,
                };
              });
              setState('vms', reconcile(transformedVMs, { key: 'id' }));

              // Lifecycle cleanup: remove metrics for VMs that disappeared
              const vmIds = new Set(transformedVMs.map((vm: VM) => vm.id).filter(Boolean));
              pruneMetricsByPrefix(getMetricKeyPrefix('vm'), vmIds);
            }
            if (message.data.containers !== undefined) {
              // Transform tags from comma-separated strings to arrays
              const transformedContainers = message.data.containers.map((container: Container) => {
                const originalTags = container.tags;
                let transformedTags: string[];

                if (originalTags && typeof originalTags === 'string' && originalTags.trim()) {
                  // String with content - split into array
                  transformedTags = originalTags
                    .split(',')
                    .map((t: string) => t.trim())
                    .filter((t: string) => t.length > 0);
                } else if (Array.isArray(originalTags)) {
                  // Already an array - filter out empty/whitespace-only tags
                  transformedTags = originalTags.filter(
                    (tag: string) => typeof tag === 'string' && tag.trim().length > 0,
                  );
                } else {
                  // null, undefined, empty string, or other - convert to empty array
                  transformedTags = [];
                }

                return {
                  ...container,
                  tags: transformedTags,
                };
              });
              setState('containers', reconcile(transformedContainers, { key: 'id' }));

              // Lifecycle cleanup: remove metrics for containers that disappeared
              const containerIds = new Set(transformedContainers.map((c: Container) => c.id).filter(Boolean));
              pruneMetricsByPrefix(getMetricKeyPrefix('container'), containerIds);
            }
            // Process dockerHosts and hosts together to prevent UI flapping.
            // When a unified agent reports both host and docker data, the UI's
            // allHosts memo depends on both state.hosts and state.dockerHosts.
            // Updating them separately causes brief inconsistent states where
            // the component sees only dockerHosts updated but not hosts yet,
            // making the agent type badge flap between "Docker" and "Host & Docker".
            // Fix: batch both updates into a single setState call. See #778.
            const hasDockerHostsUpdate = message.data.dockerHosts !== undefined && message.data.dockerHosts !== null;
            const hasHostsUpdate = message.data.hosts !== undefined && message.data.hosts !== null;

            // Prepare dockerHosts data if present
            let processedDockerHosts: DockerHost[] | null = null;
            let shouldApplyDockerHosts = false;

            if (hasDockerHostsUpdate) {
              if (Array.isArray(message.data.dockerHosts)) {
                const incomingHosts = message.data.dockerHosts;
                if (incomingHosts.length === 0) {
                  consecutiveEmptyDockerUpdates += 1;

                  // Check if all existing docker hosts are stale (>60s since lastSeen)
                  // If so, they're probably really gone - apply the empty update immediately
                  const now = Date.now();
                  const staleThresholdMs = 60_000; // 60 seconds
                  const existingDockerHosts = state.dockerHosts || [];
                  const allStale = existingDockerHosts.length === 0 || existingDockerHosts.every(
                    (h) => !h.lastSeen || (now - h.lastSeen) > staleThresholdMs
                  );

                  shouldApplyDockerHosts =
                    !hasReceivedNonEmptyDockerHosts ||
                    allStale ||
                    consecutiveEmptyDockerUpdates >= 3 ||
                    message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE;

                  if (shouldApplyDockerHosts) {
                    logger.debug('[WebSocket] Updating dockerHosts', {
                      count: incomingHosts.length,
                      reason: allStale ? 'allStale' : 'threshold',
                    });
                    processedDockerHosts = mergeDockerHostRevocations(incomingHosts);
                  } else {
                    logger.debug('[WebSocket] Skipping transient empty dockerHosts payload', {
                      streak: consecutiveEmptyDockerUpdates,
                    });
                  }
                } else {
                  consecutiveEmptyDockerUpdates = 0;
                  hasReceivedNonEmptyDockerHosts = true;
                  shouldApplyDockerHosts = true;
                  logger.debug('[WebSocket] Updating dockerHosts', {
                    count: incomingHosts.length,
                  });
                  processedDockerHosts = mergeDockerHostRevocations(incomingHosts);
                }
              } else {
                logger.warn('[WebSocket] Received non-array dockerHosts payload', {
                  type: typeof message.data.dockerHosts,
                });
              }
            } else if (message.data.dockerHosts === null) {
              logger.debug('[WebSocket] Received null dockerHosts payload');
            }

            // Prepare hosts data if present (with same transient empty protection as dockerHosts)
            let processedHosts: Host[] | null = null;
            let shouldApplyHosts = false;

            if (hasHostsUpdate) {
              if (Array.isArray(message.data.hosts)) {
                const incomingHosts = message.data.hosts;
                if (incomingHosts.length === 0) {
                  consecutiveEmptyHostUpdates += 1;

                  // Check if all existing hosts are stale (>60s since lastSeen)
                  // If so, they're probably really gone - apply the empty update immediately
                  const now = Date.now();
                  const staleThresholdMs = 60_000; // 60 seconds
                  const existingHosts = state.hosts || [];
                  const allHostsStale = existingHosts.length === 0 || existingHosts.every(
                    (h) => !h.lastSeen || (now - h.lastSeen) > staleThresholdMs
                  );

                  shouldApplyHosts =
                    !hasReceivedNonEmptyHosts ||
                    allHostsStale ||
                    consecutiveEmptyHostUpdates >= 3 ||
                    message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE;

                  if (shouldApplyHosts) {
                    logger.debug('[WebSocket] Updating hosts', {
                      count: incomingHosts.length,
                      reason: allHostsStale ? 'allStale' : 'threshold',
                    });
                    processedHosts = mergeHostRevocations(incomingHosts);
                  } else {
                    logger.debug('[WebSocket] Skipping transient empty hosts payload', {
                      streak: consecutiveEmptyHostUpdates,
                    });
                  }
                } else {
                  consecutiveEmptyHostUpdates = 0;
                  hasReceivedNonEmptyHosts = true;
                  shouldApplyHosts = true;
                  logger.debug('[WebSocket] Updating hosts', {
                    count: incomingHosts.length,
                  });
                  processedHosts = mergeHostRevocations(incomingHosts);
                }
              } else {
                logger.warn('[WebSocket] Received non-array hosts payload', {
                  type: typeof message.data.hosts,
                });
              }
            } else if (message.data.hosts === null) {
              logger.debug('[WebSocket] Received null hosts payload');
            }

            // Apply updates - batch together if both are present to prevent flapping
            if (shouldApplyDockerHosts && shouldApplyHosts) {
              // Both dockerHosts and hosts in this message - batch them atomically
              batch(() => {
                setState('dockerHosts', reconcile(processedDockerHosts!, { key: 'id' }));
                setState('hosts', reconcile(processedHosts!, { key: 'id' }));
              });

              // Lifecycle cleanup for Docker hosts and containers
              const hostIds = new Set(processedDockerHosts!.map((h: DockerHost) => h.id).filter(Boolean));
              const dockerContainerIds = new Set<string>();
              processedDockerHosts!.forEach((h: DockerHost) => {
                h.containers?.forEach((c: any) => {
                  if (c.id) dockerContainerIds.add(c.id);
                });
              });
              pruneMetricsByPrefix(getMetricKeyPrefix('dockerHost'), hostIds);
              pruneMetricsByPrefix(getMetricKeyPrefix('dockerContainer'), dockerContainerIds);
            } else {
              // Update separately if only one is present
              if (shouldApplyDockerHosts && processedDockerHosts !== null) {
                setState('dockerHosts', reconcile(processedDockerHosts, { key: 'id' }));

                // Lifecycle cleanup for Docker hosts and containers
                const hostIds = new Set(processedDockerHosts.map((h: DockerHost) => h.id).filter(Boolean));
                const dockerContainerIds = new Set<string>();
                processedDockerHosts.forEach((h: DockerHost) => {
                  h.containers?.forEach((c: any) => {
                    if (c.id) dockerContainerIds.add(c.id);
                  });
                });
                pruneMetricsByPrefix(getMetricKeyPrefix('dockerHost'), hostIds);
                pruneMetricsByPrefix(getMetricKeyPrefix('dockerContainer'), dockerContainerIds);
              }
              if (shouldApplyHosts && processedHosts !== null) {
                setState('hosts', reconcile(processedHosts, { key: 'id' }));
              }
            }

            // Sync container update states with host command statuses for real-time progress
            if (shouldApplyDockerHosts && processedDockerHosts !== null) {
              processedDockerHosts.forEach((host: DockerHost) => {
                if (host.command) {
                  syncWithHostCommand(host.id, host.command);
                }
              });
            }
            if (message.data.removedDockerHosts !== undefined) {
              const removed = Array.isArray(message.data.removedDockerHosts)
                ? (message.data.removedDockerHosts as RemovedDockerHost[])
                : [];
              setState('removedDockerHosts', reconcile(removed, { key: 'id' }));
            }
            // Process Kubernetes clusters with transient empty payload protection
            // (same logic as dockerHosts/hosts to prevent UI flapping)
            if (message.data.kubernetesClusters !== undefined) {
              if (Array.isArray(message.data.kubernetesClusters)) {
                const incomingClusters = message.data.kubernetesClusters as KubernetesCluster[];
                if (incomingClusters.length === 0) {
                  consecutiveEmptyK8sUpdates += 1;

                  // Check if all existing clusters are stale (>60s since lastSeen)
                  // If so, they're probably really gone - apply the empty update immediately
                  const now = Date.now();
                  const staleThresholdMs = 60_000; // 60 seconds
                  const existingClusters = state.kubernetesClusters || [];
                  const allStale = existingClusters.length === 0 || existingClusters.every(
                    (c) => !c.lastSeen || (now - c.lastSeen) > staleThresholdMs
                  );

                  const shouldApply =
                    !hasReceivedNonEmptyK8sClusters ||
                    allStale ||
                    consecutiveEmptyK8sUpdates >= 3 ||
                    message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE;

                  if (shouldApply) {
                    logger.debug('[WebSocket] Updating kubernetesClusters', {
                      count: incomingClusters.length,
                      reason: allStale ? 'allStale' : 'threshold',
                    });
                    setState('kubernetesClusters', reconcile(incomingClusters, { key: 'id' }));
                  } else {
                    logger.debug('[WebSocket] Skipping transient empty kubernetesClusters payload', {
                      streak: consecutiveEmptyK8sUpdates,
                    });
                  }
                } else {
                  consecutiveEmptyK8sUpdates = 0;
                  hasReceivedNonEmptyK8sClusters = true;
                  logger.debug('[WebSocket] Updating kubernetesClusters', {
                    count: incomingClusters.length,
                  });
                  setState('kubernetesClusters', reconcile(incomingClusters, { key: 'id' }));
                }
              } else {
                logger.warn('[WebSocket] Received non-array kubernetesClusters payload', {
                  type: typeof message.data.kubernetesClusters,
                });
              }
            }
            if (message.data.removedKubernetesClusters !== undefined) {
              const removed = Array.isArray(message.data.removedKubernetesClusters)
                ? (message.data.removedKubernetesClusters as RemovedKubernetesCluster[])
                : [];
              setState('removedKubernetesClusters', reconcile(removed, { key: 'id' }));
            }
            if (message.data.storage !== undefined) setState('storage', reconcile(message.data.storage, { key: 'id' }));
            if (message.data.cephClusters !== undefined)
              setState('cephClusters', reconcile(message.data.cephClusters, { key: 'id' }));
            if (message.data.pbs !== undefined) setState('pbs', reconcile(message.data.pbs, { key: 'id' }));
            if (message.data.pmg !== undefined) setState('pmg', reconcile(message.data.pmg, { key: 'id' }));
            if (message.data.replicationJobs !== undefined)
              setState('replicationJobs', reconcile(message.data.replicationJobs, { key: 'id' }));
            if (message.data.backups !== undefined) {
              setState('backups', message.data.backups);
              if (message.data.backups.pve !== undefined)
                setState('pveBackups', message.data.backups.pve);
              if (message.data.backups.pbs !== undefined)
                setState('pbsBackups', message.data.backups.pbs);
              if (message.data.backups.pmg !== undefined)
                setState('pmgBackups', message.data.backups.pmg);
            }
            if (message.data.pbsBackups !== undefined)
              setState('pbsBackups', message.data.pbsBackups);
            if (message.data.pmgBackups !== undefined)
              setState('pmgBackups', message.data.pmgBackups);
            if (message.data.metrics !== undefined) setState('metrics', message.data.metrics);
            if (message.data.pveBackups !== undefined)
              setState('pveBackups', message.data.pveBackups);
            if (message.data.performance !== undefined)
              setState('performance', message.data.performance);
            if (message.data.connectionHealth !== undefined)
              setState('connectionHealth', message.data.connectionHealth);
            if (message.data.stats !== undefined) setState('stats', message.data.stats);
            if (message.data.physicalDisks !== undefined)
              setState('physicalDisks', reconcile(message.data.physicalDisks, { key: 'id' }));
            // Handle unified resources
            if (message.data.resources !== undefined) {
              logger.debug('[WebSocket] Updating resources', {
                count: message.data.resources?.length || 0,
                types: [...new Set(message.data.resources?.map((r: any) => r.type) || [])],
              });
              setState('resources', reconcile(message.data.resources, { key: 'id' }));
            }
            // Sync active alerts from state
            if (message.data.activeAlerts !== undefined) {
              const newAlerts: Record<string, Alert> = {};
              if (message.data.activeAlerts && Array.isArray(message.data.activeAlerts)) {
                message.data.activeAlerts.forEach((alert: Alert) => {
                  newAlerts[alert.id] = alert;
                });
              }

              lastActiveAlertsPayload = newAlerts;
              applyActiveAlerts(alertsEnabled ? newAlerts : {});
            }
            // Sync recently resolved alerts
            if (message.data.recentlyResolved !== undefined) {
              // Received recentlyResolved update

              // Update resolved alerts atomically to prevent race conditions
              const newResolvedAlerts: Record<string, ResolvedAlert> = {};
              if (message.data.recentlyResolved && Array.isArray(message.data.recentlyResolved)) {
                message.data.recentlyResolved.forEach((alert: ResolvedAlert) => {
                  newResolvedAlerts[alert.id] = alert;
                });
              }

              // Clear existing resolved alerts and set new ones
              const currentResolvedIds = Object.keys(recentlyResolved);
              currentResolvedIds.forEach((id) => {
                if (!newResolvedAlerts[id]) {
                  setRecentlyResolved(id, undefined as unknown as ResolvedAlert);
                }
              });

              // Add new resolved alerts
              Object.entries(newResolvedAlerts).forEach(([id, alert]) => {
                setRecentlyResolved(id, alert);
              });

              // Updated recentlyResolved
            }
            setState('lastUpdate', message.data.lastUpdate || new Date().toISOString());
          });
          logger.debug('message', {
            type: message.type,
            hasData: !!message.data,
            nodeCount: message.data?.nodes?.length || 0,
            vmCount: message.data?.vms?.length || 0,
            containerCount: message.data?.containers?.length || 0,
          });
        } else if (message.type === WEBSOCKET.MESSAGE_TYPES.ERROR) {
          logger.debug('error', message.error);
        } else if (message.type === 'ping') {
          // Respond to ping with pong
          if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'pong', data: { timestamp: Date.now() } }));
          }
        } else if (message.type === 'pong') {
          // Server acknowledged our ping
          logger.debug('Received pong from server');
        } else if (message.type === 'welcome') {
          // Welcome message from server
          logger.info('WebSocket connection established');
        } else if (message.type === 'alert') {
          // Individual alerts now handled via state sync
          logger.warn('New alert received (will sync with next state update)', message.data);
        } else if (message.type === 'alertResolved') {
          // Individual alert resolution now handled via state sync
          logger.info('Alert resolved (will sync with next state update)', {
            alertId: message.data.alertId,
          });
        } else if (message.type === 'update:progress') {
          // Update progress event
          setUpdateProgress(message.data);
          logger.info('Update progress:', message.data);
        } else if (message.type === 'node_auto_registered') {
          // Node was successfully auto-registered
          // Received node_auto_registered message
          const node = message.data;
          const nodeName = node.name || node.host;
          const nodeType = node.type === 'pve' ? 'Proxmox VE' : 'Proxmox Backup Server';

          notificationStore.success(
            `${nodeType} node "${nodeName}" was successfully auto-registered and is now being monitored!`,
            8000,
          );
          logger.info('Node auto-registered:', node);

          // Emit event to trigger UI updates
          eventBus.emit('node_auto_registered', node);

          // Trigger a refresh of nodes
          eventBus.emit('refresh_nodes');
        } else if (message.type === 'node_deleted' || message.type === 'nodes_changed') {
          // Nodes configuration has changed, refresh the list
          eventBus.emit('refresh_nodes');
        } else if (message.type === 'discovery_update') {
          // Discovery scan completed with new results
          eventBus.emit('discovery_updated', message.data);
        } else if (message.type === 'discovery_started') {
          eventBus.emit('discovery_status', {
            scanning: true,
            subnet: message.data?.subnet,
            timestamp: message.data?.timestamp,
          });
        } else if (message.type === 'discovery_complete') {
          eventBus.emit('discovery_status', {
            scanning: false,
            timestamp: message.data?.timestamp,
          });
        } else if (message.type === 'settingsUpdate') {
          // Settings have been updated (e.g., theme change)
          if (message.data?.theme) {
            // Emit event for theme change
            eventBus.emit('theme_changed', message.data.theme);
            logger.info('Theme update received via WebSocket:', message.data.theme);
          }
        } else {
          // Log any unhandled message types in dev mode only
          if (import.meta.env.DEV) {
            // Silently ignore unhandled message types
          }
        }
      } catch (err) {
        logger.error('Failed to process WebSocket message', err);
      }
    };

    ws.onclose = (event) => {
      logger.debug('disconnect', { code: event.code, reason: event.reason });
      setConnected(false);
      setInitialDataReceived(false);

      // Clear heartbeat interval
      if (heartbeatInterval) {
        window.clearInterval(heartbeatInterval);
        heartbeatInterval = 0;
      }

      // Don't try to reconnect if the close was intentional (code 1000)
      if (event.code === 1000 && event.reason === 'Reconnecting') {
        return;
      }

      // If we get a 1008 (policy violation) close code, it's likely an auth failure
      // Redirect to login page to re-authenticate
      if (event.code === 1008) {
        logger.warn('WebSocket closed due to authentication failure, redirecting to login');
        // Clear auth and reload to trigger login
        if (typeof window !== 'undefined') {
          localStorage.setItem('just_logged_out', 'true');
          window.location.href = '/';
        }
        return;
      }

      handleReconnect();
    };

    ws.onerror = (error) => {
      // Don't log connection errors if we're already connected
      // Browser may show errors for initial connection attempts even after success
      if (!connected()) {
        logger.debug('error', error);
      }
    };
  };

  // Connect immediately
  connect();

  // Cleanup on unmount
  onCleanup(() => {
    window.clearTimeout(reconnectTimeout);
    window.clearInterval(heartbeatInterval);
    if (ws) {
      ws.close(1000, 'Component unmounting');
    }
  });

  return {
    state,
    activeAlerts,
    recentlyResolved,
    connected,
    reconnecting,
    initialDataReceived,
    updateProgress,
    reconnect: () => {
      ws?.close();
      window.clearTimeout(reconnectTimeout);
      reconnectAttempt = 0; // Reset attempts for manual reconnect
      connect();
    },
    markDockerHostsTokenRevoked: (tokenId: string, hostIds: string[]) => {
      if (!hostIds || hostIds.length === 0) {
        return;
      }
      const timestamp = Date.now();
      setState(
        'dockerHosts',
        produce((draft: DockerHost[]) => {
          if (!Array.isArray(draft)) return;
          hostIds.forEach((hostId) => {
            const target = draft.find((host) => host.id === hostId);
            if (target) {
              target.revokedTokenId = tokenId;
              target.tokenRevokedAt = timestamp;
            }
          });
        }),
      );
    },
    markHostsTokenRevoked: (tokenId: string, hostIds: string[]) => {
      if (!hostIds || hostIds.length === 0) {
        return;
      }
      const timestamp = Date.now();
      setState(
        'hosts',
        produce((draft: Host[]) => {
          if (!Array.isArray(draft)) return;
          hostIds.forEach((hostId) => {
            const target = draft.find((host) => host.id === hostId);
            if (target) {
              target.revokedTokenId = tokenId;
              target.tokenRevokedAt = timestamp;
            }
          });
        }),
      );
    },
    removeAlerts: (predicate: (alert: Alert) => boolean) => {
      const keysToRemove: string[] = [];
      Object.entries(activeAlerts).forEach(([alertId, alert]) => {
        if (!alert) {
          keysToRemove.push(alertId);
          return;
        }
        try {
          if (predicate(alert)) {
            pendingAckChanges.delete(alertId);
            keysToRemove.push(alertId);
          }
        } catch (error) {
          logger.error('Failed to evaluate alert removal predicate', error);
        }
      });

      if (keysToRemove.length > 0) {
        setActiveAlerts(
          produce((draft) => {
            keysToRemove.forEach((key) => {
              delete draft[key];
            });
          }),
        );
      }
    },
    // Method to update an alert locally (e.g., after acknowledgment)
    updateAlert: (alertId: string, updates: Partial<Alert>) => {
      const existingAlert = activeAlerts[alertId];
      if (existingAlert) {
        // Track this alert as having pending changes if acknowledgment is changing
        if ('acknowledged' in updates) {
          const previousAckTime = existingAlert.ackTime;
          pendingAckChanges.set(alertId, {
            ack: !!updates.acknowledged,
            previousAckTime,
          });
          // Safety valve: if we never hear back from the server (e.g., request failed silently),
          // clear the pending flag after a generous timeout so we eventually resync with reality.
          setTimeout(() => {
            if (pendingAckChanges.has(alertId)) {
              logger.warn(`Clearing stale pending ack change for alert ${alertId}`);
              pendingAckChanges.delete(alertId);
              notificationStore.error(
                'Server did not confirm the alert acknowledgment in time. Re-syncing from latest data.',
              );
            }
          }, 15000);
        }
        setActiveAlerts(alertId, { ...existingAlert, ...updates });
      }
    },
  };
}
