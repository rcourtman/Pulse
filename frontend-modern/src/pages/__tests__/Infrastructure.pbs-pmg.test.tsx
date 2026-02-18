import { fireEvent, render, waitFor } from '@solidjs/testing-library';
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
  UnifiedResourceTable: (props: { resources: Resource[]; onExpandedResourceChange?: (id: string | null) => void }) => (
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
  InfrastructureSummary: (props: { hosts: Resource[] }) => (
    <div data-testid="infra-summary">{props.hosts.length} resources</div>
  ),
}));

describe('Infrastructure PBS/PMG integration', () => {
  beforeEach(() => {
    // Ensure the desktop filter controls are rendered (some suites resize the viewport).
    Object.defineProperty(window, 'innerWidth', { writable: true, configurable: true, value: 1024 });
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

    expect(getByTestId('infra-summary')).toBeInTheDocument();
    expect(getByTestId('infra-table')).toHaveTextContent('pbs-main,pmg-main');
    expect(getByText('PBS')).toBeInTheDocument();
    expect(getByText('PMG')).toBeInTheDocument();
    expect(getByText('2 resources')).toBeInTheDocument();
  });

  it('canonicalizes legacy search query params to q', async () => {
    mockLocationSearch = '?search=pmg&migrated=1&from=services';

    render(() => <Infrastructure />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('pmg');
    expect(params.get('search')).toBeNull();
    expect(params.get('migrated')).toBe('1');
    expect(params.get('from')).toBe('services');
    expect(options?.replace).toBe(true);
  });

  it('syncs source filter selection to query params', async () => {
    const { getByLabelText } = render(() => <Infrastructure />);

    fireEvent.change(getByLabelText('Source'), { target: { value: 'pmg' } });

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('source')).toBe('pmg');
    expect(options?.replace).toBe(true);
  });

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
