import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PBSJobHealthEvidence } from '@/types/api';
import type { Resource } from '@/types/resource';

import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Existing owner detail</div>,
}));

const job = (
  id: string,
  family: string,
  store: string,
  posture: string,
  postureReason: string,
): PBSJobHealthEvidence => ({
  id,
  family,
  store,
  enabled: true,
  confidence: 'config-last-run',
  posture,
  postureReason,
});

const server = (connectionHealth = 'healthy'): Resource => ({
  id: 'resource-vault',
  name: 'vault',
  displayName: 'vault',
  type: 'pbs',
  status: 'online',
  lastSeen: 1_793_300_000_000,
  platformId: 'proxmox-pbs',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  pbs: {
    instanceId: 'pbs-vault',
    connectionHealth,
    datastores: [
      { name: 'local', total: 1000, used: 400 },
      { name: 'offsite', total: 1000, used: 400 },
    ],
    jobHealthEvidence: [
      job('garbage:local', 'garbage', 'local', 'healthy', 'last-run-success'),
      job('s-offsite', 'sync', 'offsite', 'critical', 'last-run-failed'),
      job('v-offsite', 'verify', 'offsite', 'warning', 'job-overdue'),
    ],
  },
});

const jobCells = (container: HTMLElement) =>
  [...container.querySelectorAll('[data-backup-server-jobs]')].map((cell) => [
    cell.getAttribute('data-backup-server-jobs'),
    cell.textContent?.trim(),
  ]);

afterEach(cleanup);

describe('Backup server job results', () => {
  it('names the worst job per datastore in its own column once there is room', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server()]} layoutWidth={() => 600} />
    ));
    expect([...container.querySelectorAll('thead th')].map((th) => th.textContent)).toContain(
      'Jobs',
    );
    expect(jobCells(container)).toEqual([
      ['ok', 'OK'],
      ['critical', 'Sync failed'],
    ]);
    const failed = container.querySelector('[data-backup-server-jobs="critical"]')!;
    expect(failed.getAttribute('title')).toBe('Sync failed · Verify late');
  });

  it('puts a job problem in State on phones, where there is no Jobs column', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server()]} layoutWidth={() => 375} />
    ));
    const rows = [...container.querySelectorAll('tbody tr')].filter(
      (row) => row.querySelectorAll('td').length > 1,
    );
    expect(rows[0].querySelectorAll('td')[1]).toHaveTextContent('Healthy');
    expect(rows[1].querySelectorAll('td')[1]).toHaveTextContent(/^Sync failed$/);
  });

  it('keeps an unreachable server in State on phones, ahead of its jobs', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server('unreachable')]} layoutWidth={() => 375} />
    ));
    expect(jobCells(container)).toEqual([]);
    expect(container.querySelector('tbody td:nth-child(2)')).toHaveTextContent('Unreachable');
  });
});
