import { Component, createSignal, createEffect, createMemo, onCleanup, Show, For } from 'solid-js';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';
import { apiFetchJSON } from '@/utils/apiClient';
import { getPulseBaseUrl } from '@/utils/url';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
} from '@/components/Settings/infrastructureWorkspaceModel';
import type { WizardState } from '../SetupWizard';
import {
  buildSetupCompletionConnectedSystems,
  buildSetupCompletionViewModel,
} from './setupCompletionModel';

interface CompleteStepProps {
  state: WizardState;
  onComplete: (nextPath?: string) => void;
  connectedResourcesOverride?: readonly Resource[];
}

const SOURCE_STRATEGY_OPTIONS = [
  {
    title: 'Platform API',
    description: 'Inventory and health from Proxmox, TrueNAS, VMware, PBS, or PMG.',
  },
  {
    title: 'Pulse Agent',
    description: 'Node-local telemetry for standalone hosts, services, Docker, and Kubernetes.',
  },
  {
    title: 'Use both',
    description: 'Combine platform inventory with Agent telemetry when full coverage matters.',
  },
] as const;

const ADD_INFRASTRUCTURE_PATH = buildInfrastructureOnboardingPath('pick');
const AGENT_INSTALL_PATH = buildInfrastructureOnboardingPath('agent');
const INFRASTRUCTURE_WORKSPACE_PATH = buildInfrastructureWorkspacePath();

