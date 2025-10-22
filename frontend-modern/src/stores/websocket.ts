import { createSignal, onCleanup } from 'solid-js';
import { createStore, produce } from 'solid-js/store';
import type {
  State,
  WSMessage,
  Alert,
  ResolvedAlert,
  PVEBackups,
  VM,
  Container,
} from '@/types/api';
import type { ActivationState as ActivationStateType } from '@/types/alerts';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';
import { notificationStore } from './notifications';
import { eventBus } from './events';
import { ALERTS_ACTIVATION_EVENT, isAlertsActivationEnabled } from '@/utils/alertsActivation';

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
  });
  const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
  const [recentlyResolved, setRecentlyResolved] = createStore<Record<string, ResolvedAlert>>({});
  const [updateProgress, setUpdateProgress] = createSignal<unknown>(null);

  // Track consecutive empty dockerHost payloads so we can tolerate transient
  // blanks without spamming the UI.
  let consecutiveEmptyDockerUpdates = 0;
  let hasReceivedNonEmptyDockerHosts = false;

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
      setConnected(true);
      setReconnecting(false); // Clear reconnecting state
      reconnectAttempt = 0; // Reset reconnect attempts on successful connection
      isReconnecting = false;
      consecutiveEmptyDockerUpdates = 0;
      hasReceivedNonEmptyDockerHosts = false;

      // Start heartbeat to keep connection alive
      if (heartbeatInterval) {
        window.clearInterval(heartbeatInterval);
      }
      heartbeatInterval = window.setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping', data: { timestamp: Date.now() } }));
        }
      }, heartbeatIntervalMs);

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
          // Update state properties individually to ensure reactivity
          if (message.data) {
            // Mark that we've received initial data
            if (message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE) {
              setInitialDataReceived(true);
            }

            // Only update if we have actual data, don't overwrite with empty arrays
            if (message.data.nodes !== undefined) {
              console.log('[WebSocket] Updating nodes:', message.data.nodes?.length || 0);
              setState('nodes', message.data.nodes);
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
              setState('vms', transformedVMs);
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
              setState('containers', transformedContainers);
            }
            if (message.data.dockerHosts !== undefined && message.data.dockerHosts !== null) {
              // Only update if dockerHosts is present and not null
              if (Array.isArray(message.data.dockerHosts)) {
                const incomingHosts = message.data.dockerHosts;
                if (incomingHosts.length === 0) {
                  consecutiveEmptyDockerUpdates += 1;

                  const shouldApplyEmptyState =
                    !hasReceivedNonEmptyDockerHosts ||
                    consecutiveEmptyDockerUpdates >= 3 ||
                    message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE;

                  if (shouldApplyEmptyState) {
                    console.log('[WebSocket] Updating dockerHosts:', incomingHosts.length, 'hosts');
                    setState('dockerHosts', incomingHosts);
                  } else {
                    console.debug(
                      '[WebSocket] Skipping transient empty dockerHosts payload',
                      consecutiveEmptyDockerUpdates,
                    );
                  }
                } else {
                  consecutiveEmptyDockerUpdates = 0;
                  hasReceivedNonEmptyDockerHosts = true;
                  console.log('[WebSocket] Updating dockerHosts:', incomingHosts.length, 'hosts');
                  setState('dockerHosts', incomingHosts);
                }
              } else {
                console.warn('[WebSocket] Received non-array dockerHosts:', typeof message.data.dockerHosts);
              }
            } else if (message.data.dockerHosts === null) {
              console.log('[WebSocket] Received null dockerHosts, ignoring');
            }
            if (message.data.storage !== undefined) setState('storage', message.data.storage);
            if (message.data.hosts !== undefined) setState('hosts', message.data.hosts);
            if (message.data.cephClusters !== undefined)
              setState('cephClusters', message.data.cephClusters);
            if (message.data.pbs !== undefined) setState('pbs', message.data.pbs);
            if (message.data.pmg !== undefined) setState('pmg', message.data.pmg);
            if (message.data.replicationJobs !== undefined)
              setState('replicationJobs', message.data.replicationJobs);
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
              setState('physicalDisks', message.data.physicalDisks);
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
          }
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
            `🎉 ${nodeType} node "${nodeName}" was successfully auto-registered and is now being monitored!`,
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
