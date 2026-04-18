import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { AddSystemPicker, type AddSystemChoice } from './AddSystemPicker';
import { ConnectionsTable, type ConnectionsTableHeaderAction } from './ConnectionsTable';
import {
  buildConnectionRows,
  type ConnectionRow,
  type ConnectionManageAction,
} from './connectionsTableModel';
import { InfrastructureActiveRowDetails } from './InfrastructureActiveRowDetails';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureInventorySection } from './InfrastructureInventorySection';
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

const scrollSectionIntoView = (section?: HTMLDivElement) => {
  if (
    typeof window === 'undefined' ||
    !section ||
    typeof section.scrollIntoView !== 'function'
  ) {
    return;
  }

  window.requestAnimationFrame(() => {
    section.scrollIntoView({ block: 'start', behavior: 'smooth' });
  });
};

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

  let inventorySectionRef: HTMLDivElement | undefined;
  let platformSectionRef: HTMLDivElement | undefined;
  let installSectionRef: HTMLDivElement | undefined;

  const rows = createMemo<ConnectionRow[]>(() =>
    buildConnectionRows({
      activeRows: state.activeRows(),
      monitoringStoppedRows: state.monitoringStoppedRows(),
      pveNodes: props.pveNodes(),
      pbsNodes: props.pbsNodes(),
      pmgNodes: props.pmgNodes(),
      truenasConnections: props.trueNASSettings.connections(),
      vmwareConnections: props.vmwareSettings.connections(),
      includeConfigurationRows: !readOnlyWorkspace(),
    }),
  );
  const headerActions = createMemo<ConnectionsTableHeaderAction[]>(() =>
    readOnlyWorkspace()
      ? []
      : [
          {
            label: 'Agent profiles',
            onSelect: () => setProfilesOpen(true),
            tone: 'secondary' as const,
          },
          {
            label: '+ Add a system',
            onSelect: () => setPickerOpen(true),
            tone: 'primary' as const,
          },
        ],
  );

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
    navigate(proxmoxRouteForKind(nodeKind));
    scrollSectionIntoView(platformSectionRef);
  };

  const handleAddSystem = (choice: AddSystemChoice) => {
    setPickerOpen(false);

    if (choice.kind === 'agent') {
      navigate(buildInfrastructureWorkspacePath('install'));
      scrollSectionIntoView(installSectionRef);
      return;
    }

    if (choice.kind === 'truenas') {
      props.trueNASSettings.openCreateDialog();
      navigate('/settings/infrastructure/platforms/truenas');
      scrollSectionIntoView(platformSectionRef);
      return;
    }

    if (choice.kind === 'vmware') {
      props.vmwareSettings.openCreateDialog();
      navigate('/settings/infrastructure/platforms/vmware');
      scrollSectionIntoView(platformSectionRef);
      return;
    }

    openProxmoxNode(choice.kind, '');
  };

  const handleManageAction = (action: ConnectionManageAction) => {
    switch (action.kind) {
      case 'inventory-active':
        state.setExpandedRowKey(action.rowKey);
        state.setSelectedIgnoredRowKey(null);
        return;
      case 'inventory-ignored':
        state.setExpandedRowKey(null);
        state.setSelectedIgnoredRowKey(action.rowKey);
        return;
      case 'proxmox-node':
        openProxmoxNode(action.nodeKind, action.nodeId);
        return;
      case 'truenas-connection': {
        const connection = props
          .trueNASSettings
          .connections()
          .find((candidate) => candidate.id === action.connectionId);
        if (connection) {
          props.trueNASSettings.openEditDialog(connection);
        }
        navigate('/settings/infrastructure/platforms/truenas');
        scrollSectionIntoView(platformSectionRef);
        return;
      }
      case 'vmware-connection': {
        const connection = props
          .vmwareSettings
          .connections()
          .find((candidate) => candidate.id === action.connectionId);
        if (connection) {
          props.vmwareSettings.openEditDialog(connection);
        }
        navigate('/settings/infrastructure/platforms/vmware');
        scrollSectionIntoView(platformSectionRef);
        return;
      }
      default:
        return;
    }
  };

  createEffect(() => {
    if (readOnlyWorkspace() && activeView() !== 'inventory') {
      navigate(buildInfrastructureWorkspacePath('inventory'), { replace: true });
      return;
    }

    const view = activeView();
    if (view === 'inventory') {
      return;
    }

    if (view === 'operations') {
      scrollSectionIntoView(inventorySectionRef);
      return;
    }

    if (view === 'platforms') {
      scrollSectionIntoView(platformSectionRef);
      return;
    }

    if (view === 'install') {
      scrollSectionIntoView(installSectionRef);
    }
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
      />

      <InfrastructureStopMonitoringDialog />

      <Dialog
        isOpen={profilesOpen()}
        onClose={() => setProfilesOpen(false)}
        layout="drawer-right"
        panelClass="max-w-[960px]"
        ariaLabel="Agent profiles"
      >
        <div class="h-full overflow-y-auto bg-surface p-4 sm:p-6">
          <AgentProfilesPanel />
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

      <Show when={activeView() === 'operations'}>
        <div ref={inventorySectionRef}>
          <InfrastructureInventorySection />
        </div>
      </Show>

      <Show when={!readOnlyWorkspace() && activeView() === 'platforms'}>
        <div ref={platformSectionRef} class="space-y-6">
          <PlatformConnectionsWorkspace {...props} />
        </div>
      </Show>

      <Show when={!readOnlyWorkspace() && activeView() === 'install'}>
        <div ref={installSectionRef}>
          <InfrastructureInstallerSection />
        </div>
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
