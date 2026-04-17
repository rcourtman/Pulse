import { Component, Match, Switch, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { InfrastructureInstallPanel } from './InfrastructureInstallPanel';
import { InfrastructureReportingPanel } from './InfrastructureReportingPanel';
import { PlatformConnectionsWorkspace } from './PlatformConnectionsWorkspace';
import { ConnectionsTable } from './ConnectionsTable';
import { AddSystemPicker, type AddSystemChoice } from './AddSystemPicker';
import { buildConnectionRows, type ConnectionRow } from './connectionsTableModel';
import {
  buildInfrastructureWorkspacePath,
  getInfrastructureWorkspaceViewFromPath,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const activeView = createMemo(() => getInfrastructureWorkspaceViewFromPath(location.pathname));
  const readOnlyWorkspace = createMemo(() => presentationPolicyIsReadOnly());
  const [pickerOpen, setPickerOpen] = createSignal(false);

  const rows = createMemo<ConnectionRow[]>(() =>
    buildConnectionRows({
      pveNodes: props.pveNodes(),
      pbsNodes: props.pbsNodes(),
      pmgNodes: props.pmgNodes(),
      truenasConnections: props.trueNASSettings.connections(),
      vmwareConnections: props.vmwareSettings.connections(),
      agentResources: props.agentStateResources?.() ?? [],
    }),
  );

  const handleAddSystem = (choice: AddSystemChoice) => {
    setPickerOpen(false);
    if (choice.kind === 'agent') {
      navigate('/settings/infrastructure/install');
      return;
    }
    if (choice.kind === 'truenas') {
      navigate('/settings/infrastructure/platforms/truenas');
      return;
    }
    if (choice.kind === 'vmware') {
      navigate('/settings/infrastructure/platforms/vmware');
      return;
    }
    props.onSelectAgent(choice.kind);
    navigate('/settings/infrastructure/platforms/proxmox');
  };

  createEffect(() => {
    if (readOnlyWorkspace() && activeView() === 'install') {
      navigate(buildInfrastructureWorkspacePath('inventory'), { replace: true });
    }
  });

  return (
    <div class="space-y-6">
      <Switch>
        <Match when={activeView() === 'inventory'}>
          <ConnectionsTable
            rows={rows}
            onAddSystem={readOnlyWorkspace() ? undefined : () => setPickerOpen(true)}
          />
          <AddSystemPicker
            isOpen={pickerOpen()}
            onClose={() => setPickerOpen(false)}
            onSelect={handleAddSystem}
          />
        </Match>

        <Match when={activeView() === 'install'}>
          <InfrastructureInstallPanel />
        </Match>

        <Match when={activeView() === 'platforms'}>
          <PlatformConnectionsWorkspace {...props} />
        </Match>

        <Match when={activeView() === 'operations'}>
          <InfrastructureReportingPanel
            {...props}
            onManagePlatformConnections={() =>
              navigate('/settings/infrastructure/platforms')
            }
          />
        </Match>
      </Switch>
    </div>
  );
};
