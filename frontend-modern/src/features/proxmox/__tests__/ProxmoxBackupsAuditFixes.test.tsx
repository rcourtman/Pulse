import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  ProxmoxBackupServersTable,
  effectiveBackupServerStatus,
} from '../ProxmoxBackupServersTable';
import { ProxmoxBackupsCoverageStrip } from '../ProxmoxBackupsCoverageStrip';
import { ProxmoxRecoverableTable } from '../ProxmoxRecoverableTable';
import { getRecoverableColumnWidthStyle } from '../proxmoxBackupsTablePresentation';
import type { RecoverableArtifact } from '../proxmoxBackupRecoveryModel';
import { collectDatastoreJobFailures } from '../proxmoxBackupServerJobs';
import type { BackupVerdict } from '../proxmoxBackupVerdict';

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isAtLeast: () => false,
    isMobile: () => true,
  }),
}));

const pbsServerWithFailedSync = (id: string, name: string): Resource =>
  ({
    id,
    type: 'pbs',
    name,
    displayName: name,
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    pbs: {
      instanceId: `pbs-${name}`,
      connectionHealth: 'healthy',
      datastores: [
        {
          name: 'primary-vault',
          total: 2_000_000_000_000,
          used: 500_000_000_000,
          status: 'online',
        },
      ],
      jobHealthEvidence: [
        {
          family: 'sync',
          id: 's-primary-vault',
          store: 'primary-vault',
          posture: 'critical',
          postureReason: 'failed',
          lastRunAt: '2026-11-23T18:00:00Z',
        },
      ],
    },
  }) as unknown as Resource;

describe('Proxmox backups and PBS audit fixes', () => {
  afterEach(cleanup);

  it('turns PBS server and datastore status dots non-green when a datastore sync job fails', () => {
    expect(
      effectiveBackupServerStatus('success', 'Healthy', [
        { state: 'critical', label: 'Sync failed', entries: [] },
      ]),
    ).toEqual({ variant: 'danger', label: 'Sync failed' });

    expect(
      effectiveBackupServerStatus('success', 'Healthy', [
        { state: 'warning', label: 'Verify warning', entries: [] },
      ]),
    ).toEqual({ variant: 'warning', label: 'Verify warning' });

    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[pbsServerWithFailedSync('pbs:dr-vault', 'dr-vault')]}
        backups={[]}
      />
    ));

    const groupRow = container.querySelector(
      '[data-backup-server-group="dr-vault"]',
    ) as HTMLElement;
    const dot = within(groupRow).getByRole('img', { name: 'Sync failed' });
    expect(dot.className).toContain('bg-red-500');
    expect(dot.className).not.toContain('bg-emerald-500');
  });

  it('shows In progress instead of Unknown size for running backups in By date table and drawer', () => {
    const runningArtifact: RecoverableArtifact = {
      id: 'running-1',
      nativeId: 'task-1',
      sourceKind: 'pbs',
      sourceLabel: 'PBS',
      workload: {
        key: 'pve1:qemu:101',
        type: 'vm',
        typeLabel: 'VM',
        vmid: '101',
        label: 'checkout-web-02',
        name: 'checkout-web-02',
        node: 'west-a',
      },
      createdAt: '2026-11-23T18:30:00Z',
      createdMs: Date.parse('2026-11-23T18:30:00Z'),
      size: 0,
      location: 'backup-vault / primary-vault',
      detail: 'Running PBS backup',
      protected: false,
      running: true,
    };

    render(() => (
      <ProxmoxRecoverableTable
        artifacts={[runningArtifact]}
        hasAnyArtifacts
        emptyIcon={<span />}
        emptyTitle="No backups"
        emptyDescription="No backups"
        sortKey={() => 'created'}
        sortDirection={() => 'desc'}
        onSort={() => undefined}
        sizeMaxBytes={1024}
        layoutWidth={() => 1280}
      />
    ));

    expect(screen.getByText('In progress')).toBeInTheDocument();
    expect(screen.queryByText('Unknown size')).toBeNull();
    fireEvent.click(screen.getByText('checkout-web-02'));
    expect(screen.getAllByText('In progress').length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByText('Unknown size')).toBeNull();
    expect(
      screen.getByText('Size will be reported after the backup finishes.'),
    ).toBeInTheDocument();
  });

  it('preserves full server context on Datastore jobs rows without single-line truncation', () => {
    const failures = collectDatastoreJobFailures([
      pbsServerWithFailedSync('pbs:backup-vault', 'backup-vault'),
      pbsServerWithFailedSync('pbs:dr-vault', 'dr-vault'),
    ]);
    expect(failures[0]?.datastores).toEqual([
      'primary-vault (backup-vault)',
      'primary-vault (dr-vault)',
    ]);

    const verdict: BackupVerdict = {
      tone: 'protected',
      headline: '10 / 10 protected',
      protected: 10,
      rated: 10,
      checking: 0,
      notEvaluated: 0,
      exceptions: [],
    };
    const onSelectDatastoreProblem = vi.fn();

    const { container } = render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={verdict}
        datastoreProblems={failures}
        onSelectDatastoreProblem={onSelectDatastoreProblem}
      />
    ));

    const names = container.querySelector('[data-backup-datastore-names]') as HTMLElement;
    expect(names).toHaveTextContent('primary-vault (backup-vault), primary-vault (dr-vault)');
    expect(names).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(names).not.toHaveClass('truncate');

    fireEvent.click(screen.getByRole('button', { name: /Sync failed on 2 datastores/i }));
    expect(onSelectDatastoreProblem).toHaveBeenCalledTimes(1);
  });

  it('gives the By date Workload column generous desktop width and prioritizes the workload name over the connection label', () => {
    const fullWidth = Number.parseFloat(
      String(getRecoverableColumnWidthStyle('workload', 'full').width ?? '0'),
    );
    expect(fullWidth).toBeGreaterThanOrEqual(22);

    const artifact: RecoverableArtifact = {
      id: 'rec-1',
      nativeId: 'backup-1',
      sourceKind: 'pbs',
      sourceLabel: 'PBS',
      workload: {
        key: 'pve1:qemu:101',
        type: 'vm',
        typeLabel: 'VM',
        vmid: '101',
        label: 'checkout-web-02',
        name: 'checkout-web-02',
        node: 'west-a',
        instance: 'Production West',
      },
      createdAt: '2026-11-23T18:30:00Z',
      createdMs: Date.parse('2026-11-23T18:30:00Z'),
      size: 1024 * 1024,
      location: 'backup-vault / primary-vault',
      detail: 'PBS backup',
      protected: true,
      verified: true,
    };

    render(() => (
      <ProxmoxRecoverableTable
        artifacts={[artifact]}
        hasAnyArtifacts
        emptyIcon={<span />}
        emptyTitle="No backups"
        emptyDescription="No backups"
        sortKey={() => 'created'}
        sortDirection={() => 'desc'}
        onSort={() => undefined}
        sizeMaxBytes={1024 * 1024}
        layoutWidth={() => 1440}
        showWorkloadConnection
      />
    ));

    const nameEl = screen.getByText('checkout-web-02');
    expect(nameEl).toHaveClass('shrink-0', 'max-w-[72%]');
    expect(screen.getByText('Production West')).toBeInTheDocument();
  });
});
