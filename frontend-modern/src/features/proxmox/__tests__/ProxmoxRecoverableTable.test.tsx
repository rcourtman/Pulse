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
      name: 'Expand details for web (VM 100) · PVE connection: Not reported',
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
    expect(within(panel).getByRole('link', { name: 'Open web' })).toHaveAttribute(
      'href',
      '/proxmox/overview?resource=vm-100',
    );

    // Opening another restore point closes the first.
    await fireEvent.click(
      screen.getByRole('button', {
        name: 'Expand details for db (VM 101) · PVE connection: Not reported',
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
