import { Component, For, Show } from 'solid-js';
import type {
  DockerContainer,
  DockerHost,
  DockerService,
  DockerTask,
} from '@/types/api';
import { Card } from '@/components/shared/Card';

export type DockerTreeSelection =
  | { type: 'host'; hostId: string; id: string }
  | { type: 'service'; hostId: string; id: string }
  | { type: 'task'; hostId: string; serviceKey: string; id: string }
  | { type: 'container'; hostId: string; id: string };

export interface DockerTreeTaskEntry {
  nodeId: string;
  task: DockerTask;
}

export interface DockerTreeServiceEntry {
  key: string;
  service: DockerService;
  tasks: DockerTreeTaskEntry[];
}

export interface DockerTreeContainerEntry {
  nodeId: string;
  container: DockerContainer;
}

export interface DockerTreeHostEntry {
  host: DockerHost;
  hostId: string;
  containers: DockerContainer[];
  services: DockerTreeServiceEntry[];
  standaloneContainers: DockerTreeContainerEntry[];
}

interface ExpandState {
  isExpanded: () => boolean;
  toggle: () => void;
  setExpanded: (value: boolean) => void;
}

interface DockerTreeProps {
  hosts: DockerTreeHostEntry[];
  selected?: DockerTreeSelection | null;
  onSelect?: (selection: DockerTreeSelection) => void;
  getHostState: (hostId: string) => ExpandState;
  getServiceState: (serviceKey: string) => ExpandState;
}

