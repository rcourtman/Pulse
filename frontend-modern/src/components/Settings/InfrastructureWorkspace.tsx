import { Component, Match, Show, Switch, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import X from 'lucide-solid/icons/x';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import { Dialog } from '@/components/shared/Dialog';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import type { AgentUninstallCommands } from './ConnectionsTable';
import { ConnectionEditor } from './ConnectionEditor/ConnectionEditor';
import { NodeCredentialSlot } from './ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import { TrueNASCredentialSlot } from './ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { VMwareCredentialSlot } from './ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import type { Connection, ConnectionType } from '@/api/connections';
import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';
import type { NodeConfigWithStatus } from '@/types/nodes';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureSourceManager } from './InfrastructureSourceManager';
import { InfrastructureSourcePicker } from './InfrastructureSourcePicker';
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLocation,
  type InfrastructureAddStep,
  type InfrastructurePanelStep,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import { useConnectionsLedger } from './useConnectionsLedger';
import { useConnectionRowActions } from './useConnectionRowActions';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from './useInfrastructureOperationsState';
import {
  clearSharedInfrastructureOnboardingMetricsTracker,
  getSharedInfrastructureOnboardingMetricsTracker,
  type InfrastructureOnboardingMetricsTracker,
} from '@/utils/infrastructureOnboardingMetrics';
import {
  getInfrastructureOnboardingProductPresentation,
  type InfrastructureOnboardingConnectionType,
} from '@/utils/infrastructureOnboardingPresentation';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

type ManagedAddTypeStep = Exclude<InfrastructureAddStep, 'detect'>;

const ADD_STEP_TO_TYPE: Record<ManagedAddTypeStep, ConnectionType> = {
  agent: 'agent',
  pve: 'pve',
  pbs: 'pbs',
  pmg: 'pmg',
  truenas: 'truenas',
  vmware: 'vmware',
};

const closeButtonClass =
  'inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover';
const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';

