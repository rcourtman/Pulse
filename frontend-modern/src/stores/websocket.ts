import { createSignal, onCleanup } from 'solid-js';
import { createStore } from 'solid-js/store';
import type { State, WSMessage, Alert } from '@/types/api';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS, WEBSOCKET } from '@/constants';

// Type-safe WebSocket store
export function createWebSocketStore(url: string) {
  const [connected, setConnected] = createSignal(false);
  const [state, setState] = createStore<State>({
    nodes: [],
    vms: [],
    containers: [],
    storage: [],
    pbs: [],
    metrics: [],
    pveBackups: {} as any,
    performance: {} as any,
    connectionHealth: {},
    stats: {} as any,
    lastUpdate: ''
  });
  const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});

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
        logger.logWebSocket('connect');
        setConnected(true);
        reconnectAttempt = 0; // Reset reconnect attempts on successful connection
        
        // Fetch active alerts on connection
        fetch('/api/alerts/active')
          .then(res => res.json())
          .then(alerts => {
            if (Array.isArray(alerts)) {
              // Clear existing alerts first
              const currentAlertIds = Object.keys(activeAlerts);
              currentAlertIds.forEach(id => setActiveAlerts(id, undefined));
              
              // Add new alerts
              alerts.forEach(alert => {
                setActiveAlerts(alert.id, alert);
              });
            }
          })
          .catch(err => console.error('[WebSocket] Failed to fetch active alerts:', err));
      };

      ws.onmessage = (event) => {
        try {
          const message: WSMessage = JSON.parse(event.data);
          
          if (message.type === WEBSOCKET.MESSAGE_TYPES.INITIAL_STATE || message.type === WEBSOCKET.MESSAGE_TYPES.RAW_DATA) {
            // Update state properties individually to ensure reactivity
            if (message.data) {
              // Only update if we have actual data, don't overwrite with empty arrays
              if (message.data.nodes !== undefined) setState('nodes', message.data.nodes);
              if (message.data.vms !== undefined) setState('vms', message.data.vms);
              if (message.data.containers !== undefined) setState('containers', message.data.containers);
              if (message.data.storage !== undefined) setState('storage', message.data.storage);
              if (message.data.pbs !== undefined) setState('pbs', message.data.pbs);
              if (message.data.metrics !== undefined) setState('metrics', message.data.metrics);
              if (message.data.pveBackups !== undefined) setState('pveBackups', message.data.pveBackups);
              if (message.data.performance !== undefined) setState('performance', message.data.performance);
              if (message.data.connectionHealth !== undefined) setState('connectionHealth', message.data.connectionHealth);
              if (message.data.stats !== undefined) setState('stats', message.data.stats);
              setState('lastUpdate', message.data.lastUpdate || new Date().toISOString());
            }
            logger.logWebSocket('message', { 
              type: message.type, 
              hasData: !!message.data,
              nodeCount: message.data?.nodes?.length || 0,
              vmCount: message.data?.vms?.length || 0,
              containerCount: message.data?.containers?.length || 0
            });
          } else if (message.type === WEBSOCKET.MESSAGE_TYPES.ERROR) {
            logger.logWebSocket('error', message.error);
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
            // New alert received
            const alert = message.data;
            setActiveAlerts(alert.id, alert);
            logger.warn('New alert received', alert);
          } else if (message.type === 'alertResolved') {
            // Alert resolved
            const { alertId } = message.data;
            // For Solid.js stores, we need to use the property deletion syntax
            setActiveAlerts(alertId, undefined);
            logger.info('Alert resolved', { alertId });
          }
        } catch (err) {
          logger.error('Failed to parse WebSocket message', err);
        }
      };

      ws.onclose = (event) => {
        logger.logWebSocket('disconnect', { code: event.code, reason: event.reason });
        setConnected(false);
        
        // Don't reconnect if we're already trying
        if (isReconnecting) {
          return;
        }
        
        isReconnecting = true;
        
        // Calculate exponential backoff delay
        const delay = Math.min(
          initialReconnectDelay * Math.pow(2, reconnectAttempt),
          maxReconnectDelay
        );
        
        logger.info(`Reconnecting in ${delay}ms (attempt ${reconnectAttempt + 1})`);
        reconnectAttempt++;
        
        reconnectTimeout = window.setTimeout(() => {
          isReconnecting = false;
          connect();
        }, delay);
      };

      ws.onerror = (error) => {
        // Don't log connection errors if we're already connected
        // Browser may show errors for initial connection attempts even after success
        if (!connected()) {
          logger.logWebSocket('error', error);
        }
      };
    } catch (err) {
      logger.error('Failed to connect', err);
      setConnected(false);
      
      // Don't reconnect if we're already trying
      if (isReconnecting) return;
      
      isReconnecting = true;
      
      // Use exponential backoff for connection errors too
      const delay = Math.min(
        initialReconnectDelay * Math.pow(2, reconnectAttempt),
        maxReconnectDelay
      );
      
      reconnectAttempt++;
      reconnectTimeout = window.setTimeout(() => {
        isReconnecting = false;
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
    connected,
    reconnect: () => {
      ws?.close();
      window.clearTimeout(reconnectTimeout);
      reconnectAttempt = 0; // Reset attempts for manual reconnect
      connect();
    }
  };
}