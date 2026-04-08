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

  it('fetches the canonical monitored-system ledger preview endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      current_count: 1,
      projected_count: 1,
      additional_count: 0,
      limit: 5,
      would_exceed_limit: false,
      effect: 'attaches_existing',
      current_systems: [
        {
          name: 'Tower',
          type: 'host',
          status: 'online',
          source: 'agent',
        },
      ],
      projected_systems: [
        {
          name: 'tower',
          type: 'vmware-host',
          status: 'online',
          source: 'vmware',
        },
      ],
      current_system: {
        name: 'Tower',
        type: 'host',
        status: 'online',
        source: 'agent',
      },
      projected_system: {
        name: 'tower',
        type: 'vmware-host',
        status: 'online',
        source: 'vmware',
      },
    });

    const request = {
      candidate: {
        source: 'vmware',
        hostname: 'tower.local',
        resource_id: 'vc-1',
      },
    };
    const result = await MonitoredSystemLedgerAPI.preview(request);

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/monitored-system-ledger/preview', {
      method: 'POST',
      body: JSON.stringify(request),
    });
    expect(result.current_system?.source).toBe('agent');
    expect(result.projected_system?.source).toBe('vmware');
    expect(result.current_systems).toHaveLength(1);
    expect(result.projected_systems).toHaveLength(1);
    expect(result.effect).toBe('attaches_existing');
  });

  it('fetches the canonical monitored-system explanation endpoint', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      ledger: {
        systems: [
          {
            name: 'Tower',
            type: 'host',
            status: 'online',
            source: 'agent',
          },
        ],
        total: 1,
        limit: 5,
      },
      preview: {
        current_count: 1,
        projected_count: 1,
        additional_count: 0,
        limit: 5,
        would_exceed_limit: false,
        effect: 'attaches_existing',
        current_systems: [
          {
            name: 'Tower',
            type: 'host',
            status: 'online',
            source: 'agent',
          },
        ],
        projected_systems: [
          {
            name: 'tower',
            type: 'vmware-host',
            status: 'online',
            source: 'vmware',
          },
        ],
        current_system: null,
        projected_system: null,
      },
    });

    const request = {
      candidate: {
        source: 'vmware',
        hostname: 'tower.local',
        resource_id: 'vc-1',
      },
    };
    const result = await MonitoredSystemLedgerAPI.explain(request);

    expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/monitored-system-ledger/explain', {
      method: 'POST',
      body: JSON.stringify(request),
    });
    expect(result.ledger.total).toBe(1);
    expect(result.preview?.effect).toBe('attaches_existing');
    expect(result.preview?.projected_systems).toHaveLength(1);
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
          latest_included_signal: {
            name: 'server-1',
            type: 'host',
            source: 'agent',
            at: '2026-01-01T00:00:00Z',
          },
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
          latest_included_signal: {
            name: 'server-1',
            type: 'host',
            at: '2026-01-01T00:00:00Z',
          },
          source: 'agent',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.explanation.summary).toContain(
      'counts this top-level collection path',
    );
    expect(result.systems[0]?.status_explanation?.summary).toContain('currently report online');
    expect(result.systems[0]?.status_explanation?.reasons).toEqual([]);
    expect(result.systems[0]?.explanation.reasons).toEqual([]);
    expect(result.systems[0]?.explanation.surfaces).toEqual([]);
    expect(result.systems[0]?.latest_included_signal).toEqual({
      name: 'server-1',
      type: 'host',
      source: 'agent',
      at: '2026-01-01T00:00:00Z',
    });
  });

  it('normalizes missing status explanation copy from the canonical presentation helper', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'server-1',
          type: 'host',
          status: 'offline',
          latest_included_signal: {
            name: 'server-1',
            type: 'host',
            source: 'agent',
            at: '2026-01-01T00:00:00Z',
          },
          source: 'agent',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.status_explanation?.summary).toBe(
      'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
    );
  });

  it('preserves canonical warning status from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'server-1',
          type: 'host',
          status: 'warning',
          latest_included_signal: {
            name: 'server-1',
            type: 'host',
            source: 'agent',
            at: '2026-01-01T00:00:00Z',
          },
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
          latest_included_signal: {
            name: 'tower.local',
            type: 'docker-host',
            source: 'docker',
            at: '2026-03-23T11:59:50Z',
          },
          source: 'multiple',
        },
      ],
      total: 1,
      limit: 5,
    });

    const result = await MonitoredSystemLedgerAPI.getLedger();

    expect(result.systems[0]?.latest_included_signal).toEqual({
      name: 'tower.local',
      type: 'docker-host',
      source: 'docker',
      at: '2026-03-23T11:59:50Z',
    });
    expect(result.systems[0]).not.toHaveProperty('latest_included_signal_at');
    expect(result.systems[0]).not.toHaveProperty('latest_included_signal_source');
    expect(result.systems[0]).not.toHaveProperty('last_seen');
  });

  it('preserves canonical status explanation reasons from the API contract', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      systems: [
        {
          name: 'Tower',
          type: 'host',
          status: 'warning',
          status_explanation: {
            summary:
              'At least one included source is stale, so Pulse marks this monitored system as warning.',
            reasons: [
              {
                kind: 'source-stale',
                name: 'Tower',
                type: 'host',
                source: 'agent',
                status: 'stale',
                reported_at: '2026-03-23T11:55:00Z',
                summary: 'Agent data for Tower is stale (last reported 2026-03-23T11:55:00Z).',
              },
            ],
          },
          latest_included_signal: {
            name: 'tower.local',
            type: 'docker-host',
            source: 'docker',
            at: '2026-03-23T11:59:50Z',
          },
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
        reported_at: '2026-03-23T11:55:00Z',
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
          latest_included_signal: {
            name: 'server-1',
            type: 'host',
            source: 'agent',
            at: '2026-01-01T00:00:00Z',
          },
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
