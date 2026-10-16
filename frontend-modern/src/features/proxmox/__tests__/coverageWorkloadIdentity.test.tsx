import { cleanup, fireEvent, render } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { ProxmoxCoverageTable } from '../ProxmoxCoverageTable';
import type { WorkloadCoverageRow } from '../proxmoxBackupRecoveryModel';

const coverage = (key: string, vmid: string, instance?: string): WorkloadCoverageRow => ({
  key,
  workload: {
    key,
    name: 'database',
    label: `database (VM ${vmid})`,
    type: 'vm',
    typeLabel: 'VM',
    vmid,
    instance,
    node: 'Display node',
    nativeNode: 'pve-01.example.test',
  },
  artifacts: [],
  pbsCount: 0,
  archiveCount: 0,
  snapshotCount: 0,
  posture: 'unknown',
  postureRank: 2,
  isOrphaned: false,
});

function Table(props: {
  rows: WorkloadCoverageRow[];
  width: number;
  expanded?: ReadonlySet<string>;
}) {
  return (
    <ProxmoxCoverageTable
      rows={props.rows}
      hasAnyRows
      emptyIcon={<span />}
      emptyTitle=""
      emptyDescription=""
      sortKey={() => 'posture'}
      sortDirection={() => 'asc'}
      onSort={() => {}}
      expandedKeys={props.expanded ?? new Set<string>()}
      onToggleExpand={() => {}}
      showTaskColumn
      layoutWidth={() => props.width}
    />
  );
}

afterEach(cleanup);

describe('Coverage workload identity without a hover or disclosure', () => {
  it.each([320, 600, 800, 1000, 1200])(
    'keeps equal names and repeated target IDs distinct at %i px',
    (width) => {
      render(() => (
        <Table
          width={width}
          rows={[
            coverage('a', '101', 'production'),
            coverage('b', '102', 'production'),
            coverage('c', '101', 'recovery'),
          ]}
        />
      ));
      const rows = [...document.querySelectorAll('[data-proxmox-backup-row="coverage"]')];
      expect(rows).toHaveLength(3);
      for (const [index, row] of rows.entries()) {
        expect(row).toHaveTextContent(index === 1 ? '102' : '101');
        expect(row.querySelector('[data-backup-workload-connection]')).toHaveTextContent(
          index === 2 ? 'recovery' : 'production',
        );
        const identity = row.querySelector('[data-backup-workload-target]');
        if (width < 1120) expect(identity).toHaveTextContent(`VM ${index === 1 ? '102' : '101'}`);
        else expect(identity).toBeNull();
        const node = row.querySelector('[data-backup-workload-node]');
        if (width < 480) expect(node).toHaveTextContent('pve-01.example.test');
        else expect(node).toBeNull();
      }
      expect(new Set(rows.map((row) => row.textContent)).size).toBe(3);
      expect(document.querySelector('[data-inline-detail-for]')).toBeNull();
    },
  );

  it('keeps the focused disclosure while refreshed identity changes in place', async () => {
    const [rows, setRows] = createSignal([coverage('a', '101', 'old-connection')]);
    render(() => <Table width={320} rows={rows()} expanded={new Set(['a'])} />);
    const row = document.querySelector('[data-proxmox-backup-row="coverage"]');
    const toggle = row!.querySelector('button')!;
    toggle.focus();
    setRows([
      {
        ...coverage('a', '102', 'new-connection'),
        workload: {
          ...coverage('a', '102', 'new-connection').workload,
          nativeNode: 'pve-02.example.test',
        },
      },
    ]);
    await fireEvent.focus(toggle);
    expect(document.querySelector('[data-proxmox-backup-row="coverage"]')).toBe(row);
    expect(document.activeElement).toBe(toggle);
    expect(row).toHaveTextContent('VM 102');
    expect(row).toHaveTextContent('new-connection');
    expect(row).toHaveTextContent('pve-02.example.test');
    expect(row).not.toHaveTextContent('old-connection');
    expect(document.querySelector('[data-inline-detail-for="a"]')).toHaveTextContent(
      'Connection: new-connection',
    );
  });

  it('does not substitute a repository or display name for an unknown connection', () => {
    render(() => (
      <Table width={320} rows={[coverage('orphan', '101')]} expanded={new Set(['orphan'])} />
    ));
    expect(document.querySelector('[data-backup-workload-connection]')).toBeNull();
    expect(document.querySelector('[data-inline-detail-for]')).toHaveTextContent(
      'Connection: Unknown',
    );
    expect(document.querySelector('[data-backup-workload-node]')).toHaveTextContent(
      'pve-01.example.test',
    );
  });
});
