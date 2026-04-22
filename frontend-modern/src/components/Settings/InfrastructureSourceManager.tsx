import { Component, For, Show, type Accessor } from 'solid-js';
import { Archive, Cpu, Database, Mail, Search, Server, ServerCog } from 'lucide-solid';
import type { Connection } from '@/api/connections';
import SettingsPanel from '@/components/shared/SettingsPanel';
import type { InfrastructureSystemRow } from './connectionsTableModel';
import type { AgentUninstallCommands } from './ConnectionsTable';
import type { ConnectionRowActions } from './useConnectionRowActions';
import {
  getInfrastructureOnboardingProductPresentation,
  type InfrastructureOnboardingConnectionType,
} from '@/utils/infrastructureOnboardingPresentation';

interface InfrastructureSourceManagerProps {
  rows: Accessor<readonly InfrastructureSystemRow[]>;
  readOnly: boolean;
  actions?: ConnectionRowActions;
  onAddType: (type: InfrastructureOnboardingConnectionType) => void;
  onEditConnection?: (connection: Connection) => void;
  onDetectFromAddress?: () => void;
  agentUninstallCommands?: AgentUninstallCommands;
  onCopyText?: (text: string) => void;
}

const buttonClass =
  'inline-flex items-center rounded-md border border-border px-2.5 py-1.5 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex items-center rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';
const removeButtonClass =
  'inline-flex items-center rounded-md border border-rose-300 px-2.5 py-1.5 text-xs font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950';
const removeConfirmClass =
  'inline-flex items-center rounded-md bg-rose-600 px-2.5 py-1.5 text-xs font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60';

const CARD_ORDER: InfrastructureOnboardingConnectionType[] = [
  'vmware',
  'truenas',
  'pve',
  'pbs',
  'pmg',
  'agent',
];

const CARD_ICON: Record<InfrastructureOnboardingConnectionType, Component<{ class?: string }>> = {
  vmware: ServerCog,
  truenas: Database,
  pve: Server,
  pbs: Archive,
  pmg: Mail,
  agent: Cpu,
};

const EMPTY_STATE_LABEL: Record<InfrastructureOnboardingConnectionType, string> = {
  vmware: 'No VMware vCenter connections yet.',
  truenas: 'No TrueNAS SCALE connections yet.',
  pve: 'No Proxmox VE connections yet.',
  pbs: 'No Proxmox Backup Server connections yet.',
  pmg: 'No Proxmox Mail Gateway connections yet.',
  agent: 'No Pulse Agent hosts connected yet.',
};

const ACTION_LABEL: Record<InfrastructureOnboardingConnectionType, string> = {
  vmware: 'Add VMware vCenter',
  truenas: 'Add TrueNAS',
  pve: 'Add Proxmox VE',
  pbs: 'Add Proxmox Backup Server',
  pmg: 'Add Proxmox Mail Gateway',
  agent: 'Install agent',
};

const pathBadge = (type: InfrastructureOnboardingConnectionType): string =>
  type === 'agent' ? 'Agent path' : 'Platform API';

const additionalBadge = (type: InfrastructureOnboardingConnectionType): string | null => {
  if (type === 'vmware') return 'Available now';
  if (type === 'agent') return 'Docker + Kubernetes';
  return null;
};

const additionalNote = (type: InfrastructureOnboardingConnectionType): string | null => {
  if (type === 'agent') {
    return 'Docker and Kubernetes are discovered from the host after the agent is installed.';
  }
  return null;
};

const confirmHelpText = (isAgent: boolean): string =>
  isAgent
    ? 'Removing forgets this agent from Pulse. Uninstall it on the host as well if you want to detach it completely.'
    : 'Removing forgets this connection from Pulse; credentials on the platform itself are untouched.';

