import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it } from 'vitest';
import type { Accessor, JSX } from 'solid-js';

import { ProxmoxRecoverableTable } from '../ProxmoxRecoverableTable';
import type { RecoverableArtifact } from '../proxmoxBackupRecoveryModel';
import type { RecoverableSortKey } from '../proxmoxBackupsTableModel';

// Record drawers link to the object they are about, which needs a router.
const renderInRouter = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));

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
  renderInRouter(() => (
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

describe('ProxmoxRecoverableTable record drawer', () => {
  it('opens one restore point at a time with what the phone row drops', async () => {
    const second: RecoverableArtifact = {
      ...artifact,
      id: 'pbs:db:101',
      workload: {
        ...artifact.workload,
        key: 'vm:101',
        vmid: '101',
        name: 'db',
        label: 'db (VM 101)',
      },
      location: 'offsite / pbs2',
    };
    renderInRouter(() => (
      <ProxmoxRecoverableTable
        artifacts={[
          { ...artifact, workload: { ...artifact.workload, resourceId: 'vm-100' } },
          second,
        ]}
        hasAnyArtifacts
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'created') as Accessor<RecoverableSortKey>}
        sortDirection={() => 'desc'}
        onSort={() => {}}
        sizeMaxBytes={artifact.size ?? 0}
        layoutWidth={() => 330}
      />
    ));

    const toggle = screen.getByRole('button', {
      name: /^Expand details for web \(VM 100\) · PVE connection: Not reported · PBS snapshot/,
    });
    expect(toggle).toHaveAttribute('aria-controls', 'proxmox-recoverable-detail-pbs:web:100');
    await fireEvent.click(toggle);

    const panel = screen.getByTestId('proxmox-recoverable-detail');
    expect(within(panel).getByRole('heading', { level: 2, name: /web/ })).toBeInTheDocument();
    expect(within(panel).getByText('PBS snapshot in main / pve1')).toBeInTheDocument();
    expect(within(panel).getByText('main / pve1', { selector: 'span' })).toBeInTheDocument();
    expect(within(panel).getAllByText('Verified')).toHaveLength(2);
    expect(within(panel).getByText('Prune protection')).toBeInTheDocument();
    expect(within(panel).getByText('Kept when pruning')).toBeInTheDocument();
    expect(within(panel).getByText('5 PBS files')).toBeInTheDocument();
    expect(within(panel).getByText('Identifier').closest('tr')).toHaveTextContent(
      artifact.nativeId,
    );
    expect(within(panel).getByText('Created').closest('tr')).toHaveTextContent('2026');
    expect(within(panel).getByRole('link', { name: 'Open web' })).toHaveAttribute(
      'href',
      '/proxmox/overview?resource=vm-100',
    );

    // Opening another restore point closes the first.
    await fireEvent.click(
      screen.getByRole('button', {
        name: /^Expand details for db \(VM 101\) · PVE connection: Not reported · PBS snapshot/,
      }),
    );
    expect(screen.getAllByTestId('proxmox-recoverable-detail')).toHaveLength(1);
    expect(screen.getByTestId('proxmox-recoverable-detail')).toHaveAttribute(
      'data-inline-detail-for',
      'pbs:db:101',
    );
    // The db backup has no registry match, so there is nothing to open.
    expect(screen.queryByRole('link')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Collapse db details' }));
    expect(screen.queryByTestId('proxmox-recoverable-detail')).not.toBeInTheDocument();
  });
});

describe('ProxmoxRecoverableTable responsive columns', () => {
  it('groups full restore-point identity beside age and state in compact containers', () => {
    renderTable(330);

    expect([...document.querySelectorAll('thead th')].map((th) => th.textContent?.trim())).toEqual([
      'Restore point',
      'Age▼',
      'State',
    ]);
    expect(document.body.textContent).toContain('VM 100');
    // No hover is needed to identify the repository on a phone.
    expect(document.querySelector('[data-backup-location]')).toHaveTextContent('main / pve1');
    expect(document.querySelector('[data-backup-source-connection]')).toHaveTextContent(
      'Connection: Unknown connection',
    );
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
      'Created▼',
      'Size',
      'State',
      'Details',
    ]);
    expect(document.body.textContent).toContain('main / pve1');
    expect(document.body.textContent).toContain('5 PBS files');
  });

  it('windows large dated feeds without adding pagination controls', () => {
    const artifacts = Array.from({ length: 600 }, (_, index) => ({
      ...artifact,
      id: `pbs:web:${index}`,
      nativeId: `vm/${index}/2026-08-04T08:00:00Z`,
      workload: {
        ...artifact.workload,
        key: `vm:${index}`,
        vmid: String(index),
        label: `workload-${index}`,
        name: `workload-${index}`,
      },
    }));

    renderInRouter(() => (
      <ProxmoxRecoverableTable
        artifacts={artifacts}
        hasAnyArtifacts
        emptyIcon={<span />}
        emptyTitle=""
        emptyDescription=""
        sortKey={(() => 'created') as Accessor<RecoverableSortKey>}
        sortDirection={() => 'desc'}
        onSort={() => {}}
        sizeMaxBytes={artifact.size ?? 0}
        groupByDay
        layoutWidth={() => 1_200}
      />
    ));

    expect(
      document.querySelectorAll(
        '[data-proxmox-backups-table="recoverable"] [data-platform-window-spacer]',
      ),
    ).toHaveLength(2);
    expect(document.querySelectorAll('[data-proxmox-backup-row]')).toHaveLength(140);
    expect(document.body.textContent).toContain('workload-0');
    expect(document.body.textContent).not.toContain('workload-599');
  });
});

