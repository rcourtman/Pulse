import { Component, Match, Switch, createMemo } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { Subtabs } from '@/components/shared/Subtabs';
import { InfrastructureInstallPanel } from './InfrastructureInstallPanel';
import { InfrastructureReportingPanel } from './InfrastructureReportingPanel';
import { ProxmoxSettingsPanel } from './ProxmoxSettingsPanel';
import {
  INFRASTRUCTURE_WORKSPACE_TABS,
  buildInfrastructureWorkspacePath,
  getInfrastructureWorkspaceViewFromPath,
  type InfrastructureWorkspaceView,
} from './infrastructureWorkspaceModel';
import type { ProxmoxSettingsPanelProps } from './proxmoxSettingsModel';

export type InfrastructureWorkspaceProps = ProxmoxSettingsPanelProps;

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const activeView = createMemo(() => getInfrastructureWorkspaceViewFromPath(location.pathname));

  const openView = (view: InfrastructureWorkspaceView) =>
    navigate(buildInfrastructureWorkspacePath(view));

  return (
    <div class="space-y-6">
      <Card padding="lg" class="rounded-xl border border-border shadow-sm">
        <div class="space-y-2">
          <h3 class="text-base font-semibold text-base-content">Infrastructure operations</h3>
          <p class="text-sm text-muted">
            Use this workspace to install Pulse on hosts, manage fallback direct Proxmox
            connections, and control which infrastructure surfaces are actively reporting.
          </p>
          <p class="text-sm text-muted">
            Billing, installed-agent allocation, and Pulse Pro entitlement state live in Pulse
            Pro, not here.
          </p>
        </div>
      </Card>

      <div class="space-y-3">
        <Subtabs
          value={activeView()}
          onChange={(value) => openView(value as InfrastructureWorkspaceView)}
          ariaLabel="Infrastructure workspace"
          tabs={INFRASTRUCTURE_WORKSPACE_TABS.map((tab) => ({
            value: tab.id,
            label: tab.label,
          }))}
        />
      </div>

      <Switch>
        <Match when={activeView() === 'install'}>
          <InfrastructureInstallPanel />
        </Match>

        <Match when={activeView() === 'direct'}>
          <ProxmoxSettingsPanel {...props} embedded />
        </Match>

        <Match when={activeView() === 'inventory'}>
          <InfrastructureReportingPanel
            {...props}
            onManageDirectConnections={() => openView('direct')}
          />
        </Match>
      </Switch>
    </div>
  );
};
