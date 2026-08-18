import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import type { Accessor } from 'solid-js';

import { ProxmoxRecoverableTable } from '../ProxmoxRecoverableTable';
import type { RecoverableArtifact } from '../proxmoxBackupRecoveryModel';
import type { RecoverableSortKey } from '../proxmoxBackupsTableModel';

const artifact: RecoverableArtifact = {
  id: 'pbs:web:100',
  nativeId: 'vm/100/2026-08-04T08:00:00Z',
  sourceKind: 'pbs',
  sourceLabel: 'PBS',
  sourceTitle: 'Proxmox Backup Server',
  workload: {
    key: 'vm:100',
    type: 'vm',
    typeLabel: 'VM',
    vmid: '100',
    label: 'web (VM 100)',
    name: 'web',
    node: 'pve1',
  },
  createdAt: '2026-08-04T08:00:00Z',
  createdMs: Date.parse('2026-08-04T08:00:00Z'),
  size: 4_000_000_000,
  location: 'main / pve1',
  detail: '5 PBS files',
  protected: true,
  verified: true,
};

const renderTable = (layoutWidth: number) =>
  render(() => (
    <ProxmoxRecoverableTable
      artifacts={[artifact]}
      hasAnyArtifacts
      emptyIcon={<span />}
      emptyTitle=""
      emptyDescription=""
      sortKey={(() => 'created') as Accessor<RecoverableSortKey>}
      sortDirection={() => 'desc'}
      onSort={() => {}}
      sizeMaxBytes={artifact.size ?? 0}
      layoutWidth={() => layoutWidth}
    />
  ));

afterEach(cleanup);

describe('ProxmoxRecoverableTable responsive columns', () => {
  it('keeps source, location, recovery age, and state visible in compact containers', () => {
    renderTable(330);

    expect([...document.querySelectorAll('thead th')].map((th) => th.textContent?.trim())).toEqual([
      'Workload',
      'Via',
      'Loc',
      'Age',
      'State',
    ]);
    expect(document.body.textContent).toContain('VM 100');
    expect(document.body.textContent).toContain('main / pve1');
    expect(document.body.textContent).not.toContain('5 PBS files');
  });

  it('restores every metadata column when the table has full width', () => {
    renderTable(1_200);

    expect([...document.querySelectorAll('thead th')].map((th) => th.textContent?.trim())).toEqual([
      'Workload',
      'Type',
      'Target ID',
      'Source',
      'Location',
      'Created',
      'Size',
      'State',
      'Details',
    ]);
    expect(document.body.textContent).toContain('main / pve1');
    expect(document.body.textContent).toContain('5 PBS files');
  });
});
