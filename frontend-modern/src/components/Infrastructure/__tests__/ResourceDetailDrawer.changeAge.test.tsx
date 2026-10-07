import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, within } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';

const facetBundleMock = vi.hoisted(() => ({
  getFacetBundle: vi.fn(),
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: { pmg: [] as unknown[] },
    activeAlerts: {},
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: facetBundleMock.getFacetBundle,
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: vi.fn().mockResolvedValue(null),
  },
}));

vi.mock('@/api/actionAudit', () => ({
  ActionAuditAPI: {
    listActionAudits: vi.fn().mockResolvedValue({ audits: [], count: 0, available: false }),
  },
}));

vi.mock('@/api/resourceOperatorState', () => ({
  getResourceOperatorState: vi.fn().mockResolvedValue(null),
  setResourceOperatorState: vi.fn(),
  clearResourceOperatorState: vi.fn(),
}));

if (typeof globalThis.ResizeObserver === 'undefined') {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
}

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe('ResourceDetailDrawer change history ages', () => {
  it('keeps change observed and occurred ages moving while the drawer stays open', async () => {
    vi.useFakeTimers({
      toFake: ['Date', 'setInterval', 'clearInterval'],
      now: Date.parse('2026-03-18T12:11:00Z'),
    });
    facetBundleMock.getFacetBundle.mockResolvedValue({
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          from: 'running',
          to: 'restarting',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
        },
      ],
      counts: { recentChanges: 1 },
    });

    render(() => (
      <ResourceDetailDrawer
        resource={
          {
            id: 'vm:42',
            type: 'vm',
            name: 'vm-42',
            displayName: 'VM 42',
            platformId: 'vm-42',
            platformType: 'proxmox-pve',
            sourceType: 'hybrid',
            status: 'online',
            lastSeen: Date.parse('2026-03-18T12:10:00Z'),
            platformData: { sources: ['proxmox'] },
          } as Resource
        }
      />
    ));

    await screen.findByText('Changes loaded');
    const history = () => within(screen.getByTestId('resource-change-history-section'));
    expect(history().getByText('5 mins ago')).toBeInTheDocument();
    expect(history().getByText('Occurred 7 mins ago')).toBeInTheDocument();

    // No new facet read lands: the change never changes and only the clock
    // moves.
    vi.advanceTimersByTime(2 * 60 * 2 * RELATIVE_TIME_TICK_MS);

    expect(history().getByText('2 hours ago')).toBeInTheDocument();
    expect(history().getByText('Occurred 2 hours ago')).toBeInTheDocument();
  });
});
