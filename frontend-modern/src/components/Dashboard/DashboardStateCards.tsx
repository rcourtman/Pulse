import { Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';

import type { DashboardState } from './useDashboardState';

type DashboardStateCardsProps = Pick<
  DashboardState,
  | 'allGuests'
  | 'connected'
  | 'dashboardDisconnectedState'
  | 'dashboardGuestsEmptyState'
  | 'dashboardInfrastructureEmptyState'
  | 'dashboardLoadingState'
  | 'filteredGuests'
  | 'initialDataReceived'
  | 'kioskMode'
  | 'navigate'
  | 'reconnect'
  | 'workloads'
> & {
  nodeCount: number;
};

export function DashboardStateCards(props: DashboardStateCardsProps) {
  return (
    <>
      <Show when={props.connected() && !props.initialDataReceived()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="mx-auto h-12 w-12 animate-spin text-slate-400"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title={props.dashboardLoadingState().title}
            description={props.dashboardLoadingState().description}
          />
        </Card>
      </Show>

      <Show
        when={
          props.connected() &&
          props.initialDataReceived() &&
          !props.workloads.loading() &&
          props.nodeCount === 0 &&
          props.allGuests().length === 0
        }
      >
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-slate-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            }
            title={props.dashboardInfrastructureEmptyState().title}
            description={props.dashboardInfrastructureEmptyState().description}
            actions={
              !props.kioskMode() ? (
                <button
                  type="button"
                  onClick={() => props.navigate('/settings')}
                  class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                >
                  {props.dashboardInfrastructureEmptyState().actionLabel}
                </button>
              ) : undefined
            }
          />
        </Card>
      </Show>

      <Show when={!props.connected()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-red-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            }
            title={props.dashboardDisconnectedState().title}
            description={props.dashboardDisconnectedState().description}
            tone="danger"
            actions={
              props.dashboardDisconnectedState().actionLabel ? (
                <button
                  onClick={() => props.reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  {props.dashboardDisconnectedState().actionLabel}
                </button>
              ) : undefined
            }
          />
        </Card>
      </Show>

      <Show
        when={
          props.connected() &&
          props.initialDataReceived() &&
          props.filteredGuests().length === 0 &&
          props.allGuests().length > 0
        }
      >
        <Card padding="lg" class="mb-4">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-slate-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
            }
            title={props.dashboardGuestsEmptyState().title}
            description={props.dashboardGuestsEmptyState().description}
          />
        </Card>
      </Show>
    </>
  );
}
