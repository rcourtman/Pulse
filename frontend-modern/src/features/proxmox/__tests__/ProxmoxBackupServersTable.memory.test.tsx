import { cleanup, render, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource, ResourceMetric } from '@/types/resource';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { buildBackupServerRows, ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Server details</div>,
}));

const NOW = '2026-10-29T20:00:00Z';
const OBSERVED = '2026-10-29T19:59:00Z';
const server = (memory?: ResourceMetric, extra: Partial<Resource> = {}): Resource =>
  canonicalizeRealtimeResource({
    id: 'pbs-memory',
    type: 'pbs',
    name: 'pbs-memory',
    displayName: 'pbs-memory',
    status: 'online',
    platformId: 'pbs-memory',
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    lastSeen: Date.parse(NOW),
    memory,
    pbs: {
      instanceId: 'pbs-memory',
      connectionHealth: 'healthy',
      datastores: [{ name: 'tank', total: 1000, used: 950, status: 'available' }],
    },
    ...extra,
  });

function memoryCell(container: HTMLElement): HTMLElement {
  const table = container.querySelector('table')!;
  const index = [...table.querySelectorAll('th')].findIndex(
    (head) => head.textContent === 'Memory',
  );
  expect(index).toBeGreaterThan(-1);
  return table.querySelector('tbody tr')!.querySelectorAll('td')[index] as HTMLElement;
}

