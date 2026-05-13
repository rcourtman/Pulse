import { For, Show } from 'solid-js';

import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
} from '@/components/Settings/infrastructureWorkspaceModel';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';

import type { WorkloadsState } from './useWorkloadsState';

type WorkloadsStateCardsProps = Pick<
  WorkloadsState,
  | 'allGuests'
  | 'connected'
  | 'workloadsDisconnectedState'
  | 'workloadsGuestsEmptyState'
  | 'workloadsInfrastructureEmptyState'
  | 'workloadsLoadingState'
  | 'workloadsNoInventoryState'
  | 'filteredGuests'
  | 'hasInfrastructureSources'
  | 'infrastructureSourceStateReady'
  | 'initialDataReceived'
  | 'kioskMode'
  | 'navigate'
  | 'reconnect'
  | 'workloadInventoryIssues'
  | 'workloads'
>;

export function WorkloadsStateCards(props: WorkloadsStateCardsProps) {
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
            title={props.workloadsLoadingState().title}
            description={props.workloadsLoadingState().description}
          />
        </Card>
      </Show>

      <Show
        when={
          props.connected() &&
          props.initialDataReceived() &&
          !props.workloads.loading() &&
          props.workloadInventoryIssues().length > 0
        }
      >
        <div
          role="alert"
          class="mb-3 rounded-md border border-amber-400/40 bg-amber-50 px-4 py-3 text-sm text-amber-950 shadow-sm dark:border-amber-500/30 dark:bg-amber-950/30 dark:text-amber-100"
        >
          <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div class="min-w-0 space-y-2">
              <div>
                <p class="text-sm font-semibold">Workload inventory is incomplete</p>
                <p class="mt-1 text-xs text-amber-900/80 dark:text-amber-100/75">
                  One or more configured workload sources cannot currently report inventory.
                </p>
              </div>
              <ul class="space-y-1.5">
                <For each={props.workloadInventoryIssues().slice(0, 3)}>
                  {(issue) => (
                    <li class="min-w-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="font-medium text-amber-950 dark:text-amber-50">
                          {issue.name}
                        </span>
                        <span class="rounded border border-amber-400/40 px-1.5 py-0.5 text-[11px] font-medium uppercase text-amber-900 dark:border-amber-400/30 dark:text-amber-100">
                          {issue.stateLabel}
                        </span>
                      </div>
                      <p class="mt-0.5 text-xs text-amber-900/85 dark:text-amber-100/80">
                        {issue.description}
                      </p>
                      <Show when={issue.detail}>
                        {(detail) => (
                          <p class="mt-0.5 text-xs text-amber-900/70 dark:text-amber-100/65">
                            {detail()}
                          </p>
                        )}
                      </Show>
                    </li>
                  )}
                </For>
              </ul>
            </div>
            <Show when={!props.kioskMode()}>
              <button
                type="button"
                onClick={() => props.navigate(buildInfrastructureWorkspacePath())}
                class="inline-flex shrink-0 items-center justify-center rounded-md border border-amber-500/40 bg-amber-100 px-3 py-1.5 text-xs font-medium text-amber-950 transition-colors hover:bg-amber-200 focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 dark:border-amber-400/30 dark:bg-amber-900/50 dark:text-amber-50 dark:hover:bg-amber-900"
              >
                Review sources
              </button>
            </Show>
          </div>
        </div>
      </Show>

      <Show
        when={
          props.connected() &&
          props.initialDataReceived() &&
          !props.workloads.loading() &&
          props.infrastructureSourceStateReady() &&
          !props.hasInfrastructureSources() &&
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
            title={props.workloadsInfrastructureEmptyState().title}
            description={props.workloadsInfrastructureEmptyState().description}
            actions={
              !props.kioskMode() ? (
                <button
                  type="button"
                  onClick={() => props.navigate(buildInfrastructureOnboardingPath('pick'))}
                  class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                >
                  {props.workloadsInfrastructureEmptyState().actionLabel}
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
          !props.workloads.loading() &&
          props.infrastructureSourceStateReady() &&
          props.hasInfrastructureSources() &&
          props.allGuests().length === 0
        }
      >
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-amber-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v4m0 4h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"
                />
              </svg>
            }
            title={props.workloadsNoInventoryState().title}
            description={props.workloadsNoInventoryState().description}
            actions={
              !props.kioskMode() ? (
                <button
                  type="button"
                  onClick={() => props.navigate(buildInfrastructureWorkspacePath())}
                  class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded-md text-white bg-amber-600 hover:bg-amber-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-amber-500"
                >
                  {props.workloadsNoInventoryState().actionLabel}
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
            title={props.workloadsDisconnectedState().title}
            description={props.workloadsDisconnectedState().description}
            tone="danger"
            actions={
              props.workloadsDisconnectedState().actionLabel ? (
                <button
                  onClick={() => props.reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  {props.workloadsDisconnectedState().actionLabel}
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
            title={props.workloadsGuestsEmptyState().title}
            description={props.workloadsGuestsEmptyState().description}
          />
        </Card>
      </Show>
    </>
  );
}
