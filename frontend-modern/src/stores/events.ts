// Event bus for cross-component communication

// Event types
export type EventType = 'node_auto_registered' | 'refresh_nodes' | 'discovery_updated';

// Event handlers
type EventHandler = (data?: any) => void;

class EventBus {
  private handlers: Map<EventType, Set<EventHandler>> = new Map();

  on(event: EventType, handler: EventHandler) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler);
    
    // Return unsubscribe function
    return () => {
      this.handlers.get(event)?.delete(handler);
    };
  }

  emit(event: EventType, data?: any) {
    const handlers = this.handlers.get(event);
    if (handlers) {
      handlers.forEach(handler => handler(data));
    }
  }
}

export const eventBus = new EventBus();