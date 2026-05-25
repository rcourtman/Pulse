import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { InfrastructurePageSurface } from '../InfrastructurePageSurface';

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  useInfrastructurePageState: vi.fn(),
}));

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    hash: '',
    pathname: '/infrastructure',
    search: '',
  }),
  useNavigate: () => mocks.navigate,
}));

vi.mock('../useInfrastructurePageState', () => ({
  useInfrastructurePageState: mocks.useInfrastructurePageState,
}));

vi.mock('@/components/Infrastructure/InfrastructureSummary', () => ({
  InfrastructureSummary: () => <div data-testid="infrastructure-summary" />,
}));

vi.mock('@/components/Infrastructure/UnifiedResourceTable', () => ({
  UnifiedResourceTable: (props: { resources: Resource[] }) => (
    <div data-testid="infrastructure-table" data-resource-count={props.resources.length} />
  ),
}));

vi.mock('@/components/Infrastructure/AgentDeployModal', () => ({
  AgentDeployModal: () => <div data-testid="agent-deploy-modal" />,
}));

vi.mock('@/components/shared/ScrollToTopButton', () => ({
  ScrollToTopButton: () => <button type="button">Scroll to top</button>,
}));

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'resource-availability-1',
  name: 'mqtt-meter',
  displayName: 'MQTT power meter',
  platformId: 'mock-availability-mqtt-meter',
  platformType: 'availability',
  sourceType: 'api',
  status: 'online',
  type: 'network-endpoint',
  lastSeen: 1_700_000_000_000,
  platformData: { sources: ['availability'] },
  tags: [],
  ...overrides,
});

const createPageState = (overrides: Record<string, unknown> = {}) => ({
  activeSummaryResourceGroupScope: () => null,
  activeSummaryResourceId: () => null,
  chartHoverSync: () => null,
  clearFilters: vi.fn(),
  clearPinnedSummaryScope: vi.fn(),
  deployCluster: () => null,
  error: () => null,
  expandedResourceId: () => null,
  filteredResources: () => [makeResource()],
  focusedSummaryResourceGroupId: () => null,
  focusedSummaryResourceGroupScope: () => null,
  groupingMode: () => 'grouped',
  hasActiveFilters: () => false,
  hasFilteredResources: () => true,
  highlightedResourceId: () => null,
  hoveredResourceId: () => null,
  hoveredSummaryResourceGroupScope: () => null,
  infrastructureSummaryRange: () => '1h',
  initialLoadComplete: () => true,
  isMobile: () => false,
  jumpToActiveResourceRow: vi.fn(),
  kioskMode: () => false,
  loading: () => false,
  refetch: vi.fn(),
  revealedResourceId: () => null,
  searchQuery: () => '',
  selectedSource: () => '',
  selectedStatus: () => '',
  setChartHoverSync: vi.fn(),
  setDeployCluster: vi.fn(),
  setExpandedResourceId: vi.fn(),
  setFocusedResourceGroupId: vi.fn(),
  setHoveredResourceGroupScope: vi.fn(),
  setHoveredResourceId: vi.fn(),
  setInfrastructureSummaryRange: vi.fn(),
  setGroupingMode: vi.fn(),
  setSearchQuery: vi.fn(),
  setSelectedSource: vi.fn(),
  setSelectedStatus: vi.fn(),
  setSummaryClearSurfaceRootRef: vi.fn(),
  setSummaryCollapsed: vi.fn(),
  setSummaryTableRootRef: vi.fn(),
  shouldShowJumpToActiveResourceRow: () => false,
  showNoResources: () => false,
  sourceOptions: () => [{ key: 'availability', label: 'Availability' }],
  statusOptions: () => [{ key: 'online', label: 'Online' }],
  summaryCollapsed: () => false,
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('InfrastructurePageSurface', () => {
  it('surfaces availability endpoint rows through a visible source shortcut', () => {
    const setSelectedSource = vi.fn();
    mocks.useInfrastructurePageState.mockReturnValue(createPageState({ setSelectedSource }));

    render(() => <InfrastructurePageSurface />);

    const button = screen.getByRole('button', { name: 'Show availability endpoints' });
    expect(button).toHaveTextContent('Endpoints');

    fireEvent.click(button);

    expect(setSelectedSource).toHaveBeenCalledWith('availability');
  });

  it('clears the availability source shortcut when it is active', () => {
    const setSelectedSource = vi.fn();
    mocks.useInfrastructurePageState.mockReturnValue(
      createPageState({
        selectedSource: () => 'availability',
        setSelectedSource,
      }),
    );

    render(() => <InfrastructurePageSurface />);

    const button = screen.getByRole('button', { name: 'Show all infrastructure sources' });
    expect(button).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(button);

    expect(setSelectedSource).toHaveBeenCalledWith('');
  });
});
