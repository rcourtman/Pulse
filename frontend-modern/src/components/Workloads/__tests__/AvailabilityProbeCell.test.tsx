import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@solidjs/testing-library';

import { AvailabilityProbeCell } from '../GuestRowCells';
import { getAvailabilityProbePresentation } from '@/utils/availabilityProbePresentation';
import type { ResourceAvailabilityMeta } from '@/types/resource';

const renderCell = (availability: ResourceAvailabilityMeta) => {
  const presentation = getAvailabilityProbePresentation({
    type: 'system-container' as never,
    platformType: 'proxmox' as never,
    status: 'online' as never,
    availability,
    platformData: { availability },
  });
  if (!presentation) throw new Error('no availability presentation');
  const { container } = render(() => <AvailabilityProbeCell presentation={presentation} />);
  const badge = container.querySelector('span');
  if (!badge) throw new Error('cell not rendered');
  return { text: badge.textContent?.trim() ?? '', className: badge.className, badge };
};

const freshBase = (): ResourceAvailabilityMeta => ({
  enabled: true,
  protocol: 'http',
  correlationState: 'attached',
  lastChecked: new Date().toISOString(),
  pollIntervalSeconds: 60,
  failureThreshold: 2,
});

describe('AvailabilityProbeCell', () => {
  afterEach(() => {
    cleanup();
  });

  it('shows the response time on its own for a healthy probe', () => {
    const { text, className } = renderCell({ ...freshBase(), available: true, latencyMillis: 7 });

    expect(text).toBe('7ms');
    expect(className).toContain('text-emerald-600');
  });

  // Freshness is Pulse vouching for its own probe pipeline rather than a fact
  // about the guest, so it is only worth drawing when it undermines the reading.
  it('omits the freshness suffix while the reading is current', () => {
    const { text } = renderCell({ ...freshBase(), available: true, latencyMillis: 104 });

    expect(text).toBe('104ms');
    expect(text).not.toContain('fresh');
  });

  it('appends the freshness suffix once the reading goes stale', () => {
    const { text } = renderCell({
      ...freshBase(),
      available: true,
      latencyMillis: 7,
      lastChecked: new Date(Date.now() - 60 * 60 * 1000).toISOString(),
    });

    expect(text).toBe('7ms · stale');
  });

  it('replaces the response time with the failure reason when the probe times out', () => {
    const { text, className } = renderCell({
      ...freshBase(),
      available: false,
      consecutiveFailures: 4,
      lastError: 'dial tcp 10.0.0.4:443: i/o timeout',
    });

    expect(text).toBe('timed out');
    expect(className).toContain('text-red-600');
  });

  it('surfaces an HTTP failure status when the probe gets an error response', () => {
    const { text, className } = renderCell({
      ...freshBase(),
      available: false,
      consecutiveFailures: 3,
      lastError: 'unexpected status 503',
    });

    expect(text).toBe('503');
    expect(className).toContain('text-red-600');
  });

  it('keeps the probe method and last-checked detail on the hover title', () => {
    const { badge } = renderCell({ ...freshBase(), available: true, latencyMillis: 7 });

    expect(badge.getAttribute('title')).toContain('7 ms');
  });
});
