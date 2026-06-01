import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import type { Accessor } from 'solid-js';

import { ProxmoxCoverageTable } from '../ProxmoxCoverageTable';
import type { WorkloadCoverageRow } from '../proxmoxBackupRecoveryModel';
import type { CoverageSortKey } from '../proxmoxBackupsTableModel';

const row = {
  key: 'w1',
  workload: {
    key: 'w1',
    type: 'vm',
    typeLabel: 'VM',
    vmid: '100',
    label: 'web (VM 100)',
    name: 'web',
    node: 'pve1',
  },
  artifacts: [],
  pbsCount: 1,
  archiveCount: 0,
  snapshotCount: 0,
  posture: 'current',
  postureRank: 0,
} as unknown as WorkloadCoverageRow;

const headerTexts = () =>
  [...document.querySelectorAll('thead th')].map((th) => th.textContent?.trim() ?? '');

afterEach(cleanup);

describe('ProxmoxCoverageTable column visibility', () => {
  it('renders only action columns and keeps identity context in the workload cell', () => {
    render(() => (
      <ProxmoxCoverageTable
        rows={[row]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKeys={new Set<string>()}
        onToggleExpand={() => {}}
        showPbsColumn={true}
        showArchiveColumn={false}
        showSnapshotColumn={true}
        showTaskColumn={false}
      />
    ));

    const headers = headerTexts();
    expect(headers).toContain('Workload');
    expect(headers).toContain('Posture');
    expect(headers).toContain('Restore');
    expect(headers).not.toContain('Type');
    expect(headers).not.toContain('Target ID');
    expect(headers).not.toContain('Node');
    expect(headers).toContain('PBS snapshot');
    expect(headers).toContain('Guest snapshot');
    expect(headers).not.toContain('PVE file');
    expect(headers).not.toContain('Task');
    expect(document.body.textContent).toContain('VM');
    expect(document.body.textContent).toContain('ID 100');
    expect(document.body.textContent).toContain('Node pve1');
  });
});
