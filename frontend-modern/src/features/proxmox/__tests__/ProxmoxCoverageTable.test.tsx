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
  posture: 'protected',
  postureRank: 0,
  protectionPosture: {
    subjectResourceId: 'resource:vm:100',
    state: 'protected',
    freshness: 'current',
    verification: 'verified',
    coverage: 'complete',
    providerStates: [
      {
        provider: 'proxmox-pbs',
        source: 'pbs-backup-enumeration',
        scope: 'pbs-main',
        jobState: 'success',
        historyCompleteness: 'complete',
        permissions: 'sufficient',
        evidenceIds: ['evidence-provider'],
      },
    ],
    repositoryResourceIds: [],
    evidenceIds: ['evidence-provider'],
    explanation: 'A current verified backup is available from complete provider history.',
    evaluatedAt: '2026-07-19T00:00:00Z',
  },
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
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    const headers = headerTexts();
    expect(headers).toContain('Workload');
    expect(headers).toContain('Type');
    expect(headers).toContain('Target ID');
    expect(headers).toContain('Node');
    expect(headers).toContain('Posture▲');
    expect(headers).toContain('Last backup');
    // Per-source ages are expansion detail at every width.
    expect(headers).not.toContain('PBS snapshot');
    expect(headers).not.toContain('Guest snapshot');
    expect(headers).not.toContain('PVE file');
    expect(headers).not.toContain('Task');
    // Identity data lives in dedicated cells, not stacked under the name.
    expect(document.body.textContent).toContain('VM');
    expect(document.body.textContent).toContain('100');
    expect(document.body.textContent).toContain('pve1');
    expect(document.body.textContent).not.toContain('ID 100');
    expect(document.body.textContent).not.toContain('Node pve1');
  });

  it('keeps posture, backup age and job visible in compact rows', () => {
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
        showTaskColumn={true}
        layoutWidth={() => 330}
      />
    ));

    expect(headerTexts()).toEqual(['Workload', 'Posture▲', 'Age', 'Job']);
    expect(document.body.textContent).toContain('Prot.');
    expect(document.body.textContent).not.toContain('VM 100 · pve1');
  });

  it('keeps provider evidence in the workload drill-down instead of every table row', () => {
    const { unmount } = render(() => (
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
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    expect(document.body.textContent).not.toContain('Provider evidence');
    expect(document.body.textContent).not.toContain(
      'A current verified backup is available from complete provider history.',
    );
    unmount();

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
        expandedKeys={new Set<string>(['w1'])}
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    expect(document.body.textContent).toContain('Provider evidence');
    expect(document.body.textContent).toContain('Proxmox Backup Server');
    expect(document.body.textContent).toContain('History Complete');
    expect(document.body.textContent).toContain('Access Sufficient');
  });

  it('windows large coverage result sets while keeping one continuous table', () => {
    const rows = Array.from({ length: 600 }, (_, index) => ({
      ...row,
      key: `w${index}`,
      workload: {
        ...row.workload,
        key: `w${index}`,
        vmid: String(100 + index),
        label: `workload-${index}`,
        name: `workload-${index}`,
      },
    })) as WorkloadCoverageRow[];

    render(() => (
      <ProxmoxCoverageTable
        rows={rows}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKeys={new Set<string>()}
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    expect(document.querySelector('[data-proxmox-backups-table="coverage"]')).toHaveAttribute(
      'data-proxmox-backups-windowed',
      'true',
    );
    expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(140);
    expect(document.body.textContent).toContain('workload-0');
    expect(document.body.textContent).not.toContain('workload-599');
  });

  it('marks an empty evidence cell with a short label and the full reason on hover', () => {
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
        showTaskColumn={true}
        layoutWidth={() => 1_200}
      />
    ));

    // The cell says None and the full reason sits on hover, so it cannot
    // truncate to "No PBS snapshot or PVE b…".
    const empty = (title: string) => document.querySelector(`td span[title="${title}"]`);
    for (const title of ['No PBS snapshot or PVE backup file', 'No recent task']) {
      expect(empty(title)).toHaveTextContent(/^None$/);
    }
  });

  it('explains snapshot-only and unrated rows in plain words', () => {
    render(() => (
      <ProxmoxCoverageTable
        rows={[
          { ...row, key: 'snapshot-only', snapshotCount: 2 },
          { ...row, key: 'unrated', posture: 'not-evaluated', protectionPosture: undefined },
        ]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKeys={new Set<string>()}
        onToggleExpand={() => {}}
        showTaskColumn={true}
        layoutWidth={() => 1_200}
      />
    ));

    const titles = [...document.querySelectorAll('td span[title]')].map((span) =>
      span.getAttribute('title'),
    );
    expect(titles).toContain(
      'No PBS snapshot or PVE backup file. A snapshot is not a separate backup, so it does not count.',
    );
    expect(titles).toContain(
      'This backup does not match a guest Pulse currently monitors, so it is not rated.',
    );
    expect(titles.join(' ')).not.toMatch(/canonical|provider evidence|independent recovery/);
  });

  it('keeps a long node name truncating inside its own column', () => {
    // A capped inline-block sat above the row's text line; a block span fills
    // the fixed-layout cell and ends in an ellipsis there.
    const node = 'pve-production-rack-04-node-17.example.internal';
    render(() => (
      <ProxmoxCoverageTable
        rows={[{ ...row, workload: { ...row.workload, node } }] as WorkloadCoverageRow[]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKeys={new Set<string>()}
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    const span = document.querySelector(`td span[title="${node}"]`);
    expect(span).toHaveTextContent(node);
    expect(span?.parentElement?.children).toHaveLength(1);
    expect(span?.getAttribute('class')).toBe('block truncate');
    expect(span?.closest('table')).toHaveClass('table-fixed');
  });
});
