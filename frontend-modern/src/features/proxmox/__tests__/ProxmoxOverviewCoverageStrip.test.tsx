import { render, screen, waitFor } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ProxmoxOverviewCoverageStrip } from '../ProxmoxOverviewCoverageStrip';

const mockApiFetch = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetch: (...args: unknown[]) => mockApiFetch(...(args as [string])),
}));

const workload = (id: string): Resource =>
  ({
    id,
    type: 'vm',
    name: id,
    displayName: id,
    platformId: 'pve-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    sources: ['proxmox'],
    status: 'online',
    lastSeen: Date.now(),
  }) as Resource;

describe('ProxmoxOverviewCoverageStrip', () => {
  it('renders the backup coverage strip once backups resolve', async () => {
    mockApiFetch.mockImplementation(async (url: string) => ({
      ok: true,
      json: async () =>
        url.includes('/pbs')
          ? { data: { backups: [] } }
          : { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } },
    }));

    render(() => <ProxmoxOverviewCoverageStrip workloads={[workload('vm-100')]} />);

    await waitFor(() =>
      expect(screen.getByText('Backup coverage')).toBeInTheDocument(),
    );
  });

  it('renders nothing while backups are still loading', () => {
    // A fetch that never resolves keeps the strip in its loading state.
    mockApiFetch.mockReturnValue(new Promise(() => {}));

    render(() => <ProxmoxOverviewCoverageStrip workloads={[workload('vm-100')]} />);

    expect(screen.queryByText('Backup coverage')).toBeNull();
  });
});
