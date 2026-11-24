import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Details</div>,
}));

afterEach(cleanup);

const datastore = (patch: Record<string, unknown> = {}): ResourcePBSDatastore =>
  ({
    name: 'tank',
    total: 1000,
    used: 400,
    available: 600,
    status: 'available',
    deduplicationFactor: 2,
    ...patch,
  }) as ResourcePBSDatastore;

const server = (store?: ResourcePBSDatastore, health = 'healthy'): Resource => ({
  id: 'pbs-capacity',
  type: 'pbs',
  name: 'pbs-capacity',
  displayName: 'pbs-capacity',
  status: 'online',
  platformId: 'pbs-capacity',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  lastSeen: 1_790_000_000_000,
  pbs: { instanceId: 'pbs-capacity', connectionHealth: health, datastores: store ? [store] : [] },
});

function usedCell(container: HTMLElement) {
  const table = container.querySelector('table')!;
  const index = Array.from(table.querySelectorAll('th')).findIndex(
    (head) => head.textContent === 'Used',
  );
  return table.querySelector('tbody tr:not([data-backup-server-group])')!.querySelectorAll('td')[
    index
  ] as HTMLElement;
}

const expandTank = () => {
  const toggle = screen.getByRole('button', {
    name: 'Expand details for pbs-capacity · datastore tank',
  });
  fireEvent.click(toggle);
  return toggle;
};

const selectedTank = () => screen.getByRole('region', { name: 'Selected PBS datastore' });

