import { cleanup, fireEvent, render, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Details</div>,
}));
afterEach(cleanup);

const server = (patch: Partial<ResourcePBSDatastore>, canonical: boolean): Resource => {
  const pbs = {
    instanceId: 'pbs-consistency',
    connectionHealth: 'healthy',
    datastores: [
      {
        name: 'tank',
        total: 10000,
        used: 9500,
        available: 500,
        usagePercent: 95,
        status: 'available',
        deduplicationFactor: 2,
        ...patch,
      },
    ],
  };
  const resource: Resource = {
    id: 'pbs-consistency',
    type: 'pbs',
    name: 'pbs-consistency',
    displayName: 'pbs-consistency',
    platformId: 'pbs-consistency',
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.parse('2026-10-22T04:00:00Z'),
    ...(canonical ? { platformData: { sources: ['pbs'], pbs } } : { pbs }),
  };
  return canonical ? canonicalizeRealtimeResource(resource) : resource;
};
const usedCell = (container: HTMLElement) => {
  const table = container.querySelector('table')!;
  const column = Array.from(table.querySelectorAll('th')).findIndex(
    (head) => head.textContent === 'Used',
  );
  return table.querySelector('tbody tr')!.querySelectorAll('td')[column] as HTMLElement;
};

describe.each([false, true])('PBS capacity agreement, canonical=%s', (canonical) => {
  it.each([
    ['false empty', { usagePercent: 0 }],
    ['false healthy', { usagePercent: 40 }],
    ['false full', { used: 1000, usagePercent: 99 }],
    ['outside rounding budget', { usagePercent: 94.49 }],
    [
      'overflowing byte ratio despite finite percent',
      { used: Number.MAX_VALUE, total: Number.MIN_VALUE },
    ],
  ])('keeps %s unknown rather than selecting a contradictory reading', async (_name, patch) => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[server(patch, canonical)]} layoutWidth={() => 1200} />
    ));
    const cell = usedCell(container);
    const warning = within(cell).getByRole('button', {
      name: 'Explain unknown capacity for pbs-consistency / tank',
    });
    expect(cell.textContent).toBe('Unknown');
    expect(cell.querySelector('.bg-emerald-500, .bg-red-500')).toBeNull();
    expect(container.textContent).not.toContain('2.0×');
    fireEvent.click(warning);
    const region = await within(container).findByRole('region', {
      name: 'Datastore capacity for pbs-consistency / tank',
    });
    expect(region).toHaveTextContent('Datastore capacity is unknown.');
    expect(region).toHaveTextContent('This does not mean the datastore is empty.');
    if (patch.total !== Number.MIN_VALUE) {
      expect(region).toHaveTextContent(
        'The reported percentage disagrees with the used and total bytes.',
      );
    }
    expect(region.textContent).not.toMatch(/Used: .*%/);
    expect(within(container).getByText('Healthy')).toBeInTheDocument();
  });

  it.each([
    ['rounded down', { used: 9010, usagePercent: 90 }, '90.1%', 'bg-red-500'],
    ['rounded up', { used: 7460, usagePercent: 75 }, '74.6%', 'bg-emerald-500'],
    ['half-point rounding', { usagePercent: 94.5 }, '95.0%', 'bg-red-500'],
    ['no supplied percentage', { usagePercent: undefined }, '95.0%', 'bg-red-500'],
    ['overfull byte counts', { used: 11000, usagePercent: 110 }, '110.0%', 'bg-red-500'],
    ['measured empty', { used: 0, usagePercent: 0 }, '0.0%', 'bg-emerald-500'],
  ])(
    'keeps %s usable and derives the display and thresholds from bytes',
    async (_name, patch, label, tone) => {
      const { container } = render(() => (
        <ProxmoxBackupServersTable servers={[server(patch, canonical)]} layoutWidth={() => 1200} />
      ));
      const cell = usedCell(container);
      await waitFor(() => expect(within(cell).getByText(label)).toBeInTheDocument());
      expect(cell.querySelector(`.${tone}`)).not.toBeNull();
      expect(container.textContent).toContain('2.0×');
    },
  );

  it('reconciles contradiction and recovery within the same open datastore and focused warning', async () => {
    const [servers, setServers] = createSignal([server({ usagePercent: 0 }, canonical)]);
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => 320} />
    ));
    const cell = usedCell(container);
    const warning = within(cell).getByRole('button', { name: /Explain unknown capacity/ });
    warning.focus();
    fireEvent.click(warning);
    const region = within(container).getByRole('region', {
      name: 'Datastore capacity for pbs-consistency / tank',
    });
    expect(region).toHaveTextContent('The reported percentage disagrees');
    setServers([server({ usagePercent: 40 }, canonical)]);
    await waitFor(() => expect(usedCell(container)).toBe(cell));
    expect(within(cell).getByRole('button')).toBe(warning);
    expect(warning).toHaveFocus();
    expect(region).toBeInTheDocument();
    setServers([server({}, canonical)]);
    await waitFor(() => expect(region).toHaveTextContent('Used: 95.0%'));
    expect(region).not.toHaveTextContent('disagrees');
    expect(region).not.toHaveTextContent('does not mean');
    expect(usedCell(container)).toBe(cell);
    expect(within(cell).queryByRole('button')).toBeNull();
  });
});
