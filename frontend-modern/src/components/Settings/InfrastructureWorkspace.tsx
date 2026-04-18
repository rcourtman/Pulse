import { Component, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { AddSystemPicker, type AddSystemChoice } from './AddSystemPicker';
import { ConnectionsTable } from './ConnectionsTable';
import {
  buildConnectionRows,
  type ConnectionRow,
  type ConnectionManageAction,
} from './connectionsTableModel';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureInventorySection } from './InfrastructureInventorySection';
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
        navigate(buildInfrastructureWorkspacePath('operations'));
        scrollSectionIntoView(inventorySectionRef);
        return;
      case 'inventory-ignored':
        state.setSelectedIgnoredRowKey(action.rowKey);
        navigate(buildInfrastructureWorkspacePath('operations'));
        scrollSectionIntoView(inventorySectionRef);
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
        onAddSystem={readOnlyWorkspace() ? undefined : () => setPickerOpen(true)}
        onManageRow={(row) => handleManageAction(row.manage)}
      />

      <AddSystemPicker
        isOpen={pickerOpen()}
        onClose={() => setPickerOpen(false)}
        onSelect={handleAddSystem}
      />

      <InfrastructureStopMonitoringDialog />

      <div ref={inventorySectionRef}>
        <InfrastructureInventorySection />
      </div>

      <Show when={!readOnlyWorkspace()}>
        <div ref={platformSectionRef} class="space-y-6">
          <PlatformConnectionsWorkspace {...props} />
        </div>

        <div ref={installSectionRef}>
          <InfrastructureInstallerSection />
        </div>

        <AgentProfilesPanel />
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
