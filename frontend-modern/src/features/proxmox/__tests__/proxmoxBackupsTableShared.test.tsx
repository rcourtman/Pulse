import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { WorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import proxmoxBackupsTableSharedSource from '../proxmoxBackupsTableShared.tsx?raw';
import {
  ArtifactSourceBadge,
  ArtifactStateBadge,
  ProxmoxBackupWorkloadTypeBadge,
} from '../proxmoxBackupsTableShared';
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
      /inline-flex items-center rounded-sm px-1\.5 py-0\.5 text-\[10px\] font-semibold/,
    );
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
