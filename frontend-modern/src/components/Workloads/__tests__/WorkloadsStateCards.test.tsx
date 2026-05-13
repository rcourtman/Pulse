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
        workloadInventoryIssues={() => []}
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
        workloadInventoryIssues={() => []}
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

  it('surfaces partial workload inventory blockers even when other guests exist', () => {
    const navigate = vi.fn();

    render(() => (
      <WorkloadsStateCards
        allGuests={() => [{ id: 'guest-1' } as any]}
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
        filteredGuests={() => [{ id: 'guest-1' } as any]}
        hasInfrastructureSources={() => true}
        infrastructureSourceStateReady={() => true}
        initialDataReceived={() => true}
        kioskMode={() => false}
        navigate={navigate}
        reconnect={() => undefined}
        workloadInventoryIssues={() => [
          {
            id: 'pve:delly',
            name: 'delly',
            typeLabel: 'Proxmox VE',
            state: 'unauthorized',
            stateLabel: 'Credentials invalid',
            coverageLabel: 'VMs and containers',
            description:
              'Pulse has VMs and containers enabled for delly, but its Proxmox VE API credentials are invalid.',
            detail: 'Authentication failed. Re-check the API token or username/password.',
          },
        ]}
        workloads={{
          workloads: [{ id: 'guest-1' }] as any,
          loading: () => false,
          refetch: async () => [],
          mutate: (value) => (Array.isArray(value) ? value : value([])),
          error: () => null,
        }}
      />
    ));

    expect(screen.getByText('Workload inventory is incomplete')).toBeInTheDocument();
    expect(screen.getByText('delly')).toBeInTheDocument();
    expect(screen.getByText('Credentials invalid')).toBeInTheDocument();
    expect(screen.queryByText('No workload inventory available')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Review sources' }));

    expect(navigate).toHaveBeenCalledWith('/settings/infrastructure');
  });
});
