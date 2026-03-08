import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { StorageCommandCenter } from '@/components/Storage/StorageCommandCenter';

const { apiFetchJSONMock } = vi.hoisted(() => ({
  apiFetchJSONMock: vi.fn(),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: apiFetchJSONMock,
}));

describe('StorageCommandCenter', () => {
  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('renders command-center sections from canonical storage endpoints', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({
        generatedAt: '2026-03-08T10:00:00Z',
        totalResources: 9,
        riskyResources: 4,
        criticalResources: 1,
        warningResources: 3,
        protectionReducedCount: 2,
        rebuildInProgressCount: 1,
        dependentResourceCount: 5,
        protectedWorkloadCount: 3,
        affectedDatastoreCount: 1,
        byPlatform: { pbs: 1, unraid: 1, truenas: 1, agent: 1 },
        byResourceType: { pbs: 1, storage: 2, physical_disk: 1 },
        byIncidentCategory: { recoverability: 1, protection: 1, rebuild: 1, 'disk-health': 1 },
        topIncidents: [],
      })
      .mockResolvedValueOnce({
        generatedAt: '2026-03-08T10:00:00Z',
        totalResources: 4,
        criticalResources: 1,
        warningResources: 3,
        byCategory: { recoverability: 1, protection: 1, rebuild: 1, 'disk-health': 1 },
        byUrgency: { now: 1, today: 2, monitor: 1 },
        sections: [
          {
            category: 'recoverability',
            label: 'Backup & Recoverability',
            resourceCount: 1,
            criticalResources: 1,
            warningResources: 0,
            primaryUrgency: 'now',
            resources: [
              {
                resourceId: 'pbs:main',
                resourceType: 'pbs',
                name: 'pbs-main',
                incidentCount: 1,
                incidentCategory: 'recoverability',
                incidentLabel: 'Backup Coverage At Risk',
                incidentSeverity: 'critical',
                incidentSummary: 'PBS datastore archive is READ_ONLY',
                incidentImpactSummary: 'Puts backups for 3 protected workloads at risk',
                incidentUrgency: 'now',
                incidentAction:
                  'Restore backup target health immediately to protect recoverability',
                protectedWorkloads: 3,
              },
            ],
          },
          {
            category: 'protection',
            label: 'Protection & Redundancy',
            resourceCount: 1,
            criticalResources: 0,
            warningResources: 1,
            primaryUrgency: 'today',
            resources: [
              {
                resourceId: 'storage:tower-array',
                resourceType: 'storage',
                name: 'Tower Array',
                parentName: 'tower',
                platform: 'unraid',
                topology: 'array',
                incidentCount: 1,
                incidentCategory: 'protection',
                incidentLabel: 'Protection Reduced',
                incidentSeverity: 'warning',
                incidentSummary: 'Unraid parity protection is unavailable',
                incidentImpactSummary: 'Affects 2 dependent resources',
                incidentUrgency: 'today',
                incidentAction:
                  'Investigate degraded protection and schedule maintenance to restore redundancy',
                protectionReduced: true,
                consumerCount: 2,
              },
            ],
          },
        ],
      });

    const [lastUpdateToken] = createSignal('2026-03-08T10:00:00Z');
    const [search] = createSignal('');
    const [selectedNodeId] = createSignal('all');
    const [sourceFilter] = createSignal('all');

    render(() => (
      <StorageCommandCenter
        lastUpdateToken={lastUpdateToken}
        search={search}
        selectedNodeId={selectedNodeId}
        sourceFilter={sourceFilter}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Storage Command Center')).toBeInTheDocument();
      expect(
        screen.getByText(/active storage incidents across 9 tracked resources/i),
      ).toBeInTheDocument();
    });

    expect(screen.getByText('Critical')).toBeInTheDocument();
    expect(screen.getAllByText('Protection Reduced').length).toBeGreaterThan(0);
    expect(screen.getByText('Backup & Recoverability')).toBeInTheDocument();
    expect(screen.getByText('Protection & Redundancy')).toBeInTheDocument();
    expect(screen.getByText('pbs-main')).toBeInTheDocument();
    expect(screen.getByText('Tower Array')).toBeInTheDocument();
    expect(screen.getAllByText(/Recommended action:/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/Puts backups for 3 protected workloads at risk/i)).toBeInTheDocument();
    expect(screen.getAllByText(/Protection Reduced/i).length).toBeGreaterThan(0);
  });

  it('passes search, parent, and canonical source filters to storage endpoints', async () => {
    apiFetchJSONMock
      .mockResolvedValueOnce({
        generatedAt: '2026-03-08T10:00:00Z',
        totalResources: 0,
        riskyResources: 0,
        criticalResources: 0,
        warningResources: 0,
        protectionReducedCount: 0,
        rebuildInProgressCount: 0,
        dependentResourceCount: 0,
        protectedWorkloadCount: 0,
        affectedDatastoreCount: 0,
        byPlatform: {},
        byResourceType: {},
        byIncidentCategory: {},
        topIncidents: [],
      })
      .mockResolvedValueOnce({
        generatedAt: '2026-03-08T10:00:00Z',
        totalResources: 0,
        criticalResources: 0,
        warningResources: 0,
        byCategory: {},
        byUrgency: {},
        sections: [],
      });

    const [lastUpdateToken] = createSignal('2026-03-08T10:00:00Z');
    const [search] = createSignal('backup');
    const [selectedNodeId] = createSignal('node-2');
    const [sourceFilter] = createSignal('proxmox-pbs');

    render(() => (
      <StorageCommandCenter
        lastUpdateToken={lastUpdateToken}
        search={search}
        selectedNodeId={selectedNodeId}
        sourceFilter={sourceFilter}
      />
    ));

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    });

    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(
      1,
      '/api/resources/storage-summary?q=backup&parent=node-2&source=pbs',
      { cache: 'no-store' },
    );
    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(
      2,
      '/api/resources/storage-incidents?q=backup&parent=node-2&source=pbs',
      { cache: 'no-store' },
    );
  });
});
