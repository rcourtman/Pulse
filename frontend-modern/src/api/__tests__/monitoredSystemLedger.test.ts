import { beforeEach, describe, expect, it, vi } from 'vitest';

import { MonitoredSystemLedgerAPI } from '../monitoredSystemLedger';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('MonitoredSystemLedgerAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the canonical monitored-system ledger endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [],
      total: 0,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/monitored-system-ledger');
    expect(result).toEqual({
      systems: [],
      total: 0,
      limit: 5,
    });
  });

  it('preserves grouping explanation payloads from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'server-1',
          type: 'host',
          status: 'online',
          status_explanation: {
            summary: 'All included top-level collection paths currently report online status.',
            reasons: [],
          },
          latest_included_signal_at: '2026-01-01T00:00:00Z',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
          explanation: {
            summary:
              'Counts as one monitored system because Pulse sees one top-level host view from agent.',
            reasons: [
              {
                kind: 'standalone',
                signal: 'single-top-level-view',
                summary: 'No overlapping top-level source matched this system.',
              },
            ],
            surfaces: [{ name: 'server-1', type: 'host', source: 'agent' }],
          },
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.explanation.summary).toContain('Counts as one monitored system');
    expect(result.systems[0]?.status_explanation?.summary).toContain('currently report online');
    expect(result.systems[0]?.status_explanation?.reasons).toEqual([]);
    expect(result.systems[0]?.explanation.reasons).toHaveLength(1);
    expect(result.systems[0]?.explanation.surfaces).toHaveLength(1);
  });

  it('normalizes missing explanation payloads', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'server-1',
          type: 'host',
          status: 'online',
          latest_included_signal_at: '2026-01-01T00:00:00Z',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.explanation.summary).toContain('counts this top-level collection path');
    expect(result.systems[0]?.status_explanation?.summary).toContain('currently report online');
    expect(result.systems[0]?.status_explanation?.reasons).toEqual([]);
    expect(result.systems[0]?.explanation.reasons).toEqual([]);
    expect(result.systems[0]?.explanation.surfaces).toEqual([]);
  });

  it('preserves canonical warning status from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'server-1',
          type: 'host',
          status: 'warning',
          latest_included_signal_at: '2026-01-01T00:00:00Z',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.status).toBe('warning');
  });

  it('preserves the canonical latest included signal timestamp from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'Tower',
          type: 'host',
          status: 'warning',
          latest_included_signal_at: '2026-03-23T11:59:50Z',
          last_seen: '2026-03-23T11:59:50Z',
          source: 'multiple',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.latest_included_signal_at).toBe('2026-03-23T11:59:50Z');
  });

  it('falls back to the deprecated last_seen alias for older payloads', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'Tower',
          type: 'host',
          status: 'warning',
          last_seen: '2026-03-23T11:59:50Z',
          source: 'multiple',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.latest_included_signal_at).toBe('2026-03-23T11:59:50Z');
  });

  it('preserves canonical status explanation reasons from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'Tower',
          type: 'host',
          status: 'warning',
          status_explanation: {
            summary: 'At least one included source is stale, so Pulse marks this monitored system as warning.',
            reasons: [
              {
                kind: 'source-stale',
                name: 'Tower',
                type: 'host',
                source: 'agent',
                status: 'stale',
                last_seen: '2026-03-23T11:55:00Z',
                summary: 'Agent data for Tower is stale (last reported 2026-03-23T11:55:00Z).',
              },
            ],
          },
          latest_included_signal_at: '2026-03-23T11:59:50Z',
          last_seen: '2026-03-23T11:59:50Z',
          source: 'multiple',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.status_explanation?.reasons).toEqual([
      {
        kind: 'source-stale',
        name: 'Tower',
        type: 'host',
        source: 'agent',
        status: 'stale',
        last_seen: '2026-03-23T11:55:00Z',
        summary: 'Agent data for Tower is stale (last reported 2026-03-23T11:55:00Z).',
      },
    ]);
  });

  it('fails closed to unknown for unsupported status values', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'server-1',
          type: 'host',
          status: 'degraded',
          latest_included_signal_at: '2026-01-01T00:00:00Z',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.status).toBe('unknown');
  });
});
