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

    fireEvent.click(screen.getByRole('button', { name: 'Add infrastructure source' }));

    expect(navigate).toHaveBeenCalledWith('/settings/infrastructure?add=pick');
  });
});
