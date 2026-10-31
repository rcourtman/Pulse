import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { createAlertAcknowledgementRequests } from '../alertAcknowledgementRequests';

describe('createAlertAcknowledgementRequests', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('gives each alert to one unsettled request', () => {
    const requests = createAlertAcknowledgementRequests();

    const group = requests.claim('group', ['a', 'b', 'a']);
    expect(group?.alertIdentifiers).toEqual(['a', 'b']);
    const bulk = requests.claim('bulk', ['b', 'c']);
    expect(bulk?.alertIdentifiers).toEqual(['c']);
    expect(requests.claim('alert', ['a'])).toBeNull();
    expect(requests.isOwned('c')).toBe(true);
    expect(requests.isOwned('d')).toBe(false);

    // A card shows every unsettled request's alerts as processing.
    expect([...requests.processingAlerts()]).toEqual(['a', 'b', 'c']);
    expect(requests.bulkInFlight()).toBe(true);
  });

  it('releases exactly one request and holds its card or group alerts briefly', () => {
    const requests = createAlertAcknowledgementRequests();
    const group = requests.claim('group', ['a'])!;
    const bulk = requests.claim('bulk', ['b'])!;
    expect(requests.processingAlerts().has('b')).toBe(true);

    // A global request frees its alerts at once; only card and group requests hold.
    requests.release(bulk);
    expect(requests.bulkInFlight()).toBe(false);
    expect(requests.isOwned('b')).toBe(false);
    expect(requests.isOwned('a')).toBe(true);
    expect(requests.processingAlerts().has('b')).toBe(false);

    requests.release(group);
    expect(requests.isCurrent(group)).toBe(false);
    expect(requests.isOwned('a')).toBe(false);
    expect(requests.processingAlerts().has('a')).toBe(true);
    // Releasing again changes nothing, the hold included.
    vi.advanceTimersByTime(1000);
    requests.release(group);
    vi.advanceTimersByTime(500);
    expect(requests.processingAlerts().size).toBe(0);
  });

  it('keeps an alert busy while a newer request carries it after an older hold ends', () => {
    const requests = createAlertAcknowledgementRequests();
    const failed = requests.claim('group', ['a'])!;
    requests.release(failed);

    vi.advanceTimersByTime(500);
    const retry = requests.claim('group', ['a'])!;
    vi.advanceTimersByTime(1000);
    expect(requests.processingAlerts().has('a')).toBe(true);

    requests.release(retry);
    vi.advanceTimersByTime(1499);
    expect(requests.processingAlerts().has('a')).toBe(true);
    vi.advanceTimersByTime(1);
    expect(requests.processingAlerts().has('a')).toBe(false);
  });

  it('drops every claim and hold when its alert scope ends', () => {
    const requests = createAlertAcknowledgementRequests();
    const held = requests.claim('alert', ['a'])!;
    requests.release(held);
    const pending = requests.claim('group', ['b'])!;

    requests.reset();
    expect(requests.processingAlerts().size).toBe(0);
    expect(requests.isOwned('b')).toBe(false);
    expect(requests.isCurrent(pending)).toBe(false);

    // The next scope claims the same alert; the old request's release frees nothing.
    const next = requests.claim('group', ['b'])!;
    requests.release(pending);
    expect(requests.isCurrent(next)).toBe(true);
    expect(requests.isOwned('b')).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('shares nothing between separate owners', () => {
    const app = createAlertAcknowledgementRequests();
    const other = createAlertAcknowledgementRequests();

    expect(app.claim('bulk', ['a'])).not.toBeNull();
    expect(other.claim('bulk', ['a'])).not.toBeNull();
    app.reset();
    expect(other.bulkInFlight()).toBe(true);
  });
});
