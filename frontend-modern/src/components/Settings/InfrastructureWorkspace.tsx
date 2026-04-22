import { Component, Match, Show, Switch, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import X from 'lucide-solid/icons/x';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import { Dialog } from '@/components/shared/Dialog';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ConnectionsTable, type AgentUninstallCommands } from './ConnectionsTable';
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
import {
  buildInfrastructureOnboardingPath,
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLocation,
  type InfrastructureAddStep,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import { useConnectionsLedger } from './useConnectionsLedger';
import { useConnectionRowActions } from './useConnectionRowActions';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from './useInfrastructureOperationsState';
import { getInfrastructureOnboardingProductPresentation } from '@/utils/infrastructureOnboardingPresentation';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

const ADD_STEP_TO_TYPE: Record<InfrastructureAddStep, ConnectionType> = {
  agent: 'agent',
  pve: 'pve',
  pbs: 'pbs',
  pmg: 'pmg',
  truenas: 'truenas',
  vmware: 'vmware',
};

const closeButtonClass =
  'inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover';

const describeManagedSourceType = (type: ConnectionType | null): string => {
  if (!type) return 'Infrastructure';
  return getInfrastructureOnboardingProductPresentation(type as InfrastructureAddStep).label;
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
  const routeStep = createMemo(() => {
    if (readOnly()) return null;
    return deriveAddStepFromLocation(location.pathname, location.search ?? '');
  });
  const activeAddType = createMemo<ConnectionType | null>(() => {
    const step = routeStep();
    if (!step || step === 'pick') return null;
    return ADD_STEP_TO_TYPE[step];
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

  const openAddFlow = (type: InfrastructureAddStep | 'pick') => {
    navigate(buildInfrastructureOnboardingPath(type), { scroll: false });
  };

  const closeAddFlow = () => {
    resetInlineEditorState();
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

  const handleEditSaved = () => {
    ledger.reload();
    closeEditFlow();
  };

  createEffect(() => {
    if (routeStep() !== 'pick') return;
    setShowAgentProfiles(false);
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

  const handleAddSaved = () => {
    ledger.reload();
    closeAddFlow();
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
        return (
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
    return `Add ${describeManagedSourceType(activeAddType())}`;
  });

  const addDialogDescription = createMemo(() => {
    const step = routeStep();
    if (!step || step === 'pick') {
      return 'Detect a source from its address, or choose the source type you want to add.';
    }
    const presentation = getInfrastructureOnboardingProductPresentation(
      activeAddType() as InfrastructureAddStep,
    );
    return `${presentation.bestFor}. ${presentation.coverage}.`;
  });

  const editDialogTitle = createMemo(() => {
    const connection = editingConnection();
    if (!connection) return 'Edit connection';
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
        actions={readOnly() ? undefined : rowActions}
        onAddType={(type) => openAddFlow(type as InfrastructureAddStep)}
        onEditConnection={readOnly() ? undefined : (connection) => setEditingConnection(connection)}
        onDetectFromAddress={readOnly() ? undefined : () => openAddFlow('pick')}
        agentUninstallCommands={agentUninstallCommands()}
        onCopyText={(text) => void handleCopy(text)}
      />

      <ConnectionsTable rows={rows} />

      <Show when={routeStep()}>
        {(stepAccessor) => {
          const step = stepAccessor();
          return (
            <Dialog
              isOpen={true}
              onClose={closeAddFlow}
              ariaLabel={addDialogTitle()}
              panelClass={isAgentDialog() ? 'max-w-6xl' : 'max-w-5xl'}
            >
              <div class="flex h-full flex-col">
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
                    <Match when={step === 'pick'}>
                      <ConnectionEditor
                        mode="add"
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
          );
        }}
      </Show>

      <Show when={editingConnection()}>
        {(connectionAccessor) => {
          const connection = connectionAccessor();
          return (
            <Dialog
              isOpen={true}
              onClose={closeEditFlow}
              ariaLabel={editDialogTitle()}
              panelClass={connection.type === 'agent' ? 'max-w-6xl' : 'max-w-5xl'}
            >
              <div class="flex h-full flex-col">
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
