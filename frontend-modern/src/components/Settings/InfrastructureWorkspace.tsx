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
import { NodeCredentialSlot } from './ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import { TrueNASCredentialSlot } from './ConnectionEditor/CredentialSlots/TrueNASCredentialSlot';
import { VMwareCredentialSlot } from './ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import type { ConnectionType } from '@/api/connections';
import { InfrastructureActiveRowDetails } from './InfrastructureActiveRowDetails';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureIgnoredRowDetails } from './InfrastructureIgnoredRowDetails';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import {
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLegacyPath,
  type InfrastructureAddStep,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
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

  const [addMode, setAddMode] = createSignal(false);
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
      setAddMode(true);
      if (step && step !== 'pick') {
        setInitialAddType(ADD_STEP_TO_TYPE[step]);
      } else {
        setInitialAddType(null);
      }
    }
  });

  // Auto-open the agent installer when a setup handoff is waiting.
  createEffect(() => {
    if (state.setupHandoff?.() && !addMode() && !readOnly()) {
      setAddMode(true);
      setInitialAddType('agent');
    }
  });

  // Drop add mode in read-only sessions.
  createEffect(() => {
    if (readOnly() && addMode()) {
      setAddMode(false);
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

  const exitAddMode = () => {
    setAddMode(false);
    setInitialAddType(null);
    setShowAgentProfiles(false);
  };

  const renderNodeSlot = (type: 'pve' | 'pbs' | 'pmg') => {
    return (
      <NodeCredentialSlot
        nodeType={type}
        settings={props}
        onCancel={exitAddMode}
        onSaved={exitAddMode}
      />
    );
  };

  return (
    <div class="space-y-8">
      <Show
        when={addMode()}
        fallback={
          <ConnectionsTable
            rows={rows}
            headerActions={headerActions()}
            onManageRow={(row) => handleManageAction(row.manage)}
          />
        }
      >
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
      </Show>

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
