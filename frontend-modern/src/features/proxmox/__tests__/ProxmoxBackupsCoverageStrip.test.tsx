import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import type { WorkloadCoverageRow, WorkloadRecoveryPosture } from '../proxmoxBackupRecoveryModel';
import { buildBackupVerdict } from '../proxmoxBackupVerdict';
import { ProxmoxBackupsCoverageStrip } from '../ProxmoxBackupsCoverageStrip';

const row = (label: string, posture: WorkloadRecoveryPosture): WorkloadCoverageRow => ({
  key: `resource:${label}`,
  workload: {
    key: `resource:${label}`,
    resourceId: `vm:${label}`,
    type: 'vm',
    typeLabel: 'VM',
    vmid: '100',
    label,
    name: label,
  },
  artifacts: [],
  pbsCount: 0,
  archiveCount: 0,
  snapshotCount: 0,
  posture,
  postureRank: 0,
  protectionPosture:
    posture === 'checking' || posture === 'not-evaluated'
      ? undefined
      : {
          ...protectionPosture(`vm:${label}`),
          state: posture,
          reason: posture === 'unprotected' ? 'no-backup' : 'recent-verified',
        },
  isOrphaned: false,
});

const verdict = (...rows: WorkloadCoverageRow[]) => buildBackupVerdict(rows, protectionPolicy);

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('Backup health inventory context', () => {
  it('keeps one live summary through phone/desktop transfers without losing counts or controls', async () => {
    vi.stubGlobal('innerWidth', 390);
    const [failed, setFailed] = createSignal(1);
    const select = vi.fn();
    render(() => {
      // A caller may retain the same JSX node. Keep its computation owned by
      // the mounted tree; neither responsive location may duplicate it.
      const summary = (
        <span>2 completed restore points · 1 running · {failed()} failed · 1 unmatched</span>
      );
      return (
        <ProxmoxBackupsCoverageStrip
          title="Backup health"
          verdict={verdict(row('vm', 'unprotected'))}
          tail={summary}
          onSelectGroup={select}
          rule="Restore points older than 7 days need attention."
        />
      );
    });
    const currentSummary = () => screen.getByText(/2 completed restore points/);
    const ownedSummary = currentSummary();
    expect(currentSummary().closest('[data-backup-inventory-summary]')).not.toBeNull();
    expect(currentSummary()).toBeVisible();
    for (const width of [640, 390, 1440, 639]) {
      window.innerWidth = width;
      fireEvent(window, new Event('resize'));
      await waitFor(() => {
        expect(Boolean(currentSummary().closest('[data-backup-inventory-summary]'))).toBe(
          width < 640,
        );
      });
      expect(screen.getAllByText(/2 completed restore points/)).toHaveLength(1);
      expect(currentSummary()).toBe(ownedSummary);
      expect(currentSummary()).toBeVisible();
    }
    setFailed(2);
    expect(currentSummary()).toHaveTextContent('1 running · 2 failed · 1 unmatched');
    fireEvent.click(screen.getByRole('button', { name: '1 no successful backup' }));
    expect(select).toHaveBeenCalledExactlyOnceWith('no-backup');
    expect(screen.getByText('What the counts mean')).toBeVisible();
  });

  it('adds no empty inventory row when the caller has no summary', () => {
    vi.stubGlobal('innerWidth', 390);
    const view = render(() => (
      <ProxmoxBackupsCoverageStrip title="Backup health" verdict={verdict()} />
    ));
    expect(view.container.querySelector('[data-backup-inventory-summary]')).toBeNull();
    expect(screen.getAllByText('Nothing to rate yet')).toHaveLength(2);
  });
});