const createOpenedOnboardingTracker = (): InfrastructureOnboardingMetricsTracker => {
  const tracker = getSharedInfrastructureOnboardingMetricsTracker();
  tracker.recordOpened();
  return tracker;
};

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
    return getInfrastructureOnboardingProductPresentation(type as InfrastructureOnboardingConnectionType)
      .label;
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
  const [editingConnection, setEditingConnection] = createSignal<Connection | null>(null);
  const readOnly = createMemo(() => presentationPolicyIsReadOnly());
  const routeStep = createMemo<InfrastructurePanelStep | null>(() => {
    if (readOnly()) return null;
    return deriveAddStepFromLocation(location.pathname, location.search ?? '');
  });
  const [addFlowTracker, setAddFlowTracker] =
    createSignal<InfrastructureOnboardingMetricsTracker | null>(
      routeStep() ? createOpenedOnboardingTracker() : null,
    );
  const activeAddType = createMemo<ConnectionType | null>(() => {
    const step = routeStep();
    if (!step || step === 'pick' || step === 'detect') return null;
    return ADD_STEP_TO_TYPE[step as ManagedAddTypeStep];
  });

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
    if (!addFlowTracker()) {
      setAddFlowTracker(createOpenedOnboardingTracker());
    }
    navigate(buildInfrastructureOnboardingPath(step), { scroll: false });
  };

  const closeAddFlow = () => {
    resetInlineEditorState();
    setAddFlowTracker(null);
    clearSharedInfrastructureOnboardingMetricsTracker();
    navigateToWorkspace(Boolean(routeStep()));
  };

  const closeEditFlow = () => {
    const connection = editingConnection();
    if (connection) {
      rowActions.cancelRemove(connection.id);
    }
    resetInlineEditorState();
    setEditingConnection(null);
  };

  const handleAddSaved = () => {
    ledger.reload();
    closeAddFlow();
  };

  const handleEditSaved = () => {
    ledger.reload();
    closeEditFlow();
  };

  const recordCatalogSelection = (type: InfrastructureOnboardingConnectionType) => {
    const tracker = addFlowTracker();
    if (!tracker) return;
    tracker.recordPathSelected(type === 'agent' ? 'agent' : 'api');
    if (type !== 'agent') {
      tracker.recordCatalogSelected(type);
    }
  };

  createEffect(() => {
    if (activeAddType() !== 'agent') {
      setShowAgentProfiles(false);
    }
  });

  createEffect(() => {
    const step = routeStep();
    const tracker = addFlowTracker();
    if (step && !tracker) {
      setAddFlowTracker(createOpenedOnboardingTracker());
      return;
    }
    if (!step && tracker) {
      setAddFlowTracker(null);
      clearSharedInfrastructureOnboardingMetricsTracker();
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

  const renderNodeSlot = (
    type: 'pve' | 'pbs' | 'pmg',
    editingNode?: NodeConfigWithStatus | null,
    editingRow?: Connection | null,
  ) => {
    const connectionId = editingRow?.id ?? null;
    return (
      <NodeCredentialSlot
        nodeType={type}
        settings={props}
        editingNode={editingNode ?? null}
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
      <div class="flex items-center justify-end">
        <button
          type="button"
          onClick={() => setShowAgentProfiles((value) => !value)}
          class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
        >
          {showAgentProfiles() ? 'Hide agent profiles' : 'Manage agent profiles'}
        </button>
      </div>
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
      <InfrastructureInstallerSection />
    </div>
  );

  const renderAgentConnectionDetails = (connection: Connection) => {
    const pausePending = () => rowActions.pendingAction(connection.id) === 'pause';
    const removePending = () => rowActions.pendingAction(connection.id) === 'remove';
    const removeConfirming = () => rowActions.confirmingRemove(connection.id);
    const error = () => rowActions.actionError(connection.id);

    return (
      <div class="space-y-6 p-4">
        <section class="rounded-xl border border-border bg-surface p-4">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="space-y-1">
              <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                Pulse Agent
              </div>
              <div class="text-lg font-semibold text-base-content">{connection.name}</div>
              <Show when={connection.address}>
                <div class="break-words text-sm text-muted">{connection.address}</div>
              </Show>
            </div>
            <span class="inline-flex items-center rounded-full border border-border bg-surface-alt px-2.5 py-1 text-xs font-medium text-base-content">
              {connection.enabled ? 'Enabled' : 'Paused'}
            </span>
          </div>

          <div class="mt-4 grid gap-3 sm:grid-cols-2">
            <div class="rounded-lg border border-border bg-surface-alt px-3 py-3">
              <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                Connection state
              </div>
              <div class="mt-1 text-sm text-base-content">{connection.state}</div>
            </div>
            <div class="rounded-lg border border-border bg-surface-alt px-3 py-3">
              <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                Last seen
              </div>
              <div class="mt-1 text-sm text-base-content">
                {connection.lastSeen ? connection.lastSeen : 'No activity yet'}
              </div>
            </div>
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
                <div class="mt-1 break-all font-mono text-[11px] text-base-content">
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
                <div class="mt-1 break-all font-mono text-[11px] text-base-content">
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
            onClick={() => void rowActions.togglePause(connection)}
            disabled={pausePending() || removePending()}
            class={buttonClass}
          >
            {pausePending()
              ? connection.enabled
                ? 'Pausing…'
                : 'Resuming…'
              : connection.enabled
                ? 'Pause connection'
                : 'Resume connection'}
          </button>
          <button
            type="button"
            onClick={() => void rowActions.requestRemove(connection)}
            disabled={pausePending() || removePending()}
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

    switch (context.type) {
      case 'pve':
      case 'pbs':
      case 'pmg':
        return renderNodeSlot(context.type);
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
    if (step === 'detect') return 'Detect infrastructure source';
    return `Add ${describeManagedSourceType(activeAddType())}`;
  });

  const addDialogDescription = createMemo(() => {
    const step = routeStep();
    if (!step || step === 'pick') {
      return 'Choose the source type you want to add.';
    }
    if (step === 'detect') {
      return 'Probe an address and let Pulse open the matching credential flow when it recognizes the platform.';
    }
    const presentation = getInfrastructureOnboardingProductPresentation(
      activeAddType() as InfrastructureOnboardingConnectionType,
    );
    return `${presentation.bestFor}. ${presentation.coverage}.`;
  });

  const editDialogTitle = createMemo(() => {
    const connection = editingConnection();
    if (!connection) return 'Edit source';
    return `Edit ${connection.name}`;
  });

  const editDialogDescription = createMemo(() => {
    const connection = editingConnection();
    if (!connection) return 'Update this source without leaving the infrastructure manager.';
    const label = describeManagedSourceType(connection.type);
    return `${label}${connection.address ? ` · ${connection.address}` : ''}`;
  });

  const isAgentDialog = () =>
    activeAddType() === 'agent' || editingConnection()?.type === 'agent';

  return (
    <div class="space-y-6">
      <InfrastructureSourceManager
        rows={rows}
        readOnly={readOnly()}
        onAddSource={
          readOnly()
            ? undefined
            : (type) => openAddFlow(type === 'agent' ? 'agent' : (type as ManagedAddTypeStep))
        }
        onDetectFromAddress={readOnly() ? undefined : () => openAddFlow('detect')}
        onOpenConnection={readOnly() ? undefined : (connection) => setEditingConnection(connection)}
      />

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
              <button
                type="button"
                onClick={closeAddFlow}
                class={closeButtonClass}
                aria-label="Close add infrastructure dialog"
              >
                <X class="h-4 w-4" />
              </button>
            </div>

            <div class="min-h-0 flex-1 overflow-y-auto">
              <Switch>
                <Match when={routeStep() === 'pick'}>
                  <InfrastructureSourcePicker
                    onSelectType={(type) => {
                      recordCatalogSelection(type);
                      openAddFlow(type === 'agent' ? 'agent' : (type as ManagedAddTypeStep));
                    }}
                    onDetectFromAddress={() => openAddFlow('detect')}
                  />
                </Match>

                <Match when={routeStep() === 'detect'}>
                  <ConnectionEditor
                    mode="add"
                    onboardingMetricsTracker={addFlowTracker()}
                    onBackToCatalog={() => openAddFlow('pick')}
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
                    showSlotHeader={false}
                    trackInitialCatalogSelection={activeAddType() !== 'agent'}
                    onboardingMetricsTracker={addFlowTracker()}
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
                  <button
                    type="button"
                    onClick={closeEditFlow}
                    class={closeButtonClass}
                    aria-label="Close edit infrastructure dialog"
                  >
                    <X class="h-4 w-4" />
                  </button>
                </div>

                <div class="min-h-0 flex-1 overflow-y-auto">
                  <Show
                    when={connection.type === 'agent'}
                    fallback={
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
