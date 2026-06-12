import type { Component } from 'solid-js';
import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { Button, CommandCopyButton } from '@/components/shared/Button';
import { FormSelect } from '@/components/shared/FormSelect';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { copyToClipboard } from '@/utils/clipboard';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { notificationStore } from '@/stores/notifications';
import { getUnifiedAgentLookupStatusPresentation } from '@/utils/unifiedAgentStatusPresentation';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import {
  INSTALL_PROFILE_OPTIONS,
  type AgentPlatform,
  type InfrastructureCommandSection,
  type InstallProfile,
} from './infrastructureOperationsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

export type InfrastructureInstallerFocus =
  | 'agent'
  | 'linux-host'
  | 'unraid'
  | 'docker'
  | 'kubernetes';

interface InfrastructureInstallerSectionProps {
  focus?: InfrastructureInstallerFocus;
}

type InfrastructureCommandSectionWithPlatform = InfrastructureCommandSection & {
  platform: AgentPlatform;
};

type InfrastructureInstallerFocusPresentation = {
  title: string;
  description: string;
  recommendationTitle: string;
  recommendationDetail: string;
  preferredProfile: InstallProfile;
  platforms: readonly AgentPlatform[];
};

const ALL_AGENT_PLATFORMS: readonly AgentPlatform[] = ['linux', 'macos', 'freebsd', 'windows'];

const INSTALLER_FOCUS_PRESENTATION: Record<
  InfrastructureInstallerFocus,
  InfrastructureInstallerFocusPresentation
> = {
  agent: {
    title: 'Install on a host',
    description:
      'Start here to add the first system you want Pulse to monitor, then expand into Docker, Kubernetes, Proxmox, and related infrastructure.',
    recommendationTitle: 'Recommended install model',
    recommendationDetail:
      'Pulse Agent is a low-overhead background service. Machines in Pulse are systems with the agent installed and reporting full node-local telemetry such as CPU, memory, disks, network I/O, temperatures, SMART disk health, services, Docker, or Kubernetes coverage. Use Availability checks for ping-only or agentless device monitoring. For Proxmox clusters, keep the cluster API connection for platform inventory and add the agent to each node for host-level augmentation.',
    preferredProfile: 'auto',
    platforms: ALL_AGENT_PLATFORMS,
  },
  'linux-host': {
    title: 'Install on a host',
    description:
      'Choose the command for the operating system on the machine you want Pulse to monitor.',
    recommendationTitle: 'Host install path',
    recommendationDetail:
      'Install Pulse Agent on the machine itself. Pulse will collect CPU, memory, disk, network I/O, services, SMART disk health, sensors, and other host telemetry after it checks in. Use Availability checks instead when you only want an agentless reachability probe.',
    preferredProfile: 'auto',
    platforms: ALL_AGENT_PLATFORMS,
  },
  unraid: {
    title: 'Install on Unraid',
    description:
      'Run the Linux installer from the Unraid terminal or SSH session on the server you want Pulse to monitor.',
    recommendationTitle: 'Unraid install path',
    recommendationDetail:
      'Install Pulse Agent directly on the Unraid server. Pulse will classify the reporting host as Unraid from the agent host profile and collect array health, disks, SMART, services, Docker containers, and host telemetry.',
    preferredProfile: 'auto',
    platforms: ['linux'],
  },
  docker: {
    title: 'Install for Docker / Podman',
    description:
      'Run on the machine that runs Docker or Podman. For Docker inside Proxmox LXCs, use the Proxmox node path below instead of installing inside every guest.',
    recommendationTitle: 'Docker install path',
    recommendationDetail:
      'For a standalone Docker or Podman host, install Pulse Agent on that host. The Docker profile is selected for this flow so copied commands force runtime monitoring when automatic detection is restricted. For Docker inside Proxmox LXCs, install on the Proxmox node, select the Proxmox VE node profile, and enable command execution so the server-opted-in LXC inventory path can run.',
    preferredProfile: 'docker',
    platforms: ['linux'],
  },
  kubernetes: {
    title: 'Install on a Kubernetes node',
    description:
      'Run the installer on a cluster node that should report Kubernetes workload context and host telemetry.',
    recommendationTitle: 'Kubernetes install path',
    recommendationDetail:
      'Install Pulse Agent on a Kubernetes node. The Kubernetes profile is selected for this flow so copied commands enable workload context while preserving node-local telemetry.',
    preferredProfile: 'kubernetes',
    platforms: ['linux'],
  },
};

