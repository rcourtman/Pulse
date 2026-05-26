import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    fallbackMetrics?: Record<string, number | undefined>;
  }) => (
    <div
      data-testid="machine-history"
      data-resource-type={props.target?.resourceType}
      data-resource-id={props.target?.resourceId}
      data-range={props.range}
      data-cpu={props.fallbackMetrics?.cpu}
      data-netin={props.fallbackMetrics?.netin}
    />
  ),
  GuestDrawerHistoryRangeSelect: (props: {
    range: string;
    onRangeChange: (range: string) => void;
  }) => (
    <select
      aria-label="History range"
      value={props.range}
      onChange={(event) => props.onRangeChange(event.currentTarget.value)}
    >
      <option value="24h">24 hours</option>
      <option value="7d">7 days</option>
    </select>
  ),
}));

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'agent-1',
    name: overrides.name ?? overrides.id ?? 'agent-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'agent-1',
    type: overrides.type ?? 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('ResourceDetailDrawer machine metrics history', () => {
  it('adds a metrics history tab for Pulse Agent machines', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'agent-mac-mini',
          metricsTarget: { resourceType: 'agent', resourceId: 'agent-mac-mini' },
          cpu: { current: 42 },
          network: { rxBytes: 1024, txBytes: 2048 },
        })}
      />
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'History' }));

    const history = screen.getByTestId('machine-history');
    expect(history).toHaveAttribute('data-resource-type', 'agent');
    expect(history).toHaveAttribute('data-resource-id', 'agent-mac-mini');
    expect(history).toHaveAttribute('data-range', '24h');
    expect(history).toHaveAttribute('data-cpu', '42');
    expect(history).toHaveAttribute('data-netin', '1024');
  });

  it('does not add machine history to non-agent availability resources', () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'ping-mac-mini',
          type: 'network-endpoint',
          platformType: 'availability',
          sourceType: 'api',
        })}
      />
    ));

    expect(screen.queryByRole('button', { name: 'History' })).not.toBeInTheDocument();
  });
});