const show = (resource: Resource, width = 1200) =>
  render(() => <ProxmoxBackupServersTable servers={[resource]} layoutWidth={() => width} />);

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] });
  vi.setSystemTime(new Date(NOW));
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('PBS memory evidence', () => {
  it('[partial-memory] keeps a percentage-only reading instead of inventing zero used bytes', () => {
    const { container } = show(server({ current: 75, total: 8192 }));
    const cell = memoryCell(container);
    expect(cell.textContent).toBe('75.0%');
    expect(cell.innerHTML).not.toContain('0 B');
    expect(cell.querySelector('[title]')).toBeNull();
    const row = buildBackupServerRows([server({ current: 75, total: 8192 })])[0];
    expect(row.memoryUsed).toBeUndefined();
    expect(row.memoryTotal).toBe(8192);
    expect(row.memoryPercent).toBe(75);
  });

  it('uses reported byte pairs, including measured zero, ahead of inconsistent percentage metadata', () => {
    const positive = show(server({ current: 0, total: 1000, used: 400 }));
    expect(memoryCell(positive.container).textContent).toContain('40.0%');
    expect(memoryCell(positive.container).textContent).toContain('400 B/1000 B');
    positive.unmount();
    const zero = show(server({ current: 99, total: 1000, used: 0 }));
    expect(memoryCell(zero.container).textContent).toBe('0.0%(0 B/1000 B)');
  });

  it('preserves a percentage-only measured zero without claiming a byte amount', () => {
    const { container } = show(server({ current: 0, total: 1000 }));
    expect(memoryCell(container).textContent).toBe('0.0%');
  });

  it.each([undefined, null, '40', -1, 101, NaN, Infinity])(
    'does not turn invalid or absent percentage %s into a measured zero',
    (current) => {
      const { container } = show(server({ current, total: 1000 } as ResourceMetric));
      expect(memoryCell(container).textContent).toBe('—');
    },
  );

  it.each([
    { used: -1 },
    { used: 1001 },
    { used: NaN },
    { used: Infinity },
    { used: null },
    { used: '400' },
    { total: 0 },
    { total: -1 },
    { total: Infinity },
    { total: '1000' },
  ])('keeps a valid selected percentage without displaying an invalid byte pair %j', (patch) => {
    const { container } = show(
      server({ current: 40, total: 1000, used: 400, ...patch } as ResourceMetric),
    );
    expect(memoryCell(container).textContent).toBe('40.0%');
  });

  it.each([
    ['last-known', OBSERVED, 'Last known'],
    ['new-state', OBSERVED, 'Freshness unknown'],
    ['current', undefined, 'Freshness unknown'],
    ['current', '2026-10-30T20:00:00Z', 'Freshness unknown'],
  ])(
    '[retained-memory] qualifies %s observations without renewing their clock',
    (state, observedAt, label) => {
      const { container } = show(
        server({
          current: 40,
          total: 1000,
          used: 400,
          observation: { state, source: 'agent', observedAt },
        }),
      );
      const cell = memoryCell(container);
      expect(cell.textContent).toContain('40.0%');
      expect(within(cell).getByText(label)).toBeInTheDocument();
      expect(cell.querySelector('[title]')?.getAttribute('title')).toContain(
        'Not a current measurement',
      );
      expect(within(container).getByText('Healthy')).toBeInTheDocument();
      expect(container.textContent).toContain('95.0%');
    },
  );

  it('[unavailable-memory] hides an explicitly unavailable reading even when old bytes remain', () => {
    const { container } = show(
      server({
        current: 40,
        total: 1000,
        used: 400,
        observation: { state: 'unavailable', source: 'agent', observedAt: OBSERVED },
      }),
    );
    expect(memoryCell(container).textContent).toBe('Unavailable');
    expect(memoryCell(container).innerHTML).not.toContain('400 B');
    expect(container.textContent).toContain('95.0%');
  });

  it('honours a stopped agent without invalidating an independent current memory source', () => {
    const resource = server(
      {
        current: 40,
        observation: { state: 'current', source: 'agent', observedAt: OBSERVED },
      },
      { agent: { agentId: 'host-agent', stale: true } },
    );
    const agent = show(resource);
    expect(within(memoryCell(agent.container)).getByText('Last known')).toBeInTheDocument();
    expect(memoryCell(agent.container).querySelector('[title]')?.getAttribute('title')).toContain(
      'stopped reporting',
    );
    agent.unmount();
    resource.memory!.observation!.source = 'available-field';
    const independent = show(resource);
    expect(memoryCell(independent.container).textContent).toBe('40.0%');
  });

  it('keeps partial, retained, unavailable and recovered reads on the same live row', async () => {
    const [resources, setResources] = createSignal([server({ current: 75, total: 1000 })]);
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={resources()} layoutWidth={() => 1200} />
    ));
    const cell = memoryCell(container);
    const row = cell.closest('tr');
    expect(cell.textContent).toBe('75.0%');
    setResources([
      server({
        current: 40,
        total: 1000,
        used: 400,
        observation: {
          state: 'last-known',
          source: 'agent',
          observedAt: OBSERVED,
        },
      }),
    ]);
    await waitFor(() => expect(cell.textContent).toContain('Last known'));
    setResources([
      server({
        current: 40,
        total: 1000,
        used: 400,
        observation: {
          state: 'unavailable',
          source: 'agent',
          observedAt: OBSERVED,
        },
      }),
    ]);
    await waitFor(() => expect(cell.textContent).toBe('Unavailable'));
    setResources([
      server({
        current: 20,
        total: 1000,
        used: 200,
        observation: {
          state: 'current',
          source: 'agent',
          observedAt: OBSERVED,
        },
      }),
    ]);
    await waitFor(() => expect(cell.textContent).toBe('20.0%(200 B/1000 B)'));
    expect(memoryCell(container)).toBe(cell);
    expect(cell.closest('tr')).toBe(row);
  });

  it('keeps host readings suppressed offline and present only once across datastores', () => {
    const resource = server({ current: 40, total: 1000, used: 400 });
    resource.pbs!.datastores!.push({ name: 'offsite', total: 1000, used: 500 });
    const online = show(resource);
    expect(online.container.textContent?.match(/40.0%/g)).toHaveLength(1);
    online.unmount();
    resource.pbs!.connectionHealth = 'unreachable';
    const offline = show(resource);
    expect(memoryCell(offline.container).textContent).toBe('—');
    expect(offline.container.textContent).not.toContain('40.0%');
  });
});
