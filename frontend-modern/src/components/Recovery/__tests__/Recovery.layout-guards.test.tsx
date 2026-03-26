import { createSignal } from 'solid-js';
import { fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

let mockRecoverySurfaceState: any;

vi.mock('@/features/recovery/useRecoverySurfaceState', () => ({
  useRecoverySurfaceState: () => mockRecoverySurfaceState,
}));

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({ isMobile: () => false }),
}));

vi.mock('@/hooks/useKioskMode', () => ({
  useKioskMode: () => () => false,
}));

vi.mock('@/hooks/useColumnVisibility', () => ({
  useColumnVisibility: () => ({
    availableToggles: () => [],
    isHiddenByUser: () => false,
    resetToDefaults: vi.fn(),
    toggle: vi.fn(),
    visibleColumns: () => [],
  }),
}));

vi.mock('@/components/Recovery/RecoveryProtectedInventorySection', () => ({
  RecoveryProtectedInventorySection: () => <div data-testid="protected-inventory" />,
}));

vi.mock('@/components/Recovery/RecoveryActivitySection', () => ({
  RecoveryActivitySection: () => <div data-testid="activity-chart" />,
}));

vi.mock('@/components/Recovery/RecoveryHistorySection', () => ({
  RecoveryHistorySection: () => <div data-testid="history-section" />,
}));

import Recovery from '@/components/Recovery/Recovery';

describe('Recovery layout guards', () => {
  beforeEach(() => {
    const [workspaceView, setWorkspaceView] = createSignal<'inventory' | 'events'>('inventory');
    mockRecoverySurfaceState = {
      activitySummary: () => ({ totalPoints: 2, activeDays: 2, averagePerDay: 1 }),
      activeAdvancedFilterCount: () => 0,
      activeClusterLabel: () => '',
      activeNamespaceLabel: () => '',
      activeNodeLabel: () => '',
      artifactColumnVisibility: undefined,
      chartRangeDays: () => 30,
      clearClusterFilter: vi.fn(),
      clearFocusedRollup: vi.fn(),
      clearNamespaceFilter: vi.fn(),
      clearNodeFilter: vi.fn(),
      clearSelectedDate: vi.fn(),
      clusterFilter: () => 'all',
      clusterOptions: () => ['all'],
      currentPage: () => 1,
      facets: () => ({}),
      hasActiveArtifactFilters: () => false,
      hasFocusedRollup: () => false,
      historyOutcomeFilter: () => 'all',
      itemTypeFilter: () => 'all',
      itemTypeOptions: () => ['all'],
      isMobile: false,
      loading: () => false,
      modeFilter: () => 'all',
      namespaceFilter: () => 'all',
      namespaceOptions: () => ['all'],
      nodeFilter: () => 'all',
      nodeOptions: () => ['all'],
      overallRollupsSummary: () => ({ total: 2, stale: 0, neverSucceeded: 0 }),
      protectedStaleOnly: () => false,
      platformFilter: () => 'all',
      platformOptions: () => ['all'],
      queryFilter: () => '',
      recoveryPoints: {
        meta: () => ({ page: 1, limit: 200, total: 0, totalPages: 1 }),
        points: () => [],
        response: {
          error: new Error('history points unavailable'),
          loading: false,
        },
      },
      recoveryRollups: {
        rollups: () => [],
        response: { error: undefined, loading: false },
      },
      recoverySeries: {
        response: { error: undefined, loading: false },
        series: () => [
          { day: '2026-02-13', total: 1, snapshot: 1, local: 0, remote: 0 },
          { day: '2026-02-14', total: 1, snapshot: 0, local: 1, remote: 0 },
        ],
      },
      resourcesById: () => new Map(),
      rollupId: () => '',
      rollups: () => [],
      scopeFilter: () => 'all',
      selectedDateKey: () => null,
      selectedDateLabel: () => '',
      selectedHistoryItemLabel: () => null,
      setChartRangeDays: vi.fn(),
      setClusterFilter: vi.fn(),
      setCurrentPage: vi.fn(),
      setHistoryOutcomeFilter: vi.fn(),
      setItemTypeFilter: vi.fn(),
      setModeFilter: vi.fn(),
      setNamespaceFilter: vi.fn(),
      setNodeFilter: vi.fn(),
      setProtectedStaleOnly: vi.fn(),
      setPlatformFilter: vi.fn(),
      setQueryFilter: vi.fn(),
      setRollupId: vi.fn(),
      setScopeFilter: vi.fn(),
      setSelectedDateKey: vi.fn(),
      setVerificationFilter: vi.fn(),
      showClusterFilter: () => false,
      showNamespaceFilter: () => false,
      showNodeFilter: () => false,
      showVerificationFilter: () => false,
      tableWindow: () => ({ from: null, to: null }),
      tableColumnCount: () => 0,
      tableMinWidth: () => 'auto',
      totalPages: () => 1,
      verificationFilter: () => 'all',
      workspaceView,
      setWorkspaceView,
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('keeps the activity chart mounted when recovery events fail', () => {
    render(() => <Recovery />);

    expect(screen.getByTestId('protected-inventory')).toBeInTheDocument();
    expect(screen.getByTestId('activity-chart')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: /recovery events/i }));
    expect(screen.queryByTestId('protected-inventory')).not.toBeInTheDocument();
    expect(screen.queryByTestId('history-section')).not.toBeInTheDocument();
    expect(screen.getByText('Failed to load recovery points')).toBeInTheDocument();
  });
});