const getCommandSectionTitle = (
  section: InfrastructureCommandSectionWithPlatform,
  focus: InfrastructureInstallerFocus,
): string => {
  if (section.platform !== 'linux') return section.title;
  if (focus === 'unraid') return 'Run on Unraid';
  if (focus === 'docker') return 'Run on the Docker host or Proxmox node';
  if (focus === 'kubernetes') return 'Run on a Kubernetes node';
  return section.title;
};

const getCommandSectionDescription = (
  section: InfrastructureCommandSectionWithPlatform,
  focus: InfrastructureInstallerFocus,
): string => {
  if (section.platform !== 'linux') return section.description;
  if (focus === 'unraid') {
    return 'Use the Linux installer from Unraid terminal or SSH. The agent stores its local uninstall helper under the Unraid plugin path.';
  }
  if (focus === 'docker') {
    return 'Use the Linux installer on a standalone Docker or Podman host. For Docker inside Proxmox LXCs, switch Target profile to Proxmox VE node and enable command execution.';
  }
  if (focus === 'kubernetes') {
    return 'Use the Linux installer on a Kubernetes node. Commands in this flow include the Kubernetes profile.';
  }
  return section.description;
};

export const InfrastructureInstallerSection: Component<InfrastructureInstallerSectionProps> = (
  props,
) => {
  const state = useInfrastructureOperationsContext();
  const [showAdvancedOptions, setShowAdvancedOptions] = createSignal(false);
  const focus = createMemo<InfrastructureInstallerFocus>(() => props.focus ?? 'agent');
  const presentation = createMemo(() => INSTALLER_FOCUS_PRESENTATION[focus()]);
  const commandSections = createMemo(() =>
    state
      .commandSections()
      .filter((section) => presentation().platforms.includes(section.platform)),
  );

  createEffect(() => {
    state.handleInstallProfileChange(presentation().preferredProfile);
  });

  return (
    <SettingsPanel
      title={state.isEmbedded() ? presentation().title : 'Infrastructure'}
      description={
        state.isEmbedded()
          ? presentation().description
          : 'Primary setup hub for installing Pulse on the first host you want to monitor, then expanding into Docker, Kubernetes, Proxmox, and related infrastructure.'
      }
      bodyClass="space-y-5"
    >
      <Show when={state.setupHandoff()}>
        {(handoff) => (
          <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-950 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-50">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div class="space-y-2">
                <p class="font-semibold">
                  Security configured. Save these first-run credentials now.
                </p>
                <p class="text-xs text-emerald-800 dark:text-emerald-200">
                  This is the Pulse Agent handoff from first-run setup inside Add infrastructure.
                  <Show
                    when={state.setupHandoffAutoTokenPending()}
                    fallback={
                      <Show
                        when={state.latestTokenSource() === 'setup_handoff'}
                        fallback={
                          <Show
                            when={state.setupHandoffAutoTokenFailed()}
                            fallback=" Pulse will prepare the first scoped install token here before you copy agent commands."
                          >
                            {' '}
                            Pulse could not prepare the first scoped install token automatically, so
                            use the token step below and continue from there.
                          </Show>
                        }
                      >
                        {' '}
                        Pulse already prepared the first scoped install token for this handoff, so
                        you can move straight to copying the install command for the first host.
                      </Show>
                    }
                  >
                    {' '}
                    Pulse is preparing the first scoped install token now so the install commands
                    unlock without another setup step.
                  </Show>
                </p>
                <div class="grid gap-3 sm:grid-cols-2">
                  <div class="rounded-md border border-emerald-200 bg-white px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950">
                    <div class="text-[11px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                      Username
                    </div>
                    <div class="mt-1 font-mono text-sm text-base-content">{handoff().username}</div>
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
                <p class="text-xs text-emerald-800 dark:text-emerald-200">
                  Your admin password was shown once on the setup screen and isn't stored here. If
                  you didn't save it, change it from Settings → Security → Change password.
                </p>
              </div>
              <div class="flex flex-wrap gap-2 lg:w-64 lg:flex-col">
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

      <div class="space-y-5">
        <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-950 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-50">
          <p class="font-semibold">{presentation().recommendationTitle}</p>
          <p class="mt-1 text-xs text-emerald-800 dark:text-emerald-200">
            {presentation().recommendationDetail}
          </p>
        </div>

        <Show when={focus() === 'docker'}>
          <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-950 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-50">
            <p class="font-semibold">Docker inside Proxmox LXCs</p>
            <p class="mt-1 text-xs text-blue-800 dark:text-blue-200">
              Install the agent on the Proxmox node, not inside every LXC. In advanced options,
              select <span class="font-medium">Proxmox VE node</span> and enable{' '}
              <span class="font-medium">Pulse command execution</span>. Then start Pulse with{' '}
              <code>PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true</code>; optionally restrict it
              with <code>PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS=101,102</code>. Pulse uses
              bounded <code>pct exec</code> inventory for <code>docker ps</code> and{' '}
              <code>docker stats</code>, skips guests that already have their own agent, and does
              not collect inspect, environment, mount, command, or process details.
            </p>
          </div>
        </Show>

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
                ? state.setupHandoffAutoTokenPending()
                  ? 'Pulse is generating the first scoped install token for this setup handoff now.'
                  : state.latestTokenSource() === 'setup_handoff'
                    ? 'Pulse generated the first scoped install token automatically. The install commands below are ready for the first host.'
                    : state.setupHandoffAutoTokenFailed()
                      ? 'Pulse could not generate the first scoped install token automatically. Generate one here to unlock the install commands.'
                      : 'Create a fresh token for the generated install commands and host reporting.'
                : 'Tokens are optional on this Pulse instance. Generate one if you want copied commands to preserve explicit credentialed transport.'}
            </p>
          </div>

          <div class="ml-6 rounded-md border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-900">
            <p class="mb-2 text-sm font-semibold text-blue-800 dark:text-blue-200">
              What this token authorizes:
            </p>
            <ul class="space-y-1 text-xs text-blue-700 dark:text-blue-300">
              <li class="flex items-start">
                <span class="mr-2 mt-0.5 text-emerald-500">✓</span>
                <span>
                  Pulse Agent on this host reports its own telemetry to this Pulse instance.
                </span>
              </li>
              <li class="flex items-start">
                <span class="mr-2 mt-0.5 text-emerald-500">✓</span>
                <span>
                  Read-only by default. Assistant control and host shell stay off until you opt in
                  per host from <span class="font-medium">Settings → Infrastructure</span>.
                </span>
              </li>
              <li class="flex items-start">
                <span class="mr-2 mt-0.5 text-emerald-500">✓</span>
                <span>
                  Revoke the token any time from{' '}
                  <span class="font-medium">Settings → Infrastructure → Tokens</span>; the agent
                  stops reporting immediately.
                </span>
              </li>
            </ul>
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
              placeholder="Token name (optional label for your audit log)"
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
              <svg
                class="h-4 w-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
              </svg>
              <span>
                {state.latestTokenSource() === 'setup_handoff' ? (
                  <>
                    First-host install token <strong>{state.latestRecord()?.name}</strong> prepared
                    automatically. Commands below already include this credential.
                  </>
                ) : (
                  <>
                    Install token <strong>{state.latestRecord()?.name}</strong> created. Commands
                    below now include this credential.
                  </>
                )}
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
          <div class="space-y-3 rounded-md border border-border bg-surface-alt px-4 py-4">
            <div class="space-y-1">
              <h4 class="text-sm font-semibold text-base-content">
                <span class="mr-1.5 inline-flex h-5 w-5 items-center justify-center rounded-full bg-slate-400 text-xs font-bold text-white">
                  2
                </span>
                Installation commands
              </h4>
              <p class="ml-6 text-xs text-muted">
                Generate an install token first. Pulse will then build copy-ready commands with the
                credential inserted for the target host.
              </p>
            </div>
            <div class="grid gap-3 lg:grid-cols-2">
              <For each={commandSections()}>
                {(section) => (
                  <div class="rounded-md border border-border bg-surface px-3 py-3">
                    <div class="space-y-1">
                      <h5 class="text-sm font-semibold text-base-content">
                        {getCommandSectionTitle(section, focus())}
                      </h5>
                      <p class="text-xs text-muted">
                        {getCommandSectionDescription(section, focus())}
                      </p>
                    </div>
                    <div class="mt-3 flex flex-wrap gap-2">
                      <For each={section.snippets}>
                        {(snippet) => (
                          <span class="inline-flex items-center rounded-full border border-border bg-surface-alt px-2.5 py-1 text-[11px] font-medium text-base-content">
                            {snippet.label}
                          </span>
                        )}
                      </For>
                    </div>
                  </div>
                )}
              </For>
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
                    Copy the default command for the first host first. It checks this Pulse URL and
                    the matching agent binary before asking for administrator privileges, then
                    installs Pulse Agent as a background service on each machine where you want full
                    node-local telemetry.
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
                  <Button
                    type="button"
                    variant="secondary"
                    size="mdCompact"
                    onClick={() => setShowAdvancedOptions((current) => !current)}
                  >
                    {showAdvancedOptions()
                      ? 'Hide advanced connection and install options'
                      : 'Show advanced connection and install options'}
                  </Button>
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
                      Override the address agents use to connect to this server (e.g., use IP
                      address <code>http://192.0.2.50:7655</code> if DNS fails).
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
                      Shell commands pass <code>--cacert</code> to both the download and the
                      installer. Windows commands set <code>PULSE_CACERT</code> and use a
                      transport-aware PowerShell bootstrap for the initial script fetch.
                    </p>
                  </div>

                  <Show when={state.insecureMode()}>
                    <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
                      <span class="font-medium">TLS verification disabled</span> — skip cert checks
                      for self-signed setups. Not recommended for production.
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
                    title="Allow Pulse-scoped command requests on this agent for Patrol actions and opted-in Proxmox LXC Docker inventory"
                  >
                    <input
                      type="checkbox"
                      checked={state.enableCommands()}
                      onChange={(event) => state.setEnableCommands(event.currentTarget.checked)}
                      class="rounded text-blue-600 focus:ring-blue-500"
                    />
                    Enable Pulse command execution (Patrol actions and Proxmox LXC Docker inventory)
                  </label>

                  <Show when={state.enableCommands()}>
                    <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-2 text-sm text-blue-800 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200">
                      <span class="font-medium">Pulse commands enabled</span>: The agent will accept
                      Pulse-scoped command requests. On Proxmox nodes, this is required for opted-in
                      Docker-in-LXC inventory because Pulse runs bounded <code>pct exec</code>{' '}
                      Docker summary checks.
                    </div>
                  </Show>

                  <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900 dark:text-emerald-100">
                    <span class="font-medium">Config signing (optional)</span> — Require signed
                    remote config payloads with{' '}
                    <code>PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true</code>. Provide keys via{' '}
                    <code>PULSE_AGENT_CONFIG_SIGNING_KEY</code> (Pulse) and{' '}
                    <code>PULSE_AGENT_CONFIG_PUBLIC_KEYS</code> (agents).
                  </div>

                  <div class="rounded-md border border-border bg-surface-hover px-4 py-3">
                    <FormSelect
                      id="install-profile-select"
                      label="Target profile (optional)"
                      labelClass="mb-1.5 block text-xs font-medium text-base-content"
                      value={state.installProfile()}
                      onChange={(event) =>
                        state.handleInstallProfileChange(
                          event.currentTarget.value as InstallProfile,
                        )
                      }
                      selectBaseClass="w-full rounded-md border bg-surface px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:focus:border-blue-400 dark:focus:ring-blue-800"
                    >
                      <For each={INSTALL_PROFILE_OPTIONS}>
                        {(option) => <option value={option.value}>{option.label}</option>}
                      </For>
                    </FormSelect>
                    <p class="mt-1.5 text-xs text-muted">
                      {state.getSelectedInstallProfile().description}
                    </p>
                    <p class="mt-1.5 text-xs text-muted">
                      API-backed platforms such as TrueNAS connect through Add infrastructure →
                      choose source type.
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
              <For each={commandSections()}>
                {(section) => (
                  <div class="space-y-3 rounded-md border border-border p-4">
                    <div class="space-y-1">
                      <h5 class="text-sm font-semibold text-base-content">
                        {getCommandSectionTitle(section, focus())}
                      </h5>
                      <p class="text-xs text-muted">
                        {getCommandSectionDescription(section, focus())}
                      </p>
                    </div>
                    <div class="space-y-3">
                      <For each={section.snippets}>
                        {(snippet) => {
                          const copyCommand = () => snippet.command;

                          return (
                            <div class="space-y-2">
                              <h6 class="text-xs font-semibold uppercase tracking-wide text-muted">
                                {snippet.label}
                              </h6>
                              <div class="relative">
                                <CommandCopyButton
                                  onClick={async () => {
                                    const success = await copyToClipboard(copyCommand());
                                    if (success) {
                                      notificationStore.success(
                                        getUnifiedAgentClipboardCopySuccessMessage(),
                                      );
                                    } else {
                                      notificationStore.error(
                                        getUnifiedAgentClipboardCopyErrorMessage(),
                                      );
                                    }
                                  }}
                                  title="Copy command"
                                  label={`Copy ${snippet.label} command`}
                                />
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
                {state.autoLookupActive()
                  ? 'Pulse is watching for the first reporting host automatically. Enter a hostname or agent ID only if you want to check a specific machine right now.'
                  : 'Enter the hostname (or agent ID) from the machine you just installed. Pulse returns the latest status instantly.'}
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
                <p class="text-xs font-medium text-red-600 dark:text-red-300">
                  {state.lookupError()}
                </p>
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
                            {isConnected()
                              ? 'First host connected'
                              : agent().displayName || agent().hostname}
                          </div>
                          <p
                            class={`text-xs ${
                              isConnected()
                                ? 'text-emerald-800 dark:text-emerald-200'
                                : 'text-blue-800 dark:text-blue-200'
                            }`}
                          >
                            {isConnected()
                              ? state.lookupWasAutoDetected()
                                ? `${agent().displayName || agent().hostname} started reporting and Pulse detected it automatically. Open Infrastructure to inspect this host and add more infrastructure.`
                                : `${agent().displayName || agent().hostname} is reporting live telemetry to Pulse. Open Infrastructure to inspect this host and add more infrastructure.`
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
                            onClick={state.openInfrastructure}
                            class="inline-flex items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-700"
                          >
                            Open infrastructure
                          </button>
                          <button
                            type="button"
                            onClick={state.openInfrastructureInventory}
                            class="inline-flex items-center justify-center rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-medium text-emerald-900 transition-colors hover:bg-emerald-100 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-100 dark:hover:bg-emerald-800"
                          >
                            Open inventory
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
                  <p class="text-xs uppercase tracking-wide text-muted">
                    Auto-detection not working?
                  </p>
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

        <details class="mt-4 border-t border-border pt-4">
          <summary class="cursor-pointer text-sm font-semibold text-base-content">
            Uninstall agent
          </summary>
          <div class="mt-3 space-y-3">
            <p class="text-xs text-muted">
              Run the appropriate command on your machine to remove the Pulse agent:
            </p>
            <div class="space-y-1">
              <span class="text-xs font-medium text-muted">Linux / macOS / FreeBSD</span>
              <div class="relative">
                <CommandCopyButton
                  onClick={async () => {
                    const success = await copyToClipboard(state.getUninstallCommand());
                    if (success) {
                      notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                    } else {
                      notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                    }
                  }}
                  title="Copy command"
                  label="Copy Linux macOS FreeBSD uninstall command"
                />
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
              <span class="text-xs font-medium text-muted">
                Windows (PowerShell as Administrator)
              </span>
              <div class="relative">
                <CommandCopyButton
                  onClick={async () => {
                    const success = await copyToClipboard(state.getWindowsUninstallCommand());
                    if (success) {
                      notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
                    } else {
                      notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
                    }
                  }}
                  title="Copy command"
                  label="Copy Windows uninstall command"
                />
                <pre class="overflow-x-auto rounded-md bg-slate-950 p-3 pr-12 font-mono text-xs text-red-400">
                  <code>{state.getWindowsUninstallCommand()}</code>
                </pre>
              </div>
            </div>
          </div>
        </details>
      </div>
    </SettingsPanel>
  );
};

export default InfrastructureInstallerSection;
