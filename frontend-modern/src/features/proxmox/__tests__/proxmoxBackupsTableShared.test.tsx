import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { WorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import proxmoxBackupServersTableSource from '../ProxmoxBackupServersTable.tsx?raw';
import proxmoxCoverageTableSource from '../ProxmoxCoverageTable.tsx?raw';
import proxmoxRecoverableTableSource from '../ProxmoxRecoverableTable.tsx?raw';
import proxmoxBackupsTableSharedSource from '../proxmoxBackupsTableShared.tsx?raw';
import {
  ArtifactSourceBadge,
  ArtifactStateBadge,
  ProxmoxBackupAgeText,
  ProxmoxBackupWorkloadTypeBadge,
  SortableHead,
  formatCompactBackupAge,
} from '../proxmoxBackupsTableShared';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import type { RecoverableArtifact } from '../proxmoxBackupRecoveryModel';

afterEach(cleanup);

describe('proxmoxBackupsTableShared', () => {
  it('maps PBS ct rows onto the same LXC badge used by the workload overview', () => {
    render(() => (
      <>
        <WorkloadTypeBadge type="system-container" />
        <ProxmoxBackupWorkloadTypeBadge type="ct" label="LXC" />
      </>
    ));

    const [overviewBadge, backupBadge] = screen.getAllByText('LXC');
    expect(backupBadge.className).toBe(overviewBadge.className);
  });

  it('keeps host backup labels on the shared host/agent tone instead of local backup colors', () => {
    render(() => (
      <>
        <WorkloadTypeBadge type="agent" label="Host" title="Host backup" />
        <ProxmoxBackupWorkloadTypeBadge type="host" label="Host" />
      </>
    ));

    const [sharedBadge, backupBadge] = screen.getAllByText('Host');
    expect(backupBadge.className).toBe(sharedBadge.className);
    expect(backupBadge).toHaveAttribute('title', 'Host backup');
  });

  it('does not carry local VM/LXC/Host badge color branches', () => {
    expect(proxmoxBackupsTableSharedSource).toContain('SharedWorkloadTypeBadge');
    expect(proxmoxBackupsTableSharedSource).not.toContain('bg-indigo');
    expect(proxmoxBackupsTableSharedSource).not.toContain('bg-teal');
    expect(proxmoxBackupsTableSharedSource).not.toContain('bg-slate');
  });

  it('renders backup source and state chips through MetadataBadge', () => {
    render(() => (
      <>
        <ArtifactSourceBadge artifact={artifact({ sourceKind: 'pbs', sourceLabel: 'PBS' })} />
        <ArtifactStateBadge artifact={artifact({ protected: true })} label="Protected" />
        <ArtifactStateBadge artifact={artifact({ verified: true })} label="Verified" />
      </>
    ));

    expect(screen.getByText('PBS').className).toContain('whitespace-nowrap');
    expect(screen.getByText('PBS').className).toContain('bg-sky-100');
    expect(screen.getByText('Protected').className).toContain('bg-amber-100');
    expect(screen.getByText('Verified').className).toContain('bg-emerald-100');
  });

  it('keeps backup source and state chips on the shared MetadataBadge primitive', () => {
    expect(proxmoxBackupsTableSharedSource).toContain('MetadataBadge');
    expect(proxmoxBackupsTableSharedSource).toContain('PROXMOX_BACKUP_METADATA_BADGE_PROPS');
    expect(proxmoxBackupsTableSharedSource).toContain('presentation().badgeTone');
    expect(proxmoxBackupsTableSharedSource).not.toContain('presentation().badgeClassName');
    expect(proxmoxBackupsTableSharedSource).not.toMatch(
      /inline-flex items-center rounded-xs px-1\.5 py-0\.5 text-\[10px\] font-semibold/,
    );
  });

  it('keeps backup byte-size cells on the shared platform table formatter', () => {
    const sources = [
      proxmoxBackupServersTableSource,
      proxmoxCoverageTableSource,
      proxmoxRecoverableTableSource,
    ];

    for (const source of sources) {
      expect(source).toContain('formatPlatformTableBytesValue');
      expect(source).not.toContain('formatBytes(');
    }
  });

  it('drops the age suffix in the phone projection and keeps the timestamp on hover', () => {
    const createdAt = new Date(Date.now() - 18 * 60 * 60 * 1000).toISOString();
    expect(formatCompactBackupAge(createdAt)).toBe('18h');
    expect(formatCompactBackupAge(new Date().toISOString())).toBe('now');

    const artifact = {
      id: 'pbs:1',
      nativeId: '1',
      sourceKind: 'pbs',
      sourceLabel: 'PBS',
      workload: { key: 'w', type: 'vm', typeLabel: 'VM', vmid: '100', label: 'VM 100' },
      createdAt,
      createdMs: Date.parse(createdAt),
      location: 'main',
      detail: '',
      protected: false,
    } as RecoverableArtifact;
    render(() => <ProxmoxBackupAgeText artifact={artifact} compact />);
    const age = screen.getByText('18h');
    expect(age.closest('[title]')?.getAttribute('title')).toContain(createdAt);
  });

  it('keeps backup age text on the shared relative-time primitive', () => {
    expect(proxmoxBackupsTableSharedSource).toContain('PlatformTableRelativeTimeValue');
    expect(proxmoxBackupsTableSharedSource).not.toContain('formatRelativeTime(');
  });

  it('keeps backup sort headers on the shared active-only indicator', () => {
    expect(proxmoxBackupsTableSharedSource).toContain('getTableSortIndicator');
    expect(proxmoxBackupsTableSharedSource).not.toContain('ArrowUpDownIcon');
    expect(proxmoxBackupsTableSharedSource).not.toContain('SORT_ICON_CLASS');
  });

  it('keeps sortable headers in the same uppercase as their plain neighbours', () => {
    render(() => (
      <table>
        <thead>
          <tr>
            <SortableHead
              label="Workload"
              sortKey="workload"
              currentSort={() => 'posture'}
              direction={() => 'asc'}
              onSort={() => undefined}
              headClass=""
            />
          </tr>
        </thead>
      </table>
    ));

    // A button resets text-transform, so the header casing must be restated.
    expect(screen.getByRole('button', { name: 'Sort by Workload' })).toHaveClass('uppercase');
  });
});

function artifact(overrides: Partial<RecoverableArtifact> = {}): RecoverableArtifact {
  return {
    id: 'artifact-1',
    nativeId: 'backup/vm/100/2026-01-01',
    sourceKind: 'archive',
    sourceLabel: 'PVE file',
    workload: {
      key: 'vm:100',
      type: 'vm',
      typeLabel: 'VM',
      vmid: '100',
      label: 'vm-100',
    },
    createdAt: '2026-01-01T00:00:00Z',
    createdMs: Date.parse('2026-01-01T00:00:00Z'),
    location: 'local',
    detail: 'vzdump-qemu-100.vma.zst',
    protected: false,
    ...overrides,
  };
}

describe('backup-date-evidence age cell', () => {
  const now = Date.parse('2026-10-04T12:00:00Z');
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(now);
  });
  afterEach(() => vi.useRealTimers());
  for (const compact of [false, true]) {
    it.each(['not-a-date', '0001-01-01T00:00:00Z', '2026-10-04T12:00:01Z'])(
      '[regression] labels %s unknown in ' + (compact ? 'compact' : 'full') + ' age cells',
      (createdAt) => {
        render(() => (
          <ProxmoxBackupAgeText
            artifact={artifact({ createdAt, createdMs: Date.parse(createdAt) })}
            compact={compact}
          />
        ));
        const value = screen.getByText('Unknown');
        expect(value).toHaveClass('text-amber-600');
        if (compact) expect(value).toHaveClass('text-[10px]');
        expect(value).toHaveAccessibleName(/Unknown age/);
        expect(value.title).not.toContain(createdAt);
        expect(value.title).toContain('unavailable or in the future');
      },
    );
  }
  it('[control] valid completed dates retain relative age and their reported timestamp', () => {
    const createdAt = '2026-10-04T11:00:00Z';
    render(() => (
      <ProxmoxBackupAgeText
        artifact={artifact({ createdAt, createdMs: Date.parse(createdAt) })}
        compact
      />
    ));
    const value = screen.getByText('1h');
    expect(value).toHaveClass('text-emerald-600');
    expect(value.title).toContain(createdAt);
  });
});

