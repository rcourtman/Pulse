import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { AddSystemPicker, type AddSystemChoice } from './AddSystemPicker';
import { ConnectionsTable, type ConnectionsTableHeaderAction } from './ConnectionsTable';
import {
  buildInfrastructureSystemRows,
  type InfrastructureSystemRow,
  type SystemManageAction,
} from './connectionsTableModel';
import { InfrastructureActiveRowDetails } from './InfrastructureActiveRowDetails';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureIgnoredRowDetails } from './InfrastructureIgnoredRowDetails';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import { PlatformConnectionsWorkspace } from './PlatformConnectionsWorkspace';
import {
  buildInfrastructureWorkspacePath,
  getInfrastructureWorkspaceViewFromPath,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from './useInfrastructureOperationsState';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

const proxmoxRouteForKind = (kind: 'pve' | 'pbs' | 'pmg') =>
  `/settings/infrastructure/platforms/proxmox/${kind}`;

const InfrastructureWorkspaceContent: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const state = useInfrastructureOperationsContext();

  const activeView = createMemo(() => getInfrastructureWorkspaceViewFromPath(location.pathname));
  const readOnlyWorkspace = createMemo(() => presentationPolicyIsReadOnly());
  const [pickerOpen, setPickerOpen] = createSignal(false);
  const [profilesOpen, setProfilesOpen] = createSignal(false);

  const rows = createMemo<InfrastructureSystemRow[]>(() =>
    buildInfrastructureSystemRows({
      activeRows: state.activeRows(),
      monitoringStoppedRows: state.monitoringStoppedRows(),
    }),
  );
  const headerActions = createMemo<ConnectionsTableHeaderAction[]>(() =>
    readOnlyWorkspace()
      ? []
      : [
          {
            label: 'Add infrastructure',
            onSelect: () => setPickerOpen(true),
            tone: 'primary' as const,
          },
        ],
  );
  const closeConnectionsWorkspace = () => {
    props.trueNASSettings.closeDialog?.();
    props.trueNASSettings.closeDeleteDialog?.();
    props.vmwareSettings.closeDialog?.();
    props.vmwareSettings.closeDeleteDialog?.();
    props.setShowNodeModal(false);
    props.setEditingNode(null);
    props.cancelDeleteNode?.();
    navigate(buildInfrastructureWorkspacePath('inventory'), { scroll: false });
  };
  const closeInstallWorkspace = () => {
    navigate(buildInfrastructureWorkspacePath('inventory'), { scroll: false });
  };

  const openProxmoxNode = (nodeKind: 'pve' | 'pbs' | 'pmg', nodeId: string) => {
    const nodes =
      nodeKind === 'pve'
        ? props.pveNodes()
        : nodeKind === 'pbs'
          ? props.pbsNodes()
          : props.pmgNodes();
    const node = nodes.find((candidate) => candidate.id === nodeId) ?? null;

    props.onSelectAgent(nodeKind);
    props.setCurrentNodeType(nodeKind);
    props.setEditingNode(node);
    props.setModalResetKey((value) => value + 1);
    props.setShowNodeModal(true);
    navigate(proxmoxRouteForKind(nodeKind), { scroll: false });
  };

  const handleAddSystem = (choice: AddSystemChoice) => {
    setPickerOpen(false);

    if (choice.kind === 'agent') {
      navigate(buildInfrastructureWorkspacePath('install'), { scroll: false });
      return;
    }

    if (choice.kind === 'truenas') {
      props.trueNASSettings.openCreateDialog();
      navigate('/settings/infrastructure/platforms/truenas', { scroll: false });
      return;
    }

    if (choice.kind === 'vmware') {
      props.vmwareSettings.openCreateDialog();
      navigate('/settings/infrastructure/platforms/vmware', { scroll: false });
      return;
    }

    openProxmoxNode(choice.kind, '');
  };

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

  createEffect(() => {
    if (readOnlyWorkspace() && activeView() !== 'inventory') {
      navigate(buildInfrastructureWorkspacePath('inventory'), { replace: true });
    }
  });

  createEffect(() => {
    if (activeView() === 'inventory') {
      return;
    }
    setPickerOpen(false);
    setProfilesOpen(false);
    state.setExpandedRowKey(null);
    state.setSelectedIgnoredRowKey(null);
  });

  return (
    <div class="space-y-8">
      <ConnectionsTable
        rows={rows}
        headerActions={headerActions()}
        onManageRow={(row) => handleManageAction(row.manage)}
      />

      <AddSystemPicker
        isOpen={pickerOpen()}
        onClose={() => setPickerOpen(false)}
        onSelect={handleAddSystem}
        onManageProfiles={() => {
          setPickerOpen(false);
          setProfilesOpen(true);
        }}
      />

      <InfrastructureStopMonitoringDialog />

      <Dialog
        isOpen={profilesOpen()}
        onClose={() => setProfilesOpen(false)}
        panelClass="h-[calc(100dvh-2rem)] w-full max-w-[1100px]"
        ariaLabel="Agent profiles"
      >
        <div class="flex h-full flex-col bg-surface">
          <div class="border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
            <div class="flex items-start justify-between gap-4">
              <div class="space-y-1">
                <div class="text-lg font-semibold text-base-content">Agent profiles</div>
                <div class="text-sm text-muted">
                  Manage reusable install defaults for agent-based systems.
                </div>
              </div>
              <button
                type="button"
                onClick={() => setProfilesOpen(false)}
                class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
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
          </div>
          <div class="flex-1 overflow-y-auto p-4 sm:p-6">
            <AgentProfilesPanel />
          </div>
        </div>
      </Dialog>

      <Dialog
        isOpen={!readOnlyWorkspace() && activeView() === 'platforms'}
        onClose={closeConnectionsWorkspace}
        panelClass="h-[calc(100dvh-2rem)] w-full max-w-[1280px]"
        ariaLabel="Platform connections"
      >
        <div class="flex h-full flex-col bg-surface">
          <div class="border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
            <div class="flex items-start justify-between gap-4">
              <div class="space-y-1">
                <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                  Connections
                </div>
                <div class="text-lg font-semibold text-base-content">Platform connections</div>
                <div class="text-sm text-muted">
                  Configure API-backed providers without leaving the infrastructure ledger.
                </div>
              </div>
              <button
                type="button"
                onClick={closeConnectionsWorkspace}
                class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
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
          </div>
          <div class="flex-1 overflow-y-auto p-4 sm:p-6">
            <PlatformConnectionsWorkspace {...props} />
          </div>
        </div>
      </Dialog>

      <Dialog
        isOpen={!readOnlyWorkspace() && activeView() === 'install'}
        onClose={closeInstallWorkspace}
        panelClass="h-[calc(100dvh-2rem)] w-full max-w-[1280px]"
        ariaLabel="Install Pulse agent"
      >
        <div class="flex h-full flex-col bg-surface">
          <div class="border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
            <div class="flex items-start justify-between gap-4">
              <div class="space-y-1">
                <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                  Install
                </div>
                <div class="text-lg font-semibold text-base-content">Install Pulse agent</div>
                <div class="text-sm text-muted">
                  Generate Linux, macOS, FreeBSD, and Windows install commands from the same
                  workspace.
                </div>
              </div>
              <button
                type="button"
                onClick={closeInstallWorkspace}
                class="rounded-md p-1 hover:bg-surface-hover hover:text-base-content"
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
          </div>
          <div class="flex-1 overflow-y-auto p-4 sm:p-6">
            <InfrastructureInstallerSection />
          </div>
        </div>
      </Dialog>

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
