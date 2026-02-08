import { createSignal, onCleanup, batch } from 'solid-js';
import { createStore, produce, reconcile } from 'solid-js/store';
import type {
  State,
  WSMessage,
  Alert,
  ResolvedAlert,
  PVEBackups,
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
import { syncWithHostCommand } from './containerUpdates';

// --- Helpers to avoid repeating the same logic for dockerHosts / hosts / k8s ---

/** Normalize tags from comma-separated string, array, or null → clean string array. */
function normalizeTags<T extends { tags?: unknown }>(items: T[]): (T & { tags: string[] })[] {
  return items.map((item) => {
    const raw = item.tags;
    let tags: string[];
    if (raw && typeof raw === 'string' && raw.trim()) {
      tags = raw.split(',').map((t: string) => t.trim()).filter((t: string) => t.length > 0);
    } else if (Array.isArray(raw)) {
      tags = raw.filter((t: string) => typeof t === 'string' && t.trim().length > 0);
    } else {
      tags = [];
    }
    return { ...item, tags };
  });
}

/** Merge token-revocation state from previous entries into an incoming array. */
interface RevocableEntity {
  id: string;
  tokenId?: string;
  tokenLastUsedAt?: number;
  revokedTokenId?: string;
  tokenRevokedAt?: number;
}

function mergeRevocations<T extends RevocableEntity>(incoming: T[], existing: T[]): T[] {
  if (!Array.isArray(incoming) || incoming.length === 0) return incoming;
  if (!Array.isArray(existing) || existing.length === 0) return incoming;

  return incoming.map((item) => {
    const prev = existing.find((e) => e.id === item.id);
    if (!prev?.tokenRevokedAt || !prev.revokedTokenId) return item;

    const tokenChanged = prev.revokedTokenId && item.tokenId && item.tokenId !== prev.revokedTokenId;
    const usedAfterRevoke = typeof item.tokenLastUsedAt === 'number' && item.tokenLastUsedAt >= prev.tokenRevokedAt;
    if (tokenChanged || usedAfterRevoke) return item;

    return { ...item, revokedTokenId: prev.revokedTokenId, tokenRevokedAt: prev.tokenRevokedAt };
  });
}

/** Tracker for transient-empty payload protection (dockerHosts, hosts, k8sClusters). */
interface TransientEmptyTracker {
  consecutiveEmpty: number;
  hasReceivedNonEmpty: boolean;
}
function createTracker(): TransientEmptyTracker {
  return { consecutiveEmpty: 0, hasReceivedNonEmpty: false };
}

interface Staleable { lastSeen?: number }

/** Returns true if the (possibly empty) incoming array should be applied to state. */
function shouldApplyPayload<T extends Staleable>(
  incoming: T[], existing: T[], tracker: TransientEmptyTracker, isInitial: boolean,
): boolean {
  if (incoming.length > 0) {
    tracker.consecutiveEmpty = 0;
    tracker.hasReceivedNonEmpty = true;
    return true;
  }
  tracker.consecutiveEmpty += 1;
  const now = Date.now();
  const staleMs = 60_000;
  const allStale = existing.length === 0 || existing.every((e) => !e.lastSeen || (now - e.lastSeen) > staleMs);
  return !tracker.hasReceivedNonEmpty || allStale || tracker.consecutiveEmpty >= 3 || isInitial;
}

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  let wsUrl = url;
  const [connected, setConnected] = createSignal(false);
  const [reconnecting, setReconnecting] = createSignal(false);
  const [initialDataReceived, setInitialDataReceived] = createSignal(false);
  const createInitialState = (): State => ({
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
  const [state, setState] = createStore<State>(createInitialState());
  const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
  const [recentlyResolved, setRecentlyResolved] = createStore<Record<string, ResolvedAlert>>({});
  const [updateProgress, setUpdateProgress] = createSignal<unknown>(null);

  // Transient-empty payload protection for dockerHosts, hosts, k8sClusters.
  // Prevents UI flapping when a single empty array is received transiently. See #773.
  const dockerTracker = createTracker();
  const hostTracker = createTracker();
  const k8sTracker = createTracker();

  const mergeDockerHostRevocations = (incoming: DockerHost[]) =>
    mergeRevocations(incoming, state.dockerHosts || []);

  const mergeHostRevocations = (incoming: Host[]) =>
    mergeRevocations(incoming, state.hosts || []);

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
          ws = new WebSocket(wsUrl);
          setupWebSocket();
        }, delay);
        return;
      }

      ws = new WebSocket(wsUrl);
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
      Object.assign(dockerTracker, createTracker());
      Object.assign(hostTracker, createTracker());
      Object.assign(k8sTracker, createTracker());

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

            }
            if (message.data.vms !== undefined) {
              setState('vms', reconcile(normalizeTags(message.data.vms), { key: 'id' }));
            }
            if (message.data.containers !== undefined) {
              setState('containers', reconcile(normalizeTags(message.data.containers), { key: 'id' }));
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

            // --- dockerHosts + hosts (batched to prevent UI flapping, see #778) ---
            const isInitial = message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE;
            let processedDockerHosts: DockerHost[] | null = null;
            let shouldApplyDockerHosts = false;

            if (hasDockerHostsUpdate && Array.isArray(message.data.dockerHosts)) {
              const incoming = message.data.dockerHosts;
              shouldApplyDockerHosts = shouldApplyPayload(incoming, state.dockerHosts || [], dockerTracker, isInitial);
              if (shouldApplyDockerHosts) {
                processedDockerHosts = mergeDockerHostRevocations(incoming);
              }
            }

            let processedHosts: Host[] | null = null;
            let shouldApplyHosts = false;

            if (hasHostsUpdate && Array.isArray(message.data.hosts)) {
              const incoming = message.data.hosts;
              shouldApplyHosts = shouldApplyPayload(incoming, state.hosts || [], hostTracker, isInitial);
              if (shouldApplyHosts) {
                processedHosts = mergeHostRevocations(incoming);
              }
            }

            // Batch both updates atomically when both are present to prevent badge flapping
            if (shouldApplyDockerHosts && shouldApplyHosts) {
              batch(() => {
                setState('dockerHosts', reconcile(processedDockerHosts!, { key: 'id' }));
                setState('hosts', reconcile(processedHosts!, { key: 'id' }));
              });
            } else {
              if (shouldApplyDockerHosts && processedDockerHosts !== null) {
                setState('dockerHosts', reconcile(processedDockerHosts, { key: 'id' }));
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

            // --- Kubernetes clusters (same transient-empty protection) ---
            if (message.data.kubernetesClusters !== undefined && Array.isArray(message.data.kubernetesClusters)) {
              const incoming = message.data.kubernetesClusters as KubernetesCluster[];
              if (shouldApplyPayload(incoming, state.kubernetesClusters || [], k8sTracker, isInitial)) {
                setState('kubernetesClusters', reconcile(incoming, { key: 'id' }));
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
        } else if ((message as {type: string}).type === 'ai_discovery_progress') {
          // AI-powered discovery progress update
          eventBus.emit('ai_discovery_progress', (message as {data: unknown}).data as import('../types/discovery').DiscoveryProgress);
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

  const markTokenRevoked = (key: 'dockerHosts' | 'hosts', tokenId: string, hostIds: string[]) => {
    if (!hostIds || hostIds.length === 0) return;
    const timestamp = Date.now();
    setState(key, produce((draft: any[]) => {
      if (!Array.isArray(draft)) return;
      hostIds.forEach((hostId) => {
        const target = draft.find((h: RevocableEntity) => h.id === hostId);
        if (target) {
          target.revokedTokenId = tokenId;
          target.tokenRevokedAt = timestamp;
        }
      });
    }));
  };

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
    switchUrl: (nextUrl: string) => {
      if (!nextUrl || nextUrl === wsUrl) {
        return;
      }

      wsUrl = nextUrl;
      batch(() => {
        setConnected(false);
        setReconnecting(false);
        setInitialDataReceived(false);
        setUpdateProgress(null);
        setState(reconcile(createInitialState()));
        setActiveAlerts(reconcile({}));
        setRecentlyResolved(reconcile({}));
      });

      window.clearTimeout(reconnectTimeout);
      reconnectAttempt = 0;
      isReconnecting = false;
      ws?.close(1000, 'Reconnecting');
      connect();
    },
    markDockerHostsTokenRevoked: (tokenId: string, hostIds: string[]) =>
      markTokenRevoked('dockerHosts', tokenId, hostIds),
    markHostsTokenRevoked: (tokenId: string, hostIds: string[]) =>
      markTokenRevoked('hosts', tokenId, hostIds),
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
