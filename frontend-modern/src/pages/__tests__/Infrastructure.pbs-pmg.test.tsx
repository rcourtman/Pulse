import { render, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { Infrastructure } from '@/pages/Infrastructure';

const mockResources: Resource[] = [
  {
    id: 'pbs-main',
    type: 'pbs',
    name: 'pbs-main',
    displayName: 'PBS Main',
    platformId: 'pbs-main',
    platformType: 'proxmox-pbs',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { sources: ['pbs'] },
  },
  {
    id: 'pmg-main',
    type: 'pmg',
    name: 'pmg-main',
    displayName: 'PMG Main',
    platformId: 'pmg-main',
    platformType: 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { sources: ['pmg'] },
  },
];
let mockLocationSearch = '';
const navigateSpy = vi.fn();

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({
      pathname: '/infrastructure',
      get search() {
        return mockLocationSearch;
      },
    }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    resources: () => mockResources,
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
  }),
}));

vi.mock('@/components/Infrastructure/UnifiedResourceTable', () => ({
  UnifiedResourceTable: (props: {
    resources: Resource[];
    onExpandedResourceChange?: (id: string | null) => void;
  }) => (
    <div data-testid="infra-table">
      {props.resources.map((resource) => resource.name).join(',')}
      <button type="button" onClick={() => props.onExpandedResourceChange?.('pmg-main')}>
        open-pmg
      </button>
      <button type="button" onClick={() => props.onExpandedResourceChange?.(null)}>
        close-resource
      </button>
    </div>
  ),
}));

vi.mock('@/components/Infrastructure/InfrastructureSummary', () => ({
  InfrastructureSummary: (props: { resources: Resource[] }) => (
    <div data-testid="infra-summary">{props.resources.length} resources</div>
  ),
}));

describe('Infrastructure PBS/PMG integration', () => {
  beforeEach(() => {
    // Ensure the desktop filter controls are rendered (some suites resize the viewport).
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1024,
    });
    window.dispatchEvent(new Event('resize'));

    mockLocationSearch = '';
    navigateSpy.mockReset();
    navigateSpy.mockImplementation((path: string) => {
      const queryStart = path.indexOf('?');
      mockLocationSearch = queryStart >= 0 ? path.slice(queryStart) : '';
    });
  });

  it('renders native PBS and PMG resources in infrastructure view', async () => {
    const { getByTestId, getByText } = render(() => <Infrastructure />);

    expect(getByTestId('infrastructure-page')).toBeInTheDocument();
    expect(getByTestId('infra-summary')).toBeInTheDocument();
    // The mocked UnifiedResourceTable above renders resource.name joined by
    // commas, not platform-badge labels, so 'PBS'/'PMG' text appearance was
    // checking a render path the mocks short-circuit. The substantive
    // assertions are that PBS and PMG rows make it through to the table and
    // that the summary counts them.
    expect(getByTestId('infra-table')).toHaveTextContent('pbs-main,pmg-main');
    expect(getByText('2 resources')).toBeInTheDocument();
  });

  it('keeps canonical q search query params', async () => {
    mockLocationSearch = '?q=pmg';

    render(() => <Infrastructure />);

    await Promise.resolve();
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  // The 'syncs source filter selection to query params' test has been
  // removed. It drove the source filter via fireEvent.change on a labeled
  // <select>, but Infrastructure migrated to the FilterBar chip pattern
  // (PageControls / LabeledFilterSelect have been phased out). With
  // UnifiedResourceTable mocked, there is no FilterBar surface for the
  // page-level test to drive, and the state -> URL coupling is now
  // exercised inside FilterBar / source-filter unit tests rather than at
  // the Infrastructure page boundary.

  it('syncs expanded resource state to resource query param for deep-linking', async () => {
    const { getByRole } = render(() => <Infrastructure />);
    navigateSpy.mockClear();

    getByRole('button', { name: 'open-pmg' }).click();

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    let [path] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    let params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('resource')).toBe('pmg-main');

    getByRole('button', { name: 'close-resource' }).click();

    await waitFor(() => {
      [path] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
      params = new URLSearchParams(path.split('?')[1] || '');
      expect(params.get('resource')).toBeNull();
    });
  });
});
