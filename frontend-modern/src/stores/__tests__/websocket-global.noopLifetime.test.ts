import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { getGlobalWebSocketStore, shutdownGlobalWebSocketStore } from '@/stores/websocket-global';

// In a jsdom test window the shared app store is the no-op store. An inert
// WebSocket stand-in keeps any other runtime from reaching the network, and
// the first test says which store this is. Each test ends the shared app it
// used, so nothing it left behind reaches the next one.
const InertWebSocket = vi.fn().mockImplementation(function () {
  return { readyState: 0, send: vi.fn(), close: vi.fn() };
});

describe('the shared no-op app store', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal('WebSocket', InertWebSocket);
    shutdownGlobalWebSocketStore();
  });

  afterEach(() => {
    shutdownGlobalWebSocketStore();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('is the no-op store in the test window', () => {
    expect(window.navigator.userAgent).toMatch(/jsdom/i);
    const store = getGlobalWebSocketStore();
    expect(store.initialDataReceived()).toBe(true);
    expect(store.connected()).toBe(false);
    expect(InertWebSocket).not.toHaveBeenCalled();
  });

  it('keeps one request owner for the app', () => {
    const store = getGlobalWebSocketStore();
    expect(getGlobalWebSocketStore()).toBe(store);
    expect(getGlobalWebSocketStore().alertAcknowledgementRequests).toBe(
      store.alertAcknowledgementRequests,
    );
  });

  it('drops the request owner when the app shuts down, as the real store does', () => {
    const store = getGlobalWebSocketStore();
    const requests = store.alertAcknowledgementRequests;
    requests.claim('group', ['a']);

    store.shutdown();
    expect(requests.isOwned('a')).toBe(false);
    expect(requests.processingAlerts().size).toBe(0);
  });

  it('ends a discarded hold and a pending claim with the app that held them', () => {
    const first = getGlobalWebSocketStore();
    const firstRequests = first.alertAcknowledgementRequests;
    const pending = firstRequests.claim('group', ['a'])!;
    firstRequests.release(firstRequests.claim('alert', ['b'])!);
    expect([...firstRequests.processingAlerts()].sort()).toEqual(['a', 'b']);

    // A test that ends here leaves the pending claim and the hold behind; its
    // fake timer for the hold never runs.
    vi.useRealTimers();
    shutdownGlobalWebSocketStore();
    expect(firstRequests.processingAlerts().size).toBe(0);

    const next = getGlobalWebSocketStore();
    expect(next).not.toBe(first);
    const nextRequests = next.alertAcknowledgementRequests;
    expect(nextRequests).not.toBe(firstRequests);
    expect(nextRequests.processingAlerts().size).toBe(0);
    expect(nextRequests.claim('group', ['a', 'b'])?.alertIdentifiers).toEqual(['a', 'b']);

    // The old app's request settling late frees nothing in the next one.
    firstRequests.release(pending);
    expect(nextRequests.isOwned('a')).toBe(true);
  });
});
