import { render, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import RuntimeHome from '@/pages/RuntimeHome';

const navigateSpy = vi.hoisted(() => vi.fn());
const getDashboardSummaryMock = vi.hoisted(() => vi.fn());
const isHostedModeEnabledMock = vi.hoisted(() => vi.fn());
const connectedInfrastructureItems = vi.hoisted(() => [] as Array<Record<string, unknown>>);

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: {
      get connectedInfrastructure() {
        return connectedInfrastructureItems;
      },
    },
  }),
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getDashboardSummary: getDashboardSummaryMock,
  },
}));

vi.mock('@/stores/license', () => ({
  isHostedModeEnabled: () => isHostedModeEnabledMock(),
}));

describe('RuntimeHome', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    getDashboardSummaryMock.mockReset();
    connectedInfrastructureItems.length = 0;
    isHostedModeEnabledMock.mockReturnValue(true);
  });

  it('routes self-hosted runtimes straight to the dashboard', async () => {
    isHostedModeEnabledMock.mockReturnValue(false);

    render(() => <RuntimeHome />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/dashboard', { replace: true });
    });
    expect(getDashboardSummaryMock).not.toHaveBeenCalled();
  });

  it('routes hosted empty workspaces into infrastructure install', async () => {
    getDashboardSummaryMock.mockResolvedValue({
      health: { totalResources: 0 },
    });

    render(() => <RuntimeHome />);

    await waitFor(() => {
      expect(getDashboardSummaryMock).toHaveBeenCalledTimes(1);
      expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=agent', {
        replace: true,
      });
    });
  });

  it('keeps hosted workspaces with connected infrastructure on the dashboard', async () => {
    connectedInfrastructureItems.push({ id: 'agent-1' });

    render(() => <RuntimeHome />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/dashboard', { replace: true });
    });
    expect(getDashboardSummaryMock).not.toHaveBeenCalled();
  });
});
