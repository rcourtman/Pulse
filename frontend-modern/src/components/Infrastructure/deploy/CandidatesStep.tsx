import { Component, For, Show, createMemo } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import {
  getDeployCandidatesLoadingState,
  getDeployNoCandidatesState,
  getDeployNoSourceAgentsState,
} from '@/utils/deployFlowPresentation';
import CheckIcon from 'lucide-solid/icons/check';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';

interface CandidatesStepProps {
  wizard: DeployWizardState;
}

export const CandidatesStep: Component<CandidatesStepProps> = (props) => {
  const w = props.wizard;
  const allSelected = createMemo(
    () => w.deployableNodes().length > 0 && w.selectedNodeIds().size === w.deployableNodes().length,
  );

  return (
    <div class="space-y-4">
      <Show when={w.candidatesError()}>
        <div
          role="alert"
          class="rounded-md bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-700 dark:text-red-300"
        >
          {w.candidatesError()}
        </div>
      </Show>

      <Show when={w.candidatesLoading()}>
        <div
          role="status"
          aria-live="polite"
          class="flex items-center justify-center py-8 text-sm text-muted"
        >
          <div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent mr-2" />
          {getDeployCandidatesLoadingState()}
        </div>
      </Show>

      <Show when={!w.candidatesLoading() && !w.candidatesError()}>
        {/* Source agent selection */}
        <Show when={w.onlineSourceAgents().length > 1}>
          <div class="space-y-1">
            <label class="text-xs font-medium text-muted" for="source-agent-select">
              Source Agent
            </label>
            <select
              id="source-agent-select"
              value={w.selectedSourceAgent()}
              onChange={(e) => w.setSelectedSourceAgent(e.currentTarget.value)}
              class="w-full rounded-md border border-border bg-surface px-3 py-1.5 text-sm text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">Select a source agent...</option>
              <For each={w.onlineSourceAgents()}>
                {(agent) => (
                  <option value={agent.agentId}>
                    {agent.nodeId} ({agent.agentId})
                  </option>
                )}
              </For>
            </select>
            <p class="text-[11px] text-muted">
              The source agent will SSH to other nodes to install Pulse agents.
            </p>
          </div>
        </Show>

        <Show when={w.onlineSourceAgents().length === 0 && !w.candidatesLoading()}>
          <div
            role="alert"
            class="rounded-md bg-amber-50 dark:bg-amber-900/20 p-3 text-sm text-amber-700 dark:text-amber-300 flex items-start gap-2"
          >
            <AlertCircleIcon class="w-4 h-4 mt-0.5 shrink-0" />
            <span>{getDeployNoSourceAgentsState()}</span>
          </div>
        </Show>

        {/* Node selection table */}
        <Show when={w.candidates().length > 0}>
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium text-muted">
                {w.selectedNodeIds().size} of {w.deployableNodes().length} nodes selected
              </span>
              <div class="flex gap-2">
                <button
                  type="button"
                  onClick={() => w.selectAllNodes()}
                  disabled={allSelected()}
                  class="text-xs text-blue-600 dark:text-blue-400 hover:underline disabled:opacity-50 disabled:no-underline"
                >
                  Select All
                </button>
                <button
                  type="button"
                  onClick={() => w.deselectAllNodes()}
                  disabled={w.selectedNodeIds().size === 0}
                  class="text-xs text-blue-600 dark:text-blue-400 hover:underline disabled:opacity-50 disabled:no-underline"
                >
                  Deselect All
                </button>
              </div>
            </div>

            <div class="rounded-md border border-border overflow-hidden">
              <table class="w-full text-sm">
                <thead>
                  <tr class="bg-surface-alt text-left">
                    <th class="w-8 px-3 py-2" />
                    <th class="px-3 py-2 font-medium text-muted text-xs">Node</th>
                    <th class="px-3 py-2 font-medium text-muted text-xs">IP</th>
                    <th class="px-3 py-2 font-medium text-muted text-xs">Status</th>
                  </tr>
                </thead>
                <tbody>
                  <For each={w.candidates()}>
                    {(node) => {
                      const isDisabled = () => node.hasAgent || !node.deployable;
                      const isChecked = () => w.selectedNodeIds().has(node.nodeId);

                      return (
                        <tr
                          class={`border-t border-border transition-colors ${
                            isDisabled() ? 'opacity-50' : 'hover:bg-surface-hover cursor-pointer'
                          }`}
                          tabIndex={isDisabled() ? undefined : 0}
                          onClick={() => {
                            if (!isDisabled()) w.toggleNodeSelection(node.nodeId);
                          }}
                          onKeyDown={(e) => {
                            if (!isDisabled() && (e.key === 'Enter' || e.key === ' ')) {
                              e.preventDefault();
                              w.toggleNodeSelection(node.nodeId);
                            }
                          }}
                        >
                          <td class="px-3 py-2">
                            <input
                              type="checkbox"
                              checked={isChecked()}
                              disabled={isDisabled()}
                              onChange={() => w.toggleNodeSelection(node.nodeId)}
                              onClick={(e) => e.stopPropagation()}
                              class="rounded border-border"
                            />
                          </td>
                          <td class="px-3 py-2 font-medium text-base-content">{node.name}</td>
                          <td class="px-3 py-2 text-muted font-mono text-xs">{node.ip || '--'}</td>
                          <td class="px-3 py-2">
                            <Show
                              when={node.hasAgent}
                              fallback={
                                <Show
                                  when={node.deployable}
                                  fallback={
                                    <span class="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                                      <AlertCircleIcon class="w-3 h-3" />
                                      {node.reason || 'Not deployable'}
                                    </span>
                                  }
                                >
                                  <span class="text-xs text-muted">Available</span>
                                </Show>
                              }
                            >
                              <span class="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                                <CheckIcon class="w-3 h-3" />
                                Already monitored
                              </span>
                            </Show>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </div>
        </Show>

        <Show when={w.candidates().length === 0 && !w.candidatesLoading()}>
          <div class="text-center py-8 text-sm text-muted">{getDeployNoCandidatesState()}</div>
        </Show>
      </Show>
    </div>
  );
};