describe('backup age cell on the shared clock', () => {
  afterEach(() => vi.useRealTimers());

  it('keeps the age and its freshness band moving while the artifact does not change', () => {
    vi.useFakeTimers({ now: new Date('2026-10-04T12:00:00Z') });
    const createdAt = '2026-10-04T06:00:00Z';
    const backup = artifact({ createdAt, createdMs: Date.parse(createdAt) });

    render(() => (
      <>
        <span data-testid="full">
          <ProxmoxBackupAgeText artifact={backup} />
        </span>
        <span data-testid="compact">
          <ProxmoxBackupAgeText artifact={backup} compact />
        </span>
      </>
    ));

    expect(screen.getByTestId('full')).toHaveTextContent('6h ago');
    expect(screen.getByText('6h')).toHaveClass('text-emerald-600');

    // Eight days pass with no new backup and no data refresh. The next clock
    // tick moves both ages and drops the backup out of the current band.
    vi.setSystemTime(new Date('2026-10-12T12:00:00Z'));
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);

    expect(screen.getByTestId('full')).toHaveTextContent('8d ago');
    const compact = screen.getByText('8d');
    expect(compact).toHaveClass('text-amber-600');
    expect(compact.closest('[title]')?.getAttribute('title')).toBe(
      `Aging backup age · ${createdAt}`,
    );
  });

  it('keeps the full age text and its band in step between clock ticks', () => {
    vi.useFakeTimers({ now: new Date('2026-10-04T12:00:00Z') });
    // Another mounted cell starts the shared clock.
    render(() => <ProxmoxBackupAgeText artifact={artifact()} />);

    // Before the next tick, a backup crosses seven days old as its row mounts.
    vi.setSystemTime(new Date('2026-10-11T12:00:10Z'));
    const createdAt = '2026-10-04T12:00:00Z';
    render(() => (
      <span data-testid="boundary">
        <ProxmoxBackupAgeText
          artifact={artifact({ createdAt, createdMs: Date.parse(createdAt) })}
        />
      </span>
    ));

    const cell = within(screen.getByTestId('boundary')).getByTitle(/backup age/);
    expect(cell).toHaveTextContent('7d ago');
    expect(cell).toHaveClass('text-amber-600');
  });

  it('does not band a backup that finished after the last clock tick as unknown', () => {
    vi.useFakeTimers({ now: new Date('2026-10-04T12:00:00Z') });
    // Another mounted cell starts the shared clock at 12:00:00.
    render(() => <ProxmoxBackupAgeText artifact={artifact()} />);

    // A backup that finished 20s later arrives before the next tick.
    vi.setSystemTime(new Date('2026-10-04T12:00:25Z'));
    const createdAt = '2026-10-04T12:00:20Z';
    render(() => (
      <span data-testid="fresh">
        <ProxmoxBackupAgeText
          artifact={artifact({ createdAt, createdMs: Date.parse(createdAt) })}
          compact
        />
      </span>
    ));

    const fresh = within(screen.getByTestId('fresh')).getByText('now');
    expect(fresh).toHaveClass('text-emerald-600');
    expect(screen.queryByText('Unknown')).not.toBeInTheDocument();
  });
});
