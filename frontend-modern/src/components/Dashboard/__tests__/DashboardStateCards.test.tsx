import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { DashboardStateCards } from '../DashboardStateCards';

describe('DashboardStateCards', () => {
  it('routes the empty-state action directly to infrastructure setup', () => {
    const navigate = vi.fn();

    render(() => (
      <DashboardStateCards
        allGuests={() => []}
        connected={() => true}
        dashboardDisconnectedState={() => ({
          title: 'Connection lost',
          description: 'Unable to connect to the backend server',
          actionLabel: 'Reconnect now',
        })}
        dashboardGuestsEmptyState={() => ({
          title: 'No guests found',
          description: 'No guests match your current filters',
        })}
        dashboardInfrastructureEmptyState={() => ({
          title: 'No infrastructure hosts connected',
          description:
            'To start using Pulse, first add your infrastructure in Settings → Infrastructure → Install on a host. If you want an API-backed platform such as Proxmox or TrueNAS instead, use Settings → Infrastructure → Platform connections.',
          actionLabel: 'Open infrastructure setup',
        })}
        dashboardLoadingState={() => ({
          title: 'Loading dashboard data...',
          description: 'Connecting to monitoring service',
        })}
        filteredGuests={() => []}
        initialDataReceived={() => true}
        kioskMode={() => false}
        navigate={navigate}
        nodeCount={0}
        reconnect={() => undefined}
        workloads={{
          workloads: [] as any,
          loading: () => false,
          refetch: async () => [],
          mutate: (value) => (Array.isArray(value) ? value : value([])),
          error: () => null,
        }}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Open infrastructure setup' }));

    expect(navigate).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
