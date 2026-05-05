import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { WorkloadsStateCards } from '../WorkloadsStateCards';

describe('WorkloadsStateCards', () => {
  it('routes the empty-state action directly to infrastructure setup', () => {
    const navigate = vi.fn();

    render(() => (
      <WorkloadsStateCards
        allGuests={() => []}
        connected={() => true}
        workloadsDisconnectedState={() => ({
          title: 'Connection lost',
          description: 'Unable to connect to the backend server',
          actionLabel: 'Reconnect now',
        })}
        workloadsGuestsEmptyState={() => ({
          title: 'No guests found',
          description: 'No guests match your current filters',
        })}
        workloadsInfrastructureEmptyState={() => ({
          title: 'No infrastructure sources connected',
          description:
            'Start in Settings → Infrastructure by choosing a source strategy. Connect a platform API for inventory and health, install Pulse Agent for host telemetry, or use both when you want full coverage.',
          actionLabel: 'Add infrastructure source',
        })}
        workloadsLoadingState={() => ({
          title: 'Loading workloads...',
          description: 'Connecting to monitoring service',
        })}
        workloadsNoInventoryState={() => ({
          title: 'No workload inventory available',
          description:
            'Pulse has infrastructure sources, but no VM, container, or pod inventory is available right now. Review source credentials, permissions, and collection status in Settings → Infrastructure.',
          actionLabel: 'Review infrastructure sources',
        })}
        filteredGuests={() => []}
        hasInfrastructureSources={() => false}
        infrastructureSourceStateReady={() => true}
        initialDataReceived={() => true}
        kioskMode={() => false}
        navigate={navigate}
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

    fireEvent.click(screen.getByRole('button', { name: 'Add infrastructure source' }));

    expect(navigate).toHaveBeenCalledWith('/settings/infrastructure?add=pick');
  });

  it('routes the no-inventory action to existing infrastructure sources', () => {
    const navigate = vi.fn();

    render(() => (
      <WorkloadsStateCards
        allGuests={() => []}
        connected={() => true}
        workloadsDisconnectedState={() => ({
          title: 'Connection lost',
          description: 'Unable to connect to the backend server',
          actionLabel: 'Reconnect now',
        })}
        workloadsGuestsEmptyState={() => ({
          title: 'No guests found',
          description: 'No guests match your current filters',
        })}
        workloadsInfrastructureEmptyState={() => ({
          title: 'No infrastructure sources connected',
          description:
            'Start in Settings → Infrastructure by choosing a source strategy. Connect a platform API for inventory and health, install Pulse Agent for host telemetry, or use both when you want full coverage.',
          actionLabel: 'Add infrastructure source',
        })}
        workloadsLoadingState={() => ({
          title: 'Loading workloads...',
          description: 'Connecting to monitoring service',
        })}
        workloadsNoInventoryState={() => ({
          title: 'No workload inventory available',
          description:
            'Pulse has infrastructure sources, but no VM, container, or pod inventory is available right now. Review source credentials, permissions, and collection status in Settings → Infrastructure.',
          actionLabel: 'Review infrastructure sources',
        })}
        filteredGuests={() => []}
        hasInfrastructureSources={() => true}
        infrastructureSourceStateReady={() => true}
        initialDataReceived={() => true}
        kioskMode={() => false}
        navigate={navigate}
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

    expect(screen.queryByText('No infrastructure sources connected')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Review infrastructure sources' }));

    expect(navigate).toHaveBeenCalledWith('/settings/infrastructure');
  });
});
