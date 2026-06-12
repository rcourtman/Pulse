import { Component, Match, Show, Switch, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import X from 'lucide-solid/icons/x';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { hasFeature, runtimeCapabilitiesLoaded } from '@/stores/license';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import { updateStore } from '@/stores/updates';
import { Dialog } from '@/components/shared/Dialog';
import { Button } from '@/components/shared/Button';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ConnectionEditor } from './ConnectionEditor/ConnectionEditor';
import { NodeCredentialSlot } from './ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import { TrueNASCredentialSlot } from './ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { VMwareCredentialSlot } from './ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import type { Connection, ConnectionType, ProbeCandidate } from '@/api/connections';
import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import { InfrastructureDiscoverySettingsDialog } from './InfrastructureDiscoverySettingsDialog';
import { InfrastructureAgentUpdatesDialog } from './InfrastructureAgentUpdatesDialog';
import {
  InfrastructureInstallerSection,
  type InfrastructureInstallerFocus,
} from './InfrastructureInstallerSection';
import { InfrastructureSourceManager } from './InfrastructureSourceManager';
import { InfrastructureSourcePicker } from './InfrastructureSourcePicker';
import {
  connectionAgentIdentitySummary,
  connectionAgentVersionPresentation,
  connectionLastActivityText,
  type InfrastructureSystemRow,
} from './connectionsTableModel';
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
  deriveAgentUpdateScopeFromLocation,
  deriveAgentUpdatesFromLocation,
  deriveAddStepFromLocation,
  type InfrastructureAddStep,
  type InfrastructurePanelStep,
} from './infrastructureWorkspaceModel';
import { collectInfrastructureAgentUpdateTargets } from './infrastructureAgentUpdateCommandsModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import { useConnectionsLedger } from './useConnectionsLedger';
import { useConnectionRowActions } from './useConnectionRowActions';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from './useInfrastructureOperationsState';
import {
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSourcePickerItemForRouteStep,
  getInfrastructureSourceStrategyPresentation,
  type InfrastructureOnboardingConnectionType,
} from '@/utils/infrastructureOnboardingPresentation';
import {
  filterRepresentedDiscoveredServers,
  type DiscoveredServer,
} from './infrastructureSettingsModel';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

type ManagedAddTypeStep = Exclude<InfrastructureAddStep, 'detect'>;

interface AgentUninstallCommands {
  linux: string;
  windows: string;
}

const ADD_STEP_TO_TYPE: Record<ManagedAddTypeStep, ConnectionType> = {
  agent: 'agent',
  'linux-host': 'agent',
  unraid: 'agent',
  docker: 'agent',
  kubernetes: 'agent',
  pve: 'pve',
  pbs: 'pbs',
  pmg: 'pmg',
  truenas: 'truenas',
  vmware: 'vmware',
};

const ADD_STEP_TO_INSTALLER_FOCUS: Partial<
  Record<ManagedAddTypeStep, InfrastructureInstallerFocus>
> = {
  agent: 'agent',
  'linux-host': 'linux-host',
  unraid: 'unraid',
  docker: 'docker',
  kubernetes: 'kubernetes',
};

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';

const trimSentenceTerminal = (value: string): string => value.trim().replace(/\.+$/, '');

const describeManagedSourceType = (type: ConnectionType | null): string => {
  if (!type) return 'Infrastructure';
  if (
    type === 'agent' ||
    type === 'vmware' ||
    type === 'truenas' ||
    type === 'pve' ||
    type === 'pbs' ||
    type === 'pmg'
  ) {
    return getInfrastructureOnboardingProductPresentation(
      type as InfrastructureOnboardingConnectionType,
    ).label;
  }
  if (type === 'docker') return 'Docker';
  if (type === 'kubernetes') return 'Kubernetes';
  return type;
};

