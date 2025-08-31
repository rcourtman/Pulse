import { createSignal, onCleanup } from 'solid-js';
import { createStore } from 'solid-js/store';
import type { State, WSMessage, Alert, ResolvedAlert, PVEBackups } from '@/types/api';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';
import { notificationStore } from './notifications';
import { eventBus } from './events';

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  const [connected, setConnected] = createSignal(false);
  const [reconnecting, setReconnecting] = createSignal(false);
  const [initialDataReceived, setInitialDataReceived] = createSignal(false);
  const [state, setState] = createStore<State>({
    nodes: [],
    vms: [],
    containers: [],
    storage: [],
    pbs: [],
    metrics: [],
    pveBackups: {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: []
    } as PVEBackups,
    pbsBackups: [],
    performance: {
      apiCallDuration: {},
      lastPollDuration: 0,
      pollingStartTime: '',
      totalApiCalls: 0,
      failedApiCalls: 0,
      cacheHits: 0,
      cacheMisses: 0
    },
    connectionHealth: {},
    stats: {
      startTime: new Date().toISOString(),
      uptime: 0,
      pollingCycles: 0,
      webSocketClients: 0,
      version: '2.0.0'
    },
    activeAlerts: [],
    recentlyResolved: [],
    lastUpdate: ''
  });
  const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
  const [recentlyResolved, setRecentlyResolved] = createStore<Record<string, ResolvedAlert>>({});
  const [updateProgress, setUpdateProgress] = createSignal<any>(null);

  let ws: WebSocket | null = null;
  let reconnectTimeout: number;
  let reconnectAttempt = 0;
  let isReconnecting = false;
  const maxReconnectDelay = POLLING_INTERVALS.RECONNECT_MAX;
  const initialReconnectDelay = POLLING_INTERVALS.RECONNECT_BASE;

  const connect = () => {
    try {
      // Close existing connection if any
      if (ws && ws.readyState !== WebSocket.CLOSED) {
        ws.close();
      }
      
      ws = new WebSocket(url);

      ws.onopen = () => {
        logger.debug('connect');
        setConnected(true);
        setReconnecting(false); // Clear reconnecting state
        reconnectAttempt = 0; // Reset reconnect attempts on successful connection
        
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
          
          if (message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE || message.type === WEBSOCKET.MESSAGE_TYPES.RAW_DATA) {
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
                const transformedVMs = message.data.vms.map((vm: any) => {
                  const originalTags = vm.tags;
                  let transformedTags;
                  
                  if (originalTags && typeof originalTags === 'string' && originalTags.trim()) {
                    // String with content - split into array
                    transformedTags = originalTags.split(',').map((t: string) => t.trim()).filter((t: string) => t.length > 0);
                  } else if (Array.isArray(originalTags)) {
                    // Already an array - filter out empty/whitespace-only tags
                    transformedTags = originalTags.filter((tag: any) => 
                      typeof tag === 'string' && tag.trim().length > 0
                    );
                  } else {
                    // null, undefined, empty string, or other - convert to empty array
                    transformedTags = [];
                  }
                  
                  return {
                    ...vm,
                    tags: transformedTags
                  };
                });
                setState('vms', transformedVMs);
              }
              if (message.data.containers !== undefined) {
                // Transform tags from comma-separated strings to arrays
                const transformedContainers = message.data.containers.map((container: any) => {
                  const originalTags = container.tags;
                  let transformedTags;
                  
                  if (originalTags && typeof originalTags === 'string' && originalTags.trim()) {
                    // String with content - split into array
                    transformedTags = originalTags.split(',').map((t: string) => t.trim()).filter((t: string) => t.length > 0);
                  } else if (Array.isArray(originalTags)) {
                    // Already an array - filter out empty/whitespace-only tags
                    transformedTags = originalTags.filter((tag: any) => 
                      typeof tag === 'string' && tag.trim().length > 0
                    );
                  } else {
                    // null, undefined, empty string, or other - convert to empty array
                    transformedTags = [];
                  }
                  
                  return {
                    ...container,
                    tags: transformedTags
                  };
                });
                setState('containers', transformedContainers);
              }
              if (message.data.storage !== undefined) setState('storage', message.data.storage);
              if (message.data.pbs !== undefined) setState('pbs', message.data.pbs);
              if (message.data.pbsBackups !== undefined) setState('pbsBackups', message.data.pbsBackups);
              if (message.data.metrics !== undefined) setState('metrics', message.data.metrics);
              if (message.data.pveBackups !== undefined) setState('pveBackups', message.data.pveBackups);
              if (message.data.performance !== undefined) setState('performance', message.data.performance);
              if (message.data.connectionHealth !== undefined) setState('connectionHealth', message.data.connectionHealth);
              if (message.data.stats !== undefined) setState('stats', message.data.stats);
              // Sync active alerts from state
              if (message.data.activeAlerts !== undefined) {
                // Received activeAlerts update
                
                // Update alerts atomically to prevent race conditions
                const newAlerts: Record<string, Alert> = {};
                if (message.data.activeAlerts && Array.isArray(message.data.activeAlerts)) {
                  message.data.activeAlerts.forEach((alert: Alert) => {
                    newAlerts[alert.id] = alert;
                  });
                }
                
                // Clear existing alerts and set new ones
                const currentAlertIds = Object.keys(activeAlerts);
                currentAlertIds.forEach(id => {
                  if (!newAlerts[id]) {
                    setActiveAlerts(id, undefined!);
                  }
                });
                
                // Add new alerts
                Object.entries(newAlerts).forEach(([id, alert]) => {
                  setActiveAlerts(id, alert);
                });
                
                // Updated activeAlerts
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
                currentResolvedIds.forEach(id => {
                  if (!newResolvedAlerts[id]) {
                    setRecentlyResolved(id, undefined!);
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
              containerCount: message.data?.containers?.length || 0
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
            logger.info('Alert resolved (will sync with next state update)', { alertId: message.data.alertId });
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
              8000
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
        
        // Don't reconnect if we're already trying
        if (isReconnecting) {
          return;
        }
        
        // Clear any existing timeout to prevent multiple reconnections
        if (reconnectTimeout) {
          window.clearTimeout(reconnectTimeout);
          reconnectTimeout = 0;
        }
        
        isReconnecting = true;
        setReconnecting(true);
        
        // Calculate exponential backoff delay
        const delay = Math.min(
          initialReconnectDelay * Math.pow(2, reconnectAttempt),
          maxReconnectDelay
        );
        
        logger.info(`Reconnecting in ${delay}ms (attempt ${reconnectAttempt + 1})`);
        reconnectAttempt++;
        
        reconnectTimeout = window.setTimeout(() => {
          isReconnecting = false;
          setReconnecting(false);
          connect();
        }, delay);
      };

      ws.onerror = (error) => {
        // Don't log connection errors if we're already connected
        // Browser may show errors for initial connection attempts even after success
        if (!connected()) {
          logger.debug('error', error);
        }
      };
    } catch (err) {
      logger.error('Failed to connect', err);
      setConnected(false);
      
      // Don't reconnect if we're already trying
      if (isReconnecting) return;
      
      isReconnecting = true;
      setReconnecting(true);
      
      // Use exponential backoff for connection errors too
      const delay = Math.min(
        initialReconnectDelay * Math.pow(2, reconnectAttempt),
        maxReconnectDelay
      );
      
      reconnectAttempt++;
      reconnectTimeout = window.setTimeout(() => {
        isReconnecting = false;
        setReconnecting(false);
        connect();
      }, delay);
    }
  };

  // Connect immediately
  connect();

  // Cleanup on unmount
  onCleanup(() => {
    window.clearTimeout(reconnectTimeout);
    ws?.close();
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
    // Method to update an alert locally (e.g., after acknowledgment)
    updateAlert: (alertId: string, updates: Partial<Alert>) => {
      const existingAlert = activeAlerts[alertId];
      if (existingAlert) {
        setActiveAlerts(alertId, { ...existingAlert, ...updates });
      }
    }
  };
}