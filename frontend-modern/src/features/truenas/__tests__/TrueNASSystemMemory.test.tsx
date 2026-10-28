import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import type { Resource } from '@/types/resource';
import { TrueNASSystemsTable } from '../TrueNASSystemsTable';

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} }),
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 80, critical: 85 }),
  }),
}));

const GB = 1024 ** 3;
const system = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'nas-core',
  name: 'nas-core',
  type: 'agent',
  platformId: 'nas-core',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  cpu: { current: 12 },
  memory: { current: 31.25, used: 10 * GB, total: 32 * GB },
  agent: { osVersion: 'TrueNAS-13.3', memory: { cache: 20 * GB } },
  ...overrides,
});

function mount(resources = [system()]) {
  const [systems, setSystems] = createStore(resources);
  const result = render(() => (
    <TrueNASSystemsTable
      systems={systems}
      scope={systems}
      emptyIcon={<span />}
      emptyTitle="No systems"
      emptyDescription="No systems"
      showToolbar={false}
    />
  ));
  return { ...result, setSystems };
}

const rowValue = (dialog: HTMLElement, label: string) =>
  within(dialog).getByText(label, { exact: true }).nextElementSibling;

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('TrueNAS system memory inspection through the actual table', () => {
  it('opens the existing composition without expanding the system and returns focus on Escape', async () => {
    const { container } = mount();
    const trigger = screen.getByRole('button', { name: 'Memory composition for nas-core' });
    expect(trigger).toHaveAttribute('aria-haspopup', 'dialog');
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    trigger.focus();
    fireEvent.click(trigger);
    const dialog = screen.getByRole('dialog', { name: 'Memory composition for nas-core' });
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(rowValue(dialog, 'Used')).toHaveTextContent('10.0 GB');
    expect(rowValue(dialog, 'Reclaimable cache')).toHaveTextContent('20.0 GB');
    expect(rowValue(dialog, 'Free')).toHaveTextContent('2.00 GB');
    expect(rowValue(dialog, 'Used with ARC cache')).toHaveTextContent('94%');
    expect(container.querySelector('[data-inline-platform-resource-detail-for]')).toBeNull();
    await waitFor(() =>
      expect(within(dialog).getByRole('button', { name: 'Close' })).toHaveFocus(),
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
  });

  it.each([undefined, 0])('does not invent an ARC row for cache %s', (cache) => {
    mount([
      system({
        memory: { current: 93.75, used: 30 * GB, total: 32 * GB },
        agent: { memory: cache === undefined ? undefined : { cache } },
      }),
    ]);
    fireEvent.click(screen.getByRole('button', { name: 'Memory composition for nas-core' }));
    const dialog = screen.getByRole('dialog');
    expect(rowValue(dialog, 'Used')).toHaveTextContent('30.0 GB');
    expect(rowValue(dialog, 'Free')).toHaveTextContent('2.00 GB');
    expect(within(dialog).queryByText('Reclaimable cache')).toBeNull();
    expect(within(dialog).queryByText('Used with ARC cache')).toBeNull();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('retains a measured zero and does not turn percentage-only readings into bytes', () => {
    mount([
      system({
        id: 'zero',
        name: 'zero',
        memory: { current: 0, used: 0, total: 32 * GB },
        agent: {},
      }),
      system({ id: 'percent', name: 'percent', memory: { current: 73 }, agent: {} }),
    ]);
    fireEvent.click(screen.getByRole('button', { name: 'Memory composition for zero' }));
    let dialog = screen.getByRole('dialog');
    expect(rowValue(dialog, 'Used')).toHaveTextContent('0 B');
    expect(rowValue(dialog, 'Free')).toHaveTextContent('32.0 GB');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close' }));
    fireEvent.click(screen.getByRole('button', { name: 'Memory composition for percent' }));
    dialog = screen.getByRole('dialog');
    expect(rowValue(dialog, 'Utilization')).toHaveTextContent('73%');
    expect(within(dialog).queryByText('Used', { exact: true })).toBeNull();
    expect(within(dialog).queryByText('Free', { exact: true })).toBeNull();
    expect(within(dialog).queryByText('Reclaimable cache')).toBeNull();
  });

  it('updates the open record from its owning resource and withdraws unavailable metrics', () => {
    const { setSystems } = mount();
    const trigger = screen.getByRole('button', { name: 'Memory composition for nas-core' });
    fireEvent.click(trigger);
    const dialog = screen.getByRole('dialog');
    setSystems(0, 'memory', 'used', 12 * GB);
    setSystems(0, 'agent', 'memory', 'cache', 18 * GB);
    expect(screen.getByRole('dialog')).toBe(dialog);
    expect(screen.getByRole('button', { name: 'Memory composition for nas-core' })).toBe(trigger);
    expect(rowValue(dialog, 'Used')).toHaveTextContent('12.0 GB');
    expect(rowValue(dialog, 'Reclaimable cache')).toHaveTextContent('18.0 GB');
    expect(rowValue(dialog, 'Free')).toHaveTextContent('2.00 GB');
    setSystems(0, 'agent', 'memory', 'cache', undefined);
    expect(within(dialog).queryByText('Reclaimable cache')).toBeNull();
    expect(rowValue(dialog, 'Free')).toHaveTextContent('20.0 GB');
    setSystems(0, 'status', 'unknown');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Memory composition for nas-core' })).toBeNull();
    expect(document.body.style.overflow).not.toBe('hidden');
  });

  it('offers no composition from an unreported metric or unknown system', () => {
    mount([system({ memory: undefined }), system({ id: 'unknown', status: 'unknown' })]);
    expect(screen.queryByRole('button', { name: /Memory composition for/ })).toBeNull();
  });

  it('dismisses by backdrop without forwarding the click to the enclosing resource row', () => {
    const { container } = mount();
    fireEvent.click(screen.getByRole('button', { name: 'Memory composition for nas-core' }));
    fireEvent.click(document.querySelector('[data-dialog-backdrop]')!);
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(container.querySelector('[data-inline-platform-resource-detail-for]')).toBeNull();
  });

  it('leaves non-opted-in shared bars without new controls', () => {
    render(() => <StackedMemoryBar used={10 * GB} total={32 * GB} cache={20 * GB} />);
    expect(screen.queryByRole('button')).toBeNull();
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});
