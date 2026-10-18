import { cleanup, fireEvent, render } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal, type Accessor } from 'solid-js';

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
  it.each([320, 1200])(
    'shows the recorded job location, then clears it in place instead of borrowing the guest location at %i px',
    (width) => {
      const node = 'old-backup-node-<img src="/unexpected">';
      const [current, setCurrent] = createSignal<WorkloadCoverageRow>({
        ...row,
        latestTask: {
          id: 'warning',
          instance: 'original-connection',
          node,
          status: 'warning',
          label: 'Warning',
          startedAt: '2026-05-25T02:00:00Z',
          error: 'Backup completed with warnings; review the original Proxmox task log.',
        },
      });
      const { container } = render(() => (
        <ProxmoxCoverageTable
          rows={[current()]}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'posture'}
          sortDirection={() => 'asc'}
          onSort={() => {}}
          expandedKeys={new Set(['w1'])}
          onToggleExpand={() => {}}
          showTaskColumn
          layoutWidth={() => width}
        />
      ));
      const detail = container.querySelector('[data-backup-task-detail]')!;
      const value = (label: string) =>
        [...detail.querySelectorAll('dt')].find((dt) => dt.textContent === label)
          ?.nextElementSibling;
      expect(value('Job connection')).toHaveTextContent('original-connection');
      expect(value('Job node')).toHaveTextContent(node);
      expect(value('Reported warning')).toHaveTextContent('Backup completed with warnings');
      expect(value('Reported error')).toBeUndefined();
      expect(detail.querySelector('img')).toBeNull();
      expect(detail).toHaveTextContent('The workload may now be on another node.');
      setCurrent({
        ...row,
        latestTask: {
          id: 'next-job',
          instance: '',
          node: '  ',
          status: 'OK',
          label: 'OK',
          startedAt: '2026-05-25T03:00:00Z',
        },
      });
      expect(container.querySelector('[data-backup-task-detail]')).toBe(detail);
      expect(value('Job connection')).toHaveTextContent(/^Not reported$/);
      expect(value('Job node')).toHaveTextContent(/^Not reported$/);
      expect(value('Reported warning')).toBeUndefined();
      expect(detail).not.toHaveTextContent('original-connection');
      expect(detail).not.toHaveTextContent(node);
      expect(detail).not.toHaveTextContent(row.workload.node!);
    },
  );

  it.each([320, 1200])(
    'keeps a contradictory job Unknown with literal evidence at %i px, then recovers in place',
    (width) => {
      const error = 'Synthetic failure <img src="/unexpected" onerror="unsafe()">';
      const [current, setCurrent] = createSignal<WorkloadCoverageRow>({
        ...row,
        latestTask: {
          id: 'contradictory',
          status: 'completed',
          label: 'Unknown',
          startedAt: '2026-05-25T02:00:00Z',
          error,
          statusErrorConflict: true,
        },
      });
      const { container } = render(() => (
        <ProxmoxCoverageTable
          rows={[current()]}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'posture'}
          sortDirection={() => 'asc'}
          onSort={() => {}}
          expandedKeys={new Set(['w1'])}
          onToggleExpand={() => {}}
          showTaskColumn
          layoutWidth={() => width}
        />
      ));
      const stable = container.querySelector('[data-proxmox-backup-row="coverage"]')!;
      const jobCell = () =>
        stable.querySelectorAll('td')[
          headerTexts().findIndex((label) => /^(Task|Job)$/.test(label))
        ];
      expect(jobCell()).toHaveTextContent('Unknown');
      expect(jobCell()).not.toHaveTextContent('OK');
      const detail = container.querySelector('[data-backup-task-detail]')!;
      expect(detail).toHaveTextContent('Reported statuscompleted');
      expect(detail).toHaveTextContent(error);
      expect(detail.querySelector('img')).toBeNull();
      expect(detail.querySelector('[data-backup-task-status-conflict]')).toHaveTextContent(
        'The reported job status and error disagree. This result is unknown. Check the existing job history in Proxmox.',
      );
      setCurrent({
        ...row,
        latestTask: {
          id: 'corrected',
          status: 'OK',
          label: 'OK',
          startedAt: '2026-05-25T03:00:00Z',
        },
      });
      expect(container.querySelector('[data-proxmox-backup-row="coverage"]')).toBe(stable);
      expect(jobCell()).toHaveTextContent('OK');
      expect(container.querySelector('[data-backup-task-detail]')).toBe(detail);
      expect(detail).not.toHaveTextContent('Reported status');
      expect(detail).not.toHaveTextContent(error);
      expect(detail.querySelector('[data-backup-task-status-conflict]')).toBeNull();
    },
  );

  it.each([320, 1200])(
    'keeps unknown job chronology distinct from no task at %i px and replaces it in place',
    (width) => {
      const [current, setCurrent] = createSignal<WorkloadCoverageRow>({
        ...row,
        taskChronologyUnknown: true,
      });
      const { container } = render(() => (
        <ProxmoxCoverageTable
          rows={[current()]}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'posture'}
          sortDirection={() => 'asc'}
          onSort={() => {}}
          expandedKeys={new Set(['w1'])}
          onToggleExpand={() => {}}
          showTaskColumn
          layoutWidth={() => width}
        />
      ));
      expect(container.querySelector('[data-backup-task-unknown]')).toHaveAccessibleName(
        'Latest backup job unknown',
      );
      expect(container.querySelector('[data-backup-task-unknown]')).toHaveTextContent(
        width === 320 ? '?' : 'Unknown',
      );
      expect(container.querySelector('[data-backup-task-chronology]')).toHaveTextContent(
        'one or more job start times are unavailable or in the future',
      );
      expect(container.querySelector('[data-backup-task-chronology]')).toHaveTextContent(
        'Check the job history in Proxmox.',
      );
      const stable = container.querySelector('[data-proxmox-backup-row="coverage"]');
      setCurrent({
        ...row,
        taskChronologyUnknown: false,
        latestTask: {
          id: 'known-failure',
          label: 'Failed',
          status: 'failed',
          startedAt: '2026-05-25T02:00:00Z',
          startedMs: Date.parse('2026-05-25T02:00:00Z'),
        },
      });
      expect(container.querySelector('[data-proxmox-backup-row="coverage"]')).toBe(stable);
      expect(container.querySelector('[data-backup-task-unknown]')).not.toBeInTheDocument();
      expect(container.querySelector('[data-backup-task-chronology]')).not.toBeInTheDocument();
      expect(stable).toHaveTextContent('Failed');
      setCurrent({ ...row, taskChronologyUnknown: true });
      expect(container.querySelector('[data-backup-task-unknown]')).toBeInTheDocument();
      expect(stable).not.toHaveTextContent('Failed');
    },
  );

  it.each([320, 1200])(
    'explains conflicting newest outcomes without inventing invalid dates at %i px',
    (width) => {
      const [current, setCurrent] = createSignal<WorkloadCoverageRow>({
        ...row,
        taskChronologyUnknown: true,
        taskOutcomeConflict: true,
      });
      const { container } = render(() => (
        <ProxmoxCoverageTable
          rows={[current()]}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'posture'}
          sortDirection={() => 'asc'}
          onSort={() => {}}
          expandedKeys={new Set(['w1'])}
          onToggleExpand={() => {}}
          showTaskColumn
          layoutWidth={() => width}
        />
      ));
      const marker = container.querySelector('[data-backup-task-unknown]');
      expect(marker).toHaveAccessibleName('Latest backup job unknown');
      expect(marker).toHaveAttribute(
        'title',
        expect.stringContaining('same newest start time report different outcomes'),
      );
      const explanation = container.querySelector('[data-backup-task-chronology]');
      expect(explanation).toHaveTextContent('same newest start time report different outcomes');
      expect(explanation).not.toHaveTextContent('unavailable or in the future');
      expect(explanation).toHaveTextContent('Check the job history in Proxmox.');
      const stable = container.querySelector('[data-proxmox-backup-row="coverage"]');
      setCurrent({
        ...row,
        taskChronologyUnknown: false,
        taskOutcomeConflict: false,
        latestTask: {
          id: 'new-failure',
          label: 'Failed',
          status: 'failed',
          startedAt: '2026-05-25T03:00:00Z',
        },
      });
      expect(container.querySelector('[data-proxmox-backup-row="coverage"]')).toBe(stable);
      expect(container.querySelector('[data-backup-task-unknown]')).not.toBeInTheDocument();
      expect(container.querySelector('[data-backup-task-chronology]')).not.toBeInTheDocument();
      expect(stable).toHaveTextContent(width === 320 ? 'Latest task: Failed' : 'Failed');
      expect(stable).toHaveTextContent(width === 320 ? 'Prot.' : 'Protected');
      setCurrent({ ...row, taskChronologyUnknown: true, taskOutcomeConflict: true });
      expect(container.querySelector('[data-backup-task-chronology]')).toHaveTextContent(
        'same newest start time',
      );
      expect(stable).not.toHaveTextContent('Failed');
    },
  );

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
    expect(document.querySelector('[data-backup-workload-target]')).toHaveTextContent('VM 100');
    expect(document.querySelector('[data-backup-workload-node]')).toHaveTextContent('Node: pve1');
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
    expect(document.body.textContent).toContain('pbs-main');
    expect(document.body.textContent).toContain('Result Success');
    expect(document.body.textContent).not.toContain('Job Success');
    expect(document.body.textContent).toContain('pbs-backup-enumeration');
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

  it('estimates unseen summaries from all measured workload heights, not the first row', async () => {
    const rectangle = vi
      .spyOn(Element.prototype, 'getBoundingClientRect')
      .mockImplementation(function (this: Element) {
        const key = this.getAttribute('data-proxmox-coverage-key');
        const index = Number(key?.replace('sized-', '') ?? 0);
        const height = key ? 40 + (index % 3) * 40 : 32;
        return {
          x: 0,
          y: 0,
          top: 0,
          left: 0,
          right: 1200,
          bottom: height,
          width: 1200,
          height,
        } as DOMRect;
      });
    try {
      render(() => (
        <ProxmoxCoverageTable
          rows={Array.from({ length: 200 }, (_, index) => ({ ...row, key: `sized-${index}` }))}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'posture'}
          sortDirection={() => 'asc'}
          onSort={() => {}}
          expandedKeys={new Set()}
          onToggleExpand={() => {}}
          showTaskColumn
          layoutWidth={() => 1200}
        />
      ));
      await fireEvent.scroll(window);
      const measured = Array.from({ length: 140 }, (_, index) => 40 + (index % 3) * 40);
      const average = measured.reduce((sum, height) => sum + height, 0) / measured.length;
      const bottom = document.querySelector('tbody > tr[aria-hidden="true"] td')!;
      expect(Number(bottom.getAttribute('height'))).toBeCloseTo(60 * average);
      expect(Number(bottom.getAttribute('height'))).not.toBe(60 * measured[0]);
    } finally {
      cleanup();
      rectangle.mockRestore();
    }
  });

  it('keeps an expanded workload mounted and focused while its tall provider evidence is read', async () => {
    let scrollTop = 0;
    const rectangle = vi
      .spyOn(Element.prototype, 'getBoundingClientRect')
      .mockImplementation(function (this: Element) {
        const isRoot = this.getAttribute('data-proxmox-backups-table') === 'coverage';
        const height = this.hasAttribute('data-inline-detail-for') ? 6000 : 32;
        return {
          x: 0,
          y: isRoot ? -scrollTop : 0,
          top: isRoot ? -scrollTop : 0,
          left: 0,
          right: 1200,
          bottom: height,
          width: 1200,
          height,
        } as DOMRect;
      });
    const rows = Array.from({ length: 200 }, (_, index) => ({
      ...row,
      key: `fleet-${index}`,
      workload: { ...row.workload, key: `fleet-${index}`, label: `fleet-${index}` },
    }));
    try {
      render(() => (
        <ProxmoxCoverageTable
          rows={rows}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'posture'}
          sortDirection={() => 'asc'}
          onSort={() => undefined}
          expandedKeys={new Set(['fleet-0'])}
          onToggleExpand={() => undefined}
          showTaskColumn={false}
          layoutWidth={() => 1200}
        />
      ));
      const toggle = document.querySelector<HTMLButtonElement>(
        'button[aria-controls="proxmox-coverage-evidence-fleet-0"]',
      )!;
      toggle.focus();
      scrollTop = 3000;
      await fireEvent.scroll(window);
      expect(document.querySelector('[data-inline-detail-for="fleet-0"]')).not.toBeNull();
      expect(document.activeElement).toBe(toggle);
      expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(140);
      scrollTop = 10000;
      await fireEvent.scroll(window);
      expect(document.querySelector('[data-inline-detail-for="fleet-0"]')).toBeNull();
      expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(140);
    } finally {
      cleanup();
      rectangle.mockRestore();
    }
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
