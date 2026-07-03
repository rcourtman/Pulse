import { Component, Show } from 'solid-js';
import {
  PVE_MANUAL_PERMISSION_COMMAND,
  type NodeModalProps,
} from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import type { NodeModalNodeType, NodeModalSetupMode } from '@/utils/nodeModalPresentation';

interface NodeModalSetupGuideSectionProps {
  modalProps: NodeModalProps;
  state: NodeModalState;
}

const getNodeSetupStrategyPresentation = (
  nodeType: NodeModalNodeType,
  setupMode: NodeModalSetupMode,
): { label: string; detail: string } => {
  const productLabel = nodeType === 'pbs' ? 'Proxmox Backup Server' : 'Proxmox VE';

  if (setupMode === 'agent') {
    return {
      label: 'Host telemetry agent',
      detail: `Optional full host telemetry: creates the ${productLabel} API token, installs Pulse Agent as the supported root service, and registers the source automatically.`,
    };
  }

  if (setupMode === 'auto') {
    return {
      label: 'API inventory',
      detail: `Recommended least-privilege path: creates the ${productLabel} API token and registers the API connection without installing a root agent.`,
    };
  }

  return {
    label: 'Manual API token',
    detail:
      'Advanced escape hatch: use this only when you already created the API token yourself and want to paste the token details into Pulse.',
  };
};

const setupModeButtonClass = (selected: boolean): string =>
  `inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-transparent transition-colors ${
    selected
      ? 'bg-surface text-blue-600 dark:text-blue-300 border-border shadow-sm'
      : 'text-muted hover:text-blue-600 dark:hover:text-blue-300 hover:bg-surface-hover'
  }`;