export const SetupCompletionPanel: Component<CompleteStepProps> = (props) => {
  const [copied, setCopied] = createSignal<'password' | 'admin-token' | null>(null);
  const [showCredentials, setShowCredentials] = createSignal(true);
  const [connectedSystems, setConnectedSystems] = createSignal<
    ReturnType<typeof buildSetupCompletionConnectedSystems>
  >([]);

  createEffect(() => {
    if (props.connectedResourcesOverride !== undefined) {
      // Preview scenarios provide a static resource snapshot so browser proof
      // stays deterministic without depending on live runtime state.
      setConnectedSystems(buildSetupCompletionConnectedSystems(props.connectedResourcesOverride));
      return;
    }

    let pollInterval: number | undefined;
    let previousCount = 0;

    const checkForConnectedSystems = async () => {
      try {
        const state = await apiFetchJSON<State>('/api/state', {
          headers: {
            'X-API-Token': props.state.apiToken,
          },
        });
        const nextConnectedSystems = buildSetupCompletionConnectedSystems(state.resources || []);
        const totalConnectedSystems = nextConnectedSystems.length;
        const previousSystems = connectedSystems();
        const hasConnectionChanges =
          totalConnectedSystems !== previousSystems.length ||
          nextConnectedSystems.some((system, index) => {
            const previousSystem = previousSystems[index];
            return (
              !previousSystem ||
              previousSystem.id !== system.id ||
              previousSystem.name !== system.name ||
              previousSystem.typeLabel !== system.typeLabel ||
              previousSystem.host !== system.host ||
              previousSystem.connectionPath !== system.connectionPath
            );
          });

        if (hasConnectionChanges) {
          setConnectedSystems(nextConnectedSystems);
        }

        if (hasConnectionChanges || totalConnectedSystems !== previousCount) {
          previousCount = totalConnectedSystems;
        }
      } catch (error) {
        logger.error('Failed to check for connected systems:', error);
      }
    };

    pollInterval = window.setInterval(checkForConnectedSystems, 3000);
    void checkForConnectedSystems();

    onCleanup(() => {
      if (pollInterval) {
        window.clearInterval(pollInterval);
      }
    });
  });

  const handleCopy = async (type: 'password' | 'admin-token', value: string) => {
    const success = await copyToClipboard(value);
    if (success) {
      setCopied(type);
      setTimeout(() => setCopied(null), 2000);
    }
  };

  const downloadCredentials = () => {
    const baseUrl = getPulseBaseUrl();
    const infrastructureUrl = `${baseUrl.replace(/\/$/, '')}${ADD_INFRASTRUCTURE_PATH}`;
    const content = `Pulse Credentials
==================
Generated: ${new Date().toISOString()}

Web Login:
----------
URL: ${baseUrl}
Username: ${props.state.username}
Password: ${props.state.password}

Admin API Token:
----------------
${props.state.apiToken}

Infrastructure:
---------------
${infrastructureUrl}

Use Add infrastructure to choose a platform API, Pulse Agent, or both
for the first system Pulse should monitor.

Keep these credentials secure!
`;

    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `pulse-credentials-${Date.now()}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const handleOpenAddInfrastructure = () => {
    props.onComplete(ADD_INFRASTRUCTURE_PATH);
  };

  const handleOpenAgentInstall = () => {
    props.onComplete(AGENT_INSTALL_PATH);
  };

  const handleOpenInfrastructure = () => {
    props.onComplete(INFRASTRUCTURE_WORKSPACE_PATH);
  };

  const completionViewModel = createMemo(() => buildSetupCompletionViewModel(connectedSystems()));

  return (
    <div class="max-w-2xl mx-auto bg-surface border border-border overflow-hidden relative rounded-md p-6 sm:p-8 text-center text-base-content">
      <div class="relative z-10">
        <div class="mb-8">
          <div class="inline-flex items-center justify-center w-16 h-16 rounded-full bg-emerald-100 dark:bg-emerald-900 text-emerald-600 dark:text-emerald-400 mb-6 border border-emerald-200 dark:border-emerald-800">
            <svg
              class="w-8 h-8"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <h1 class="text-2xl sm:text-3xl font-bold tracking-tight text-base-content mb-2">
            {completionViewModel().heroTitle}
          </h1>
          <p class="text-slate-500 dark:text-emerald-300 font-light text-sm sm:text-base">
            {completionViewModel().heroDescription}
          </p>
        </div>

        <Show when={completionViewModel().hasConnectedSystems}>
          <div class="bg-emerald-50 dark:bg-emerald-900 rounded-md border border-emerald-200 dark:border-emerald-800 p-5 text-left mb-6">
            <h3 class="text-sm font-semibold text-emerald-800 dark:text-emerald-400 mb-3 flex items-center gap-2">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 13l4 4L19 7"
                />
              </svg>
              {completionViewModel().connectedSummaryLabel}
            </h3>
            <div class="space-y-2">
              <For each={connectedSystems()}>
                {(system) => (
                  <div class="flex items-center justify-between bg-surface rounded-md px-3 py-2.5 border border-border-subtle">
                    <div class="flex items-center gap-2.5">
                      <span class="w-2.5 h-2.5 bg-emerald-500 rounded-full"></span>
                      <span class="text-base-content text-sm font-medium">{system.name}</span>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-[10px] text-emerald-700 dark:text-emerald-300 bg-emerald-100 dark:bg-emerald-900 border border-emerald-200 dark:border-emerald-800 px-2 py-0.5 rounded-full font-medium">
                        {system.typeLabel}
                      </span>
                      <Show when={system.host}>
                        <span class="text-[10px] text-muted font-mono">{system.host}</span>
                      </Show>
                    </div>
                  </div>
                )}
              </For>
            </div>
          </div>
        </Show>

        <div class="bg-surface rounded-md border border-border mb-6 overflow-hidden">
          <button
            onClick={() => setShowCredentials(!showCredentials())}
            class="w-full p-4 sm:p-6 flex items-center justify-between gap-4 text-left hover:bg-surface-hover transition-colors group"
          >
            <div class="flex items-start gap-3">
              <div class="w-8 h-8 rounded-md bg-amber-50 dark:bg-amber-900 flex items-center justify-center border border-amber-100 dark:border-amber-800 shrink-0">
                <svg
                  class="w-4 h-4 text-amber-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                  />
                </svg>
              </div>
              <div>
                <span class="text-base-content font-semibold text-sm flex items-center gap-2 flex-wrap">
                  Credentials you must save now
                  <span class="text-[10px] text-amber-700 dark:text-amber-400 bg-amber-100 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 px-2 py-0.5 rounded-full">
                    Shown during setup
                  </span>
                </span>
                <p class="mt-1 text-xs text-muted max-w-xl">
                  Save the admin login and API token before leaving this screen, then continue into{' '}
                  {completionViewModel().credentialsContinuationText}
                </p>
              </div>
            </div>
            <svg
              class={`w-5 h-5 text-muted transition-transform duration-200 group-hover:text-base-content ${showCredentials() ? 'rotate-180' : ''}`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M19 9l-7 7-7-7"
              />
            </svg>
          </button>

          <Show when={showCredentials()}>
            <div class="px-4 pb-4 pt-0 sm:px-6 sm:pb-6 space-y-3 border-t border-border-subtle">
              <div class="bg-surface-hover dark:bg-black border rounded-md p-3 mt-4">
                <div class="text-[11px] font-medium text-muted mb-1 uppercase tracking-wider">
                  Username
                </div>
                <div class="text-base-content font-mono text-sm">{props.state.username}</div>
              </div>

              <div class="bg-surface-hover dark:bg-black border rounded-md p-3">
                <div class="text-[11px] font-medium text-muted mb-1 uppercase tracking-wider">
                  Password
                </div>
                <div class="flex items-center justify-between">
                  <code class="text-base-content font-mono text-sm break-all">
                    {props.state.password}
                  </code>
                  <button
                    onClick={() => handleCopy('password', props.state.password)}
                    class="ml-3 p-1.5 bg-surface border border-border hover:bg-surface-hover rounded-md transition-colors shrink-0"
                  >
                    {copied() === 'password' ? (
                      <svg
                        class="w-4 h-4 text-emerald-500"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                    ) : (
                      <svg
                        class="w-4 h-4 text-muted"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                        />
                      </svg>
                    )}
                  </button>
                </div>
              </div>

              <div class="bg-surface-hover dark:bg-black border border-border rounded-md p-3">
                <div class="text-[11px] font-medium text-muted mb-1 uppercase tracking-wider">
                  Admin API Token
                </div>
                <div class="flex items-center justify-between">
                  <code class="text-base-content font-mono text-xs break-all pr-4">
                    {props.state.apiToken}
                  </code>
                  <button
                    onClick={() => handleCopy('admin-token', props.state.apiToken)}
                    class="ml-2 p-1.5 bg-surface border border-border hover:bg-surface-hover rounded-md transition-colors shrink-0"
                  >
                    {copied() === 'admin-token' ? (
                      <svg
                        class="w-4 h-4 text-emerald-500"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                    ) : (
                      <svg
                        class="w-4 h-4 text-muted"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                        />
                      </svg>
                    )}
                  </button>
                </div>
              </div>

              <button
                onClick={downloadCredentials}
                class="w-full mt-2 py-2.5 text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 flex items-center justify-center gap-1.5 bg-blue-50 dark:bg-blue-900 hover:bg-blue-100 dark:hover:bg-blue-900 rounded-md transition-colors border border-blue-100 dark:border-blue-900"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                Download credentials
              </button>
            </div>
          </Show>
        </div>

        <div
          aria-label="Setup next step"
          class="bg-surface rounded-md border border-border p-5 sm:p-6 text-left mb-6"
        >
          <div class="flex items-start justify-between gap-4">
            <div>
              <h3 class="text-sm font-semibold text-base-content flex items-center gap-2">
                <svg
                  class="w-4 h-4 text-blue-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
                  />
                </svg>
                {completionViewModel().nextStepTitle}
              </h3>
            </div>
            <div class="rounded-sm bg-blue-50 px-2 py-1 text-[10px] font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-300">
              Recommended next step
            </div>
          </div>
          <div class="mt-4 rounded-md border border-border bg-surface-alt p-4">
            <div class="text-[11px] font-medium uppercase tracking-wider text-muted">Next step</div>
            <div class="mt-2 text-sm text-base-content">
              {completionViewModel().nextStepSummary}
            </div>
            <div class="mt-2 text-xs text-muted">{completionViewModel().nextStepDetail}</div>
            <div class="mt-4 flex flex-col gap-3 sm:flex-row">
              <button
                onClick={() =>
                  completionViewModel().primaryAction === 'infrastructure'
                    ? handleOpenInfrastructure()
                    : handleOpenAddInfrastructure()
                }
                class="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
              >
                {completionViewModel().primaryAction === 'infrastructure'
                  ? 'Open Infrastructure'
                  : 'Add infrastructure'}
              </button>
              <Show when={completionViewModel().showAddInfrastructureAction}>
                <button
                  onClick={handleOpenAddInfrastructure}
                  class="inline-flex items-center justify-center gap-2 rounded-md border border-border px-4 py-3 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                >
                  Add infrastructure
                </button>
              </Show>
              <Show when={completionViewModel().showAgentInstallAction}>
                <button
                  onClick={handleOpenAgentInstall}
                  class="inline-flex items-center justify-center gap-2 rounded-md border border-border px-4 py-3 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                >
                  Install Pulse Agent
                </button>
              </Show>
            </div>
            <div class="mt-4 border-t border-border-subtle pt-3">
              <div class="text-[11px] font-medium uppercase tracking-wider text-muted">
                Source choices
              </div>
              <ul class="mt-2 space-y-1.5 text-left">
                <For each={SOURCE_STRATEGY_OPTIONS}>
                  {(option) => (
                    <li class="text-xs leading-snug">
                      <span class="font-semibold text-base-content">{option.title}</span>
                      <span class="text-muted">{' — '}</span>
                      <span class="text-muted">{option.description}</span>
                    </li>
                  )}
                </For>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
