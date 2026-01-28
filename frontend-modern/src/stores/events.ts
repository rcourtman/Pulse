// Event bus for cross-component communication
import type { DiscoveryProgress } from '../types/discovery';

// Event types
export type EventType =
  | 'node_auto_registered'
  | 'refresh_nodes'
  | 'discovery_updated'
  | 'discovery_status'
  | 'ai_discovery_progress'
  | 'theme_changed'
  | 'websocket_reconnected';

// Event data types
export interface NodeAutoRegisteredData {
  type: string;
  host: string;
  name: string;
  tokenId: string;
  hasToken: boolean;
  verifySSL?: boolean;
  status?: string;
  nodeId?: string;
  nodeName?: string;
}

export interface DiscoveryUpdatedData {
  scanning?: boolean;
  cached?: boolean;
  timestamp?: number;
  servers: Array<{
    ip: string;
    port: number;
    type: string;
    version: string;
    hostname?: string;
    release?: string;
  }>;
  errors?: string[];
  immediate?: boolean;
  discoveredNodes?: number;
}

export interface DiscoveryStatusData {
  scanning: boolean;
  subnet?: string;
  timestamp?: number;
}

// Map event types to their data types
export type EventDataMap = {
  node_auto_registered: NodeAutoRegisteredData;
  refresh_nodes: void;
  discovery_updated: DiscoveryUpdatedData;
  discovery_status: DiscoveryStatusData;
  ai_discovery_progress: DiscoveryProgress;
  theme_changed: string; // 'light' or 'dark'
  websocket_reconnected: void; // Emitted when WebSocket successfully reconnects
};

// Generic event handler
type EventHandler<T = unknown> = (data?: T) => void;

class EventBus {
  private handlers: Map<EventType, Set<EventHandler<unknown>>> = new Map();

  on<T extends EventType>(event: T, handler: EventHandler<EventDataMap[T]>) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler as EventHandler<unknown>);

    // Return unsubscribe function
    return () => {
      this.handlers.get(event)?.delete(handler as EventHandler<unknown>);
    };
  }

  off<T extends EventType>(event: T, handler: EventHandler<EventDataMap[T]>) {
    this.handlers.get(event)?.delete(handler as EventHandler<unknown>);
  }

  emit<T extends EventType>(event: T, data?: EventDataMap[T]) {
    const handlers = this.handlers.get(event);
    if (handlers) {
      handlers.forEach((handler) => handler(data));
    }
  }
}

export const eventBus = new EventBus();