describe('PBS datastore capacity evidence', () => {
  it.each([
    [
      'provider failure zeros',
      { total: 0, used: 0, available: 0, usagePercent: 0, status: 'unavailable' },
      'Unavailable',
    ],
    ['retained failure values', { status: 'unavailable', usagePercent: 40 }, 'Unavailable'],
    [
      'explicit error with healthy status',
      { error: 'PRIVATE_PROVIDER_ERROR_SENTINEL' },
      'Unavailable',
    ],
    ['unknown status', { status: 'unknown', usagePercent: 0 }, 'Unknown'],
    ['future status', { status: 'new-provider-state' }, 'Unknown'],
    ['zero total', { total: 0, used: 0, usagePercent: 0 }, 'Unknown'],
    ['missing used', { used: undefined }, 'Unknown'],
    ['negative used', { used: -10 }, 'Unknown'],
    ['nonfinite total', { total: Infinity }, 'Unknown'],
    ['overflowed ratio', { used: Number.MAX_VALUE, total: Number.MIN_VALUE }, 'Unknown'],
  ])('shows %s as unknown evidence rather than healthy usage', async (_name, patch, label) => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server(datastore(patch))]} layoutWidth={() => 1200} />
    ));
    const cell = usedCell(container);
    await waitFor(() => expect(within(cell).getByText(label)).toBeInTheDocument());
    expect(cell.textContent).not.toMatch(/%|0 B/);
    expect(cell.querySelector('.bg-emerald-500')).toBeNull();
    expect(cell.querySelector('[title]')?.getAttribute('title')).toContain(
      'does not mean the datastore is empty',
    );
    expect(container.textContent).not.toContain('2.0×');
    expect(container.innerHTML).not.toContain('PRIVATE_PROVIDER_ERROR_SENTINEL');
    // Connection health is on the server header, independent of capacity.
    expect(within(container).getByRole('img', { name: 'Healthy' })).toBeInTheDocument();

    expandTank();
    const detail = selectedTank();
    const dot = within(detail).getByRole('img', {
      name: `Datastore capacity ${label.toLowerCase()}`,
    });
    expect(dot).toHaveClass('bg-amber-500');
    expect(detail).toHaveTextContent(`${label}. This does not mean the datastore is empty.`);
    expect(detail.innerHTML).not.toContain('PRIVATE_PROVIDER_ERROR_SENTINEL');
  });

  it.each([NaN, -1, null, '0'])(
    'uses valid bytes even when optional percentage metadata is invalid (%s)',
    (usagePercent) => {
      const { container } = render(() => (
        <ProxmoxBackupServersTable
          servers={[server(datastore({ usagePercent }))]}
          layoutWidth={() => 1200}
        />
      ));
      const cell = usedCell(container);
      expect(cell).toHaveTextContent('40.0%');
      expect(cell).toHaveTextContent('400 B / 1000 B');
      expect(cell.querySelector('.bg-emerald-500')).not.toBeNull();
      expect(within(cell).queryByText('Unknown')).toBeNull();
      expect(container.textContent).toContain('2.0×');
    },
  );

  it('withdraws retained capacity and dedup when the PBS connection is offline', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[server(datastore(), 'offline')]}
        layoutWidth={() => 1200}
      />
    ));
    expect(usedCell(container).textContent).toBe('Unavailable');
    expect(container.textContent).not.toContain('2.0×');
  });

  it.each([
    [0, '0.0%', 'bg-emerald-500'],
    [750, '75.0%', 'bg-amber-500'],
    [900, '90.0%', 'bg-red-500'],
    [1100, '110.0%', 'bg-red-500'],
  ])('keeps measured usage %s and the existing warning thresholds', async (used, label, tone) => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server(datastore({ used }))]} layoutWidth={() => 1200} />
    ));
    const cell = usedCell(container);
    await waitFor(() => expect(within(cell).getByText(label)).toBeInTheDocument());
    expect(cell.querySelector(`.${tone}`)).not.toBeNull();
    expect(container.textContent).toContain('2.0×');
  });

  it('keeps a server without datastore inventory distinct from an unknown named store', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server()]} layoutWidth={() => 1200} />
    ));
    expect(usedCell(container).textContent).toBe('No datastore data');
  });

  it('keeps a known failed verification visible when capacity also fails', () => {
    const resource = server(datastore({ status: 'unavailable' }));
    resource.pbs!.jobHealthEvidence = [
      {
        id: 'verify-tank',
        family: 'verify',
        store: 'tank',
        enabled: true,
        confidence: 'config-last-run',
        posture: 'critical',
        postureReason: 'last-run-failed',
      },
    ];
    render(() => <ProxmoxBackupServersTable servers={[resource]} layoutWidth={() => 360} />);
    expandTank();
    const detail = selectedTank();
    expect(within(detail).getByRole('img', { name: 'Verify failed' })).toHaveClass('bg-red-500');
    expect(detail).toHaveTextContent('Unavailable. This does not mean the datastore is empty.');
    expect(within(detail).getByText('Verify failed')).toBeVisible();
  });

  it.each(['direct', 'canonical'] as const)(
    'reconciles same-identity %s failure and recovery without remounting',
    async (transport) => {
      const present = (store: ResourcePBSDatastore): Resource => {
        const resource = server(store);
        if (transport === 'direct') return resource;
        const { pbs, ...base } = resource;
        return canonicalizeRealtimeResource({ ...base, platformData: { sources: ['pbs'], pbs } });
      };
      const [servers, setServers] = createSignal([present(datastore())]);
      const { container } = render(() => (
        <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => 1200} />
      ));
      const cell = usedCell(container);
      await waitFor(() => expect(cell.textContent).toContain('40.0%'));
      const toggle = expandTank();
      toggle.focus();
      const detail = selectedTank();
      expect(within(detail).getByRole('img', { name: 'Healthy' })).toHaveClass('bg-emerald-500');
      setServers([
        present(datastore({ total: 0, used: 0, usagePercent: 0, status: 'unavailable' })),
      ]);
      await waitFor(() => expect(cell.textContent).toBe('Unavailable'));
      expect(usedCell(container)).toBe(cell);
      expect(selectedTank()).toBe(detail);
      expect(
        within(detail).getByRole('img', { name: 'Datastore capacity unavailable' }),
      ).toHaveClass('bg-amber-500');
      setServers([present(datastore({ used: 0, usagePercent: 0 }))]);
      await waitFor(() => expect(cell.textContent).toContain('0.0%'));
      expect(usedCell(container)).toBe(cell);
      expect(selectedTank()).toBe(detail);
      expect(within(detail).getByRole('img', { name: 'Healthy' })).toHaveClass('bg-emerald-500');
      expect(toggle).toHaveFocus();
      setServers([present(datastore({ used: 950, usagePercent: 95 }))]);
      await waitFor(() => expect(cell.textContent).toContain('95.0%'));
      expect(cell.querySelector('.bg-red-500')).not.toBeNull();
    },
  );
});