const InfrastructureWorkspaceContent: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const ledger = useConnectionsLedger();
  const operations = useInfrastructureOperationsContext();
  const rowActions = useConnectionRowActions({ onMutated: () => ledger.reload() });

  const [showAgentProfiles, setShowAgentProfiles] = createSignal(false);
  const [editingRow, setEditingRow] = createSignal<InfrastructureSystemRow | null>(null);
  const [showDiscoverySettings, setShowDiscoverySettings] = createSignal(false);
  const [selectedDiscoveredSource, setSelectedDiscoveredSource] =
    createSignal<DiscoveredServer | null>(null);
  const [selectedProbeCandidate, setSelectedProbeCandidate] = createSignal<ProbeCandidate | null>(
    null,
  );
  const readOnly = createMemo(() => presentationPolicyIsReadOnly());
  const routeStep = createMemo<InfrastructurePanelStep | null>(() => {
    if (readOnly()) return null;
    return deriveAddStepFromLocation(location.pathname, location.search ?? '');
  });
  const showAgentUpdateCommands = createMemo(
    () =>
      !readOnly() &&
      deriveAgentUpdatesFromLocation(location.pathname, location.search ?? '') &&
      routeStep() === null,
  );
  const agentUpdateScope = createMemo(() =>
    deriveAgentUpdateScopeFromLocation(location.pathname, location.search ?? ''),
  );
  const activeAddType = createMemo<ConnectionType | null>(() => {
    const step = routeStep();
    if (!step || step === 'pick' || step === 'detect') return null;
    return ADD_STEP_TO_TYPE[step as ManagedAddTypeStep];
  });
  const activeCatalogItem = createMemo(() =>
    getInfrastructureSourcePickerItemForRouteStep(routeStep()),
  );
  const activeInstallerFocus = createMemo<InfrastructureInstallerFocus>(() => {
    const step = routeStep();
    if (!step || step === 'pick' || step === 'detect') return 'agent';
    return ADD_STEP_TO_INSTALLER_FOCUS[step as ManagedAddTypeStep] ?? 'agent';
  });
  const editingConnection = createMemo<Connection | null>(() => editingRow()?.connection ?? null);
  const attachedAgentConnections = createMemo(() =>
    (editingRow()?.attachedConnections ?? []).filter((connection) => connection.type === 'agent'),
  );

  const findEditableNode = (connection: Connection): NodeConfigWithStatus | null => {
    const accessor =
      connection.type === 'pve'
        ? props.pveNodes
        : connection.type === 'pbs'
          ? props.pbsNodes
          : connection.type === 'pmg'
            ? props.pmgNodes
            : null;
    if (!accessor) return null;
    return accessor().find((node) => node.name === connection.name) ?? null;
  };

  const aggregatorSuffix = (id: string): string => {
    const colonIndex = id.indexOf(':');
    return colonIndex === -1 ? id : id.slice(colonIndex + 1);
  };

  const findEditableVMware = (connection: Connection): VMwareConnection | null => {
    const suffix = aggregatorSuffix(connection.id);
    return props.vmwareSettings.connections().find((conn) => conn.id === suffix) ?? null;
  };

  const findEditableTrueNAS = (connection: Connection): TrueNASConnection | null => {
    const suffix = aggregatorSuffix(connection.id);
    return props.trueNASSettings.connections().find((conn) => conn.id === suffix) ?? null;
  };

  const rows = createMemo(() => ledger.rows());
  const agentUpdateTargets = createMemo(() =>
    collectInfrastructureAgentUpdateTargets(
      rows(),
      updateStore.versionInfo()?.agentUpdateTargetVersion,
      agentUpdateScope(),
    ),
  );
  const visibleDiscoveredNodes = createMemo(() =>
    filterRepresentedDiscoveredServers(
      props.discoveredNodes(),
      [...props.pveNodes(), ...props.pbsNodes(), ...props.pmgNodes()],
      rows(),
    ),
  );

  const agentUninstallCommands = createMemo<AgentUninstallCommands>(() => ({
    linux: operations.getUninstallCommand(),
    windows: operations.getWindowsUninstallCommand(),
  }));

  const handleCopy = async (text: string) => {
    const ok = await copyToClipboard(text);
    if (ok) {
      notificationStore.success('Copied to clipboard');
    } else {
      notificationStore.error('Copy failed');
    }
  };

  const navigateToWorkspace = (replace = false) => {
    navigate(buildInfrastructureWorkspacePath(), { replace, scroll: false });
  };

  const resetInlineEditorState = () => {
    props.vmwareSettings.closeDialog();
    props.trueNASSettings.closeDialog();
    setShowAgentProfiles(false);
  };

  const openAddFlow = (step: InfrastructurePanelStep) => {
    setSelectedDiscoveredSource(null);
    setSelectedProbeCandidate(null);
    navigate(buildInfrastructureOnboardingPath(step), { scroll: false });
  };

  const openAddFlowFromProbe = (candidate: ProbeCandidate) => {
    setSelectedDiscoveredSource(null);
    setSelectedProbeCandidate(candidate);
    navigate(buildInfrastructureOnboardingPath(candidate.type as ManagedAddTypeStep), {
      scroll: false,
    });
  };

  const reviewDiscoveredSource = (server: DiscoveredServer) => {
    setSelectedDiscoveredSource(server);
    navigate(buildInfrastructureOnboardingPath(server.type), { scroll: false });
  };

  const closeAddFlow = () => {
    resetInlineEditorState();
    setSelectedDiscoveredSource(null);
    setSelectedProbeCandidate(null);
    navigateToWorkspace(Boolean(routeStep()));
  };

  const closeAgentUpdateCommands = () => {
    navigateToWorkspace(Boolean(showAgentUpdateCommands()));
  };

  const closeEditFlow = () => {
    const connection = editingConnection();
    if (connection) {
      rowActions.cancelRemove(connection.id);
    }
    for (const attached of attachedAgentConnections()) {
      rowActions.cancelRemove(attached.id);
    }
    resetInlineEditorState();
    setEditingRow(null);
  };

  const handleAddSaved = () => {
    ledger.reload();
    void props.loadDiscoveredNodes();
    closeAddFlow();
  };

  const handleEditSaved = () => {
    ledger.reload();
    closeEditFlow();
  };

  createEffect(() => {
    if (activeAddType() !== 'agent') {
      setShowAgentProfiles(false);
    }
  });

  createEffect(() => {
    if (!editingConnection()) return;
    if (ledger.loading()) return;
    if (!ledger.findById(editingConnection()!.id)) {
      closeEditFlow();
    }
  });

  createEffect(() => {
    if (readOnly() && editingConnection()) {
      closeEditFlow();
    }
  });

  createEffect(() => {
    if (readOnly() && showDiscoverySettings()) {
      setShowDiscoverySettings(false);
    }
  });

  createEffect(() => {
    const selected = selectedDiscoveredSource();
    const activeType = activeAddType();
    if (selected && activeType && activeType !== selected.type) {
      setSelectedDiscoveredSource(null);
    }
  });

  const renderNodeSlot = (
    type: 'pve' | 'pbs' | 'pmg',
    editingNode?: NodeConfigWithStatus | null,
    editingRow?: Connection | null,
    prefillNode?: Partial<NodeConfig>,
  ) => {
    const connectionId = editingRow?.id ?? null;
    return (
      <NodeCredentialSlot
        nodeType={type}
        settings={props}
        editingNode={editingNode ?? null}
        prefillNode={prefillNode}
        onCancel={editingRow ? closeEditFlow : closeAddFlow}
        onSaved={editingRow ? handleEditSaved : handleAddSaved}
        onToggleEnabled={
          editingRow
            ? () => {
                void rowActions.togglePause(editingRow);
              }
            : undefined
        }
        togglePending={connectionId ? rowActions.pendingAction(connectionId) === 'pause' : false}
        connectionEnabled={editingRow?.enabled}
        onDelete={
          editingRow
            ? () => {
                void rowActions.requestRemove(editingRow);
              }
            : undefined
        }
        deletePending={connectionId ? rowActions.pendingAction(connectionId) === 'remove' : false}
        deleteConfirming={connectionId ? rowActions.confirmingRemove(connectionId) : false}
        deleteError={connectionId ? rowActions.actionError(connectionId) : null}
      />
    );
  };

  const renderAgentAddSlot = () => (
    <div class="space-y-4">
      <Show when={runtimeCapabilitiesLoaded() && hasFeature('agent_profiles')}>
        <div class="flex items-center justify-end">
          <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={() => setShowAgentProfiles((value) => !value)}
          >
            {showAgentProfiles() ? 'Hide agent profiles' : 'Manage agent profiles'}
          </Button>
        </div>
      </Show>
      <Show when={showAgentProfiles()}>
        <div class="rounded-xl border border-border bg-surface p-4 shadow-sm">
          <div class="mb-4 space-y-1">
            <div class="text-base font-semibold text-base-content">Agent profiles</div>
            <div class="text-sm text-muted">
              Manage reusable install defaults for agent-based systems.
            </div>
          </div>
          <AgentProfilesPanel />
        </div>
      </Show>
      <InfrastructureInstallerSection focus={activeInstallerFocus()} />
    </div>
  );

  const renderAgentConnectionDetails = (connection: Connection) => {
    const removePending = () => rowActions.pendingAction(connection.id) === 'remove';
    const removeConfirming = () => rowActions.confirmingRemove(connection.id);
    const error = () => rowActions.actionError(connection.id);
    const versionPresentation = () => connectionAgentVersionPresentation(connection);
    const identitySummary = () => connectionAgentIdentitySummary(connection);
    const infoCard = (label: string, value: string) => (
      <div class="rounded-lg border border-border bg-surface-alt px-3 py-3">
        <div class="text-[11px] font-medium uppercase tracking-wide text-muted">{label}</div>
        <div class="mt-1 text-sm text-base-content">{value}</div>
      </div>
    );

    return (
      <div class="space-y-6 p-4">
        <section class="rounded-xl border border-border bg-surface p-4">
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {infoCard('Connection state', connection.state)}
            {infoCard('Last seen', connection.lastSeen ? connection.lastSeen : 'No activity yet')}
            <Show when={versionPresentation()}>
              {(presentation) => (
                <div class="rounded-lg border border-border bg-surface-alt px-3 py-3">
                  <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                    Pulse Agent version
                  </div>
                  <div class="mt-2 flex flex-wrap items-center gap-2">
                    <span class={presentation().badgeClassName}>{presentation().badgeLabel}</span>
                    <span class="text-sm text-base-content" title={presentation().title}>
                      {presentation().detail}
                    </span>
                  </div>
                </div>
              )}
            </Show>
            <Show when={identitySummary()}>
              {(summary) => infoCard('Operating system', summary())}
            </Show>
            <Show when={connection.agentIdentity?.hostname?.trim()}>
              {(hostname) => infoCard('Reported hostname', hostname())}
            </Show>
            <Show when={connection.agentIdentity?.reportIp?.trim()}>
              {(reportIp) => infoCard('Reported IP', reportIp())}
            </Show>
            <Show when={connection.agentIdentity?.kernelVersion?.trim()}>
              {(kernelVersion) => infoCard('Kernel', kernelVersion())}
            </Show>
            <Show when={connection.agentIdentity?.architecture?.trim()}>
              {(architecture) => infoCard('Architecture', architecture())}
            </Show>
            <Show when={connection.agentIdentity}>
              {infoCard(
                'Remote commands',
                connection.agentIdentity?.commandsEnabled ? 'Enabled' : 'Not enabled',
              )}
            </Show>
          </div>

          <Show when={connection.lastError?.message}>
            <div class="mt-4 rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200">
              {connection.lastError?.message}
            </div>
          </Show>
        </section>

        <section class="rounded-xl border border-border bg-surface p-4">
          <div class="space-y-3">
            <div class="space-y-1">
              <div class="text-sm font-semibold text-base-content">Uninstall commands</div>
              <p class="text-xs text-muted">
                Removing the source forgets it from Pulse. Uninstall the agent on the host as well
                if you want to detach it completely.
              </p>
            </div>

            <div class="grid gap-3 lg:grid-cols-2">
              <div class="rounded-lg border border-border bg-surface-alt px-3 py-3">
                <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                  Linux / macOS / FreeBSD
                </div>
                <div class="mt-1 line-clamp-3 break-all font-mono text-[11px] text-base-content">
                  {agentUninstallCommands().linux}
                </div>
                <button
                  type="button"
                  onClick={() => void handleCopy(agentUninstallCommands().linux)}
                  class={`${buttonClass} mt-3`}
                >
                  Copy uninstall command
                </button>
              </div>

              <div class="rounded-lg border border-border bg-surface-alt px-3 py-3">
                <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                  Windows PowerShell
                </div>
                <div class="mt-1 line-clamp-3 break-all font-mono text-[11px] text-base-content">
                  {agentUninstallCommands().windows}
                </div>
                <button
                  type="button"
                  onClick={() => void handleCopy(agentUninstallCommands().windows)}
                  class={`${buttonClass} mt-3`}
                >
                  Copy uninstall command
                </button>
              </div>
            </div>
          </div>
        </section>

        <Show when={removeConfirming()}>
          <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-xs text-muted">
            Click remove again to confirm. Pulse will forget this agent, but it will still be
            installed on the host until you run the uninstall command.
          </div>
        </Show>

        <Show when={error()}>
          {(message) => (
            <div
              role="alert"
              class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
            >
              {message()}
            </div>
          )}
        </Show>

        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={() => void rowActions.requestRemove(connection)}
            disabled={removePending()}
            class={
              removeConfirming()
                ? 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60'
                : 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950'
            }
          >
            {removePending()
              ? 'Removing…'
              : removeConfirming()
                ? 'Click again to confirm'
                : 'Remove source'}
          </button>
        </div>
      </div>
    );
  };

  const renderAttachedAgentAugmentations = (connections: readonly Connection[]) => {
    if (connections.length === 0) return null;

    return (
      <section class="space-y-4 rounded-xl border border-border bg-surface p-4">
        <div class="space-y-1">
          <div class="text-sm font-semibold text-base-content">Pulse Agent augmentation</div>
          <p class="text-xs text-muted">
            This source is also collecting local host telemetry through the Pulse Agent. You can
            remove the agent attachment here without removing the primary platform connection.
          </p>
        </div>

        <div class="space-y-3">
          {connections.map((connection) => {
            const removePending = () => rowActions.pendingAction(connection.id) === 'remove';
            const removeConfirming = () => rowActions.confirmingRemove(connection.id);
            const error = () => rowActions.actionError(connection.id);
            const versionPresentation = () => connectionAgentVersionPresentation(connection);

            return (
              <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
                <div class="flex flex-wrap items-start justify-between gap-4">
                  <div class="space-y-1">
                    <div class="text-sm font-medium text-base-content">{connection.name}</div>
                    <Show when={connection.address && connection.address !== connection.name}>
                      <div class="break-words text-xs text-muted">{connection.address}</div>
                    </Show>
                    <Show when={versionPresentation()}>
                      {(presentation) => (
                        <div class="flex flex-wrap items-center gap-2 pt-1">
                          <span class={presentation().badgeClassName}>
                            {presentation().badgeLabel}
                          </span>
                          <span class="text-xs text-muted" title={presentation().title}>
                            {presentation().detail}
                          </span>
                        </div>
                      )}
                    </Show>
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-[11px] font-medium text-green-800 dark:bg-green-900 dark:text-green-300">
                      {connection.state === 'active' ? 'Active' : connection.state}
                    </span>
                    <span class="text-xs text-muted">{connectionLastActivityText(connection)}</span>
                  </div>
                </div>

                <Show when={error()}>
                  {(message) => (
                    <div
                      role="alert"
                      class="mt-3 rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                    >
                      {message()}
                    </div>
                  )}
                </Show>

                <div class="mt-4 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                  <button
                    type="button"
                    onClick={() => void handleCopy(agentUninstallCommands().linux)}
                    class={buttonClass}
                  >
                    Copy uninstall command
                  </button>
                  <button
                    type="button"
                    onClick={() => void rowActions.requestRemove(connection)}
                    disabled={removePending()}
                    class={
                      removeConfirming()
                        ? 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60'
                        : 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950'
                    }
                  >
                    {removePending()
                      ? 'Removing…'
                      : removeConfirming()
                        ? 'Click again to confirm'
                        : 'Remove Pulse Agent'}
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      </section>
    );
  };

  const renderConnectionSlot = (context: {
    mode: 'add' | 'edit';
    type: ConnectionType;
    onCancel: () => void;
    onSaved: () => void;
  }) => {
    if (context.mode === 'edit') {
      const connection = editingConnection();
      if (!connection) {
        return (
          <div
            role="alert"
            class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
          >
            The selected connection is no longer available. Reload and try again.
          </div>
        );
      }

      switch (context.type) {
        case 'pve':
        case 'pbs':
        case 'pmg': {
          const editableNode = findEditableNode(connection);
          if (!editableNode) {
            return (
              <div
                role="alert"
                class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
              >
                Couldn't find the saved configuration for {connection.name}. It may have already
                been removed.
              </div>
            );
          }
          return renderNodeSlot(context.type, editableNode, connection);
        }
        case 'vmware': {
          const vmware = findEditableVMware(connection);
          if (!vmware) {
            return (
              <div
                role="alert"
                class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
              >
                Couldn't find the saved VMware configuration for {connection.name}. It may have
                already been removed.
              </div>
            );
          }
          return (
            <VMwareCredentialSlot
              state={props.vmwareSettings}
              editingConnection={vmware}
              onCancel={context.onCancel}
              onSaved={context.onSaved}
              onToggleEnabled={() => void rowActions.togglePause(connection)}
              togglePending={rowActions.pendingAction(connection.id) === 'pause'}
              connectionEnabled={connection.enabled}
              onDelete={() => void rowActions.requestRemove(connection)}
              deletePending={rowActions.pendingAction(connection.id) === 'remove'}
              deleteConfirming={rowActions.confirmingRemove(connection.id)}
              deleteError={rowActions.actionError(connection.id)}
            />
          );
        }
        case 'truenas': {
          const truenas = findEditableTrueNAS(connection);
          if (!truenas) {
            return (
              <div
                role="alert"
                class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
              >
                Couldn't find the saved TrueNAS configuration for {connection.name}. It may have
                already been removed.
              </div>
            );
          }
          return (
            <TrueNASCredentialSlot
              state={props.trueNASSettings}
              editingConnection={truenas}
              onCancel={context.onCancel}
              onSaved={context.onSaved}
              onToggleEnabled={() => void rowActions.togglePause(connection)}
              togglePending={rowActions.pendingAction(connection.id) === 'pause'}
              connectionEnabled={connection.enabled}
              onDelete={() => void rowActions.requestRemove(connection)}
              deletePending={rowActions.pendingAction(connection.id) === 'remove'}
              deleteConfirming={rowActions.confirmingRemove(connection.id)}
              deleteError={rowActions.actionError(connection.id)}
            />
          );
        }
        default:
          return (
            <div class="text-sm text-muted">
              Editing is not available for the {context.type} source type here yet.
            </div>
          );
      }
    }

    const discoveredPrefill = () => {
      const discovered = selectedDiscoveredSource();
      if (discovered && discovered.type === context.type) {
        return {
          type: discovered.type,
          name: discovered.hostname?.trim() || discovered.ip,
          host: `https://${discovered.hostname?.trim() || discovered.ip}:${discovered.port}`,
        } satisfies Partial<NodeConfig>;
      }

      const probed = selectedProbeCandidate();
      if (probed && probed.type === context.type) {
        const url = (() => {
          try {
            return new URL(probed.host);
          } catch {
            return null;
          }
        })();
        const derivedName = url?.hostname || probed.host.replace(/^https?:\/\//, '').split(':')[0];
        return {
          type: probed.type as NodeConfig['type'],
          name: derivedName,
          host: probed.host,
        } satisfies Partial<NodeConfig>;
      }

      return undefined;
    };

    switch (context.type) {
      case 'pve':
      case 'pbs':
      case 'pmg':
        return renderNodeSlot(context.type, undefined, undefined, discoveredPrefill());
      case 'truenas':
        return (
          <TrueNASCredentialSlot
            state={props.trueNASSettings}
            onCancel={context.onCancel}
            onSaved={context.onSaved}
          />
        );
      case 'vmware':
        return (
          <VMwareCredentialSlot
            state={props.vmwareSettings}
            onCancel={context.onCancel}
            onSaved={context.onSaved}
          />
        );
      case 'agent':
        return renderAgentAddSlot();
      default:
        return (
          <div class="text-sm text-muted">
            No credential form is wired up for the {context.type} source type yet.
          </div>
        );
    }
  };

  const addDialogTitle = createMemo(() => {
    const step = routeStep();
    if (!step || step === 'pick') return 'Add infrastructure';
    if (step === 'detect') return 'Detect API platform';
    const catalogItem = activeCatalogItem();
    if (catalogItem) return `Add ${catalogItem.label}`;
    return `Add ${describeManagedSourceType(activeAddType())}`;
  });

  const addDialogDescription = createMemo(() => {
    const step = routeStep();
    if (!step || step === 'pick') {
      return 'Choose the system, device, host, or service you want Pulse to monitor.';
    }
    if (step === 'detect') {
      return 'Probe a management API endpoint and let Pulse open the matching credential flow when it recognizes the platform.';
    }
    const discovered = selectedDiscoveredSource();
    if (discovered && discovered.type === activeAddType()) {
      const endpoint = `https://${discovered.hostname?.trim() || discovered.ip}:${discovered.port}`;
      return `Pulse discovered ${endpoint}. Review the endpoint, finish the credentials, and then save it as a monitored source.`;
    }
    const probed = selectedProbeCandidate();
    if (probed && probed.type === activeAddType()) {
      return `Pulse detected ${probed.host}. Review the endpoint, finish the credentials, and then save it as a monitored source.`;
    }
    const catalogItem = activeCatalogItem();
    if (catalogItem) {
      const strategy = getInfrastructureSourceStrategyPresentation(catalogItem.sourceStrategy);
      // Subtitle is just strategy + bestFor. The detailed coverage (telemetry
      // breakdown) is repeated by the green install-path explanation card
      // inside the dialog body, so dropping it here removes a redundant
      // sentence from a header the user reads first.
      return `${strategy.label}. ${trimSentenceTerminal(catalogItem.bestFor)}.`;
    }
    const presentation = getInfrastructureOnboardingProductPresentation(
      activeAddType() as InfrastructureOnboardingConnectionType,
    );
    const strategy = getInfrastructureSourceStrategyPresentation(presentation.sourceStrategy);
    return `${strategy.label}. ${trimSentenceTerminal(presentation.bestFor)}.`;
  });

  const editDialogTitle = createMemo(() => {
    const row = editingRow();
    if (!row) return 'Manage source';
    return `Manage ${row.name}`;
  });

  const editDialogDescription = createMemo(() => {
    const connection = editingConnection();
    if (!connection) return 'Update source state, credentials, and lifecycle actions here.';
    const label = describeManagedSourceType(connection.type);
    const methods = attachedAgentConnections().length > 0 ? ' · API + Pulse Agent' : '';
    const row = editingRow();
    if (row?.isCluster) {
      const contact = connection.name || connection.address || 'contact node';
      // Drop the 'Editing via' phrasing that read awkwardly; just identify
      // the contact node and address. The dialog body and the row the user
      // came from already make the editing context clear.
      return `${label} cluster · ${contact}${connection.address ? ` (${connection.address})` : ''}${methods}`;
    }
    return `${label}${connection.address ? ` · ${connection.address}` : ''}${methods}`;
  });

  const isAgentDialog = () => activeAddType() === 'agent' || editingConnection()?.type === 'agent';

  return (
    <div class="space-y-6">
      <InfrastructureSourceManager
        rows={rows}
        discoveredNodes={visibleDiscoveredNodes}
        discoveryEnabled={props.discoveryEnabled()}
        discoveryScanStatus={props.discoveryScanStatus}
        readOnly={readOnly()}
        onAddSource={
          readOnly()
            ? undefined
            : (type) => openAddFlow(type === 'agent' ? 'agent' : (type as ManagedAddTypeStep))
        }
        onAddSourceStep={readOnly() ? undefined : (step) => openAddFlow(step as ManagedAddTypeStep)}
        onAddInfrastructure={readOnly() ? undefined : () => openAddFlow('pick')}
        onRunDiscovery={
          readOnly()
            ? undefined
            : () => {
                void props.triggerDiscoveryScan();
              }
        }
        onOpenDiscoverySettings={readOnly() ? undefined : () => setShowDiscoverySettings(true)}
        onOpenConnection={readOnly() ? undefined : (row) => setEditingRow(row)}
        onReviewDiscoveredSource={
          readOnly() ? undefined : (server) => reviewDiscoveredSource(server)
        }
      />

      <InfrastructureDiscoverySettingsDialog
        isOpen={showDiscoverySettings()}
        onClose={() => setShowDiscoverySettings(false)}
        discoveryEnabled={props.discoveryEnabled}
        discoveryMode={props.discoveryMode}
        discoverySubnetDraft={props.discoverySubnetDraft}
        discoverySubnetError={props.discoverySubnetError}
        savingDiscoverySettings={props.savingDiscoverySettings}
        envOverrides={props.envOverrides}
        handleDiscoveryEnabledChange={props.handleDiscoveryEnabledChange}
        handleDiscoveryModeChange={props.handleDiscoveryModeChange}
        setDiscoveryMode={props.setDiscoveryMode}
        setDiscoverySubnetDraft={props.setDiscoverySubnetDraft}
        setDiscoverySubnetError={props.setDiscoverySubnetError}
        setLastCustomSubnet={props.setLastCustomSubnet}
        commitDiscoverySubnet={props.commitDiscoverySubnet}
        parseSubnetList={props.parseSubnetList}
        normalizeSubnetList={props.normalizeSubnetList}
        isValidCIDR={props.isValidCIDR}
        currentDraftSubnetValue={props.currentDraftSubnetValue}
        discoverySubnetInputRef={props.discoverySubnetInputRef}
      />

      <Show when={showAgentUpdateCommands()}>
        <InfrastructureAgentUpdatesDialog
          isOpen={true}
          targets={agentUpdateTargets()}
          onClose={closeAgentUpdateCommands}
        />
      </Show>

      <Show when={routeStep() !== null}>
        <Dialog
          isOpen={true}
          onClose={closeAddFlow}
          ariaLabel={addDialogTitle()}
          panelClass={isAgentDialog() ? 'max-w-6xl' : 'max-w-5xl'}
        >
          <div class="flex h-full min-h-0 flex-col">
            <div class="flex items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
              <div class="space-y-1">
                <h2 class="text-base font-semibold text-base-content">{addDialogTitle()}</h2>
                <p class="text-sm text-muted">{addDialogDescription()}</p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="iconMd"
                onClick={closeAddFlow}
                aria-label="Close add infrastructure dialog"
              >
                <X class="h-4 w-4" />
              </Button>
            </div>

            <div class="min-h-0 flex-1 overflow-y-auto">
              <Switch>
                <Match when={routeStep() === 'pick'}>
                  <InfrastructureSourcePicker
                    onSelectStep={(step) => openAddFlow(step as ManagedAddTypeStep)}
                    onDetectApiPlatform={() => openAddFlow('detect')}
                  />
                </Match>

                <Match when={routeStep() === 'detect'}>
                  <ConnectionEditor
                    mode="add"
                    onBackToCatalog={() => openAddFlow('pick')}
                    onSelectAgentRoute={() => openAddFlow('linux-host')}
                    onSelectCandidate={openAddFlowFromProbe}
                    onClose={closeAddFlow}
                    onSaved={handleAddSaved}
                    renderCredentialSlot={({ type, onCancel, onSaved }) =>
                      renderConnectionSlot({ mode: 'add', type, onCancel, onSaved })
                    }
                  />
                </Match>

                <Match when={Boolean(activeAddType())}>
                  <ConnectionEditor
                    mode="add"
                    initialType={activeAddType() ?? undefined}
                    initialCandidate={selectedProbeCandidate()}
                    showSlotHeader={false}
                    onBackToCatalog={() => openAddFlow('pick')}
                    onClose={closeAddFlow}
                    onSaved={handleAddSaved}
                    renderCredentialSlot={({ type, onCancel, onSaved }) =>
                      renderConnectionSlot({ mode: 'add', type, onCancel, onSaved })
                    }
                  />
                </Match>
              </Switch>
            </div>
          </div>
        </Dialog>
      </Show>

      <Show when={editingConnection()}>
        {(connectionAccessor) => {
          const connection = connectionAccessor();
          return (
            <Dialog
              isOpen={true}
              onClose={closeEditFlow}
              ariaLabel={editDialogTitle()}
              panelClass={connection.type === 'agent' ? 'max-w-5xl' : 'max-w-5xl'}
            >
              <div class="flex h-full min-h-0 flex-col">
                <div class="flex items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
                  <div class="space-y-1">
                    <h2 class="text-base font-semibold text-base-content">{editDialogTitle()}</h2>
                    <p class="text-sm text-muted">{editDialogDescription()}</p>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="iconMd"
                    onClick={closeEditFlow}
                    aria-label="Close edit infrastructure dialog"
                  >
                    <X class="h-4 w-4" />
                  </Button>
                </div>

                <div class="min-h-0 flex-1 overflow-y-auto">
                  <Show
                    when={connection.type === 'agent'}
                    fallback={
                      <div class="space-y-4">
                        <ConnectionEditor
                          mode="edit"
                          initialType={connection.type}
                          showSlotHeader={false}
                          onClose={closeEditFlow}
                          onSaved={handleEditSaved}
                          renderCredentialSlot={({ type, onCancel, onSaved }) =>
                            renderConnectionSlot({ mode: 'edit', type, onCancel, onSaved })
                          }
                        />
                        <Show when={attachedAgentConnections().length > 0}>
                          {renderAttachedAgentAugmentations(attachedAgentConnections())}
                        </Show>
                      </div>
                    }
                  >
                    {renderAgentConnectionDetails(connection)}
                  </Show>
                </div>
              </div>
            </Dialog>
          );
        }}
      </Show>
    </div>
  );
};

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  return (
    <InfrastructureOperationsStateProvider embedded>
      <InfrastructureWorkspaceContent {...props} />
    </InfrastructureOperationsStateProvider>
  );
};