export const InfrastructureSourceManager: Component<InfrastructureSourceManagerProps> = (props) => {
  const rowsForType = (type: InfrastructureOnboardingConnectionType) =>
    props.rows().filter((row) => row.connection.type === type);

  return (
    <SettingsPanel
      title="Connection types"
      description="Add and manage infrastructure sources by product type. Existing sources stay visible here while add and edit open in-place dialogs."
      icon={<Server class="h-5 w-5" strokeWidth={2} />}
      action={
        <Show when={!props.readOnly && props.onDetectFromAddress}>
          <button type="button" onClick={props.onDetectFromAddress} class={buttonClass}>
            <Search class="mr-1.5 h-3.5 w-3.5" />
            Detect from address
          </button>
        </Show>
      }
    >
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <For each={CARD_ORDER}>
          {(type) => {
            const presentation = getInfrastructureOnboardingProductPresentation(type);
            const Icon = CARD_ICON[type];
            const groupRows = () => rowsForType(type);
            const secondaryBadge = () => additionalBadge(type);
            const note = () => additionalNote(type);

            return (
              <div class="rounded-xl border border-border bg-surface-alt p-4">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="flex min-w-0 items-start gap-3">
                    <div class="flex h-10 w-10 flex-none items-center justify-center rounded-md border border-border bg-surface text-base-content">
                      <Icon class="h-5 w-5" />
                    </div>
                    <div class="min-w-0 space-y-1">
                      <div class="flex flex-wrap items-center gap-2">
                        <h3 class="text-sm font-semibold text-base-content">{presentation.label}</h3>
                        <span class="inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted">
                          {pathBadge(type)}
                        </span>
                        <Show when={secondaryBadge()}>
                          {(badge) => (
                            <span class="inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200">
                              {badge()}
                            </span>
                          )}
                        </Show>
                      </div>
                      <p class="text-xs text-muted">{presentation.bestFor}</p>
                    </div>
                  </div>

                  <div class="flex items-center gap-2">
                    <span class="rounded-full border border-border bg-surface px-2 py-0.5 text-[11px] font-medium text-base-content">
                      {groupRows().length} configured
                    </span>
                    <Show when={!props.readOnly}>
                      <button
                        type="button"
                        onClick={() => props.onAddType(type)}
                        class={primaryButtonClass}
                      >
                        {ACTION_LABEL[type]}
                      </button>
                    </Show>
                  </div>
                </div>

                <p class="mt-3 text-xs text-muted">{presentation.coverage}</p>
                <Show when={note()}>
                  {(value) => <p class="mt-2 text-xs text-muted">{value()}</p>}
                </Show>

                <div class="mt-4 space-y-2">
                  <Show
                    when={groupRows().length > 0}
                    fallback={
                      <div class="rounded-lg border border-dashed border-border bg-surface px-3 py-4 text-sm text-muted">
                        {EMPTY_STATE_LABEL[type]}
                      </div>
                    }
                  >
                    <For each={groupRows()}>
                      {(row) => {
                        const pauseBusy = () => props.actions?.pendingAction(row.id) === 'pause';
                        const removeBusy = () => props.actions?.pendingAction(row.id) === 'remove';
                        const anyBusy = () => props.actions?.pendingAction(row.id) !== null;
                        const removeConfirming = () => Boolean(props.actions?.confirmingRemove(row.id));
                        const rowError = () => props.actions?.actionError(row.id) ?? null;

                        return (
                          <div class="rounded-lg border border-border bg-surface px-3 py-3">
                            <div class="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                              <div class="min-w-0 flex-1 space-y-1">
                                <div class="flex flex-wrap items-center gap-2">
                                  <div class="min-w-0 truncate text-sm font-medium text-base-content">
                                    {row.name}
                                  </div>
                                  <span
                                    class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${row.statusClassName}`}
                                  >
                                    {row.statusLabel}
                                  </span>
                                </div>
                                <Show when={row.host}>
                                  <div class="break-words text-xs text-muted">{row.host}</div>
                                </Show>
                                <Show when={row.subtitle}>
                                  <div class="break-words text-xs text-muted">{row.subtitle}</div>
                                </Show>
                                <div class="text-xs text-muted">Last activity: {row.lastActivityText}</div>
                                <Show when={row.lastErrorMessage}>
                                  <div class="break-words text-xs text-rose-700 dark:text-rose-300">
                                    {row.lastErrorMessage}
                                  </div>
                                </Show>
                              </div>

                              <Show when={!props.readOnly && props.actions}>
                                <div class="flex flex-wrap items-center gap-1.5">
                                  <Show when={row.canEdit && props.onEditConnection}>
                                    <button
                                      type="button"
                                      disabled={anyBusy()}
                                      onClick={() => props.onEditConnection?.(row.connection)}
                                      class={buttonClass}
                                    >
                                      Edit
                                    </button>
                                  </Show>
                                  <Show when={row.canPause}>
                                    <button
                                      type="button"
                                      disabled={anyBusy()}
                                      onClick={() => void props.actions?.togglePause(row.connection)}
                                      class={buttonClass}
                                    >
                                      {pauseBusy()
                                        ? 'Working…'
                                        : row.enabled
                                          ? 'Pause'
                                          : 'Resume'}
                                    </button>
                                  </Show>
                                  <Show when={row.canRemove}>
                                    <button
                                      type="button"
                                      disabled={anyBusy()}
                                      onClick={() => void props.actions?.requestRemove(row.connection)}
                                      class={removeConfirming() ? removeConfirmClass : removeButtonClass}
                                    >
                                      {removeBusy()
                                        ? 'Removing…'
                                        : removeConfirming()
                                          ? 'Click again to confirm'
                                          : 'Remove'}
                                    </button>
                                  </Show>
                                </div>
                              </Show>
                            </div>

                            <Show when={removeConfirming()}>
                              <div class="mt-3 space-y-2 rounded-md border border-border bg-surface-alt px-3 py-2">
                                <p class="text-xs text-muted">{confirmHelpText(row.isAgent)}</p>
                                <Show when={row.isAgent && props.agentUninstallCommands}>
                                  <div class="space-y-2">
                                    <div class="rounded-md border border-border bg-surface px-3 py-2">
                                      <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                                        Linux / macOS / FreeBSD
                                      </div>
                                      <div class="mt-1 break-all font-mono text-[11px] text-base-content">
                                        {props.agentUninstallCommands!.linux}
                                      </div>
                                      <Show when={props.onCopyText}>
                                        <button
                                          type="button"
                                          class={`${buttonClass} mt-2`}
                                          onClick={() =>
                                            props.onCopyText?.(props.agentUninstallCommands!.linux)
                                          }
                                        >
                                          Copy uninstall command
                                        </button>
                                      </Show>
                                    </div>
                                  </div>
                                </Show>
                              </div>
                            </Show>

                            <Show when={rowError()}>
                              <div
                                role="alert"
                                class="mt-3 rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                              >
                                {rowError()}
                              </div>
                            </Show>
                          </div>
                        );
                      }}
                    </For>
                  </Show>
                </div>
              </div>
            );
          }}
        </For>
      </div>
    </SettingsPanel>
  );
};

export default InfrastructureSourceManager;