export const DockerTree: Component<DockerTreeProps> = (props) => {
  const isNodeSelected = (node: DockerTreeSelection) => {
    const current = props.selected;
    if (!current) return false;
    return (
      current.type === node.type &&
      current.hostId === node.hostId &&
      current.id === node.id
    );
  };

  const hostDisplayName = (host: DockerHost) =>
    host.displayName || host.hostname || host.id || 'Unknown host';

  const hostStatusVariant = (host: DockerHost) => {
    const status = host.status?.toLowerCase() ?? 'unknown';
    if (status === 'online') {
      return 'bg-green-500';
    }
    if (
      status === 'offline' ||
      status === 'error' ||
      status === 'down' ||
      status === 'unreachable'
    ) {
      return 'bg-red-500';
    }
    return 'bg-yellow-500';
  };

  const taskStatusVariant = (task: DockerTask) => {
    const state = task.currentState?.toLowerCase() ?? '';
    if (state === 'running') return 'bg-green-500';
    if (state === 'failed' || state === 'error') return 'bg-red-500';
    return 'bg-yellow-500';
  };

  const describeTaskLabel = (task: DockerTask) => {
    if (task.slot !== undefined && task.slot !== null) {
      return `${task.containerName || task.containerId || 'Task'}:${task.slot}`;
    }
    return task.containerName || task.containerId || task.id?.slice(0, 12) || 'Task';
  };

  return (
    <Card
      padding="none"
      class="bg-white dark:bg-slate-900/60 p-2"
    >
      <nav class="space-y-1.5 text-xs">
        <For each={props.hosts}>
          {(entry) => {
            const hostState = props.getHostState(entry.hostId);

            const selectHost = () => {
              hostState.setExpanded(true);
              props.onSelect?.({
                type: 'host',
                hostId: entry.hostId,
                id: entry.hostId,
              });
            };

            return (
              <div class="space-y-0.5">
                <div class="flex items-start gap-1.5">
                  {/*
                    Small expand/collapse control for the host section. We avoid re-rendering
                    the entire row by leaving the host summary text in a separate button.
                  */}
                  <button
                    type="button"
                    onClick={() => hostState.toggle()}
                    aria-label={
                      hostState.isExpanded() ? 'Collapse host section' : 'Expand host section'
                    }
                    class="mt-0 h-4 w-4 flex items-center justify-center rounded border border-transparent text-slate-500 hover:text-sky-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-sky-500"
                  >
                    <svg
                      class={`h-3 w-3 transition-transform duration-150 ${
                        hostState.isExpanded() ? 'rotate-90' : ''
                      }`}
                      viewBox="0 0 20 20"
                      fill="currentColor"
                    >
                      <path d="M6 4l6 6-6 6" />
                    </svg>
                  </button>

                  <button
                    type="button"
                    onClick={selectHost}
                    class={`flex w-full items-center gap-2 truncate rounded border border-transparent px-1.5 py-0.5 text-left text-[12px] font-medium transition-colors ${
                      isNodeSelected({ type: 'host', hostId: entry.hostId, id: entry.hostId })
                        ? 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200'
                        : 'hover:bg-slate-100 dark:hover:bg-slate-800/50'
                    }`}
                  >
                    <span
                      class={`h-2 w-2 flex-shrink-0 rounded-full ${hostStatusVariant(entry.host)}`}
                      aria-hidden="true"
                    />
                    <span class="truncate text-slate-700 dark:text-slate-100">{hostDisplayName(entry.host)}</span>
                  </button>
                </div>

                <Show when={hostState.isExpanded()}>
                  <div class="space-y-1 border-l border-slate-200 pl-2 dark:border-slate-700">
                    <Show when={entry.services.length > 0}>
                      <div class="space-y-0.5">
                        <div class="text-[11px] font-semibold uppercase tracking-tight text-slate-400 dark:text-slate-500">
                          Services
                        </div>
                        <div class="space-y-0.5">
                          <For each={entry.services}>
                            {(serviceEntry) => {
                              const serviceState = props.getServiceState(serviceEntry.key);

                                    const selectService = () => {
                                      hostState.setExpanded(true);
                                      props.onSelect?.({
                                        type: 'service',
                                        hostId: entry.hostId,
                                        id: serviceEntry.key,
                                      });
                                    };

                              const selectTask = (taskNodeId: string) => {
                                hostState.setExpanded(true);
                                serviceState.setExpanded(true);
                                props.onSelect?.({
                                  type: 'task',
                                  hostId: entry.hostId,
                                  serviceKey: serviceEntry.key,
                                  id: taskNodeId,
                                });
                              };

                              return (
                                <div class="space-y-0.5">
                                  <div class="flex items-start gap-1.5">
                                    <Show when={serviceEntry.tasks.length > 0}>
                                      <button
                                        type="button"
                                        aria-label={
                                          serviceState.isExpanded()
                                            ? 'Collapse task list'
                                            : 'Expand task list'
                                        }
                                        class="mt-0.5 h-3.5 w-3.5 flex items-center justify-center rounded border border-transparent text-slate-400 hover:text-sky-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-sky-500"
                                        onClick={(event) => {
                                          event.stopPropagation();
                                          serviceState.toggle();
                                        }}
                                      >
                                        <svg
                                          class={`h-3 w-3 transition-transform duration-150 ${
                                            serviceState.isExpanded() ? 'rotate-90' : ''
                                          }`}
                                          viewBox="0 0 20 20"
                                          fill="currentColor"
                                        >
                                          <path d="M6 4l6 6-6 6" />
                                        </svg>
                                      </button>
                                    </Show>
                                    <button
                                      type="button"
                                      onClick={selectService}
                                      class={`flex min-h-[24px] w-full items-center gap-1.5 rounded px-1.5 py-0.5 text-left text-[11px] transition-colors ${
                                        isNodeSelected({
                                          type: 'service',
                                          hostId: entry.hostId,
                                          id: serviceEntry.key,
                                        })
                                          ? 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200'
                                          : 'hover:bg-slate-100 dark:hover:bg-slate-800/50'
                                      }`}
                                    >
                                      <span class="truncate text-slate-600 dark:text-slate-200">
                                        {serviceEntry.service.name || serviceEntry.service.id}
                                      </span>
                                    </button>
                                  </div>

                                  <Show when={serviceEntry.tasks.length > 0 && serviceState.isExpanded()}>
                                    <div class="space-y-0.5 border-l border-slate-200 pl-2 dark:border-slate-700">
                                      <For each={serviceEntry.tasks}>
                                        {(taskEntry) => {
                                          const task = taskEntry.task;
                                          return (
                                            <button
                                              type="button"
                                              onClick={() => selectTask(taskEntry.nodeId)}
                                              class={`flex w-full items-center gap-1.5 rounded px-1.5 py-0.5 text-left text-[11px] transition-colors ${
                                                isNodeSelected({
                                                  type: 'task',
                                                  hostId: entry.hostId,
                                                  serviceKey: serviceEntry.key,
                                                  id: taskEntry.nodeId,
                                                })
                                                  ? 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200'
                                                  : 'hover:bg-slate-100 dark:hover:bg-slate-800/50'
                                              }`}
                                            >
                                              <span
                                                class={`h-1.5 w-1.5 flex-shrink-0 rounded-full ${taskStatusVariant(task)}`}
                                                aria-hidden="true"
                                              />
                                              <span class="truncate text-slate-500 dark:text-slate-300">
                                                {describeTaskLabel(task)}
                                              </span>
                                            </button>
                                          );
                                        }}
                                      </For>
                                    </div>
                                  </Show>
                                </div>
                              );
                            }}
                          </For>
                        </div>
                      </div>
                    </Show>

                    <Show when={entry.standaloneContainers.length > 0}>
                      <div class="space-y-0.5">
                        <div class="text-[11px] font-semibold uppercase tracking-tight text-slate-400 dark:text-slate-500">
                          Standalone Containers
                        </div>
                        <div class="space-y-0.5">
                          <For each={entry.standaloneContainers}>
                            {(containerEntry) => {
                              const container = containerEntry.container;
                              const selectContainer = () => {
                                hostState.setExpanded(true);
                                props.onSelect?.({
                                  type: 'container',
                                  hostId: entry.hostId,
                                  id: containerEntry.nodeId,
                                });
                              };

                              return (
                                <button
                                  type="button"
                                  onClick={selectContainer}
                                  class={`flex w-full items-center gap-1.5 rounded px-1.5 py-0.5 text-left text-[11px] transition-colors ${
                                    isNodeSelected({
                                      type: 'container',
                                      hostId: entry.hostId,
                                      id: containerEntry.nodeId,
                                    })
                                      ? 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200'
                                      : 'hover:bg-slate-100 dark:hover:bg-slate-800/50'
                                  }`}
                                >
                                  <span class="truncate text-slate-600 dark:text-slate-200">
                                    {container.name || container.id || 'Unknown container'}
                                  </span>
                                </button>
                              );
                            }}
                          </For>
                        </div>
                      </div>
                    </Show>
                  </div>
                </Show>
              </div>
            );
          }}
        </For>
      </nav>
    </Card>
  );
};
