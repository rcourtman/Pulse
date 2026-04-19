import { Component, Match, Show, Switch, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate, useSearchParams } from '@solidjs/router';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ConnectionDetailPanel } from './ConnectionDetailPanel';
import { ConnectionsTable, type ConnectionsTableHeaderAction } from './ConnectionsTable';
import type { InfrastructureSystemRow, SystemManageAction } from './connectionsTableModel';
import { ConnectionEditor } from './ConnectionEditor/ConnectionEditor';
import { NodeCredentialSlot } from './ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import { TrueNASCredentialSlot } from './ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { VMwareCredentialSlot } from './ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import type { Connection, ConnectionType } from '@/api/connections';
import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';
import type { NodeConfigWithStatus } from '@/types/nodes';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import {
  buildInfrastructureWorkspacePath,
  deriveAddStepFromSearch,
  deriveAddStepFromLocation,
  INFRASTRUCTURE_ADD_QUERY_PARAM,
  type InfrastructureAddStep,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import { useConnectionsLedger } from './useConnectionsLedger';
import { InfrastructureOperationsStateProvider } from './useInfrastructureOperationsState';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

const ADD_STEP_TO_TYPE: Record<InfrastructureAddStep, ConnectionType> = {
  agent: 'agent',
  pve: 'pve',
  pbs: 'pbs',
  pmg: 'pmg',
  truenas: 'truenas',
  vmware: 'vmware',
};

const InfrastructureWorkspaceContent: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [, setSearchParams] = useSearchParams();
  const ledger = useConnectionsLedger();

  const [addMode, setAddMode] = createSignal(false);
  const [initialAddType, setInitialAddType] = createSignal<ConnectionType | null>(null);
  const [showAgentProfiles, setShowAgentProfiles] = createSignal(false);
  const [selectedConnectionId, setSelectedConnectionId] = createSignal<string | null>(null);
  const [editingConnection, setEditingConnection] = createSignal<Connection | null>(null);
  const readOnly = createMemo(() => presentationPolicyIsReadOnly());
  const selectedConnection = createMemo(() => {
    const id = selectedConnectionId();
    return id ? ledger.findById(id) : undefined;
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

  // Redirect legacy deep links and pre-select the matching type in the editor.
  createEffect(() => {
    const path = location.pathname;
    const queryStep = deriveAddStepFromSearch(location.search ?? '');
    const step = deriveAddStepFromLocation(path, location.search ?? '');
    if (path === '/settings/infrastructure' && !step) return;

    if (queryStep) {
      setSearchParams({ [INFRASTRUCTURE_ADD_QUERY_PARAM]: null }, { replace: true });
    } else {
      navigate(buildInfrastructureWorkspacePath(), { replace: true });
    }
    if (!readOnly() && step) {
      setAddMode(true);
      if (step && step !== 'pick') {
        setInitialAddType(ADD_STEP_TO_TYPE[step]);
      } else {
        setInitialAddType(null);
      }
      return;
    }

    setAddMode(false);
    setInitialAddType(null);
    setShowAgentProfiles(false);
  });

  // Drop add/edit mode in read-only sessions.
  createEffect(() => {
    if (readOnly() && addMode()) {
      setAddMode(false);
    }
    if (readOnly() && editingConnection()) {
      setEditingConnection(null);
    }
  });

  const rows = createMemo<InfrastructureSystemRow[]>(() => ledger.rows());

  const headerActions = createMemo<ConnectionsTableHeaderAction[]>(() =>
    readOnly()
      ? []
      : [
          {
            label: 'Add connection',
            onSelect: () => {
              setInitialAddType(null);
              setAddMode(true);
              setShowAgentProfiles(false);
            },
            tone: 'primary' as const,
          },
        ],
  );

  const handleManageAction = (action: SystemManageAction) => {
    if (action.kind === 'connection') {
      setSelectedConnectionId(action.connectionId);
    }
  };

  const exitAddMode = () => {
    setAddMode(false);
    setInitialAddType(null);
    setShowAgentProfiles(false);
  };

  const exitEditMode = () => {
    setEditingConnection(null);
  };

  const handleEditConnection = (connection: Connection) => {
    setSelectedConnectionId(null);
    setEditingConnection(connection);
  };

  const handleEditSaved = () => {
    ledger.reload();
    exitEditMode();
  };

  const renderNodeSlot = (
    type: 'pve' | 'pbs' | 'pmg',
    editingNode?: NodeConfigWithStatus | null,
  ) => {
    const onExit = editingNode ? exitEditMode : exitAddMode;
    const onSaved = editingNode ? handleEditSaved : exitAddMode;
    return (
      <NodeCredentialSlot
        nodeType={type}
        settings={props}
        editingNode={editingNode ?? null}
        onCancel={onExit}
        onSaved={onSaved}
      />
    );
  };

  const mode = createMemo<'ledger' | 'add' | 'edit' | 'detail'>(() => {
    if (editingConnection()) return 'edit';
    if (addMode()) return 'add';
    if (selectedConnection()) return 'detail';
    return 'ledger';
  });

  const exitDetailMode = () => setSelectedConnectionId(null);

  return (
    <div class="space-y-8">
      <Switch
        fallback={
          <ConnectionsTable
            rows={rows}
            headerActions={headerActions()}
            onManageRow={(row) => handleManageAction(row.manage)}
          />
        }
      >
        <Match when={mode() === 'edit' && editingConnection()}>
          {(accessor) => {
            const connection = accessor();
            const editableSlot = (() => {
              switch (connection.type) {
                case 'pve':
                case 'pbs':
                case 'pmg': {
                  const node = findEditableNode(connection);
                  return node
                    ? { kind: 'node' as const, render: () => renderNodeSlot(connection.type as 'pve' | 'pbs' | 'pmg', node) }
                    : null;
                }
                case 'vmware': {
                  const vmware = findEditableVMware(connection);
                  return vmware
                    ? {
                        kind: 'vmware' as const,
                        render: () => (
                          <VMwareCredentialSlot
                            state={props.vmwareSettings}
                            editingConnection={vmware}
                            onCancel={exitEditMode}
                            onSaved={handleEditSaved}
                          />
                        ),
                      }
                    : null;
                }
                case 'truenas': {
                  const truenas = findEditableTrueNAS(connection);
                  return truenas
                    ? {
                        kind: 'truenas' as const,
                        render: () => (
                          <TrueNASCredentialSlot
                            state={props.trueNASSettings}
                            editingConnection={truenas}
                            onCancel={exitEditMode}
                            onSaved={handleEditSaved}
                          />
                        ),
                      }
                    : null;
                }
                default:
                  return null;
              }
            })();

            if (!editableSlot) {
              return (
                <div class="space-y-4">
                  <div class="flex items-center justify-between gap-3">
                    <div class="text-base font-semibold text-base-content">
                      Edit connection
                    </div>
                    <button
                      type="button"
                      onClick={exitEditMode}
                      class="inline-flex items-center gap-1 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                    >
                      ← Back to systems
                    </button>
                  </div>
                  <div
                    role="alert"
                    class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                  >
                    Couldn't find the underlying configuration for {connection.name}. It may have
                    been removed. Reload and try again.
                  </div>
                </div>
              );
            }
            return (
              <div class="space-y-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-base font-semibold text-base-content">
                      Edit {connection.name}
                    </div>
                    <div class="mt-0.5 text-xs text-muted">{connection.address}</div>
                  </div>
                  <button
                    type="button"
                    onClick={exitEditMode}
                    class="inline-flex items-center gap-1 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                  >
                    ← Back to systems
                  </button>
                </div>

                <div class="rounded-lg border border-border bg-surface p-4">
                  {editableSlot.render()}
                </div>
              </div>
            );
          }}
        </Match>

        <Match when={mode() === 'detail' && selectedConnection()}>
          {(accessor) => {
            const connection = accessor();
            const handleEditFromDetail = () => handleEditConnection(connection);
            return (
              <div class="space-y-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="text-base font-semibold text-base-content">
                      {connection.name || connection.address || connection.id}
                    </div>
                    <div class="mt-0.5 text-xs text-muted">{connection.address}</div>
                  </div>
                  <button
                    type="button"
                    onClick={exitDetailMode}
                    class="inline-flex items-center gap-1 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                  >
                    ← Back to systems
                  </button>
                </div>

                <ConnectionDetailPanel
                  connection={selectedConnection}
                  onMutated={() => ledger.reload()}
                  onEdit={handleEditFromDetail}
                  onRemoved={exitDetailMode}
                />
              </div>
            );
          }}
        </Match>

        <Match when={mode() === 'add'}>
        <div class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-base font-semibold text-base-content">Add connection</div>
              <div class="mt-0.5 text-xs text-muted">
                Paste an address — Pulse detects the product, you enter credentials, save.
              </div>
            </div>
            <button
              type="button"
              onClick={exitAddMode}
              class="inline-flex items-center gap-1 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              ← Back to systems
            </button>
          </div>

          <div class="rounded-lg border border-border bg-surface">
            <ConnectionEditor
              mode="add"
              initialType={initialAddType() ?? undefined}
              onClose={exitAddMode}
              renderCredentialSlot={({ type }) => {
                switch (type) {
                  case 'pve':
                  case 'pbs':
                  case 'pmg':
                    return renderNodeSlot(type);
                  case 'truenas':
                    return (
                      <TrueNASCredentialSlot
                        state={props.trueNASSettings}
                        onCancel={exitAddMode}
                        onSaved={exitAddMode}
                      />
                    );
                  case 'vmware':
                    return (
                      <VMwareCredentialSlot
                        state={props.vmwareSettings}
                        onCancel={exitAddMode}
                        onSaved={exitAddMode}
                      />
                    );
                  case 'agent':
                    return (
                      <div class="space-y-4">
                        <div class="flex items-center justify-end">
                          <button
                            type="button"
                            onClick={() => setShowAgentProfiles((v) => !v)}
                            class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                          >
                            {showAgentProfiles() ? 'Hide agent profiles' : 'Manage agent profiles'}
                          </button>
                        </div>
                        <Show when={showAgentProfiles()}>
                          <div class="rounded-xl border border-border bg-surface p-4 shadow-sm">
                            <div class="mb-4 space-y-1">
                              <div class="text-base font-semibold text-base-content">
                                Agent profiles
                              </div>
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
                        No credential form is wired up for the {type} type yet.
                      </div>
                    );
                }
              }}
            />
          </div>
        </div>
        </Match>
      </Switch>
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
