import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import type { ProtectionPosture, ProtectionProviderState } from '@/types/recovery';
import { BACKUP_IDENTITY_NOW, identityGuest } from '../__fixtures__/backupIdentity';
import { ProxmoxCoverageTable } from '../ProxmoxCoverageTable';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  type WorkloadCoverageRow,
} from '../proxmoxBackupRecoveryModel';

const westScope = 'pbs-west/department-with-a-long-name/production';
const providers: ProtectionProviderState[] = [
  {
    provider: 'proxmox-pbs',
    source: 'pbs-backup-enumeration',
    scope: 'pbs-east',
    jobState: 'success',
    historyCompleteness: 'complete',
    permissions: 'sufficient',
    evidenceIds: [],
  },
  {
    provider: 'proxmox-pbs',
    source: 'pbs-backup-enumeration',
    scope: westScope,
    jobState: 'failed',
    historyCompleteness: 'unavailable',
    permissions: 'denied',
    evidenceIds: [],
  },
  {
    provider: 'proxmox-pbs',
    source: 'legacy-recovery-point',
    scope: westScope,
    jobState: 'unknown',
    historyCompleteness: 'unknown',
    permissions: 'unknown',
    evidenceIds: [],
  },
];

function makeRow(providerStates = providers): WorkloadCoverageRow {
  const posture: ProtectionPosture = {
    subjectResourceId: 'east-100',
    state: 'unknown',
    freshness: 'unknown',
    verification: 'unknown',
    coverage: 'unknown',
    // Separate HTTP responses do not alias the current store's objects. Solid
    // reconciliation may mutate its previous row while applying the next one.
    providerStates: providerStates.map((provider) => ({
      ...provider,
      evidenceIds: [...provider.evidenceIds],
    })),
    repositoryResourceIds: [],
    evidenceIds: [],
    explanation: 'Some provider history is unavailable.',
    evaluatedAt: '2026-10-04T12:00:00Z',
  };
  return buildProxmoxBackupRecoveryModel({
    nowMs: BACKUP_IDENTITY_NOW,
    workloads: [identityGuest('east-100', 'guest-connection', 'pve-1')],
    pbsBackups: [],
    archives: [],
    snapshots: [],
    tasks: [],
    protectionPostures: new Map([[posture.subjectResourceId, posture]]),
  }).coverageRows[0];
}

function EvidenceTable(props: { rows: WorkloadCoverageRow[]; width?: number }) {
  return (
    <ProxmoxCoverageTable
      rows={props.rows}
      hasAnyRows
      emptyIcon={<span />}
      emptyTitle="No guests"
      emptyDescription="No guests"
      sortKey={() => 'workload'}
      sortDirection={() => 'asc'}
      onSort={() => undefined}
      expandedKeys={new Set(['resource:east-100'])}
      onToggleExpand={() => undefined}
      showTaskColumn={false}
      layoutWidth={() => props.width ?? 1365}
    />
  );
}

describe('scoped protection provider evidence', () => {
  afterEach(cleanup);

  it.each([320, 640, 1365])(
    'keeps each observed scope/source beside its own facts at %s px',
    (width) => {
      const row = makeRow();
      const before = JSON.stringify(row);
      const { container } = render(() => <EvidenceTable rows={[row]} width={width} />);
      const cards = container.querySelectorAll<HTMLElement>('[data-backup-provider-evidence]');
      expect(cards).toHaveLength(3);
      for (const [index, provider] of providers.entries()) {
        const card = within(cards[index]);
        expect(card.getByText(provider.scope)).toHaveClass('whitespace-normal', 'wrap-anywhere');
        expect(card.getByText(provider.source)).not.toHaveClass('truncate');
        expect(
          card.getByText(`Result ${provider.jobState[0].toUpperCase()}${provider.jobState.slice(1)}`),
        ).toBeInTheDocument();
        expect(
          card.getByText(
            `Access ${provider.permissions[0].toUpperCase()}${provider.permissions.slice(1)}`,
          ),
        ).toBeInTheDocument();
      }
      expect(
        screen.getByText('No restore evidence has been discovered for this workload.'),
      ).toBeInTheDocument();
      expect(row.posture).toBe('unknown');
      expect(row.artifacts).toEqual([]);
      expect(JSON.stringify(row)).toBe(before);
    },
  );

  it('finds unavailable sources without needing an artifact or changing guest identity', () => {
    const row = makeRow();
    for (const term of [
      westScope.toUpperCase(),
      ' PBS-EAST ',
      'pbs-backup-enumeration',
      'legacy-recovery-point',
      'denied',
      'unavailable',
    ]) {
      expect(coverageRowMatchesSearch(row, term)).toBe(true);
    }
    expect(coverageRowMatchesSearch(row, 'pbs-not-observed')).toBe(false);
    expect(row.workload.instance).toBe('guest-connection');
    expect(row.workload.resourceId).toBe('east-100');
  });

  it('keeps scope and facts paired through a same-open refresh and reordered observations', () => {
    const [rows, setRows] = createSignal([makeRow()]);
    const { container } = render(() => <EvidenceTable rows={rows()} />);
    setRows([
      makeRow([
        {
          ...providers[1],
          permissions: 'sufficient',
          historyCompleteness: 'complete',
          jobState: 'success',
        },
        providers[0],
      ]),
    ]);
    const cards = container.querySelectorAll<HTMLElement>('[data-backup-provider-evidence]');
    expect(cards).toHaveLength(2);
    expect(within(cards[0]).getByText(westScope)).toBeInTheDocument();
    expect(within(cards[0]).getByText('Access Sufficient')).toBeInTheDocument();
    expect(within(cards[1]).getByText('pbs-east')).toBeInTheDocument();
    expect(screen.queryByText('Access Denied')).not.toBeInTheDocument();
    expect(coverageRowMatchesSearch(rows()[0], 'denied')).toBe(false);
    expect(coverageRowMatchesSearch(rows()[0], westScope)).toBe(true);
  });

  it('does not infer missing scope/source from guest placement and renders identifiers as text', () => {
    const { container } = render(() => (
      <EvidenceTable
        rows={[
          makeRow([
            { ...providers[0], scope: '  ', source: '' },
            { ...providers[1], scope: '<script>untrusted-scope</script>' },
          ]),
        ]}
      />
    ));
    expect(container.querySelectorAll('[data-backup-provider-scope]')[0]).toHaveTextContent(
      'Unknown',
    );
    expect(container.querySelectorAll('[data-backup-provider-source]')[0]).toHaveTextContent(
      'Unknown',
    );
    expect(screen.getByText('<script>untrusted-scope</script>')).toBeInTheDocument();
    expect(container.querySelector('script')).toBeNull();
    expect(container.querySelectorAll('[data-backup-provider-scope]')[0]).not.toHaveTextContent(
      'guest-connection',
    );
  });
});
