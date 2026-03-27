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
            'To start using Pulse, first add your infrastructure in Settings → Infrastructure → Install on a host. If you want to connect Proxmox directly instead, use Settings → Infrastructure → Direct Proxmox.',
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
          loading: () => false,
        }}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: 'Open infrastructure setup' }));

    expect(navigate).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
