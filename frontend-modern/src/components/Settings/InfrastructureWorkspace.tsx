import { Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ADD_SYSTEM_CHOICES, type AddSystemChoice } from './AddSystemPicker';
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
  const [showInstallProfiles, setShowInstallProfiles] = createSignal(false);

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
            onSelect: () => navigate(buildInfrastructureWorkspacePath('install'), { scroll: false }),
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
    setShowInstallProfiles(false);
    navigate(buildInfrastructureWorkspacePath('inventory'), { scroll: false });
  };

  const handleAddSystem = (choice: AddSystemChoice) => {
    setShowInstallProfiles(false);

    if (choice.kind === 'agent') {
      navigate(buildInfrastructureWorkspacePath('install'), { scroll: false });
      return;
    }

    if (choice.kind === 'truenas') {
      navigate('/settings/infrastructure/platforms/truenas', { scroll: false });
      return;
    }

    if (choice.kind === 'vmware') {
      navigate('/settings/infrastructure/platforms/vmware', { scroll: false });
      return;
    }

    navigate(proxmoxRouteForKind(choice.kind), { scroll: false });
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
    if (activeView() !== 'install') {
      setShowInstallProfiles(false);
    }
    state.setExpandedRowKey(null);
    state.setSelectedIgnoredRowKey(null);
  });

  return (
    <div class="space-y-8">
      <Show when={activeView() === 'inventory'}>
        <ConnectionsTable
          rows={rows}
          headerActions={headerActions()}
          onManageRow={(row) => handleManageAction(row.manage)}
        />
      </Show>

      <Show when={!readOnlyWorkspace() && activeView() === 'install'}>
        <div class="space-y-6">
          <div class="space-y-3">
            <button
              type="button"
              onClick={closeInstallWorkspace}
              class="inline-flex items-center gap-2 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              Back to monitored systems
            </button>
            <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div class="space-y-1">
                <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                  Add infrastructure
                </div>
                <div class="text-xl font-semibold text-base-content">Choose what to connect</div>
                <div class="text-sm text-muted">
                  Start with a host install or jump straight to an API-backed platform connection.
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowInstallProfiles((value) => !value)}
                class="inline-flex items-center rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
              >
                {showInstallProfiles() ? 'Hide agent profiles' : 'Manage agent profiles'}
              </button>
            </div>
          </div>

          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            <For each={ADD_SYSTEM_CHOICES}>
              {(choice) => (
                <button
                  type="button"
                  onClick={() => handleAddSystem(choice)}
                  class={`flex h-full flex-col items-start justify-between gap-4 rounded-xl border px-4 py-4 text-left transition-colors ${
                    choice.kind === 'agent'
                      ? 'border-blue-300 bg-blue-50/60 text-base-content dark:border-blue-700 dark:bg-blue-950/20'
                      : 'border-border bg-surface hover:bg-surface-hover'
                  }`}
                >
                  <div class="space-y-1.5">
                    <div class="text-sm font-semibold text-base-content">{choice.title}</div>
                    <div class="text-xs text-muted">{choice.description}</div>
                  </div>
                  <span
                    class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                      choice.method === 'api'
                        ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100'
                        : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-100'
                    }`}
                  >
                    {choice.methodLabel}
                  </span>
                </button>
              )}
            </For>
          </div>

          <div class="rounded-xl border border-border bg-surface p-4 shadow-sm sm:p-6">
            <InfrastructureInstallerSection />
          </div>

          <Show when={showInstallProfiles()}>
            <div class="rounded-xl border border-border bg-surface p-4 shadow-sm sm:p-6">
              <div class="mb-4 space-y-1">
                <div class="text-lg font-semibold text-base-content">Agent profiles</div>
                <div class="text-sm text-muted">
                  Manage reusable install defaults for agent-based systems.
                </div>
              </div>
              <AgentProfilesPanel />
            </div>
          </Show>
        </div>
      </Show>

      <Show when={!readOnlyWorkspace() && activeView() === 'platforms'}>
        <div class="space-y-6">
          <div class="space-y-3">
            <button
              type="button"
              onClick={closeConnectionsWorkspace}
              class="inline-flex items-center gap-2 rounded-md border border-border px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              Back to monitored systems
            </button>
            <div class="space-y-1">
              <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                Connections
              </div>
              <div class="text-xl font-semibold text-base-content">Platform connections</div>
              <div class="text-sm text-muted">
                Configure API-backed providers in a full workspace instead of a modal overlay.
              </div>
            </div>
          </div>
          <PlatformConnectionsWorkspace {...props} />
        </div>
      </Show>

      <InfrastructureStopMonitoringDialog />

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
