import { Component, createSignal, createEffect, onCleanup, Show, For } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { unwrap } from 'solid-js/store';
import { copyToClipboard } from '@/utils/clipboard';
import { logger } from '@/utils/logger';
import { apiFetchJSON } from '@/utils/apiClient';
import { getPulseBaseUrl } from '@/utils/url';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';
import { showSuccess, showError } from '@/utils/toast';
import {
  getActionableAgentIdFromResource,
  hasAgentFacet as resourceHasAgentFacet,
} from '@/utils/agentResources';
import {
  trackAgentFirstConnected,
  trackPaywallViewed,
  trackUpgradeClicked,
} from '@/utils/upgradeMetrics';
import {
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  loadLicenseStatus,
  entitlements,
  getUpgradeActionUrlOrFallback,
  startProTrial,
} from '@/stores/license';
import {
  RELAY_ONBOARDING_SETUP_LABEL,
  RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL,
  RELAY_ONBOARDING_TRIAL_HINT,
  RELAY_ONBOARDING_TRIAL_STARTING_LABEL,
} from '@/utils/relayPresentation';
import type { WizardState } from '../SetupWizard';

interface CompleteStepProps {
  state: WizardState;
  onComplete: (nextPath?: string) => void;
}

const UNIFIED_RESOURCE_GUIDANCE = {
  title: 'Unified Resource Inventory',
  description:
    'Pulse v6 starts with the Unified Agent. Install it on a system, let Pulse create one monitored system in inventory, then enrich that same inventory with workloads and linked platforms.',
  steps: [
    {
      title: 'Secure Pulse',
      description:
        'Finish first-run setup so your admin account and API access are ready for real monitoring.',
    },
    {
      title: 'Open Infrastructure Install',
      description:
        'Use the canonical install workspace to generate the right Unified Agent commands for Linux, macOS, Windows, and related platforms.',
    },
    {
      title: 'Bring Systems Into Pulse',
      description:
        'Each install creates one monitored system in Pulse, then Docker, Kubernetes, Proxmox, and other context can attach to that same system.',
    },
  ],
  inventoryFacts: [
    'One install becomes one monitored system in Pulse.',
    'Infrastructure Operations owns token generation, connection URL, TLS/CA, and platform-specific install commands.',
    'Settings no longer splits install across a separate setup-only command surface.',
  ],
} as const;

interface ConnectedAgent {
  id: string;
  name: string;
  type: string;
  host: string;
  addedAt: Date;
}

const RELAY_SETTINGS_PATH = '/settings/system-relay';
const INFRASTRUCTURE_INSTALL_PATH = '/settings/infrastructure/install';
const SETUP_WIZARD_TELEMETRY_SURFACE = 'setup_wizard_complete';

const pd = (resource: Resource) =>
  resource.platformData ? (unwrap(resource.platformData) as Record<string, unknown>) : undefined;
const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
const hasAgentFacet = (resource: Resource): boolean => resourceHasAgentFacet(resource);

const toNodeSummaryShape = (resource: Resource) => {
  const platformData = pd(resource);
  const proxmox = asRecord(platformData?.proxmox);
  const name = getPreferredResourceDisplayName(resource);
  return {
    id: resource.id,
    name,
    displayName: name,
    host: asString(proxmox?.instance) || '',
  };
};

const toAgentSummaryShape = (resource: Resource) => {
  const hostname = getPreferredResourceHostname(resource) || resource.id;
  const name = getPreferredResourceDisplayName(resource);
  const id = getActionableAgentIdFromResource(resource) || resource.id;
  return {
    id,
    hostname,
    displayName: name,
  };
};

