import { createSignal, onCleanup } from 'solid-js';
import { createStore } from 'solid-js/store';
import type { State, WSMessage, Alert, ResolvedAlert, PVEBackups } from '@/types/api';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  const [connected, setConnected] = createSignal(false);
  const [reconnecting, setReconnecting] = createSignal(false);
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
        try {
          const data = JSON.parse(event.data);
          
          const message: WSMessage = data;
          
          if (message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE || message.type === WEBSOCKET.MESSAGE_TYPES.RAW_DATA) {
            // Update state properties individually to ensure reactivity
            if (message.data) {
              // Only update if we have actual data, don't overwrite with empty arrays
              if (message.data.nodes !== undefined) setState('nodes', message.data.nodes);
              if (message.data.vms !== undefined) setState('vms', message.data.vms);
              if (message.data.containers !== undefined) setState('containers', message.data.containers);
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
                console.log('[WebSocket] Received activeAlerts:', message.data.activeAlerts);
                
                // First, remove all existing alerts
                const currentAlertIds = Object.keys(activeAlerts);
                currentAlertIds.forEach(id => {
                  setActiveAlerts(id, undefined!);
                });
                
                // Then add the new alerts
                message.data.activeAlerts.forEach((alert: Alert) => {
                  setActiveAlerts(alert.id, alert);
                });
                
                console.log('[WebSocket] Updated activeAlerts to:', activeAlerts);
              }
              // Sync recently resolved alerts
              if (message.data.recentlyResolved !== undefined) {
                console.log('[WebSocket] Received recentlyResolved:', message.data.recentlyResolved);
                
                // First, remove all existing resolved alerts
                const currentResolvedIds = Object.keys(recentlyResolved);
                currentResolvedIds.forEach(id => {
                  setRecentlyResolved(id, undefined!);
                });
                
                // Then add the new resolved alerts
                message.data.recentlyResolved.forEach((alert: ResolvedAlert) => {
                  setRecentlyResolved(alert.id, alert);
                });
                
                console.log('[WebSocket] Updated recentlyResolved to:', recentlyResolved);
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
          }
        } catch (err) {
          logger.error('Failed to parse WebSocket message', err);
        }
      };

      ws.onclose = (event) => {
        logger.debug('disconnect', { code: event.code, reason: event.reason });
        setConnected(false);
        
        // Don't reconnect if we're already trying
        if (isReconnecting) {
          return;
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
    updateProgress,
    reconnect: () => {
      ws?.close();
      window.clearTimeout(reconnectTimeout);
      reconnectAttempt = 0; // Reset attempts for manual reconnect
      connect();
    }
  };
}