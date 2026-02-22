import { describe, expect, it, vi } from 'vitest';
import { eventBus, type EventType } from '@/stores/events';

describe('eventBus', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('on/off', () => {
    it('calls handler when event is emitted', () => {
      const handler = vi.fn();
      eventBus.on('refresh_nodes', handler);
      
      eventBus.emit('refresh_nodes');
      
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('passes data to handler when event is emitted with data', () => {
      const handler = vi.fn();
      eventBus.on('theme_changed', handler);
      
      eventBus.emit('theme_changed', 'dark');
      
      expect(handler).toHaveBeenCalledWith('dark');
    });

    it('calls handler with undefined when no data provided', () => {
      const handler = vi.fn();
      eventBus.on('refresh_nodes', handler);
      
      eventBus.emit('refresh_nodes');
      
      expect(handler).toHaveBeenCalledWith(undefined);
    });

    it('returns unsubscribe function', () => {
      const handler = vi.fn();
      const unsubscribe = eventBus.on('refresh_nodes', handler);
      
      unsubscribe();
      eventBus.emit('refresh_nodes');
      
      expect(handler).not.toHaveBeenCalled();
    });

    it('can unsubscribe using off method', () => {
      const handler = vi.fn();
      eventBus.on('refresh_nodes', handler);
      eventBus.off('refresh_nodes', handler);
      
      eventBus.emit('refresh_nodes');
      
      expect(handler).not.toHaveBeenCalled();
    });

    it('handles multiple handlers for same event', () => {
      const handler1 = vi.fn();
      const handler2 = vi.fn();
      eventBus.on('refresh_nodes', handler1);
      eventBus.on('refresh_nodes', handler2);
      
      eventBus.emit('refresh_nodes');
      
      expect(handler1).toHaveBeenCalledTimes(1);
      expect(handler2).toHaveBeenCalledTimes(1);
    });

    it('only calls handlers for the subscribed event', () => {
      const handler1 = vi.fn();
      const handler2 = vi.fn();
      eventBus.on('theme_changed', handler1);
      eventBus.on('refresh_nodes', handler2);
      
      eventBus.emit('refresh_nodes');
      
      expect(handler1).not.toHaveBeenCalled();
      expect(handler2).toHaveBeenCalledTimes(1);
    });
  });

  describe('node_auto_registered event', () => {
    it('emits node_auto_registered with correct data', () => {
      const handler = vi.fn();
      eventBus.on('node_auto_registered', handler);
      
      const data = {
        type: 'qemu',
        host: '192.168.1.1',
        name: 'test-node',
        tokenId: 'token-123',
        hasToken: true,
      };
      
      eventBus.emit('node_auto_registered', data);
      
      expect(handler).toHaveBeenCalledWith(data);
    });
  });

  describe('discovery_updated event', () => {
    it('emits discovery_updated with server list', () => {
      const handler = vi.fn();
      eventBus.on('discovery_updated', handler);
      
      const data = {
        scanning: false,
        servers: [
          { ip: '192.168.1.1', port: 8006, type: 'qemu', version: '8.0' },
        ],
      };
      
      eventBus.emit('discovery_updated', data);
      
      expect(handler).toHaveBeenCalledWith(data);
    });
  });

  describe('org_switched event', () => {
    it('emits org_switched with org ID', () => {
      const handler = vi.fn();
      eventBus.on('org_switched', handler);
      
      eventBus.emit('org_switched', 'org-123');
      
      expect(handler).toHaveBeenCalledWith('org-123');
    });
  });

  describe('websocket_reconnected event', () => {
    it('emits websocket_reconnected', () => {
      const handler = vi.fn();
      eventBus.on('websocket_reconnected', handler);
      
      eventBus.emit('websocket_reconnected');
      
      expect(handler).toHaveBeenCalledWith(undefined);
    });
  });
});