export const SetupCompletionPanel: Component<CompleteStepProps> = (props) => {
  const navigate = useNavigate();
  const [copied, setCopied] = createSignal<'password' | 'admin-token' | null>(null);
  const [showCredentials, setShowCredentials] = createSignal(false);
  const [connectedAgents, setConnectedAgents] = createSignal<ConnectedAgent[]>([]);
  const [trialStarting, setTrialStarting] = createSignal(false);
  const [trialStarted, setTrialStarted] = createSignal(false);
  const [relayPaywallTracked, setRelayPaywallTracked] = createSignal(false);
  let firstConnectionTracked = false;

  createEffect(() => {
    if (connectedAgents().length > 0 && !relayPaywallTracked()) {
      trackPaywallViewed('relay', 'setup_wizard');
      setRelayPaywallTracked(true);
    }
  });

  createEffect(() => {
    let pollInterval: number | undefined;
    let previousCount = 0;

    const checkForAgents = async () => {
      try {
        const state = await apiFetchJSON<State>('/api/state', {
          headers: {
            'X-API-Token': props.state.apiToken,
          },
        });
        const resources = state.resources || [];
        const nodeResources = resources.filter((resource) => resource.type === 'agent');
        const agentFacetResources = resources.filter(
          (resource) =>
            (resource.type === 'agent' ||
              resource.type === 'pbs' ||
              resource.type === 'pmg' ||
              resource.type === 'truenas') &&
            hasAgentFacet(resource),
        );

        const nodes = nodeResources.map(toNodeSummaryShape);
        const agents = agentFacetResources.map(toAgentSummaryShape);
        const agentMap = new Map<string, ConnectedAgent>();

        for (const node of nodes) {
          const name = node.displayName || node.name || 'Unknown';
          const existing = agentMap.get(name);
          if (existing) {
            if (!existing.type.includes('Proxmox')) {
              existing.type = `${existing.type} + Proxmox VE`;
            }
            if (node.host && !existing.host) {
              existing.host = node.host;
            }
          } else {
            agentMap.set(name, {
              id: node.id || `node-${name}`,
              name,
              type: 'Proxmox VE',
              host: node.host || '',
              addedAt: new Date(),
            });
          }
        }

        for (const agent of agents) {
          const name = agent.displayName || agent.hostname || 'Unknown';
          const existing = agentMap.get(name);
          if (existing) {
            if (!existing.type.includes('Agent')) {
              existing.type = `${existing.type} + Agent`;
            }
          } else {
            agentMap.set(name, {
              id: agent.id || `agent-${name}`,
              name,
              type: 'Agent',
              host: '',
              addedAt: new Date(),
            });
          }
        }

        const nextConnectedAgents = Array.from(agentMap.values());
        const totalAgents = nextConnectedAgents.length;
        const previousAgents = connectedAgents();
        const hasConnectionChanges =
          totalAgents !== previousAgents.length ||
          nextConnectedAgents.some((agent, index) => {
            const previousAgent = previousAgents[index];
            return (
              !previousAgent ||
              previousAgent.id !== agent.id ||
              previousAgent.name !== agent.name ||
              previousAgent.type !== agent.type ||
              previousAgent.host !== agent.host
            );
          });

        if (!firstConnectionTracked && totalAgents > 0) {
          trackAgentFirstConnected(SETUP_WIZARD_TELEMETRY_SURFACE, 'first_agent');
          firstConnectionTracked = true;
        }

        if (hasConnectionChanges) {
          setConnectedAgents(nextConnectedAgents);
        }

        if (hasConnectionChanges || totalAgents !== previousCount) {
          previousCount = totalAgents;
        }
      } catch (error) {
        logger.error('Failed to check for agents:', error);
      }
    };

    pollInterval = window.setInterval(checkForAgents, 3000);
    void checkForAgents();

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
    const installWorkspaceUrl = `${baseUrl.replace(/\/$/, '')}${INFRASTRUCTURE_INSTALL_PATH}`;
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

Infrastructure Install Workspace:
---------------------------------
${installWorkspaceUrl}

Use the Infrastructure Install workspace to:
- generate Unified Agent tokens
- choose the agent connection URL
- configure TLS and custom CA options
- copy Linux, macOS, Windows, and related install commands

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

  const handleOpenInstallWorkspace = () => {
    props.onComplete(INFRASTRUCTURE_INSTALL_PATH);
  };

  const handleGoToDashboard = () => {
    props.onComplete('/');
  };

  const handleStartTrial = async () => {
    trackUpgradeClicked('setup_wizard', 'relay');
    if (trialStarting()) return;

    setTrialStarting(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        if (typeof window !== 'undefined') {
          window.location.href = result.actionUrl;
        }
        return;
      }

      showSuccess('14-day Pro trial started! Set up Relay to monitor from your phone.');
      setTrialStarted(true);
      await loadLicenseStatus(true);
    } catch (err) {
      logger.warn('[SetupCompletionPanel] Failed to start trial; falling back to upgrade URL', err);
      showError('Unable to start trial. Redirecting to upgrade options...');
      const upgradeUrl = getUpgradeActionUrlOrFallback('relay');
      if (typeof window !== 'undefined') {
        window.location.href = upgradeUrl;
      }
    } finally {
      setTrialStarting(false);
    }
  };

  const handleSetupRelay = () => {
    props.onComplete(RELAY_SETTINGS_PATH);
    navigate(RELAY_SETTINGS_PATH);
  };

  return (
    <div class="max-w-2xl mx-auto bg-surface border border-border overflow-hidden animate-fade-in relative rounded-md p-6 sm:p-8 text-center text-base-content">
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
            Security Configured
          </h1>
          <p class="text-slate-500 dark:text-emerald-300 font-light text-sm sm:text-base">
            Open Infrastructure Install to bring your first monitored system into Pulse.
          </p>
        </div>

        <Show when={connectedAgents().length > 0}>
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
              Connected ({connectedAgents().length} agent{connectedAgents().length !== 1 ? 's' : ''}
              )
            </h3>
            <div class="space-y-2">
              <For each={connectedAgents()}>
                {(agent) => (
                  <div class="flex items-center justify-between bg-surface rounded-md px-3 py-2.5 border border-border-subtle">
                    <div class="flex items-center gap-2.5">
                      <span class="w-2.5 h-2.5 bg-emerald-500 rounded-full"></span>
                      <span class="text-base-content text-sm font-medium">{agent.name}</span>
                    </div>
                    <div class="flex items-center gap-2">
                      <span class="text-[10px] text-emerald-700 dark:text-emerald-300 bg-emerald-100 dark:bg-emerald-900 border border-emerald-200 dark:border-emerald-800 px-2 py-0.5 rounded-full font-medium">
                        {agent.type}
                      </span>
                      <Show when={agent.host}>
                        <span class="text-[10px] text-muted font-mono">{agent.host}</span>
                      </Show>
                    </div>
                  </div>
                )}
              </For>
            </div>
          </div>
        </Show>

        <div class="bg-surface rounded-md border border-border p-6 text-left mb-6">
          <h3 class="text-sm font-semibold text-base-content mb-3 flex items-center gap-2">
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
                d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
              />
            </svg>
            {UNIFIED_RESOURCE_GUIDANCE.title}
          </h3>

          <p class="text-muted text-xs mb-4">{UNIFIED_RESOURCE_GUIDANCE.description}</p>

          <div class="grid gap-3 sm:grid-cols-3 mb-4">
            <For each={UNIFIED_RESOURCE_GUIDANCE.steps}>
              {(step, index) => (
                <div class="rounded-md border border-border bg-surface-alt p-4">
                  <div class="mb-3 flex items-center gap-3">
                    <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-slate-900 text-[11px] font-semibold text-white dark:bg-slate-100 dark:text-slate-900">
                      {index() + 1}
                    </div>
                    <div class="text-sm font-semibold text-base-content">{step.title}</div>
                  </div>
                  <p class="text-xs leading-5 text-muted">{step.description}</p>
                </div>
              )}
            </For>
          </div>

          <div class="rounded-md border border-border bg-surface-alt p-4">
            <div class="mb-3 flex items-center justify-between gap-3">
              <div class="text-xs font-semibold uppercase tracking-[0.14em] text-muted">
                What Pulse Builds
              </div>
              <div class="rounded-sm bg-emerald-100 px-2 py-1 text-[10px] font-medium text-emerald-800 dark:bg-emerald-900 dark:text-emerald-300">
                Unified by default
              </div>
            </div>
            <div class="space-y-2">
              <For each={UNIFIED_RESOURCE_GUIDANCE.inventoryFacts}>
                {(fact) => (
                  <div class="flex items-start gap-3">
                    <div class="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500"></div>
                    <p class="text-xs leading-5 text-muted">{fact}</p>
                  </div>
                )}
              </For>
            </div>
          </div>
        </div>

        <div class="bg-surface rounded-md border border-border p-6 text-left mb-6">
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
                Install Unified Agent
              </h3>
              <p class="mt-2 text-xs text-muted max-w-xl">
                The canonical install flow now lives in Infrastructure Operations. Open that
                workspace to generate tokens, set the agent connection URL, configure TLS or custom
                CA options, and copy the correct install commands for Linux, macOS, Windows, and
                related platforms.
              </p>
            </div>
            <div class="rounded-sm bg-blue-50 px-2 py-1 text-[10px] font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-300">
              Single source of truth
            </div>
          </div>
          <div class="mt-4 rounded-md border border-border bg-surface-alt p-4">
            <div class="text-[11px] font-medium uppercase tracking-wider text-muted">
              Next step
            </div>
            <div class="mt-2 text-sm text-base-content">
              Open Infrastructure Install to bring your first monitored system into Pulse.
            </div>
            <div class="mt-2 text-xs text-muted">
              Use that workspace any time you want to add more systems later.
            </div>
          </div>
          <div class="mt-4 flex flex-col gap-3 sm:flex-row">
            <button
              onClick={handleOpenInstallWorkspace}
              class="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
            >
              Open Infrastructure Install
            </button>
            <Show when={connectedAgents().length > 0}>
              <button
                onClick={handleGoToDashboard}
                class="inline-flex items-center justify-center gap-2 rounded-md border border-border px-4 py-3 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
              >
                Go to Dashboard
              </button>
            </Show>
          </div>
        </div>

        <div class="bg-surface rounded-md border border-border mb-8 overflow-hidden">
          <button
            onClick={() => setShowCredentials(!showCredentials())}
            class="w-full p-4 flex items-center justify-between text-left hover:bg-surface-hover transition-colors group"
          >
            <div class="flex items-center gap-3">
              <div class="w-8 h-8 rounded-md bg-amber-50 dark:bg-amber-900 flex items-center justify-center border border-amber-100 dark:border-amber-800">
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
                <span class="text-base-content font-semibold text-sm flex items-center gap-2">
                  Your Credentials
                  <span class="text-[10px] text-amber-700 dark:text-amber-400 bg-amber-100 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 px-2 py-0.5 rounded-full">
                    Save these
                  </span>
                </span>
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
            <div class="p-4 pt-0 space-y-3 border-t border-border-subtle mt-2">
              <div class="bg-surface-hover dark:bg-black border rounded-md p-3">
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

              <div class="bg-surface-hover dark:bg-black border border-border rounded-md p-3">
                <div class="text-[11px] font-medium text-muted mb-1 uppercase tracking-wider">
                  Infrastructure Install Workspace
                </div>
                <code class="text-base-content font-mono text-xs break-all">
                  {INFRASTRUCTURE_INSTALL_PATH}
                </code>
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

        <div class="pt-4 border-t border-border">
          <button
            onClick={connectedAgents().length > 0 ? handleGoToDashboard : handleOpenInstallWorkspace}
            class="w-full py-4 px-6 bg-blue-600 hover:bg-blue-700 text-white text-base font-semibold rounded-md transition-all duration-200"
          >
            {connectedAgents().length > 0 ? 'Go to Dashboard' : 'Open Infrastructure Install'}
          </button>
          <p class="mt-4 text-xs text-muted">
            {connectedAgents().length > 0
              ? 'You can add more systems anytime from Infrastructure Operations.'
              : 'You can return here later from Infrastructure Operations if you skip install for now.'}
          </p>
        </div>

        <Show when={connectedAgents().length > 0}>
          <div class="bg-indigo-50 dark:bg-indigo-900 rounded-md border border-indigo-100 dark:border-indigo-800 p-5 text-left mt-8 overflow-hidden relative">
            <div class="flex items-start gap-4 relative z-10">
              <div class="flex h-12 w-12 items-center justify-center rounded-md bg-indigo-600 text-white shrink-0 border border-indigo-500">
                <svg
                  class="w-6 h-6"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z"
                  />
                </svg>
              </div>
              <div class="flex-1 min-w-0">
                <h3 class="text-sm font-bold text-base-content mb-1">Monitor from Anywhere</h3>
                <p class="text-xs text-slate-600 dark:text-indigo-200 mb-4 leading-relaxed">
                  Get push notifications and manage your infrastructure from your phone with Pulse
                  Relay.
                </p>
                <Show
                  when={!trialStarted() && entitlements()?.subscription_state !== 'trial'}
                  fallback={
                    <button
                      type="button"
                      onClick={handleSetupRelay}
                      class="inline-flex items-center gap-2 rounded-md bg-indigo-600 hover:bg-indigo-700 px-4 py-2 text-xs font-semibold text-white transition-colors"
                    >
                      <svg
                        class="w-4 h-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M13 7l5 5m0 0l-5 5m5-5H6"
                        />
                      </svg>
                      {RELAY_ONBOARDING_SETUP_LABEL}
                    </button>
                  }
                >
                  <button
                    type="button"
                    onClick={() => void handleStartTrial()}
                    disabled={trialStarting()}
                    class="inline-flex items-center gap-2 rounded-md bg-indigo-600 hover:bg-indigo-700 px-4 py-2 text-xs font-semibold text-white transition-colors disabled:opacity-50"
                  >
                    {trialStarting() ? (
                      <>
                        <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                          <circle
                            class="opacity-25"
                            cx="12"
                            cy="12"
                            r="10"
                            stroke="currentColor"
                            stroke-width="4"
                          ></circle>
                          <path
                            class="opacity-75"
                            fill="currentColor"
                            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                          ></path>
                        </svg>
                        {RELAY_ONBOARDING_TRIAL_STARTING_LABEL}
                      </>
                    ) : (
                      RELAY_ONBOARDING_SETUP_WIZARD_TRIAL_LABEL
                    )}
                  </button>
                </Show>
                <p class="mt-3 text-[10px] text-slate-500 dark:text-indigo-300 font-medium tracking-wide">
                  {RELAY_ONBOARDING_TRIAL_HINT}
                </p>
              </div>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
};