describe('restore-point disclosure identity', () => {
  it.each([320, 479, 720, 1200])('keeps a visible native disclosure at %ipx', (width) => {
    renderTable(width);
    const toggle = screen.getByRole('button', { name: /Expand details/ });
    expect(toggle).not.toHaveClass('sr-only');
    expect(toggle).toHaveClass('h-9', 'w-9', 'my-0');
    expect(toggle).toHaveAttribute('type', 'button');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle.closest('tr')).not.toHaveAttribute('tabindex');
  });

  it.each(['pbs', 'archive', 'snapshot'] as const)(
    'names tied and unknown %s points by their own scope and native identifier',
    (sourceKind) => {
      const points = ['first', 'second', 'unknown'].map((id) => ({
        ...artifact,
        id,
        sourceKind,
        nativeId: `native:${id}`,
        locationConnection: 'backup-reader',
        // Same workload, source, repository and instant; no row-index naming.
        createdMs: id === 'unknown' ? undefined : artifact.createdMs,
      }));
      renderInRouter(() => (
        <ProxmoxRecoverableTable
          artifacts={points}
          hasAnyArtifacts
          emptyIcon={<span />}
          emptyTitle=""
          emptyDescription=""
          sortKey={() => 'created'}
          sortDirection={() => 'desc'}
          onSort={() => {}}
          sizeMaxBytes={0}
          layoutWidth={() => 320}
        />
      ));
      const toggles = screen.getAllByRole('button', { name: /Expand details/ });
      expect(new Set(toggles.map((button) => button.getAttribute('aria-label'))).size).toBe(3);
      for (const [index, button] of toggles.entries()) {
        expect(button).toHaveAccessibleName(/web \(VM 100\) · PVE connection: Not reported/);
        expect(button).toHaveAccessibleName(/Backup connection: backup-reader/);
        expect(button).toHaveAccessibleName(/Location: main \/ pve1/);
        expect(button).toHaveAccessibleName(new RegExp(`Identifier: native:${points[index].id}$`));
        if (points[index].createdMs === undefined) {
          expect(button).toHaveAccessibleName(/Created: Unknown/);
        } else {
          expect(button).not.toHaveAccessibleName(/Created: Unknown/);
          expect(button).toHaveAccessibleName(/Created: .*2026/);
        }
      }
      fireEvent.click(toggles[2]);
      const detail = screen.getByTestId('proxmox-recoverable-detail');
      expect(detail).toHaveAttribute('data-inline-detail-for', 'unknown');
      expect(within(detail).getByText('Created').closest('tr')).toHaveTextContent('Unknown');
      expect(within(detail).getByText('Identifier').closest('tr')).toHaveTextContent(
        'native:unknown',
      );
      expect(toggles[2]).toHaveAttribute('aria-expanded', 'true');
      expect(toggles[0]).toHaveAttribute('aria-expanded', 'false');
    },
  );

  it('does not double-toggle native Enter or Space through the containing row', () => {
    renderTable(320);
    const toggle = screen.getByRole('button', { name: /Expand details/ });
    toggle.focus();
    fireEvent.keyDown(toggle, { key: 'Enter' });
    fireEvent.keyUp(toggle, { key: 'Enter' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getAllByTestId('proxmox-recoverable-detail')).toHaveLength(1);
    expect(toggle).toHaveFocus();
    fireEvent.keyDown(toggle, { key: ' ', code: 'Space' });
    fireEvent.keyUp(toggle, { key: ' ', code: 'Space' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByTestId('proxmox-recoverable-detail')).not.toBeInTheDocument();
    expect(toggle).toHaveFocus();
  });
});
