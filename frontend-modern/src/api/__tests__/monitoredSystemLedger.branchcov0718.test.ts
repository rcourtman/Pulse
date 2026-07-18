import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  MonitoredSystemLedgerAPI,
  normalizeMonitoredSystemLedgerExplainResponse,
  normalizeMonitoredSystemLedgerPreviewResponse,
  normalizeMonitoredSystemLedgerResponse,
} from '../monitoredSystemLedger';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

type RawLedgerResponse = Parameters<typeof normalizeMonitoredSystemLedgerResponse>[0];
type RawPreviewResponse = Parameters<typeof normalizeMonitoredSystemLedgerPreviewResponse>[0];
type RawExplainResponse = Parameters<typeof normalizeMonitoredSystemLedgerExplainResponse>[0];

describe('MonitoredSystemLedgerAPI branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('explain() request shaping', () => {
    it('posts an empty-object body when invoked with no arguments (default param {})', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        ledger: { systems: [], total: 0 },
      });

      await MonitoredSystemLedgerAPI.explain();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/license/monitored-system-ledger/explain',
        { method: 'POST', body: '{}' },
      );
    });

    it('serializes both candidate and replacement objects into the POST body', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        ledger: { systems: [], total: 0 },
      });

      const request = {
        candidate: {
          source: 'vmware',
          hostname: 'esxi-01.lab.local',
          resource_id: 'vc-1',
          active: true,
        },
        replacement: { resource_id: 'vc-2', hostname: 'esxi-02.lab.local' },
      };
      await MonitoredSystemLedgerAPI.explain(request);

      const [path, opts] = vi.mocked(apiFetchJSON).mock.calls[0]!;
      expect(path).toBe('/api/license/monitored-system-ledger/explain');
      expect(opts).toEqual({ method: 'POST', body: JSON.stringify(request) });
      expect(JSON.parse((opts as { body: string }).body)).toEqual(request);
    });
  });

  describe('preview() request shaping', () => {
    it('serializes the full candidate and replacement when every optional field is populated', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        current_count: 0,
        projected_count: 1,
        additional_count: 1,
        effect: 'creates_new',
        current_systems: [],
        projected_systems: [],
      });

      const request = {
        candidate: {
          source: 'proxmox',
          type: 'proxmox-node',
          name: 'pve-01',
          hostname: 'pve-01.local',
          host_url: 'https://pve-01.local:8006',
          agent_id: 'agent-7',
          machine_id: 'mach-7',
          resource_id: 'res-7',
          active: true,
        },
        replacement: {
          source: 'agent',
          name: 'tower',
          hostname: 'tower.local',
          host_url: 'https://tower.local',
          agent_id: 'agent-9',
          machine_id: 'mach-9',
          resource_id: 'res-9',
        },
      };
      await MonitoredSystemLedgerAPI.preview(request);

      const [path, opts] = vi.mocked(apiFetchJSON).mock.calls[0]!;
      expect(path).toBe('/api/license/monitored-system-ledger/preview');
      expect(opts).toEqual({ method: 'POST', body: JSON.stringify(request) });
    });
  });

  describe('transport error propagation', () => {
    it('rethrows the underlying apiFetchJSON rejection verbatim', async () => {
      const error = new Error('network down');
      vi.mocked(apiFetchJSON).mockRejectedValueOnce(error);

      await expect(MonitoredSystemLedgerAPI.getLedger()).rejects.toBe(error);
      expect(apiFetchJSON).toHaveBeenCalledWith('/api/license/monitored-system-ledger');
    });
  });

  describe('normalizeMonitoredSystemLedgerResponse', () => {
    it('substitutes an empty systems array when the response omits systems entirely', () => {
      const raw = { total: 0 } as unknown as RawLedgerResponse;

      const result = normalizeMonitoredSystemLedgerResponse(raw);

      expect(result.systems).toEqual([]);
      expect(result.total).toBe(0);
    });
  });

  describe('normalizeMonitoredSystemLedgerPreviewResponse', () => {
    it('defaults both current_systems and projected_systems to empty arrays when absent', () => {
      const raw = {
        current_count: 0,
        projected_count: 0,
        additional_count: 0,
        effect: 'no_change',
      } as unknown as RawPreviewResponse;

      const result = normalizeMonitoredSystemLedgerPreviewResponse(raw);

      expect(result.current_systems).toEqual([]);
      expect(result.projected_systems).toEqual([]);
      expect(result.current_system).toBeNull();
      expect(result.projected_system).toBeNull();
    });

    it('returns null for current_system when more than one current_systems entry exists and current_system is null', () => {
      const raw = {
        current_count: 2,
        projected_count: 2,
        additional_count: 0,
        effect: 'no_change',
        current_systems: [
          { name: 'A', type: 'host', status: 'online', source: 'agent' },
          { name: 'B', type: 'host', status: 'online', source: 'docker' },
        ],
        current_system: null,
      } as unknown as RawPreviewResponse;

      const result = normalizeMonitoredSystemLedgerPreviewResponse(raw);

      expect(result.current_system).toBeNull();
      expect(result.current_systems).toHaveLength(2);
    });

    it('infers projected_system from a sole projected_systems entry when projected_system is null', () => {
      const raw = {
        current_count: 0,
        projected_count: 1,
        additional_count: 1,
        effect: 'creates_new',
        projected_systems: [
          { name: 'Projected', type: 'host', status: 'unknown', source: 'agent' },
        ],
        projected_system: null,
      } as unknown as RawPreviewResponse;

      const result = normalizeMonitoredSystemLedgerPreviewResponse(raw);

      expect(result.projected_system).toBe(result.projected_systems[0]);
      expect(result.projected_system?.name).toBe('Projected');
    });
  });

  describe('normalizeMonitoredSystemLedgerExplainResponse', () => {
    it('synthesizes an empty ledger when the response omits ledger entirely', () => {
      const raw = { preview: null } as unknown as RawExplainResponse;

      const result = normalizeMonitoredSystemLedgerExplainResponse(raw);

      expect(result.ledger.systems).toEqual([]);
      expect(result.ledger.total).toBe(0);
      expect(result.preview).toBeNull();
    });

    it('returns a null preview when preview is explicitly null', () => {
      const raw = {
        ledger: { systems: [], total: 0 },
        preview: null,
      } as unknown as RawExplainResponse;

      const result = normalizeMonitoredSystemLedgerExplainResponse(raw);

      expect(result.preview).toBeNull();
      expect(result.ledger.total).toBe(0);
    });

    it('returns a null preview when preview is undefined', () => {
      const raw = {
        ledger: { systems: [], total: 0 },
      } as unknown as RawExplainResponse;

      const result = normalizeMonitoredSystemLedgerExplainResponse(raw);

      expect(result.preview).toBeNull();
    });
  });

  describe('entry / status / signal normalization arms', () => {
    it('falls back to status-keyed copy and an empty reasons array when status_explanation is absent', () => {
      const raw = {
        systems: [{ name: 'N', type: 'host', status: 'warning', source: 'agent' }],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.status_explanation.summary).toBe(
        'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.',
      );
      expect(entry.status_explanation.reasons).toEqual([]);
    });

    it('coerces a missing entry status to unknown and uses the unknown fallback summary', () => {
      const raw = {
        systems: [{ name: 'N', type: 'host', source: 'agent' }],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.status).toBe('unknown');
      expect(entry.status_explanation.summary).toBe(
        'Pulse cannot determine a canonical runtime status for this monitored system yet.',
      );
    });

    it('defaults a missing status_reason.reported_at to an empty string', () => {
      const raw = {
        systems: [
          {
            name: 'N',
            type: 'host',
            status: 'online',
            source: 'agent',
            status_explanation: {
              summary: 's',
              reasons: [
                {
                  kind: 'k',
                  name: 'n',
                  type: 'host',
                  source: 'agent',
                  status: 'online',
                  summary: 'sum',
                },
              ],
            },
          },
        ],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.status_explanation.reasons[0]!.reported_at).toBe('');
    });

    it('coerces a missing status_reason.status to unknown', () => {
      const raw = {
        systems: [
          {
            name: 'N',
            type: 'host',
            status: 'online',
            source: 'agent',
            status_explanation: {
              summary: 's',
              reasons: [
                {
                  kind: 'k',
                  name: 'n',
                  type: 'host',
                  source: 'agent',
                  summary: 'sum',
                },
              ],
            },
          },
        ],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.status_explanation.reasons[0]!.status).toBe('unknown');
    });

    it('derives latest_included_signal from entry fields when the signal is entirely absent', () => {
      const raw = {
        systems: [
          { name: 'Cluster', type: 'kubernetes-cluster', status: 'online', source: 'kubernetes' },
        ],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.latest_included_signal).toEqual({
        name: 'Cluster',
        type: 'kubernetes-cluster',
        source: 'kubernetes',
        at: '',
      });
    });

    it('drops the latest_included_signal source when entry.source is "multiple" and the signal omits source', () => {
      const raw = {
        systems: [{ name: 'Multi', type: 'host', status: 'online', source: 'multiple' }],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.latest_included_signal.source).toBeUndefined();
      expect(entry.latest_included_signal.name).toBe('Multi');
    });

    it('falls back to "Unnamed source" and "system" when both the signal and entry name/type are blank', () => {
      const raw = {
        systems: [{ name: '', type: '', status: 'online', source: '' }],
        total: 1,
      } as unknown as RawLedgerResponse;

      const [entry] = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(entry.latest_included_signal.name).toBe('Unnamed source');
      expect(entry.latest_included_signal.type).toBe('system');
      expect(entry.latest_included_signal.source).toBeUndefined();
      expect(entry.latest_included_signal.at).toBe('');
    });

    it('lowercases a recognized latest_included_signal source and drops an unrecognized source', () => {
      const raw = {
        systems: [
          {
            name: 'NAS',
            type: 'truenas-system',
            status: 'online',
            source: 'truenas',
            latest_included_signal: {
              name: 'nas1',
              type: 'truenas-system',
              source: 'TrueNAS',
              at: '2026-01-01T00:00:00Z',
            },
          },
          {
            name: 'Other',
            type: 'host',
            status: 'online',
            source: 'agent',
            latest_included_signal: {
              name: 'x',
              type: 'host',
              source: 'not-a-real-source',
              at: '2026-01-01T00:00:00Z',
            },
          },
        ],
        total: 2,
      } as unknown as RawLedgerResponse;

      const systems = normalizeMonitoredSystemLedgerResponse(raw).systems;

      expect(systems[0]!.latest_included_signal.source).toBe('truenas');
      expect(systems[1]!.latest_included_signal.source).toBeUndefined();
    });
  });
});