describe('Backup health unrated counts', () => {
  it('does not hide checking or unevaluated targets behind a protected ratio', () => {
    const view = render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={verdict(
          row('protected', 'protected'),
          row('reading', 'checking'),
          row('host', 'not-evaluated'),
          { ...row('orphan', 'not-evaluated'), isOrphaned: true },
        )}
      />
    ));

    expect(view.container.querySelector('[data-backup-verdict]')).toHaveTextContent('1 / 1');
    for (const label of [/1 checking/, /1 not evaluated/]) {
      const count = screen.getByText(label);
      expect(count).toBeVisible();
      // JSDOM does not apply Tailwind's responsive CSS. Reject base hiding
      // here; source-bound desktop/phone browser proof must check real layout.
      expect(count.closest('[hidden], .hidden, .invisible')).toBeNull();
    }
    expect(screen.queryByText(/2 not evaluated/)).not.toBeInTheDocument();
    expect(view.container.querySelector('[data-backup-protected-row]')).toHaveTextContent('100%');
  });

  it('shows the unassessed count when no target can be rated yet', () => {
    render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={verdict(row('first', 'checking'), row('second', 'checking'))}
      />
    ));
    expect(screen.getByText(/2 checking/)).toBeVisible();
    expect(screen.queryByText(/not evaluated/)).not.toBeInTheDocument();
    expect(screen.getAllByText('Checking backups…')).toHaveLength(2);
  });

  it('keeps unevaluated distinct from checking and from unknown assessments', () => {
    const view = render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={verdict(row('host', 'not-evaluated'), row('unknown', 'unknown'))}
      />
    ));
    expect(screen.getByText(/1 not evaluated/)).toBeVisible();
    expect(screen.queryByText(/checking/)).not.toBeInTheDocument();
    expect(screen.getByText('Status unknown')).toBeVisible();
    expect(view.container.querySelector('[data-backup-verdict]')).toHaveTextContent('0 / 1');
  });

  it('retires the counts when assessment completes without keeping phantom separators', () => {
    const [current, setCurrent] = createSignal(verdict(row('vm', 'checking')));
    const view = render(() => (
      <ProxmoxBackupsCoverageStrip title="Backup health" verdict={current()} />
    ));
    expect(screen.getByText(/1 checking/)).toBeVisible();
    setCurrent(verdict(row('vm', 'protected')));
    expect(screen.queryByText(/checking|not evaluated/)).not.toBeInTheDocument();
    expect(view.container.querySelector('[data-backup-verdict]')).toHaveTextContent('1 / 1');
    expect(view.container.textContent).not.toContain('·');
    setCurrent(verdict());
    expect(screen.getAllByText('Nothing to rate yet')).toHaveLength(2);
    expect(view.container.querySelector('[data-backup-protected-row]')).toBeNull();
  });

  it('preserves exception selection, rules and datastore controls beside the counts', () => {
    const onSelectGroup = vi.fn();
    const onSelectDatastoreProblem = vi.fn();
    render(() => (
      <ProxmoxBackupsCoverageStrip
        title="Backup health"
        verdict={verdict(row('vm', 'unprotected'), row('host', 'not-evaluated'))}
        rule="Restore points older than 7 days need attention."
        onSelectGroup={onSelectGroup}
        isGroupPressed={() => true}
        datastoreProblems={[
          { family: 'verify', label: 'Verification failed', datastores: ['main'] },
        ]}
        onSelectDatastoreProblem={onSelectDatastoreProblem}
      />
    ));
    expect(screen.getByText(/1 not evaluated/)).toBeVisible();
    const exception = screen.getByRole('button', { name: '1 no successful backup' });
    expect(exception).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(exception);
    expect(onSelectGroup).toHaveBeenCalledExactlyOnceWith('no-backup');
    fireEvent.click(
      screen.getByRole('button', { name: 'Verification failed on 1 datastore: main' }),
    );
    expect(onSelectDatastoreProblem).toHaveBeenCalledOnce();
    expect(screen.getByText('What the counts mean')).toBeVisible();
    expect(
      screen.getByText('Restore points older than 7 days need attention.'),
    ).toBeInTheDocument();
  });
});
