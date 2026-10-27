import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal, type Accessor, type JSX } from 'solid-js';

import { ProxmoxCoverageTable } from '../ProxmoxCoverageTable';
import { ProxmoxRecoverableTable } from '../ProxmoxRecoverableTable';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
} from '../__fixtures__/backupIdentity';
import type { WorkloadCoverageRow } from '../proxmoxBackupRecoveryModel';
import type { CoverageSortKey } from '../proxmoxBackupsTableModel';

// Record drawers link to the object they are about, which needs a router.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

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
  it('keeps full-width identity columns alongside the workload connection', () => {
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[row]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey={null}
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
    // Full width retains its dedicated identity cells. The connection remains
    // in the name cell at every width, including explicit unknown scope.
    expect(document.body.textContent).toContain('VM');
    expect(document.body.textContent).toContain('100');
    expect(document.body.textContent).toContain('pve1');
    expect(document.body.textContent).not.toContain('ID 100');
    expect(document.body.textContent).not.toContain('Node pve1');
  });

  it('keeps posture, backup age and job visible in compact rows', () => {
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[row]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey={null}
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
    const completedWithWarning: WorkloadCoverageRow = {
      ...row,
      latestTask: {
        id: 'task-with-warning',
        label: 'OK',
        status: 'OK',
        startedAt: '2026-07-18T23:00:00Z',
        startedMs: Date.parse('2026-07-18T23:00:00Z'),
        error: 'Provider reported completion with a warning',
      },
    };
    const { unmount } = renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[completedWithWarning]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey={null}
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    expect(document.body.textContent).not.toContain('Provider evidence');
    expect(document.body.textContent).not.toContain(completedWithWarning.latestTask!.error);
    expect(document.body.textContent).not.toContain(
      'A current verified backup is available from complete provider history.',
    );
    unmount();

    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[completedWithWarning]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey="w1"
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    expect(document.body.textContent).toContain('Provider evidence');
    expect(document.body.textContent).toContain('Proxmox Backup Server');
    expect(document.body.textContent).toContain('History Complete');
    expect(document.body.textContent).toContain('Access Sufficient');
    const task = screen.getByText('Latest task', { selector: 'th' }).closest('tbody')!;
    expect(task).toHaveTextContent('OK');
    expect(task).toHaveTextContent(completedWithWarning.latestTask!.error!);
    expect(task).toHaveTextContent('task-with-warning');
    expect(task).toHaveTextContent('Not reported');
    expect(
      within(task).getByText('Task status does not verify a restore or guest responsiveness.'),
    ).toHaveClass('whitespace-normal');
    // A reported OK does not erase a provider warning or override protection.
    expect(screen.getByText(completedWithWarning.latestTask!.error!)).toHaveClass(
      'whitespace-pre-wrap',
      'wrap-anywhere',
    );
  });

  it('keeps each provider scope beside its own access and history evidence', () => {
    const scoped = {
      ...row,
      protectionPosture: {
        ...row.protectionPosture!,
        providerStates: [
          row.protectionPosture!.providerStates[0],
          {
            ...row.protectionPosture!.providerStates[0],
            scope: 'pbs-other-estate',
            jobState: 'unknown',
            historyCompleteness: 'unavailable',
            permissions: 'denied',
          },
        ],
      },
    } as WorkloadCoverageRow;
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[scoped]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey="w1"
        onToggleExpand={() => {}}
        showTaskColumn={false}
      />
    ));

    const readable = screen.getByText('Scope pbs-main').closest('tr')!;
    const denied = screen.getByText('Scope pbs-other-estate').closest('tr')!;
    expect(readable).not.toBe(denied);
    expect(within(readable).getByText('Access Sufficient')).toBeVisible();
    expect(within(readable).getByText('History Complete')).toBeVisible();
    expect(within(denied).getByText('Access Denied')).toBeVisible();
    expect(within(denied).getByText('History Unavailable')).toBeVisible();
    expect(within(denied).queryByText('Access Sufficient')).not.toBeInTheDocument();
  });

  it.each(['', '   ', undefined])(
    'names absent scope %j Unknown instead of guessing the guest node',
    (scope) => {
      const unscoped = {
        ...row,
        protectionPosture: {
          ...row.protectionPosture!,
          providerStates: [{ ...row.protectionPosture!.providerStates[0], scope }],
        },
      } as WorkloadCoverageRow;
      renderInRouter(() => (
        <ProxmoxCoverageTable
          rows={[unscoped]}
          hasAnyRows
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
          sortDirection={() => 'asc'}
          onSort={() => {}}
          expandedKey="w1"
          onToggleExpand={() => {}}
          showTaskColumn={false}
        />
      ));

      const provider = screen.getByText('Scope Unknown').closest('tr')!;
      expect(within(provider).getByText('Access Sufficient')).toBeVisible();
      expect(provider).not.toHaveTextContent('pve1');
    },
  );

  it('renders a long markup-like scope as wrapping text, not HTML or a link', () => {
    const scope = '<img src=x onerror=alert(1)>' + 'a'.repeat(200);
    const scoped = {
      ...row,
      protectionPosture: {
        ...row.protectionPosture!,
        providerStates: [{ ...row.protectionPosture!.providerStates[0], scope: ` ${scope} ` }],
      },
    };
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[scoped]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey="w1"
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 320}
      />
    ));

    const label = screen.getByText(`Scope ${scope}`);
    expect(label).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(label.querySelector('img, a')).toBeNull();
  });

  it('heads the workload drawer with its posture and links to the workload', async () => {
    const onToggleExpand = vi.fn();
    const linked = {
      ...row,
      workload: { ...row.workload, resourceId: 'vm-100-resource' },
      artifacts: [
        {
          id: 'pbs-1',
          nativeId: 'pbs-1',
          sourceKind: 'pbs',
          sourceLabel: 'PBS',
          workload: row.workload,
          createdAt: '2026-07-18T23:00:00Z',
          createdMs: Date.parse('2026-07-18T23:00:00Z'),
          location: 'pbs-main:datastore-with-a-very-long-namespace/that-must-wrap',
          detail: 'vm/100/2026-07-18T23:00:00Z',
          protected: false,
          verified: true,
        },
      ],
    } as unknown as WorkloadCoverageRow;
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[linked]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey="w1"
        onToggleExpand={onToggleExpand}
        showTaskColumn={false}
        layoutWidth={() => 1_600}
      />
    ));

    const panel = screen.getByTestId('proxmox-coverage-detail');
    expect(within(panel).getByRole('heading', { level: 2, name: /web/ })).toBeInTheDocument();
    expect(
      within(panel).getByText(
        'A current verified backup is available from complete provider history.',
      ),
    ).toBeInTheDocument();
    expect(within(panel).getByRole('link', { name: 'Open web' })).toHaveAttribute(
      'href',
      '/proxmox/overview?resource=vm-100-resource',
    );
    // Restore evidence leads; provider quality is a section, not a grid.
    expect(within(panel).getByText('Restore evidence')).toBeInTheDocument();
    // `.table-fixed td` forces nowrap on cells, so the wrap lives on the span.
    const location = panel.querySelector('[data-coverage-evidence-location]');
    expect(location).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(location).not.toHaveClass('truncate');
    expect(panel.querySelector('[data-coverage-evidence-detail]')).toHaveClass(
      'whitespace-normal',
      'wrap-anywhere',
    );
    // The source label wraps in the drawer too; a long snapshot name is not cut.
    expect(within(panel).getByText('PBS', { selector: 'td span' })).toHaveClass(
      'whitespace-normal',
    );
    expect(panel.querySelector('.grid')).toBeNull();
    expect(within(panel).getByText('Provider evidence', { selector: 'th' })).toBeInTheDocument();

    await fireEvent.click(within(panel).getByRole('button', { name: 'Collapse web details' }));
    expect(onToggleExpand).toHaveBeenCalledWith('w1');
  });

  it('states an empty restore-point list in a shared notice', () => {
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[{ ...row, posture: 'unprotected' } as WorkloadCoverageRow]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey="w1"
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    const notice = screen.getByRole('status');
    expect(notice).toHaveTextContent(
      'No restore-point details are visible in the current list. The posture above may use previously recorded evidence.',
    );
    expect(notice.className).toContain('border-amber-300');
    // No registry match, so nothing to link to.
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
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

    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={rows}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey={null}
        onToggleExpand={() => {}}
        showTaskColumn={false}
        layoutWidth={() => 1_200}
      />
    ));

    expect(
      document.querySelectorAll(
        '[data-proxmox-backups-table="coverage"] [data-platform-window-spacer]',
      ),
    ).toHaveLength(2);
    expect(document.querySelectorAll('[data-proxmox-backup-row="coverage"]')).toHaveLength(140);
    expect(document.body.textContent).toContain('workload-0');
    expect(document.body.textContent).not.toContain('workload-599');
  });

  it('marks an empty evidence cell with a short label and the full reason on hover', () => {
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[row]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey={null}
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
    renderInRouter(() => (
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
        expandedKey={null}
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
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[{ ...row, workload: { ...row.workload, node } }] as WorkloadCoverageRow[]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'posture') as Accessor<CoverageSortKey>}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey={null}
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

describe('Coverage absence requires readable, complete history', () => {
  const unknownTitle =
    'No completed backup is visible in the current list. Missing entries do not mean no backups exist. Expand this row to inspect the evidence.';
  const completePosture = row.protectionPosture!;
  const provider = completePosture.providerStates[0];
  const mount = (subject: WorkloadCoverageRow, historyReadIncomplete = false) =>
    renderInRouter(() => (
      <ProxmoxCoverageTable
        rows={[subject]}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={() => 'posture'}
        sortDirection={() => 'asc'}
        onSort={() => {}}
        expandedKey="w1"
        onToggleExpand={() => {}}
        showTaskColumn={false}
        historyReadIncomplete={historyReadIncomplete}
        layoutWidth={() => 320}
      />
    ));

  it.each([
    ['partial history', { ...provider, historyCompleteness: 'partial' as const }],
    ['unavailable history', { ...provider, historyCompleteness: 'unavailable' as const }],
    ['unknown history', { ...provider, historyCompleteness: 'unknown' as const }],
    ['partial access', { ...provider, permissions: 'partial' as const }],
    ['denied access', { ...provider, permissions: 'denied' as const }],
    ['unknown access', { ...provider, permissions: 'unknown' as const }],
  ])('does not turn %s into confirmed absence', (_label, incompleteProvider) => {
    mount({
      ...row,
      posture: 'attention',
      protectionPosture: { ...completePosture, providerStates: [incompleteProvider] },
    });

    expect(screen.getByTitle(unknownTitle)).toHaveTextContent(/^Unknown$/);
    expect(screen.getByLabelText(`Unknown age. ${unknownTitle}`)).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent(
      'Restore-point history is not confirmed complete. Missing entries do not mean no backups exist.',
    );
    expect(document.body.textContent).not.toContain('No restore points found');
    expect(document.body.textContent).not.toContain('No restore points reported');
    // This copy qualifies the list; it does not synthesise a hidden archive.
    expect(screen.queryByText('Restore evidence')).not.toBeInTheDocument();
  });

  it.each([
    ['missing posture', undefined],
    ['no provider evidence', { ...completePosture, providerStates: [] }],
    ['unknown scope', { ...completePosture, coverage: 'unknown' as const }],
    ['partial scope', { ...completePosture, coverage: 'partial' as const }],
    [
      'one readable source beside an unreadable source',
      {
        ...completePosture,
        providerStates: [provider, { ...provider, historyCompleteness: 'unavailable' as const }],
      },
    ],
  ])('keeps %s unknown even when raw requests fulfilled', (_label, protectionPosture) => {
    mount({ ...row, protectionPosture });
    expect(screen.getByTitle(unknownTitle)).toHaveTextContent(/^Unknown$/);
    expect(screen.getByRole('status')).toHaveTextContent('not confirmed complete');
  });

  it('qualifies pending/failed reads even beside retained complete provider quality', () => {
    mount(row, true);
    expect(screen.getByTitle(unknownTitle)).toHaveTextContent(/^Unknown$/);
    expect(screen.getByRole('status')).toHaveTextContent('not confirmed complete');
  });

  it('keeps a known backup outside the raw list unknown, not None', () => {
    mount({
      ...row,
      protectionPosture: {
        ...completePosture,
        lastSuccessfulPointAt: '2026-10-23T00:00:00Z',
      },
    });
    expect(screen.getByTitle(unknownTitle)).toHaveTextContent(/^Unknown$/);
    expect(screen.getByRole('status')).toHaveTextContent(
      'No restore-point details are visible in the current list. The posture above may use previously recorded evidence.',
    );
    expect(screen.queryByText('Restore evidence')).not.toBeInTheDocument();
  });

  it('preserves accepted-empty history as a scoped empty observation', () => {
    mount({
      ...row,
      posture: 'unprotected',
      protectionPosture: { ...completePosture, state: 'unprotected', coverage: 'none' },
    });
    expect(screen.getByTitle('No PBS snapshot or PVE backup file')).toHaveTextContent(/^None$/);
    expect(screen.getByRole('status')).toHaveTextContent(
      'No restore-point details are visible in the current list. The posture above may use previously recorded evidence.',
    );
    expect(screen.queryByTitle(unknownTitle)).not.toBeInTheDocument();
  });

  it.each(['pve-backup-enumeration', 'pbs-backup-enumeration'])(
    'does not broaden complete %s into absence of retained snapshots',
    (source) => {
      const explanation =
        'Only snapshots were found. A snapshot is not a separate backup, so it does not count.';
      mount({
        ...row,
        posture: 'unprotected',
        protectionPosture: {
          ...completePosture,
          state: 'unprotected',
          coverage: 'none',
          explanation,
          providerStates: [{ ...provider, source }],
        },
      });

      expect(screen.getByText(explanation)).toBeInTheDocument();
      expect(screen.getByText('History Complete')).toBeInTheDocument();
      expect(screen.getByRole('status')).toHaveTextContent(
        'No restore-point details are visible in the current list. The posture above may use previously recorded evidence.',
      );
      expect(screen.getByTitle('No PBS snapshot or PVE backup file')).toHaveTextContent('None');
      // No lastSuccessfulPointAt is expected for a guest snapshot. Nor may the
      // notice synthesize a raw snapshot, date or completed backup from prose.
      expect(screen.queryByText('Restore evidence')).not.toBeInTheDocument();
      expect(document.body.textContent).not.toContain('No restore points reported');
    },
  );
});

const scrollingModel = () =>
  buildProxmoxBackupRecoveryModel({
    workloads: Array.from({ length: 170 }, (_, i) =>
      identityGuest(`guest-${i}`, 'east', 'pve-1', { vmid: 100 + i }),
    ),
    archives: Array.from({ length: 170 }, (_, i) =>
      identityArchive({
        id: `archive-${i}`,
        vmid: 100 + i,
        ctime: BACKUP_IDENTITY_NOW / 1000 - i * 60,
      }),
    ),
    snapshots: [],
    pbsBackups: [],
    tasks: [],
    nowMs: BACKUP_IDENTITY_NOW,
  });

describe('backup evidence native scrolling', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });
  it.each([
    ['coverage', 390],
    ['coverage', 1365],
    ['date', 390],
    ['date', 1365],
  ] as const)(
    'keeps a tall %s drawer and its scroll extent at %ipx, including after a live snapshot',
    async (view, width) => {
      let scrollTop = 0;
      const drawerHeight = 4800;
      vi.stubGlobal('innerWidth', width);
      vi.stubGlobal('innerHeight', 120);
      vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (
        this: Element,
      ) {
        const top =
          this.getAttribute('data-platform-window-spacer') === 'top' ||
          this.hasAttribute('data-proxmox-backups-table')
            ? -scrollTop
            : 0;
        const height = this.hasAttribute('data-inline-detail-row') ? drawerHeight : 32;
        return {
          x: 0,
          y: top,
          top,
          left: 0,
          right: width,
          bottom: top + height,
          width,
          height,
          toJSON: () => ({}),
        } as DOMRect;
      });
      const initial = scrollingModel();
      const [rows, setRows] = createSignal(initial.coverageRows);
      const [artifacts, setArtifacts] = createSignal(initial.recoverableArtifacts);
      const [expanded, setExpanded] = createSignal<string | null>(null);
      const { container } = renderInRouter(() =>
        view === 'coverage' ? (
          <ProxmoxCoverageTable
            rows={rows()}
            hasAnyRows
            emptyIcon={<span />}
            emptyTitle="No guests"
            emptyDescription="No guests"
            sortKey={() => 'workload'}
            sortDirection={() => 'asc'}
            onSort={() => {}}
            expandedKey={expanded()}
            onToggleExpand={(key) => setExpanded((current) => (current === key ? null : key))}
            showTaskColumn={false}
            layoutWidth={() => width}
          />
        ) : (
          <ProxmoxRecoverableTable
            artifacts={artifacts()}
            hasAnyArtifacts
            emptyIcon={<span />}
            emptyTitle="No backups"
            emptyDescription="No backups"
            sortKey={() => 'created'}
            sortDirection={() => 'desc'}
            onSort={() => {}}
            sizeMaxBytes={1024}
            groupByDay
            layoutWidth={() => width}
          />
        ),
      );
      const rowSelector = `[data-proxmox-backup-row="${view === 'coverage' ? 'coverage' : 'recoverable'}"]`;
      const mountedCount = () => container.querySelectorAll(rowSelector).length;
      const budget = width < 640 ? 36 : 140;
      expect(mountedCount()).toBeLessThanOrEqual(budget);
      expect(mountedCount()).toBeGreaterThan(0);
      const first = container.querySelector(rowSelector)!;
      const disclosure = first.querySelector<HTMLButtonElement>('button[aria-controls]')!;
      await fireEvent.click(disclosure);
      const detail = await screen.findByTestId(
        view === 'coverage' ? 'proxmox-coverage-detail' : 'proxmox-recoverable-detail',
      );
      disclosure.focus();
      // The whole logical row includes the expanded panel. Scrolling inside
      // it must not advance the runway as if its pixels were 118 summary rows.
      scrollTop = 3800;
      await fireEvent.scroll(window);
      expect(detail).toBeInTheDocument();
      expect(disclosure).toHaveFocus();

      const refreshed = scrollingModel();
      refreshed.coverageRows[0]!.workload.name = 'refreshed-guest';
      refreshed.recoverableArtifacts[0]!.workload.name = 'refreshed-guest';
      setRows(refreshed.coverageRows);
      setArtifacts(refreshed.recoverableArtifacts);
      await waitFor(() => {
        expect(container.querySelector(rowSelector)?.querySelector('button')).toBe(disclosure);
        expect(disclosure).toHaveFocus();
        expect(detail).toBeInTheDocument();
        expect(first).toHaveTextContent('refreshed-guest');
      });

      scrollTop = drawerHeight + 3200;
      await fireEvent.scroll(window);
      await waitFor(() => expect(first).not.toBeInTheDocument());
      const spacer = container.querySelector<HTMLElement>('[data-platform-window-spacer="top"]')!;
      expect(parseFloat(spacer.style.height)).toBeGreaterThan(drawerHeight);
      expect(mountedCount()).toBeLessThanOrEqual(budget);

      scrollTop = 0;
      await fireEvent.scroll(window);
      await waitFor(() =>
        expect(
          screen.getByTestId(
            view === 'coverage' ? 'proxmox-coverage-detail' : 'proxmox-recoverable-detail',
          ),
        ).toBeInTheDocument(),
      );
      const table = container.querySelector('table')!;
      const columns = table.querySelectorAll(':scope > thead th').length;
      for (const cell of table.querySelectorAll<HTMLTableCellElement>(
        '[data-platform-window-spacer] > td',
      )) {
        expect(cell.colSpan).toBe(columns);
      }

      // Full-result filtering can shrink a previously scrolled estate below
      // the mounted budget. No old spacers or stale selected panel survive.
      setRows(refreshed.coverageRows.slice(-1));
      setArtifacts(refreshed.recoverableArtifacts.slice(-1));
      await waitFor(() => {
        expect(mountedCount()).toBe(1);
        expect(container.querySelector('[data-platform-window-spacer]')).toBeNull();
        expect(detail).not.toBeInTheDocument();
      });
    },
  );
});