export const NodeModalSetupGuideSection: Component<NodeModalSetupGuideSectionProps> = (props) => {
  const { modalProps, state } = props;
  const setupHandoffDisabled = () => Boolean(modalProps.setupHandoffDisabled?.());
  const setupHandoffDisabledReason = () =>
    modalProps.setupHandoffDisabledReason ??
    'Complete the required review before generating setup commands.';
  const setupCommandButtonTitle = () =>
    setupHandoffDisabled() ? setupHandoffDisabledReason() : 'Copy command';
  const setupCommandButtonClass = (className: string) =>
    `${className} disabled:cursor-not-allowed disabled:opacity-50`;
  const setupStrategy = () =>
    getNodeSetupStrategyPresentation(modalProps.nodeType, state.formData().setupMode);
  const setupStrategyPanel = () => (
    <div class="rounded-md border border-blue-200 bg-surface px-3 py-2 dark:border-blue-800">
      <div class="text-[10px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300">
        Source strategy
      </div>
      <div class="mt-1 text-sm font-semibold text-base-content">{setupStrategy().label}</div>
      <p class="mt-1 text-xs leading-5 text-muted">{setupStrategy().detail}</p>
    </div>
  );
  const setupModeControls = () => (
    <div class="flex gap-2 flex-wrap">
      <button
        type="button"
        onClick={() => state.updateField('setupMode', 'auto')}
        class={setupModeButtonClass(state.formData().setupMode === 'auto')}
      >
        Connect via API
        <span class="ml-1.5 px-1.5 py-0.5 text-[10px] font-semibold bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
          Recommended
        </span>
      </button>
      <button
        type="button"
        onClick={() => state.updateField('setupMode', 'agent')}
        class={setupModeButtonClass(state.formData().setupMode === 'agent')}
      >
        Host Telemetry Agent
      </button>
      <button
        type="button"
        onClick={() => state.updateField('setupMode', 'manual')}
        class={setupModeButtonClass(state.isAdvancedSetupMode())}
      >
        Manual Token Setup
      </button>
    </div>
  );

  return (
    <div class="space-y-4">
      <Show when={modalProps.nodeType === 'pve'}>
        <div class="space-y-3 text-xs">
          <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-4">
            <h5 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-3 flex items-center gap-2">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <path d="M12 6v6l4 2"></path>
              </svg>
              Connection Setup
            </h5>
            {setupStrategyPanel()}
            <Show when={setupHandoffDisabled()}>
              <p class="mt-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
                {setupHandoffDisabledReason()}
              </p>
            </Show>

            <div class="space-y-3 text-xs">
              {setupModeControls()}

              <Show when={state.formData().setupMode === 'agent'}>
                <div class="space-y-3">
                  <p class="text-xs text-muted">
                    Optional full host telemetry setup. This command creates the API token, installs
                    the Pulse Agent root service, registers the node, and leaves Pulse waiting for
                    the agent check-in:
                  </p>
                  <ul class="text-xs text-muted list-disc list-inside space-y-1">
                    <li>Creates monitoring user and API token automatically</li>
                    <li>Registers the node with Pulse</li>
                    <li>
                      Adds host-local telemetry such as temperatures, SMART, ZFS, Ceph, and mdadm
                    </li>
                    <li>
                      Enables Pulse command execution for Patrol actions and opted-in Docker-in-LXC
                      inventory
                    </li>
                  </ul>
                  <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-950/30">
                    <p class="text-xs text-blue-800 dark:text-blue-200">
                      <strong>Docker inside Proxmox LXCs:</strong> use this host-agent path instead
                      of installing Pulse Agent in every guest. The copied command enables Pulse
                      command execution on the Proxmox node; the Pulse server still must be opted in
                      with{' '}
                      <code class="break-all rounded bg-blue-100 px-1 font-mono dark:bg-blue-900">
                        PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true
                      </code>
                      , and you can limit guests with{' '}
                      <code class="break-all rounded bg-blue-100 px-1 font-mono dark:bg-blue-900">
                        PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS=101,102
                      </code>
                      . Pulse uses bounded <code>pct exec</code> Docker summary checks and skips
                      guests that already report through their own agent.
                    </p>
                  </div>
                  <p class="text-blue-800 dark:text-blue-200 font-medium">
                    Run this command on your Proxmox VE node:
                  </p>
                  <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                    <button
                      type="button"
                      disabled={state.loadingAgentCommand() || setupHandoffDisabled()}
                      onClick={async () => {
                        await state.copyProxmoxAgentInstallCommand(
                          'pve',
                          'Command copied! Run it on your Proxmox node.',
                        );
                      }}
                      class={setupCommandButtonClass(
                        'absolute top-2 right-2 p-1.5 hover:text-slate-200 bg-surface-hover rounded-md transition-colors',
                      )}
                      title={setupCommandButtonTitle()}
                    >
                      <Show
                        when={state.loadingAgentCommand()}
                        fallback={
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
                        }
                      >
                        <svg
                          class="animate-spin"
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                        >
                          <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                          <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"></path>
                        </svg>
                      </Show>
                    </button>
                    <Show
                      when={state.agentInstallCommand().length > 0}
                      fallback={
                        <code class="text-muted">
                          Click the copy button to generate the install command
                        </code>
                      }
                    >
                      <code class="block text-base-content whitespace-pre-wrap break-words">
                        {state.agentInstallCommand()}
                      </code>
                    </Show>
                  </div>
                  <Show when={state.agentCommandError()}>
                    <p class="text-xs text-red-500">{state.agentCommandError()}</p>
                  </Show>
                  <p class="text-[11px] text-muted italic">
                    No token fields are needed here. The node appears in Pulse automatically after
                    the agent starts.
                  </p>
                </div>
              </Show>

              <Show when={state.formData().setupMode === 'auto'}>
                <div class="space-y-3">
                  <div class="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950/30">
                    <p class="text-xs text-emerald-800 dark:text-emerald-200">
                      <strong>Recommended API inventory path:</strong> this connects Pulse to
                      Proxmox without installing a root agent. Add the host telemetry agent later
                      only where you need temperatures, SMART, local storage details, or
                      agent-driven operations.
                    </p>
                  </div>
                  <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-950/30">
                    <p class="text-xs text-blue-800 dark:text-blue-200">
                      <strong>Docker inside Proxmox LXCs?</strong> Switch to Host Telemetry Agent
                      for the Proxmox node. API inventory alone does not run the opted-in host-side
                      Docker inventory path.
                    </p>
                  </div>
                  <Show when={state.isEditingExistingNode()}>
                    <div class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-950/30">
                      <p class="text-xs text-blue-800 dark:text-blue-200">
                        <strong>Existing source repair:</strong> rerun this command and choose
                        Audit/Repair to recheck the Pulse-managed user, token expiry, ACLs, and old
                        tokens without rotating the current API token. Choose Install/Configure only
                        when the token value needs to be replaced.
                      </p>
                    </div>
                  </Show>
                  <p class="text-blue-800 dark:text-blue-200">
                    Just copy and run this one command on your Proxmox VE server:
                  </p>

                  <div class="space-y-3">
                    <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                      <button
                        type="button"
                        onClick={async () => {
                          await state.copyQuickSetupCommand(
                            'pve',
                            true,
                            'Command copied to clipboard! Run it on the server; the one-time setup token is already embedded.',
                          );
                        }}
                        class="absolute top-2 right-2 p-1.5 text-slate-400 hover:text-slate-200 bg-surface-hover rounded-md transition-colors"
                        disabled={setupHandoffDisabled()}
                        title={
                          setupHandoffDisabled() ? setupHandoffDisabledReason() : 'Copy command'
                        }
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
                      <Show
                        when={state.quickSetupPreviewCommand().length > 0}
                        fallback={
                          <code class="text-muted">
                            {state.formData().host
                              ? 'Click the copy button to generate the setup command'
                              : 'Please enter the Endpoint URL above first'}
                          </code>
                        }
                      >
                        <code class="block text-base-content whitespace-pre-wrap break-words">
                          {state.quickSetupPreviewCommand()}
                        </code>
                      </Show>
                      <Show when={state.quickSetupTokenHint().length > 0}>
                        <div class="mt-2 text-xs text-blue-800 dark:text-blue-200">
                          <span class="font-semibold">Setup token hint:</span>
                          <code class="ml-1 font-mono break-all text-blue-900 dark:text-blue-100">
                            {state.quickSetupTokenHint()}
                          </code>
                          <Show when={state.quickSetupExpiry()}>
                            <span class="ml-2">Expires at {state.quickSetupExpiryLabel()}</span>
                          </Show>
                        </div>
                      </Show>
                    </div>

                    <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-3">
                      <div class="flex items-start space-x-2">
                        <svg
                          class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                          />
                        </svg>
                        <div class="text-xs text-amber-700 dark:text-amber-300">
                          <p class="font-semibold mb-1">If the command doesn't work:</p>
                          <p>
                            Your Proxmox server may not be able to reach Pulse. Use the alternative
                            method below.
                          </p>
                        </div>
                      </div>
                    </div>

                    <details class="bg-surface-alt rounded-md p-3">
                      <summary class="cursor-pointer text-sm font-medium text-base-content hover:text-base-content">
                        Alternative: Download script manually
                      </summary>
                      <div class="mt-3 space-y-3">
                        <button
                          type="button"
                          onClick={async () => {
                            await state.downloadProxmoxSetupScript('pve', true);
                          }}
                          disabled={setupHandoffDisabled()}
                          title={setupHandoffDisabled() ? setupHandoffDisabledReason() : undefined}
                          class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          Download setup script
                        </button>
                        <div class="text-xs text-muted">
                          1. Click to download the script
                          <br />
                          2. Upload to your server via SCP/SFTP
                          <br />
                          3. Run:{' '}
                          <code class="bg-surface-alt px-1 rounded">
                            bash &lt;downloaded-script&gt;
                          </code>
                        </div>
                      </div>
                    </details>
                  </div>

                  <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                    <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                      What this does:
                    </p>
                    <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>
                          Creates monitoring user{' '}
                          <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">
                            pulse-monitor@pve
                          </code>
                        </span>
                      </li>
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>Generates secure API token</span>
                      </li>
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>
                          Sets up monitoring permissions (PVEAuditor + guest agent access + backup
                          visibility)
                        </span>
                      </li>
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>Automatically registers node with Pulse</span>
                      </li>
                    </ul>
                    <p class="text-xs text-green-600 dark:text-green-400 mt-2 font-semibold">
                      Fully automatic: no manual token copying needed.
                    </p>
                  </div>
                </div>
              </Show>

              <Show when={state.formData().setupMode === 'manual'}>
                <div class="space-y-3">
                  <p class="text-blue-800 dark:text-blue-200 mb-2">
                    Advanced manual token setup. Run these commands one by one on your Proxmox VE
                    server, then paste the token into the fields below:
                  </p>

                  <div class="space-y-3">
                    <div>
                      <p class="text-sm font-medium text-base-content mb-1">
                        1. Create monitoring user:
                      </p>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                        <button
                          type="button"
                          onClick={async () => {
                            const command =
                              'pveum user add pulse-monitor@pve --comment "Pulse monitoring service"';
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          pveum user add pulse-monitor@pve --comment "Pulse monitoring service"
                        </code>
                      </div>
                    </div>

                    <div>
                      <p class="text-sm font-medium text-base-content mb-1">
                        2. Generate API token (save the output!):
                      </p>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                        <button
                          type="button"
                          onClick={async () => {
                            const command =
                              'pveum user token add pulse-monitor@pve pulse-token --privsep 0';
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          pveum user token add pulse-monitor@pve pulse-token --privsep 0
                        </code>
                      </div>
                      <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                        Important: Copy the token value immediately - it won't be shown again!
                      </p>
                    </div>

                    <div>
                      <p class="text-sm font-medium text-base-content mb-1">
                        3. Set up monitoring permissions:
                      </p>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs mb-1">
                        <button
                          type="button"
                          onClick={async () => {
                            await state.copyCommand(PVE_MANUAL_PERMISSION_COMMAND);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 hover:text-muted transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content whitespace-pre-line">
                          {PVE_MANUAL_PERMISSION_COMMAND}
                        </code>
                      </div>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                        <button
                          type="button"
                          onClick={async () => {
                            const command =
                              'pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin';
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 hover:text-muted transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin
                        </code>
                      </div>
                      <p class="text-muted text-xs mt-1">
                        Note: PVEAuditor gives read-only API access. PulseMonitor adds Sys.Audit
                        plus VM.GuestAgent.Audit/FileRead (PVE 9+) or VM.Monitor (PVE 8) for disk
                        and guest metrics. PVEDatastoreAdmin on /storage adds backup visibility.
                      </p>
                    </div>

                    <div class="bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded-md p-2">
                      <p class="text-sm font-medium text-green-900 dark:text-green-100 mb-1">
                        4. Add to Pulse with:
                      </p>
                      <ul class="text-xs text-green-800 dark:text-green-200 ml-4 list-disc">
                        <li>
                          <strong>Token ID:</strong> pulse-monitor@pve!pulse-token
                        </li>
                        <li>
                          <strong>Token Value:</strong> [The value from step 2]
                        </li>
                        <li>
                          <strong>Endpoint URL:</strong>{' '}
                          {state.formData().host || 'https://your-server:8006'}
                        </li>
                      </ul>
                    </div>
                  </div>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </Show>

      <Show when={modalProps.nodeType === 'pbs'}>
        <div class="space-y-3 text-xs">
          <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-4">
            <h5 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-3 flex items-center gap-2">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <path d="M12 6v6l4 2"></path>
              </svg>
              Connection Setup
            </h5>
            {setupStrategyPanel()}
            <Show when={setupHandoffDisabled()}>
              <p class="mt-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
                {setupHandoffDisabledReason()}
              </p>
            </Show>

            <div class="space-y-3 text-xs">
              {setupModeControls()}

              <Show when={state.formData().setupMode === 'agent'}>
                <div class="space-y-3">
                  <p class="text-xs text-muted">
                    Optional full host telemetry setup. This command creates the API token, installs
                    the Pulse Agent root service, registers the server, and leaves Pulse waiting for
                    the agent check-in:
                  </p>
                  <ul class="text-xs text-muted list-disc list-inside space-y-1">
                    <li>One-command setup (creates API user and token automatically)</li>
                    <li>
                      Host-local telemetry such as temperatures, SMART, services, and local disks
                    </li>
                    <li>Agent-driven commands and service telemetry when explicitly used</li>
                    <li>Automatic reconnection on network issues</li>
                  </ul>
                  <p class="text-blue-800 dark:text-blue-200 text-xs mt-3">
                    Run this command on your PBS node:
                  </p>
                  <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                    <button
                      type="button"
                      onClick={() =>
                        state.copyProxmoxAgentInstallCommand('pbs', 'Command copied to clipboard')
                      }
                      class="absolute top-2 right-2 p-1.5 text-slate-400 hover:text-white rounded bg-surface hover:bg-slate-700 transition-colors"
                      title={
                        setupHandoffDisabled() ? setupHandoffDisabledReason() : 'Copy to clipboard'
                      }
                      disabled={state.loadingAgentCommand() || setupHandoffDisabled()}
                    >
                      <Show
                        when={state.loadingAgentCommand()}
                        fallback={
                          <svg
                            xmlns="http://www.w3.org/2000/svg"
                            class="h-4 w-4"
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
                        }
                      >
                        <svg
                          class="animate-spin h-4 w-4"
                          xmlns="http://www.w3.org/2000/svg"
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
                          ></circle>
                          <path
                            class="opacity-75"
                            fill="currentColor"
                            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                          ></path>
                        </svg>
                      </Show>
                    </button>
                    <code class="block text-base-content whitespace-pre-wrap break-all pr-10">
                      {state.agentInstallCommand() ||
                        'Click the copy button to generate and copy the install command'}
                    </code>
                  </div>
                  <Show when={state.agentCommandError()}>
                    <p class="text-xs text-red-500">{state.agentCommandError()}</p>
                  </Show>
                  <p class="text-xs text-muted">
                    No token fields are needed here. The server appears in Pulse automatically after
                    the agent connects.
                  </p>
                </div>
              </Show>

              <Show when={state.formData().setupMode === 'auto'}>
                <div class="space-y-3">
                  <div class="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 dark:border-emerald-800 dark:bg-emerald-950/30">
                    <p class="text-xs text-emerald-800 dark:text-emerald-200">
                      <strong>Recommended API inventory path:</strong> this connects Pulse to
                      Proxmox Backup Server without installing a root agent. Add the host telemetry
                      agent later only where you need host-local telemetry or agent-driven
                      operations.
                    </p>
                  </div>
                  <p class="text-blue-800 dark:text-blue-200">
                    Just copy and run this one command on your Proxmox Backup Server:
                  </p>

                  <div class="space-y-3">
                    <div class="relative bg-base rounded-md p-3 font-mono text-xs overflow-x-auto">
                      <Show when={state.formData().host && state.formData().host.trim() !== ''}>
                        <button
                          type="button"
                          onClick={async () => {
                            await state.copyQuickSetupCommand(
                              'pbs',
                              false,
                              'Command copied to clipboard! Run it on the server; the one-time setup token is already embedded.',
                            );
                          }}
                          class="absolute top-2 right-2 p-1.5 text-slate-400 hover:text-slate-200 bg-surface-hover rounded-md transition-colors"
                          disabled={setupHandoffDisabled()}
                          title={
                            setupHandoffDisabled() ? setupHandoffDisabledReason() : 'Copy command'
                          }
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
                      </Show>
                      <Show
                        when={state.quickSetupPreviewCommand().length > 0}
                        fallback={
                          <code class="text-muted">
                            {state.formData().host
                              ? 'Click the copy button to generate the setup command'
                              : 'Please enter the Endpoint URL above first'}
                          </code>
                        }
                      >
                        <code class="block text-base-content whitespace-pre-wrap break-words">
                          {state.quickSetupPreviewCommand()}
                        </code>
                      </Show>
                      <Show when={state.quickSetupTokenHint().length > 0}>
                        <div class="mt-2 text-xs text-blue-800 dark:text-blue-200">
                          <span class="font-semibold">Setup token hint:</span>
                          <code class="ml-1 font-mono break-all text-blue-900 dark:text-blue-100">
                            {state.quickSetupTokenHint()}
                          </code>
                          <Show when={state.quickSetupExpiry()}>
                            <span class="ml-2">Expires at {state.quickSetupExpiryLabel()}</span>
                          </Show>
                        </div>
                      </Show>
                    </div>

                    <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-3">
                      <div class="flex items-start space-x-2">
                        <svg
                          class="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                          />
                        </svg>
                        <div class="text-xs text-amber-700 dark:text-amber-300">
                          <p class="font-semibold mb-1">If the command doesn't work:</p>
                          <p>
                            Your PBS server may not be able to reach Pulse. Use the alternative
                            method below.
                          </p>
                        </div>
                      </div>
                    </div>

                    <details class="bg-surface-alt rounded-md p-3">
                      <summary class="cursor-pointer text-sm font-medium text-base-content hover:text-base-content">
                        Alternative: Download script manually
                      </summary>
                      <div class="mt-3 space-y-3">
                        <button
                          type="button"
                          onClick={async () => {
                            await state.downloadProxmoxSetupScript('pbs');
                          }}
                          disabled={setupHandoffDisabled()}
                          title={setupHandoffDisabled() ? setupHandoffDisabledReason() : undefined}
                          class="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md transition-colors text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          Download setup script
                        </button>
                        <div class="text-xs text-muted">
                          1. Click to download the script
                          <br />
                          2. Upload to your PBS via SCP/SFTP
                          <br />
                          3. Run:{' '}
                          <code class="bg-surface-alt px-1 rounded">
                            bash &lt;downloaded-script&gt;
                          </code>
                        </div>
                      </div>
                    </details>
                  </div>

                  <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                    <p class="text-sm font-semibold text-blue-800 dark:text-blue-200 mb-2">
                      What this does:
                    </p>
                    <ul class="text-xs text-blue-700 dark:text-blue-300 space-y-1">
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>
                          Creates monitoring user{' '}
                          <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">
                            pulse-monitor@pbs
                          </code>
                        </span>
                      </li>
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>Generates secure API token</span>
                      </li>
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>
                          Sets up Audit permissions (read-only access to backups + system stats)
                        </span>
                      </li>
                      <li class="flex items-start">
                        <span class="text-emerald-400 mr-2 mt-0.5">✓</span>
                        <span>Automatically registers server with Pulse</span>
                      </li>
                    </ul>
                    <p class="text-xs text-green-600 dark:text-green-400 mt-2 font-semibold">
                      Fully automatic: no manual token copying needed.
                    </p>
                  </div>
                </div>
              </Show>

              <Show when={state.formData().setupMode === 'manual'}>
                <div class="space-y-3">
                  <p class="text-blue-800 dark:text-blue-200 mb-2">
                    Advanced manual token setup. Run these commands one by one on your Proxmox
                    Backup Server, then paste the token into the fields below:
                  </p>

                  <div class="space-y-3">
                    <div>
                      <p class="text-sm font-medium text-base-content mb-1">
                        1. Create monitoring user:
                      </p>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                        <button
                          type="button"
                          onClick={async () => {
                            const command = 'proxmox-backup-manager user create pulse-monitor@pbs';
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          proxmox-backup-manager user create pulse-monitor@pbs
                        </code>
                      </div>
                    </div>

                    <div>
                      <p class="text-sm font-medium text-base-content mb-1">
                        2. Generate API token (save the output!):
                      </p>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                        <button
                          type="button"
                          onClick={async () => {
                            const command =
                              'proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token';
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 text-slate-500 hover:text-base-content transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token
                        </code>
                      </div>
                      <p class="text-amber-600 dark:text-amber-400 text-xs mt-1">
                        Copy the token value immediately - it won't be shown again!
                      </p>
                    </div>

                    <div>
                      <p class="text-sm font-medium text-base-content mb-1">
                        3. Set up read-only permissions (includes system stats):
                      </p>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs mb-1">
                        <button
                          type="button"
                          onClick={async () => {
                            const command =
                              'proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs';
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 hover:text-muted transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs
                        </code>
                      </div>
                      <div class="relative bg-surface rounded-md p-2 font-mono text-xs">
                        <button
                          type="button"
                          onClick={async () => {
                            const command =
                              "proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'";
                            await state.copyCommand(command);
                          }}
                          disabled={setupHandoffDisabled()}
                          class={setupCommandButtonClass(
                            'absolute top-1 right-1 p-1 hover:text-muted transition-colors',
                          )}
                          title={setupCommandButtonTitle()}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"></path>
                          </svg>
                        </button>
                        <code class="text-base-content">
                          proxmox-backup-manager acl update / Audit --auth-id
                          'pulse-monitor@pbs!pulse-token'
                        </code>
                      </div>
                    </div>

                    <div class="bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 rounded-md p-2">
                      <p class="text-sm font-medium text-green-900 dark:text-green-100 mb-1">
                        4. Add to Pulse with:
                      </p>
                      <ul class="text-xs text-green-800 dark:text-green-200 ml-4 list-disc">
                        <li>
                          <strong>Token ID:</strong> pulse-monitor@pbs!pulse-token
                        </li>
                        <li>
                          <strong>Token Value:</strong> [The value from step 2]
                        </li>
                        <li>
                          <strong>Endpoint URL:</strong>{' '}
                          {state.formData().host || 'https://your-server:8007'}
                        </li>
                      </ul>
                    </div>

                    <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-2 mt-3">
                      <p class="text-xs font-semibold text-amber-800 dark:text-amber-200 mb-1">
                        About PBS Permissions:
                      </p>
                      <ul class="text-xs text-amber-700 dark:text-amber-300 space-y-0.5">
                        <li>
                          <strong>Basic (DatastoreAudit):</strong> View backups only
                        </li>
                        <li>
                          <strong>Enhanced (Audit on /):</strong> View backups + CPU/memory/uptime
                          stats
                        </li>
                        <li class="text-amber-600 dark:text-amber-400">
                          → We use Enhanced for better monitoring visibility
                        </li>
                      </ul>
                    </div>
                  </div>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </Show>

      <Show when={modalProps.nodeType === 'pmg'}>
        <div class="space-y-3 text-xs text-base-content">
          <p>
            Generate a dedicated API token in <strong>Configuration → API Tokens</strong> on your
            Mail Gateway. We recommend creating a service user such as{' '}
            <code class="font-mono">pulse-monitor@pmg</code>
            with <em>Auditor</em> privileges.
          </p>
          <ol class="list-decimal ml-4 space-y-1">
            <li>
              Click <em>Add</em> and choose the service user (or create one if needed).
            </li>
            <li>
              Enable <em>Privilege Separation</em> and assign the <em>Auditor</em> role.
            </li>
            <li>
              Copy the generated Token ID (e.g.{' '}
              <code class="font-mono">pulse-monitor@pmg!pulse-edge</code>) and the secret value into
              the fields below.
            </li>
          </ol>
          <p class="text-xs text-muted">
            Pulse only requires read-only access. Avoid granting administrator permissions to the
            token.
          </p>
        </div>
      </Show>
    </div>
  );
};
