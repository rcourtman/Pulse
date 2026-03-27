import type { Component } from 'solid-js';
import { For, Show, createSignal } from 'solid-js';
import Server from 'lucide-solid/icons/server';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { copyToClipboard } from '@/utils/clipboard';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { notificationStore } from '@/stores/notifications';
import { getUnifiedAgentLookupStatusPresentation } from '@/utils/unifiedAgentStatusPresentation';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import { trackAgentInstallCommandCopied } from '@/utils/upgradeMetrics';
import {
  INSTALL_PROFILE_OPTIONS,
  normalizeTelemetryPart,
  type InstallProfile,
  UNIFIED_AGENT_TELEMETRY_SURFACE,
} from './infrastructureOperationsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

export const InfrastructureInstallerSection: Component = () => {
  const state = useInfrastructureOperationsContext();
  const [showAdvancedOptions, setShowAdvancedOptions] = createSignal(false);

  return (
    <SettingsPanel
      title={state.isEmbedded() ? 'Install on a host' : 'Infrastructure'}
      description={
        state.isEmbedded()
          ? 'Start here to add the first system you want Pulse to monitor, then expand into Docker, Kubernetes, Proxmox, and related infrastructure.'
          : 'Primary setup hub for installing Pulse on the first host you want to monitor, then expanding into Docker, Kubernetes, Proxmox, and related infrastructure.'
      }
      icon={<Server class="h-5 w-5" strokeWidth={2} />}
      bodyClass="space-y-5"
    >
      <Show when={state.setupHandoff()}>
        {(handoff) => (
          <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-950 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-50">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div class="space-y-2">
                <p class="font-semibold">Security configured. Save these first-run credentials now.</p>
                <p class="text-xs text-emerald-800 dark:text-emerald-200">
                  This is the canonical handoff from first-run setup into Infrastructure Install.
                  Generate a scoped install token below before copying agent commands.
                </p>
                <div class="grid gap-3 sm:grid-cols-3">
                  <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                    <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                      Username
                    </div>
                    <div class="mt-1 font-mono text-sm text-base-content">{handoff().username}</div>
                  </div>
                  <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                    <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                      Password
                    </div>
                    <div class="mt-1 break-all font-mono text-sm text-base-content">
                      {handoff().password}
                    </div>
                  </div>
                  <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                    <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                      Admin API Token
                    </div>
                    <div class="mt-1 break-all font-mono text-sm text-base-content">
                      {handoff().apiToken}
                    </div>
                  </div>
                </div>
              </div>
              <div class="flex flex-wrap gap-2 lg:w-64 lg:flex-col">
                <button
                  type="button"
                  onClick={() =>
                    void state.copySetupHandoffField(handoff().password, 'Copied first-run password.')
                  }
                  class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                >
                  Copy password
                </button>
                <button
                  type="button"
                  onClick={() =>
                    void state.copySetupHandoffField(
                      handoff().apiToken,
                      'Copied first-run admin API token.',
                    )
                  }
                  class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                >
                  Copy admin token
                </button>
                <button
                  type="button"
                  onClick={state.downloadSetupHandoff}
                  class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                >
                  Download credentials
                </button>
                <button
                  type="button"
                  onClick={state.clearSetupHandoff}
                  class="inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-medium text-emerald-900 hover:bg-emerald-100 dark:text-emerald-100 dark:hover:bg-emerald-800"
                >
                  Dismiss
                </button>
              </div>
            </div>
          </div>
        )}
      </Show>

      <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100">
        <p class="font-semibold">Start with the first host you want Pulse to monitor.</p>
        <p class="mt-1 text-xs text-blue-800 dark:text-blue-200">
          Install the Pulse agent on that system first. Once it connects, Pulse can keep using this
          workspace to add more hosts and layered platform integrations.
        </p>
      </div>

      <Show when={!state.isEmbedded()}>
        <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100">
          <div class="flex items-start gap-3">
            <ProxmoxIcon class="mt-0.5 h-5 w-5 shrink-0 text-amber-500" />
            <div class="flex-1">
              <p class="text-sm">
                Proxmox nodes can be added here with the unified agent for extra capabilities like
                temperature monitoring and Pulse Patrol automation (auto-creates the required token
                and links the node).
              </p>
              <button
                type="button"
                onClick={state.openDirectProxmoxSetup}
                class="mt-2 inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2 py-1.5 text-sm font-medium text-emerald-800 underline hover:bg-emerald-100 hover:text-emerald-900 dark:text-emerald-200 dark:hover:bg-emerald-900 dark:hover:text-emerald-100"
              >
                Need direct setup instead? Open Proxmox →
              </button>
            </div>
          </div>
        </div>
      </Show>

      <div class="space-y-5">
        <div class="space-y-3">
          <div class="space-y-1">
            <p class="text-sm font-semibold text-base-content">
              <span class="mr-1.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-blue-600 text-xs font-bold text-white">
                1
              </span>
              Generate install token
            </p>
            <p class="ml-6 text-sm text-muted">
              {state.requiresToken()
                ? 'Create a fresh token for the generated install commands and host reporting.'
                : 'Tokens are optional on this Pulse instance. Generate one if you want copied commands to preserve explicit credentialed transport.'}
            </p>
          </div>

          <div class="flex gap-2">
            <input
              type="text"
              value={state.tokenName()}
              onInput={(event) => state.setTokenName(event.currentTarget.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !state.isGeneratingToken()) {
                  void state.handleGenerateToken();
                }
              }}
              placeholder="Token name (optional)"
              class="flex-1 rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900"
            />
            <button
              type="button"
              onClick={() => void state.handleGenerateToken()}
              disabled={state.isGeneratingToken()}
              class="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {state.isGeneratingToken()
                ? 'Generating…'
                : state.hasToken()
                  ? 'Generate another'
                  : 'Generate token'}
            </button>
          </div>

          <Show when={state.latestRecord()}>
            <div class="flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-4 py-2 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-200">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
              </svg>
              <span>
                Install token <strong>{state.latestRecord()?.name}</strong> created. Commands below
                now include this credential.
              </span>
            </div>
          </Show>
        </div>

        <Show when={!state.requiresToken()}>
          <div class="space-y-3">
            <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
              Tokens are optional on this Pulse instance. Confirm to generate commands without
              embedding a token.
            </div>
            <button
              type="button"
              onClick={state.acknowledgeNoToken}
              disabled={state.confirmedNoToken()}
              class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                state.confirmedNoToken()
                  ? 'cursor-default bg-green-600 text-white'
                  : 'border border-border bg-surface text-base-content hover:bg-surface-hover'
              }`}
            >
              {state.confirmedNoToken() ? 'No token confirmed' : 'Confirm without token'}
            </button>
          </div>
        </Show>

        <Show when={state.requiresToken() && !state.commandsUnlocked()}>
          <div class="pointer-events-none select-none space-y-3 opacity-60">
            <div class="flex items-center justify-between">
              <div>
                <h4 class="text-sm font-semibold text-base-content">
                  <span class="mr-1.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-slate-400 text-xs font-bold text-white">
                    2
                  </span>
                  Installation commands
                </h4>
                <p class="ml-6 mt-0.5 text-xs text-muted">
                  Generate an install token above to unlock the commands for your first host.
                </p>
              </div>
            </div>
            <div class="rounded-md border border-border bg-surface-hover px-4 py-6 text-center">
              <svg
                class="mx-auto mb-2 h-8 w-8 text-muted"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.5"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"
                />
              </svg>
              <p class="text-sm text-muted">
                Click "Generate token" above to see the install commands for your first host.
              </p>
            </div>
          </div>
        </Show>

        <Show when={state.commandsUnlocked()}>
          <div class="space-y-3">
            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <div>
                  <h4 class="text-sm font-semibold text-base-content">
                    <Show when={state.requiresToken()}>
                      <span class="mr-1.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-green-600 text-xs font-bold text-white">
                        2
                      </span>
                    </Show>
                    Installation commands
                  </h4>
                  <p class={`mt-0.5 text-xs text-muted ${state.requiresToken() ? 'ml-6' : ''}`}>
                    Copy the default command for the first host first. Open advanced options only if
                    this machine needs custom connection or install settings.
                  </p>
                </div>
              </div>

              <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div class="space-y-1">
                    <p class="text-sm font-semibold text-base-content">
                      Advanced connection and install options
                    </p>
                    <p class="text-xs text-muted">
                      Default commands use the auto-detected Pulse URL. Open this only if the target
                      host needs a different URL, custom CA trust, TLS override, command execution,
                      or a specific install profile.
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setShowAdvancedOptions((current) => !current)}
                    class="inline-flex items-center justify-center rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface"
                  >
                    {showAdvancedOptions()
                      ? 'Hide advanced connection and install options'
                      : 'Show advanced connection and install options'}
                  </button>
                </div>
              </div>

              <Show when={showAdvancedOptions()}>
                <div class="space-y-3 rounded-md border border-border bg-surface px-4 py-4">
                  <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                    <label class="mb-1.5 block text-xs font-medium text-base-content">
                      Connection URL (Agent → Pulse)
                    </label>
                    <div class="flex gap-2">
                      <input
                        type="text"
                        value={state.customAgentUrl()}
                        onInput={(event) => state.setCustomAgentUrl(event.currentTarget.value)}
                        placeholder={state.agentUrl()}
                        class="flex-1 rounded-md border bg-surface px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                      />
                    </div>
                    <p class="mt-1.5 text-xs text-muted">
                      Override the address agents use to connect to this server (e.g., use IP address{' '}
                      <code>http://192.0.2.50:7655</code> if DNS fails).
                      <Show when={!state.customAgentUrl()}>
                        <span class="ml-1 opacity-75">
                          Currently using auto-detected: {state.agentUrl()}
                        </span>
                      </Show>
                    </p>
                  </div>

                  <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                    <label
                      for="custom-ca-certificate-path"
                      class="mb-1.5 block text-xs font-medium text-base-content"
                    >
                      Custom CA certificate path (optional)
                    </label>
                    <div class="flex gap-2">
                      <input
                        id="custom-ca-certificate-path"
                        type="text"
                        value={state.customCaPath()}
                        onInput={(event) => state.setCustomCaPath(event.currentTarget.value)}
                        placeholder={
                          state.selectedAgentUrl().startsWith('http://')
                            ? 'Not needed for plain HTTP'
                            : 'Examples: /etc/pulse/ca.pem or C:\\Pulse\\ca.cer'
                        }
                        class="flex-1 rounded-md border bg-surface px-3 py-1.5 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                      />
                    </div>
                    <p class="mt-1.5 text-xs text-muted">
                      Preserves custom trust for copied install, upgrade, and uninstall commands.
                      Shell commands pass <code>--cacert</code> to both the download and the installer.
                      Windows commands set <code>PULSE_CACERT</code> and use a transport-aware
                      PowerShell bootstrap for the initial script fetch.
                    </p>
                  </div>

                  <Show when={state.insecureMode()}>
                    <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                      <span class="font-medium">TLS verification disabled</span> — skip cert checks for
                      self-signed setups. Not recommended for production.
                    </div>
                  </Show>

                  <label
                    class="inline-flex cursor-pointer items-center gap-2 text-sm text-base-content"
                    title="Skip TLS certificate verification (for self-signed certificates)"
                  >
                    <input
                      type="checkbox"
                      checked={state.insecureMode()}
                      onChange={(event) => state.setInsecureMode(event.currentTarget.checked)}
                      class="rounded text-blue-600 focus:ring-blue-500"
                    />
                    Skip TLS certificate verification (self-signed certs; not recommended)
                  </label>

                  <label
                    class="inline-flex cursor-pointer items-center gap-2 text-sm text-base-content"
                    title="Allow Pulse Patrol to execute diagnostic and fix commands on this agent (auto-fix requires Pulse Pro)"
                  >
                    <input
                      type="checkbox"
                      checked={state.enableCommands()}
                      onChange={(event) => state.setEnableCommands(event.currentTarget.checked)}
                      class="rounded text-blue-600 focus:ring-blue-500"
                    />
                    Enable Pulse command execution (for Patrol auto-fix)
                  </label>

                  <Show when={state.enableCommands()}>
                    <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-2 text-sm text-blue-800 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200">
                      <span class="font-medium">Pulse commands enabled</span> — The agent will accept
                      diagnostic and fix commands from Pulse Patrol features.
                    </div>
                  </Show>

                  <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100">
                    <span class="font-medium">Config signing (optional)</span> — Require signed remote
                    config payloads with <code>PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true</code>.
                    Provide keys via <code>PULSE_AGENT_CONFIG_SIGNING_KEY</code> (Pulse) and{' '}
                    <code>PULSE_AGENT_CONFIG_PUBLIC_KEYS</code> (agents).
                  </div>

                  <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                    <label
                      for="install-profile-select"
                      class="mb-1.5 block text-xs font-medium text-base-content"
                    >
                      Target profile (optional)
                    </label>
                    <select
                      id="install-profile-select"
                      value={state.installProfile()}
                      onChange={(event) =>
                        state.handleInstallProfileChange(event.currentTarget.value as InstallProfile)
                      }
                      class="w-full rounded-md border bg-surface px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                    >
                      <For each={INSTALL_PROFILE_OPTIONS}>
                        {(option) => <option value={option.value}>{option.label}</option>}
                      </For>
                    </select>
                    <p class="mt-1.5 text-xs text-muted">
                      {state.getSelectedInstallProfile().description}
                    </p>
                    <Show when={state.getInstallProfileFlags().length > 0}>
                      <p class="mt-1.5 text-xs text-muted">
                        Adds flags to shell-based install commands:{' '}
                        <code>{state.getInstallProfileFlags().join(' ')}</code>
                      </p>
                    </Show>
                  </div>
                </div>
              </Show>
            </div>

            <div class="space-y-4">
              <For each={state.commandSections()}>
                {(section) => (
                  <div class="space-y-3 rounded-md border border-border p-4">
                    <div class="space-y-1">
                      <h5 class="text-sm font-semibold text-base-content">{section.title}</h5>
                      <p class="text-xs text-muted">{section.description}</p>
                    </div>
                    <div class="space-y-3">
                      <For each={section.snippets}>
                        {(snippet) => {
                          const copyCommand = () => snippet.command;
                          const commandTelemetryCapability = () => {
                            const label = normalizeTelemetryPart(snippet.label) || 'install';
                            return `${section.platform}:${state.installProfile()}:${label}`;
                          };

                          return (
                            <div class="space-y-2">
                              <h6 class="text-xs font-semibold uppercase tracking-wide text-muted">
                                {snippet.label}
                              </h6>
                              <div class="relative">
                                <button
                                  type="button"
                                  onClick={async () => {
                                    const success = await copyToClipboard(copyCommand());
                                    if (success) {
                                      trackAgentInstallCommandCopied(
                                        UNIFIED_AGENT_TELEMETRY_SURFACE,
                                        commandTelemetryCapability(),
                                      );
                                      notificationStore.success(
                                        getUnifiedAgentClipboardCopySuccessMessage(),
                                      );
                                    } else {
                                      notificationStore.error(
                                        getUnifiedAgentClipboardCopyErrorMessage(),
                                      );
                                    }
                                  }}
                                  class="absolute right-2 top-2 inline-flex min-h-10 min-w-10 items-center justify-center rounded-md bg-surface-hover p-2 transition-colors hover:text-slate-200 sm:min-h-9 sm:min-w-9"
                                  title="Copy command"
                                >
                                  <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
                                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                                    <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                                  </svg>
                                </button>
                                <pre class="overflow-x-auto rounded-md bg-base p-3 pr-12 text-xs text-base-content">
                                  <code>{copyCommand()}</code>
                                </pre>
                              </div>
                              <Show when={snippet.note}>
                                <p class="text-xs text-muted">{snippet.note}</p>
                              </Show>
                            </div>
                          );
                        }}
                      </For>
                    </div>
                  </div>
                )}
              </For>
            </div>

            <div class="space-y-3 rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-100">
              <div class="flex items-center justify-between gap-3">
                <h5 class="text-sm font-semibold">Check installation status</h5>
                <button
                  type="button"
                  onClick={() => void state.handleLookup()}
                  disabled={state.lookupLoading()}
                  class="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {state.lookupLoading() ? 'Checking…' : 'Check status'}
                </button>
              </div>
              <p class="text-xs text-blue-800 dark:text-blue-200">
                Enter the hostname (or agent ID) from the machine you just installed. Pulse returns
                the latest status instantly.
              </p>
              <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                <input
                  type="text"
                  value={state.lookupValue()}
                  onInput={(event) => {
                    state.setLookupValue(event.currentTarget.value);
                    state.clearLookupState();
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      void state.handleLookup();
                    }
                  }}
                  placeholder="Hostname or agent ID"
                  class="flex-1 rounded-md border border-blue-200 bg-surface px-3 py-2 text-sm text-blue-900 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100 dark:focus:border-blue-300 dark:focus:ring-blue-800"
                />
              </div>
              <Show when={state.lookupError()}>
                <p class="text-xs font-medium text-red-600 dark:text-red-300">{state.lookupError()}</p>
              </Show>
              <Show when={state.lookupResult()}>
                {(result) => {
                  const agent = () => result().agent!;
                  const lookupStatusPresentation = () =>
                    getUnifiedAgentLookupStatusPresentation(agent().connected);
                  const isConnected = () => agent().connected;

                  return (
                    <div
                      class={`space-y-3 rounded-md border px-3 py-3 text-xs ${
                        isConnected()
                          ? 'border-emerald-200 bg-emerald-50 text-emerald-950 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-50'
                          : 'border-blue-200 bg-surface text-blue-900 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-100'
                      }`}
                    >
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div class="space-y-1">
                          <div class="text-sm font-semibold">
                            {isConnected() ? 'First host connected' : agent().displayName || agent().hostname}
                          </div>
                          <p
                            class={`text-xs ${
                              isConnected()
                                ? 'text-emerald-800 dark:text-emerald-200'
                                : 'text-blue-800 dark:text-blue-200'
                            }`}
                          >
                            {isConnected()
                              ? `${agent().displayName || agent().hostname} is reporting live telemetry to Pulse. Open the dashboard to verify your first overview, or continue in Reporting & control to inspect this host and add more infrastructure.`
                              : `${agent().displayName || agent().hostname} has been found, but Pulse is not receiving a live check-in yet. Keep the installer running on that machine and check again.`}
                          </p>
                        </div>
                        <div class="flex items-center gap-2">
                          <span
                            class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold ${lookupStatusPresentation().badgeClass}`}
                          >
                            {lookupStatusPresentation().label}
                          </span>
                          <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900 dark:text-blue-200">
                            {agent().status || 'unknown'}
                          </span>
                        </div>
                      </div>
                      <div class="space-y-1">
                        <div>
                          Last seen {formatRelativeTime(agent().lastSeen)} (
                          {formatAbsoluteTime(agent().lastSeen)})
                        </div>
                        <Show when={agent().agentVersion}>
                          <div
                            class={`text-xs ${
                              isConnected()
                                ? 'text-emerald-800 dark:text-emerald-200'
                                : 'text-blue-700 dark:text-blue-200'
                            }`}
                          >
                            Agent version {agent().agentVersion}
                          </div>
                        </Show>
                      </div>
                      <Show when={isConnected()}>
                        <div class="flex flex-col gap-2 sm:flex-row">
                          <button
                            type="button"
                            onClick={state.openDashboard}
                            class="inline-flex items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-700"
                          >
                            Open dashboard
                          </button>
                          <button
                            type="button"
                            onClick={state.openInfrastructureInventory}
                            class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 transition-colors hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                          >
                            Open reporting &amp; control
                          </button>
                        </div>
                      </Show>
                    </div>
                  );
                }}
              </Show>
            </div>

            <details class="rounded-md border border-border bg-surface-hover px-4 py-3 text-sm">
              <summary class="cursor-pointer text-sm font-medium text-base-content">
                Troubleshooting
              </summary>
              <div class="mt-3 space-y-4">
                <div>
                  <p class="text-xs uppercase tracking-wide text-muted">Auto-detection not working?</p>
                  <p class="mt-1 text-xs text-muted">
                    If Docker, Kubernetes, or Proxmox isn't detected automatically, add these flags
                    to the install command:
                  </p>
                  <ul class="mt-2 list-inside list-disc space-y-1 text-xs text-muted">
                    <li>
                      <code class="rounded bg-surface-hover px-1">--enable-docker</code> — Force
                      enable Docker/Podman monitoring
                    </li>
                    <li>
                      <code class="rounded bg-surface-hover px-1">--enable-kubernetes</code> — Force
                      enable Kubernetes monitoring
                    </li>
                    <li>
                      <code class="rounded bg-surface-hover px-1">--enable-proxmox</code> — Force
                      enable Proxmox integration (creates API token)
                    </li>
                    <li>
                      <code class="rounded bg-surface-hover px-1">--proxmox-type pve|pbs</code> —
                      Set Proxmox node mode explicitly
                    </li>
                    <li>
                      <code class="rounded bg-surface-hover px-1">--disable-docker</code> — Skip
                      Docker even if detected
                    </li>
                  </ul>
                </div>
              </div>
            </details>
          </div>
        </Show>

        <div class="mt-4 border-t border-border pt-4">
          <div class="space-y-3">
            <h4 class="text-sm font-semibold text-base-content">Uninstall agent</h4>
            <p class="text-xs text-muted">
              Run the appropriate command on your machine to remove the Pulse agent:
            </p>
            <div class="space-y-1">
              <span class="text-xs font-medium text-muted">Linux / macOS / FreeBSD</span>
              <div class="relative">
                <button
                  type="button"
                  onClick={async () => {
                    const success = await copyToClipboard(state.getUninstallCommand());
                    if (success) {
                      notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                    } else {
                      notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                    }
                  }}
                  class="absolute right-2 top-2 inline-flex min-h-10 min-w-10 items-center justify-center rounded-md bg-surface-hover p-2 text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-200 sm:min-h-9 sm:min-w-9"
                  title="Copy command"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                    <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                  </svg>
                </button>
                <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-12 font-mono text-xs text-red-400">
                  <code>{state.getUninstallCommand()}</code>
                </pre>
              </div>
            </div>
            <p class="text-xs italic text-muted">
              If the agent can't reach this server, run directly on the machine:{' '}
              <code class="rounded bg-surface-hover px-1 not-italic">
                sudo bash /var/lib/pulse-agent/install.sh --uninstall
              </code>{' '}
              (TrueNAS:{' '}
              <code class="rounded bg-surface-hover px-1 not-italic">
                /data/pulse-agent/install.sh
              </code>
              , Unraid:{' '}
              <code class="rounded bg-surface-hover px-1 not-italic">
                /boot/config/plugins/pulse-agent/install.sh
              </code>
              )
            </p>
            <div class="space-y-1">
              <span class="text-xs font-medium text-muted">Windows (PowerShell as Administrator)</span>
              <div class="relative">
                <button
                  type="button"
                  onClick={async () => {
                    const success = await copyToClipboard(state.getWindowsUninstallCommand());
                    if (success) {
                      notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                    } else {
                      notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                    }
                  }}
                  class="absolute right-2 top-2 inline-flex min-h-10 min-w-10 items-center justify-center rounded-md bg-surface-hover p-2 text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-200 sm:min-h-9 sm:min-w-9"
                  title="Copy command"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                    <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                  </svg>
                </button>
                <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-12 font-mono text-xs text-red-400">
                  <code>{state.getWindowsUninstallCommand()}</code>
                </pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </SettingsPanel>
  );
};

export default InfrastructureInstallerSection;
