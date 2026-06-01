import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { WorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import proxmoxBackupsTableSharedSource from '../proxmoxBackupsTableShared.tsx?raw';
import { ProxmoxBackupWorkloadTypeBadge } from '../proxmoxBackupsTableShared';

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
});
