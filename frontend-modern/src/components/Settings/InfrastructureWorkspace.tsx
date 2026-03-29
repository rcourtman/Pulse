import { Component, Match, Switch, createMemo } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { Subtabs } from '@/components/shared/Subtabs';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import { InfrastructureInstallPanel } from './InfrastructureInstallPanel';
import { InfrastructureReportingPanel } from './InfrastructureReportingPanel';
import { PlatformConnectionsWorkspace } from './PlatformConnectionsWorkspace';
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
  const installPath = createMemo(() => buildInfrastructureWorkspacePath('install'));
  const platformsPath = createMemo(() => buildInfrastructureWorkspacePath('platforms'));

  const openView = (view: InfrastructureWorkspaceView) =>
    navigate(buildInfrastructureWorkspacePath(view));

  return (
    <div class="space-y-6">
      <Card padding="lg" class="rounded-xl border border-border shadow-sm">
        <div class="space-y-4">
          <div class="space-y-2">
            <h3 class="text-base font-semibold text-base-content">Connect your first system</h3>
            <p class="text-sm text-muted">
              Start with Install on a host to connect the first machine you want Pulse to monitor.
              If you already know you want an API-backed platform such as Proxmox or TrueNAS
              instead, go straight to Platform connections.
            </p>
          </div>
          <div class="grid gap-3 lg:grid-cols-3">
            <div class="rounded-md border border-border bg-surface px-4 py-3">
              <p class="text-xs font-semibold uppercase tracking-wide text-muted">1. Choose path</p>
              <p class="mt-1 text-sm text-base-content">
                Install Pulse on a host first, or open Platform connections for Proxmox, TrueNAS,
                and other systems Pulse should poll through their own APIs.
              </p>
            </div>
            <div class="rounded-md border border-border bg-surface px-4 py-3">
              <p class="text-xs font-semibold uppercase tracking-wide text-muted">
                2. Generate access
              </p>
              <p class="mt-1 text-sm text-base-content">
                Create the install token Pulse expects for the first monitored host, or add the API
                credentials Pulse should store for API-backed platforms like Proxmox and TrueNAS.
              </p>
            </div>
            <div class="rounded-md border border-border bg-surface px-4 py-3">
              <p class="text-xs font-semibold uppercase tracking-wide text-muted">
                3. Confirm reporting
              </p>
              <p class="mt-1 text-sm text-base-content">
                Run the command on that machine, then use Reporting &amp; control once the first
                system starts reporting.
              </p>
            </div>
          </div>
          <div class="flex flex-wrap gap-3">
            <button
              type="button"
              onClick={() => navigate(installPath())}
              class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                activeView() === 'install'
                  ? 'bg-blue-600 text-white'
                  : 'border border-border bg-surface text-base-content hover:bg-surface-hover'
              }`}
            >
              {activeView() === 'install' ? 'Install on a host selected' : 'Open Install on a host'}
            </button>
            <button
              type="button"
              onClick={() => navigate(platformsPath())}
              class={`inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                activeView() === 'platforms'
                  ? 'bg-emerald-600 text-white'
                  : 'border border-border bg-surface text-base-content hover:bg-surface-hover'
              }`}
            >
              {activeView() === 'platforms'
                ? 'Platform connections selected'
                : 'Open Platform connections'}
            </button>
          </div>
          <p class="text-sm text-muted">
            {SELF_HOSTED_PRO_BILLING_PRESENTATION.infrastructureWorkspaceReferral}
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

        <Match when={activeView() === 'platforms'}>
          <PlatformConnectionsWorkspace {...props} />
        </Match>

        <Match when={activeView() === 'inventory'}>
          <InfrastructureReportingPanel
            {...props}
            onManagePlatformConnections={() => openView('platforms')}
          />
        </Match>
      </Switch>
    </div>
  );
};
