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
  it('renders single-line rows with identity columns matching the by-date table', () => {
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
    expect(headers).toContain('Type');
    expect(headers).toContain('Target ID');
    expect(headers).toContain('Node');
    expect(headers).toContain('Posture');
    expect(headers).toContain('Restore');
    expect(headers).toContain('PBS snapshot');
    expect(headers).toContain('Guest snapshot');
    expect(headers).not.toContain('PVE file');
    expect(headers).not.toContain('Task');
    // Identity data lives in dedicated cells, not stacked under the name.
    expect(document.body.textContent).toContain('VM');
    expect(document.body.textContent).toContain('100');
    expect(document.body.textContent).toContain('pve1');
    expect(document.body.textContent).not.toContain('ID 100');
    expect(document.body.textContent).not.toContain('Node pve1');
  });
});
