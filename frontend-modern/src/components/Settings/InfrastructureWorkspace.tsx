import { Component, createEffect, createMemo, createSignal, Match, Switch } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import Server from 'lucide-solid/icons/server';
import Boxes from 'lucide-solid/icons/boxes';
import Waypoints from 'lucide-solid/icons/waypoints';
import type { Resource } from '@/types/resource';
import { Card } from '@/components/shared/Card';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { DockerRuntimeSettingsCard } from './DockerRuntimeSettingsCard';
import { ProxmoxSettingsPanel, type ProxmoxSettingsPanelProps } from './ProxmoxSettingsPanel';
import { UnifiedAgents } from './UnifiedAgents';
import { useResources } from '@/hooks/useResources';

type InfrastructureWorkspaceView = 'install' | 'direct' | 'inventory';

const inferViewFromPath = (pathname: string): InfrastructureWorkspaceView =>
  pathname.startsWith('/settings/infrastructure/proxmox') ? 'direct' : 'install';

const countManagedHosts = (resources: Resource[]) =>
  resources.filter((resource) => resource.agent != null).length;

export interface InfrastructureWorkspaceProps extends ProxmoxSettingsPanelProps {
  disableDockerUpdateActions: () => boolean;
  disableDockerUpdateActionsLocked: () => boolean;
  savingDockerUpdateActions: () => boolean;
  handleDisableDockerUpdateActionsChange: (disabled: boolean) => Promise<void>;
}

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const { resources, byType } = useResources();
  const [activeView, setActiveView] = createSignal<InfrastructureWorkspaceView>(
    inferViewFromPath(location.pathname),
  );

  createEffect(() => {
    if (location.pathname.startsWith('/settings/infrastructure/proxmox')) {
      setActiveView('direct');
    }
  });

  const summary = createMemo(() => {
    const allResources = resources();
    return {
      managedHosts: countManagedHosts(allResources),
      dockerRuntimes: byType('docker-host').length,
      kubernetesClusters: byType('k8s-cluster').length,
      proxmoxDirect: props.pveNodes().length + props.pbsNodes().length + props.pmgNodes().length,
    };
  });

  const totalManagedEndpoints = createMemo(
    () =>
      summary().managedHosts +
      summary().dockerRuntimes +
      summary().kubernetesClusters +
      summary().proxmoxDirect,
  );

  const openView = (view: InfrastructureWorkspaceView) => {
    setActiveView(view);
    if (view === 'direct') {
      navigate('/settings/infrastructure/proxmox');
      return;
    }
    if (location.pathname.startsWith('/settings/infrastructure/proxmox')) {
      navigate('/settings');
    }
  };

  return (
    <div class="space-y-6">
      <Card padding="lg" class="rounded-xl border border-border shadow-sm">
        <div class="space-y-5">
          <div class="space-y-2">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600 dark:text-blue-300">
              Infrastructure
            </p>
            <div class="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
              <div class="space-y-1">
                <h2 class="text-2xl font-semibold text-base-content">
                  Add and manage infrastructure from one workspace
                </h2>
                <p class="max-w-3xl text-sm text-muted">
                  Use the unified agent for normal host onboarding, fall back to direct Proxmox
                  connections only when needed, and keep the connected estate visible in one place.
                </p>
              </div>
              <div class="rounded-lg border border-border bg-surface-alt px-4 py-3 text-right">
                <div class="text-[11px] uppercase tracking-wide text-muted">Connected endpoints</div>
                <div class="text-2xl font-semibold text-base-content">{totalManagedEndpoints()}</div>
              </div>
            </div>
          </div>

          <div class="grid gap-3 lg:grid-cols-3">
            <button
              type="button"
              onClick={() => openView('install')}
              class={`rounded-xl border p-4 text-left transition-colors ${
                activeView() === 'install'
                  ? 'border-blue-500 bg-blue-50 shadow-sm dark:border-blue-400 dark:bg-blue-950/40'
                  : 'border-border bg-surface hover:border-blue-300 hover:bg-surface-hover'
              }`}
            >
              <div class="flex items-start justify-between gap-3">
                <div class="space-y-1">
                  <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
                    <Server class="h-4 w-4" />
                    Install on a host
                  </div>
                  <p class="text-sm text-muted">
                    Recommended path for Linux, macOS, Windows, Docker, Kubernetes, and
                    agent-managed Proxmox.
                  </p>
                </div>
                <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                  Default
                </span>
              </div>
            </button>

            <button
              type="button"
              onClick={() => openView('direct')}
              class={`rounded-xl border p-4 text-left transition-colors ${
                activeView() === 'direct'
                  ? 'border-amber-500 bg-amber-50 shadow-sm dark:border-amber-400 dark:bg-amber-950/40'
                  : 'border-border bg-surface hover:border-amber-300 hover:bg-surface-hover'
              }`}
            >
              <div class="space-y-1">
                <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
                  <Waypoints class="h-4 w-4" />
                  Connect Proxmox directly
                </div>
                <p class="text-sm text-muted">
                  Use direct PVE, PBS, or PMG connections only when the unified agent cannot be
                  installed on the host.
                </p>
              </div>
            </button>

            <button
              type="button"
              onClick={() => openView('inventory')}
              class={`rounded-xl border p-4 text-left transition-colors ${
                activeView() === 'inventory'
                  ? 'border-slate-500 bg-surface-alt shadow-sm dark:border-slate-400'
                  : 'border-border bg-surface hover:border-slate-300 hover:bg-surface-hover'
              }`}
            >
              <div class="space-y-1">
                <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
                  <Boxes class="h-4 w-4" />
                  View connected infrastructure
                </div>
                <p class="text-sm text-muted">
                  Review agent inventory, connected runtimes, direct Proxmox coverage, and
                  infrastructure policies.
                </p>
              </div>
            </button>
          </div>

          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-lg border border-border bg-surface px-4 py-3">
              <div class="text-[11px] uppercase tracking-wide text-muted">Managed hosts</div>
              <div class="mt-1 text-xl font-semibold text-base-content">{summary().managedHosts}</div>
            </div>
            <div class="rounded-lg border border-border bg-surface px-4 py-3">
              <div class="text-[11px] uppercase tracking-wide text-muted">Docker runtimes</div>
              <div class="mt-1 text-xl font-semibold text-base-content">
                {summary().dockerRuntimes}
              </div>
            </div>
            <div class="rounded-lg border border-border bg-surface px-4 py-3">
              <div class="text-[11px] uppercase tracking-wide text-muted">Kubernetes clusters</div>
              <div class="mt-1 text-xl font-semibold text-base-content">
                {summary().kubernetesClusters}
              </div>
            </div>
            <div class="rounded-lg border border-border bg-surface px-4 py-3">
              <div class="text-[11px] uppercase tracking-wide text-muted">Direct Proxmox nodes</div>
              <div class="mt-1 text-xl font-semibold text-base-content">
                {summary().proxmoxDirect}
              </div>
            </div>
          </div>
        </div>
      </Card>

      <Switch>
        <Match when={activeView() === 'install'}>
          <UnifiedAgents embedded showInventory={false} />
        </Match>

        <Match when={activeView() === 'direct'}>
          <ProxmoxSettingsPanel {...props} embedded />
        </Match>

        <Match when={activeView() === 'inventory'}>
          <div class="space-y-6">
            <UnifiedAgents embedded showInstaller={false} />

            <Card padding="lg" class="rounded-xl border border-border shadow-sm">
              <div class="space-y-3">
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <h3 class="text-base font-semibold text-base-content">
                      Direct Proxmox connections
                    </h3>
                    <p class="text-sm text-muted">
                      Review direct Proxmox coverage separately from agent-managed hosts.
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => openView('direct')}
                    class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                  >
                    Manage direct connections
                  </button>
                </div>

                <div class="grid gap-3 sm:grid-cols-3">
                  <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">PVE</div>
                    <div class="mt-1 text-xl font-semibold text-base-content">
                      {props.pveNodes().length}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">PBS</div>
                    <div class="mt-1 text-xl font-semibold text-base-content">
                      {props.pbsNodes().length}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">PMG</div>
                    <div class="mt-1 text-xl font-semibold text-base-content">
                      {props.pmgNodes().length}
                    </div>
                  </div>
                </div>
              </div>
            </Card>

            <DockerRuntimeSettingsCard
              disableDockerUpdateActions={props.disableDockerUpdateActions}
              disableDockerUpdateActionsLocked={props.disableDockerUpdateActionsLocked}
              savingDockerUpdateActions={props.savingDockerUpdateActions}
              handleDisableDockerUpdateActionsChange={props.handleDisableDockerUpdateActionsChange}
            />

            <AgentProfilesPanel />
          </div>
        </Match>
      </Switch>
    </div>
  );
};
