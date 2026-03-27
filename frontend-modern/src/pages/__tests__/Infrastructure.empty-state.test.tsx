import { fireEvent, render, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Infrastructure } from '@/pages/Infrastructure';

const navigateSpy = vi.fn();

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({
      pathname: '/infrastructure',
      get search() {
        return '';
      },
    }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    resources: () => [],
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(),
  }),
}));

vi.mock('@/components/Infrastructure/UnifiedResourceTable', () => ({
  UnifiedResourceTable: () => <div data-testid="infra-table" />,
}));

vi.mock('@/components/Infrastructure/InfrastructureSummary', () => ({
  InfrastructureSummary: () => <div data-testid="infra-summary" />,
}));

describe('Infrastructure empty state', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
  });

  it('shows empty state with direct install guidance when no resources exist', async () => {
    const { getByText } = render(() => <Infrastructure />);

    await waitFor(() => {
      expect(getByText('No infrastructure resources yet')).toBeInTheDocument();
    });

    const button = getByText('Open Infrastructure Install');
    expect(button).toBeInTheDocument();
    expect(button.closest('button')).toBeInTheDocument();
  });

  it('navigates to the install workspace when the empty-state button is clicked', async () => {
    const { getByText } = render(() => <Infrastructure />);

    await waitFor(() => {
      expect(getByText('Open Infrastructure Install')).toBeInTheDocument();
    });

    fireEvent.click(getByText('Open Infrastructure Install'));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
