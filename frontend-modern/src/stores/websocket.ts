import { createSignal, onCleanup, batch } from 'solid-js';
import { createStore, produce, reconcile } from 'solid-js/store';
import type {
  State,
  WSMessage,
  Alert,
  ResolvedAlert,
  RemovedDockerHost,
  RemovedKubernetesCluster,
} from '@/types/api';
import type { ActivationState as ActivationStateType } from '@/types/alerts';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';
import { notificationStore } from './notifications';
import { eventBus } from './events';
import { ALERTS_ACTIVATION_EVENT, isAlertsActivationEnabled } from '@/utils/alertsActivation';
import { syncWithAgentCommand } from './containerUpdates';
import {
  getAgentDiscoveryResourceId,
  isAppContainerDiscoveryResourceType,
} from '@/utils/discoveryTarget';

const MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES = 8 * 1024 * 1024; // 8 MiB

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  let wsUrl = url;
  const [connected, setConnected] = createSignal(false);
  const [reconnecting, setReconnecting] = createSignal(false);
  const [initialDataReceived, setInitialDataReceived] = createSignal(false);
  const createInitialState = (): State => ({
    // Canonical v6 state comes from unified resources.
    removedDockerHosts: [],
    removedKubernetesClusters: [],
    metrics: [],
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

  // Track alerts with pending acknowledgment changes to prevent race conditions
  const pendingAckChanges = new Map<string, { ack: boolean; previousAckTime?: string }>();
  const pendingAckTimeouts = new Map<string, number>();

  let alertsEnabled = isAlertsActivationEnabled();
  let lastActiveAlertsPayload: Record<string, Alert> = {};
  const handleAlertsActivation = (event: Event) => {
    const detail = (event as CustomEvent<ActivationStateType | null>).detail;
    alertsEnabled = detail === 'active';
    applyActiveAlerts(alertsEnabled ? lastActiveAlertsPayload : {});
  };

  const clearPendingAckTimeout = (alertId: string) => {
    const timeout = pendingAckTimeouts.get(alertId);
    if (timeout) {
      window.clearTimeout(timeout);
      pendingAckTimeouts.delete(alertId);
    }
  };

  const clearPendingAck = (alertId: string) => {
    pendingAckChanges.delete(alertId);
    clearPendingAckTimeout(alertId);
  };

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

        clearPendingAck(id);
      }

      setActiveAlerts(id, alert);
    });

    setState('activeAlerts', Object.values(alertsMap));
  };

  const handleAlertsActivationEvent = (event: Event) => {
    const detail = (event as CustomEvent<ActivationStateType | null>).detail;
    alertsEnabled = detail === 'active';
    applyActiveAlerts(alertsEnabled ? lastActiveAlertsPayload : {});
  };

  if (typeof window !== 'undefined') {
    window.addEventListener(ALERTS_ACTIVATION_EVENT, handleAlertsActivationEvent, {
      passive: true,
    });
  }

  let ws: WebSocket | null = null;
  let reconnectTimeout = 0;
  let reconnectDelayTimeout = 0;
  let lastServerActivityAt = Date.now();
  let heartbeatInterval = 0;
  let reconnectAttempt = 0;
  let isReconnecting = false;
  let isDisposed = false;
  const maxReconnectDelay = POLLING_INTERVALS.RECONNECT_MAX;
  const initialReconnectDelay = POLLING_INTERVALS.RECONNECT_BASE;
  const heartbeatIntervalMs = 30000; // Send heartbeat every 30 seconds
  const heartbeatTimeoutMs = heartbeatIntervalMs * 3;
  const reconnectJitterRatio = 0.2;

  const clearHeartbeatTimer = () => {
    if (heartbeatInterval) {
      window.clearInterval(heartbeatInterval);
      heartbeatInterval = 0;
    }
  };

  const computeReconnectDelay = (attempt: number) => {
    const baseDelay = Math.min(initialReconnectDelay * Math.pow(2, attempt), maxReconnectDelay);
    const jitterWindow = Math.floor(baseDelay * reconnectJitterRatio);
    if (jitterWindow <= 0) {
      return baseDelay;
    }

    const jitter = Math.floor(Math.random() * (jitterWindow * 2 + 1)) - jitterWindow;
    return Math.min(maxReconnectDelay, Math.max(0, baseDelay + jitter));
  };

  const clearReconnectTimeout = () => {
    if (reconnectTimeout) {
      window.clearTimeout(reconnectTimeout);
      reconnectTimeout = 0;
    }
  };

  const clearReconnectDelayTimeout = () => {
    if (reconnectDelayTimeout) {
      window.clearTimeout(reconnectDelayTimeout);
      reconnectDelayTimeout = 0;
    }
  };

  const clearHeartbeatInterval = () => {
    if (heartbeatInterval) {
      window.clearInterval(heartbeatInterval);
      heartbeatInterval = 0;
    }
  };

  const shutdown = () => {
    if (isDisposed) return;
    isDisposed = true;
    isReconnecting = false;
    setReconnecting(false);
    clearReconnectTimeout();
    clearReconnectDelayTimeout();
    clearHeartbeatInterval();
    pendingAckTimeouts.forEach((timeout) => window.clearTimeout(timeout));
    pendingAckTimeouts.clear();
    pendingAckChanges.clear();

    if (typeof window !== 'undefined') {
      window.removeEventListener(ALERTS_ACTIVATION_EVENT, handleAlertsActivationEvent);
    }

    if (ws) {
      ws.onopen = null;
      ws.onmessage = null;
      ws.onclose = null;
      ws.onerror = null;
      ws.close(1000, 'Component unmounting');
      ws = null;
    }
  };

  const connect = () => {
    if (isDisposed) return;
    clearReconnectDelayTimeout();

    try {
      // Close existing connection if any
      if (ws) {
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
          ws.close(1000, 'Reconnecting');
        }
        ws = null;
      }

      ws = new WebSocket(wsUrl);
      setupWebSocket();
    } catch (err) {
      logger.error('Failed to create WebSocket', err);
      handleReconnect();
    }
  };

  const handleReconnect = () => {
    if (isDisposed || isReconnecting) return;

    isReconnecting = true;
    setReconnecting(true);

    // Clear any existing timeout
    clearReconnectTimeout();

    // Calculate exponential backoff delay with jitter
    const delay = computeReconnectDelay(reconnectAttempt);

    logger.info(`Reconnecting in ${delay}ms (attempt ${reconnectAttempt + 1})`);
    reconnectAttempt++;

    reconnectTimeout = window.setTimeout(() => {
      if (isDisposed) {
        isReconnecting = false;
        return;
      }
      isReconnecting = false;
      if (isDisposed) return;
      connect();
    }, delay);
  };

  const setupWebSocket = () => {
    if (!ws || isDisposed) return;

    ws.onopen = () => {
      if (isDisposed) return;
      logger.debug('connect');
      const wasReconnecting = reconnectAttempt > 0;
      setConnected(true);
      setReconnecting(false); // Clear reconnecting state
      reconnectAttempt = 0; // Reset reconnect attempts on successful connection
      isReconnecting = false;
      lastServerActivityAt = Date.now();

      // Start heartbeat to keep connection alive
      clearHeartbeatTimer();
      heartbeatInterval = window.setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          const silenceDuration = Date.now() - lastServerActivityAt;
          if (silenceDuration >= heartbeatTimeoutMs) {
            logger.warn('WebSocket heartbeat timeout, forcing reconnect', {
              silenceMs: silenceDuration,
            });
            ws.close(4000, 'Heartbeat timeout');
            return;
          }
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
      if (typeof event.data !== 'string') {
        logger.warn('Ignoring non-text WebSocket payload');
        return;
      }

      const payloadSizeBytes = new Blob([event.data]).size;
      if (payloadSizeBytes > MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES) {
        logger.warn('Ignoring oversized WebSocket payload', {
          sizeBytes: payloadSizeBytes,
          maxBytes: MAX_INBOUND_WEBSOCKET_MESSAGE_BYTES,
        });
        return;
      }

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
          if (message.data)
            batch(() => {
              // Mark that we've received usable data (initial payload or raw update)
              if (!initialDataReceived()) {
                setInitialDataReceived(true);
              }

              // Canonical resource contract:
              // `state.resources` is the authoritative frontend model.
              // Per-type arrays are no longer hydrated here.
              if (message.data.removedDockerHosts !== undefined) {
                const removed = Array.isArray(message.data.removedDockerHosts)
                  ? (message.data.removedDockerHosts as RemovedDockerHost[])
                  : [];
                setState('removedDockerHosts', reconcile(removed, { key: 'id' }));
              }
              if (message.data.removedKubernetesClusters !== undefined) {
                const removed = Array.isArray(message.data.removedKubernetesClusters)
                  ? (message.data.removedKubernetesClusters as RemovedKubernetesCluster[])
                  : [];
                setState('removedKubernetesClusters', reconcile(removed, { key: 'id' }));
              }
              if (message.data.metrics !== undefined) setState('metrics', message.data.metrics);
              if (message.data.performance !== undefined)
                setState('performance', message.data.performance);
              if (message.data.connectionHealth !== undefined)
                setState('connectionHealth', message.data.connectionHealth);
              if (message.data.stats !== undefined) setState('stats', message.data.stats);
              // Handle unified resources
              if (message.data.resources !== undefined) {
                logger.debug('[WebSocket] Updating resources', {
                  count: message.data.resources?.length || 0,
                  types: [...new Set(message.data.resources?.map((r: any) => r.type) || [])],
                });
                setState('resources', reconcile(message.data.resources, { key: 'id' }));

                // Sync container update states with docker host command status payloads.
                const resources = Array.isArray(message.data.resources)
                  ? message.data.resources
                  : [];
                resources.forEach((resource: any) => {
                  if (resource?.type !== 'docker-host') return;
                  const platformData = asRecord(resource.platformData);
                  const dockerData = asRecord(platformData?.docker);
                  const command = dockerData?.command || platformData?.command;
                  if (!command || typeof command !== 'object') return;

                  const agentIds = new Set<string>([
                    resource.id,
                    asString(dockerData?.hostSourceId) || '',
                    asString(platformData?.hostSourceId) || '',
                    asString(resource?.discoveryTarget?.agentId) || '',
                    isAppContainerDiscoveryResourceType(resource?.discoveryTarget?.resourceType)
                      ? asString(resource?.discoveryTarget?.resourceId) || ''
                      : '',
                  ]);
                  agentIds.forEach((agentId) => {
                    if (agentId) {
                      syncWithAgentCommand(agentId, command as any);
                    }
                  });
                });
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
            resourceCount: message.data?.resources?.length || 0,
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
          const alertIdentifier = message.data.alertIdentifier || message.data.alertId;
          logger.info('Alert resolved (will sync with next state update)', {
            alertIdentifier,
            alertId: message.data.alertId || alertIdentifier,
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
        } else if ((message as { type: string }).type === 'ai_discovery_progress') {
          // AI-powered discovery progress update
          eventBus.emit(
            'ai_discovery_progress',
            (message as { data: unknown }).data as import('../types/discovery').DiscoveryProgress,
          );
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
      if (isDisposed) return;
      logger.debug('disconnect', { code: event.code, reason: event.reason });
      setConnected(false);
      setInitialDataReceived(false);

      // Clear heartbeat interval
      clearHeartbeatTimer();

      if (isDisposed) {
        return;
      }

      // Don't try to reconnect if the close was intentional (code 1000)
      if (
        event.code === 1000 &&
        (event.reason === 'Reconnecting' || event.reason === 'Component unmounting')
      ) {
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
      if (isDisposed) return;
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
    isDisposed = true;
    isReconnecting = false;
    window.clearTimeout(reconnectDelayTimeout);
    window.clearTimeout(reconnectTimeout);
    window.clearInterval(heartbeatInterval);
    if (typeof window !== 'undefined') {
      window.removeEventListener(ALERTS_ACTIVATION_EVENT, handleAlertsActivation);
    }
    if (ws) {
      ws.close(1000, 'Component unmounting');
      ws = null;
    }
  });

  const markTokenRevoked = (
    key: 'dockerRuntimes' | 'agents',
    tokenId: string,
    agentIds: string[],
  ) => {
    if (!agentIds || agentIds.length === 0) return;
    const timestamp = Date.now();
    const targetIds = new Set(agentIds.filter(Boolean));
    setState(
      'resources',
      produce((draft: any[]) => {
        if (!Array.isArray(draft)) return;

        draft.forEach((resource) => {
          if (!resource || typeof resource !== 'object') return;

          const platformData =
            resource.platformData && typeof resource.platformData === 'object'
              ? resource.platformData
              : (resource.platformData = {});
          const agentData =
            platformData.agent && typeof platformData.agent === 'object'
              ? platformData.agent
              : (platformData.agent = {});
          const dockerData =
            platformData.docker && typeof platformData.docker === 'object'
              ? platformData.docker
              : (platformData.docker = {});

          const agentActionId =
            asString(agentData.agentId) ||
            asString(platformData.agentId) ||
            getAgentDiscoveryResourceId(resource?.discoveryTarget) ||
            asString(resource?.discoveryTarget?.agentId) ||
            asString(resource.id);
          const runtimeActionId =
            asString(dockerData.hostSourceId) ||
            asString(platformData.hostSourceId) ||
            asString(resource?.discoveryTarget?.agentId) ||
            (isAppContainerDiscoveryResourceType(resource?.discoveryTarget?.resourceType)
              ? asString(resource?.discoveryTarget?.resourceId)
              : undefined) ||
            asString(resource.id);

          const matchedId = key === 'agents' ? agentActionId : runtimeActionId;
          if (!matchedId || !targetIds.has(matchedId)) return;

          platformData.revokedTokenId = tokenId;
          platformData.tokenRevokedAt = timestamp;
          if (key === 'agents') {
            agentData.revokedTokenId = tokenId;
            agentData.tokenRevokedAt = timestamp;
          } else {
            dockerData.revokedTokenId = tokenId;
            dockerData.tokenRevokedAt = timestamp;
          }
        });
      }),
    );
  };

  return {
    state,
    activeAlerts,
    recentlyResolved,
    connected,
    reconnecting,
    initialDataReceived,
    updateProgress,
    shutdown,
    reconnect: () => {
      if (isDisposed) return;
      ws?.close();
      clearReconnectTimeout();
      clearReconnectDelayTimeout();
      reconnectAttempt = 0; // Reset attempts for manual reconnect
      isReconnecting = false;
      setReconnecting(false);
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

      clearReconnectTimeout();
      clearReconnectDelayTimeout();
      reconnectAttempt = 0;
      isReconnecting = false;
      ws?.close(1000, 'Reconnecting');
      connect();
    },
    markDockerRuntimesTokenRevoked: (tokenId: string, agentIds: string[]) =>
      markTokenRevoked('dockerRuntimes', tokenId, agentIds),
    markAgentsTokenRevoked: (tokenId: string, agentIds: string[]) =>
      markTokenRevoked('agents', tokenId, agentIds),
    removeAlerts: (predicate: (alert: Alert) => boolean) => {
      const keysToRemove: string[] = [];
      Object.entries(activeAlerts).forEach(([alertId, alert]) => {
        if (!alert) {
          keysToRemove.push(alertId);
          return;
        }
        try {
          if (predicate(alert)) {
            clearPendingAck(alertId);
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
          clearPendingAckTimeout(alertId);
          // Safety valve: if we never hear back from the server (e.g., request failed silently),
          // clear the pending flag after a generous timeout so we eventually resync with reality.
          const pendingTimeout = window.setTimeout(() => {
            if (pendingAckChanges.has(alertId)) {
              logger.warn(`Clearing stale pending ack change for alert ${alertId}`);
              clearPendingAck(alertId);
              notificationStore.error(
                'Server did not confirm the alert acknowledgment in time. Re-syncing from latest data.',
              );
            }
          }, 15000);
          pendingAckTimeouts.set(alertId, pendingTimeout);
        }
        setActiveAlerts(alertId, { ...existingAlert, ...updates });
      }
    },
  };
}
