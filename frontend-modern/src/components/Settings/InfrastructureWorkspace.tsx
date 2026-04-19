import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ConnectionsTable, type ConnectionsTableHeaderAction } from './ConnectionsTable';
import {
  buildInfrastructureSystemRows,
  type InfrastructureSystemRow,
  type SystemManageAction,
} from './connectionsTableModel';
import { ConnectionEditor } from './ConnectionEditor/ConnectionEditor';
import type { ConnectionType } from '@/api/connections';
import { InfrastructureActiveRowDetails } from './InfrastructureActiveRowDetails';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureIgnoredRowDetails } from './InfrastructureIgnoredRowDetails';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import { ProxmoxSettingsPanel } from './ProxmoxSettingsPanel';
import { TrueNASSettingsPanel } from './TrueNASSettingsPanel';
import { VMwareSettingsPanel } from './VMwareSettingsPanel';
import {
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLegacyPath,
  type InfrastructureAddStep,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import type { NodeType } from './infrastructureSettingsModel';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from './useInfrastructureOperationsState';

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
  const state = useInfrastructureOperationsContext();

  const [addDrawerOpen, setAddDrawerOpen] = createSignal(false);
  const [initialAddType, setInitialAddType] = createSignal<ConnectionType | null>(null);
  const [showAgentProfiles, setShowAgentProfiles] = createSignal(false);
  const readOnly = createMemo(() => presentationPolicyIsReadOnly());

  // Redirect legacy deep links and pre-select the matching type in the editor.
  createEffect(() => {
    const path = location.pathname;
    if (path === '/settings/infrastructure') return;

    const step = deriveAddStepFromLegacyPath(path);
    navigate(buildInfrastructureWorkspacePath(), { replace: true });
    if (!readOnly()) {
      setAddDrawerOpen(true);
      if (step && step !== 'pick') {
        setInitialAddType(ADD_STEP_TO_TYPE[step]);
      } else {
        setInitialAddType(null);
      }
    }
  });

  // Auto-open the agent installer when a setup handoff is waiting.
  createEffect(() => {
    if (state.setupHandoff?.() && !addDrawerOpen() && !readOnly()) {
      setAddDrawerOpen(true);
      setInitialAddType('agent');
    }
  });

  // Close add panel in read-only mode.
  createEffect(() => {
    if (readOnly() && addDrawerOpen()) {
      setAddDrawerOpen(false);
    }
  });

  const rows = createMemo<InfrastructureSystemRow[]>(() =>
    buildInfrastructureSystemRows({
      activeRows: state.activeRows(),
      monitoringStoppedRows: state.monitoringStoppedRows(),
    }),
  );

  const headerActions = createMemo<ConnectionsTableHeaderAction[]>(() =>
    readOnly()
      ? []
      : [
          {
            label: 'Add infrastructure',
            onSelect: () => {
              setInitialAddType(null);
              setAddDrawerOpen(true);
              setShowAgentProfiles(false);
            },
            tone: 'primary' as const,
          },
        ],
  );

  const handleManageAction = (action: SystemManageAction) => {
    switch (action.kind) {
      case 'inventory-active':
        state.setExpandedRowKey(action.rowKey);
        state.setSelectedIgnoredRowKey(null);
        return;
      case 'inventory-ignored':
        state.setExpandedRowKey(null);
        state.setSelectedIgnoredRowKey(action.rowKey);
        return;
      default:
        return;
    }
  };

  const closeAddDrawer = () => {
    setAddDrawerOpen(false);
    setInitialAddType(null);
    setShowAgentProfiles(false);
  };

  const renderProxmoxSlot = (type: NodeType) => {
    const selectedAgent = () => type;
    return (
      <ProxmoxSettingsPanel
        {...props}
        selectedAgent={selectedAgent}
        onSelectAgent={() => {}}
        embedded
      />
    );
  };

  return (
    <div class="space-y-8">
      <ConnectionsTable
        rows={rows}
        headerActions={headerActions()}
        onManageRow={(row) => handleManageAction(row.manage)}
      />

      {/* Unified add-connection drawer */}
      <Dialog
        isOpen={addDrawerOpen()}
        onClose={closeAddDrawer}
        layout="drawer-right"
        panelClass="max-w-[820px]"
        ariaLabel="Add connection"
      >
        <div class="flex h-full flex-col overflow-hidden">
          <div class="flex shrink-0 items-center justify-between gap-3 border-b border-border px-4 py-3">
            <span class="text-sm font-semibold text-base-content">Add connection</span>
            <button
              type="button"
              onClick={closeAddDrawer}
              class="rounded-md p-1 text-muted hover:bg-surface-hover hover:text-base-content"
              aria-label="Close"
            >
              <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <div class="flex-1 overflow-y-auto">
            <Show when={addDrawerOpen()}>
              <ConnectionEditor
                mode="add"
                initialType={initialAddType() ?? undefined}
                onClose={closeAddDrawer}
                renderCredentialSlot={({ type }) => {
                  switch (type) {
                    case 'pve':
                    case 'pbs':
                    case 'pmg':
                      return renderProxmoxSlot(type);
                    case 'truenas':
                      return <TrueNASSettingsPanel state={props.trueNASSettings} />;
                    case 'vmware':
                      return <VMwareSettingsPanel state={props.vmwareSettings} />;
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
            </Show>
          </div>
        </div>
      </Dialog>

      {/* Active system detail drawer */}
      <Dialog
        isOpen={Boolean(state.selectedActiveRow())}
        onClose={() => state.setExpandedRowKey(null)}
        layout="drawer-right"
        panelClass="max-w-[760px]"
        ariaLabel="Reporting item details"
      >
        <Show when={state.selectedActiveRow()}>
          {(rowAccessor) => <InfrastructureActiveRowDetails rowAccessor={rowAccessor} />}
        </Show>
      </Dialog>

      {/* Ignored system detail drawer */}
      <Dialog
        isOpen={Boolean(state.selectedIgnoredRow())}
        onClose={() => state.setSelectedIgnoredRowKey(null)}
        layout="drawer-right"
        panelClass="max-w-[760px]"
        ariaLabel="Ignored item details"
      >
        <Show when={state.selectedIgnoredRow()}>
          {(rowAccessor) => <InfrastructureIgnoredRowDetails rowAccessor={rowAccessor} />}
        </Show>
      </Dialog>

      <InfrastructureStopMonitoringDialog />
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
