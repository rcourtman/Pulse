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
    useLocation: () => ({ pathname: '/infrastructure', search: mockLocationSearch }),
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
  UnifiedResourceTable: (props: { resources: Resource[] }) => (
    <div data-testid="infra-table">
      {props.resources.map((resource) => resource.name).join(',')}
    </div>
  ),
}));

vi.mock('@/components/Infrastructure/InfrastructureSummary', () => ({
  InfrastructureSummary: () => <div data-testid="infra-summary">summary</div>,
}));

describe('Infrastructure PBS/PMG integration', () => {
  beforeEach(() => {
    mockLocationSearch = '';
    navigateSpy.mockReset();
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
    const { getByRole } = render(() => <Infrastructure />);

    getByRole('button', { name: 'PMG' }).click();

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('source')).toBe('pmg');
    expect(options?.replace).toBe(true);
  });
});
