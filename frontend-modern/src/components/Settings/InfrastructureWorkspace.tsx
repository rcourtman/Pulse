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

const VIEW_META: Record<
  InfrastructureWorkspaceView,
  {
    label: string;
    title: string;
    description: string;
    accentClass: string;
  }
> = {
  install: {
    label: 'Recommended path',
    title: 'Install the unified agent',
    description:
      'Use one installer for hosts, Docker, Kubernetes, and agent-managed Proxmox. This is the normal onboarding path.',
    accentClass:
      'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-100',
  },
  direct: {
    label: 'Fallback path',
    title: 'Connect Proxmox directly',
    description:
      'Use direct PVE, PBS, or PMG connections only when the unified agent cannot run on the target host.',
    accentClass:
      'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100',
  },
  inventory: {
    label: 'Operations',
    title: 'Review connected infrastructure',
    description:
      'Audit agent coverage, direct Proxmox links, Docker runtime policy, and supporting infrastructure controls from one place.',
    accentClass: 'border-border bg-surface-alt text-base-content',
  },
};

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

  const activeViewMeta = createMemo(() => VIEW_META[activeView()]);

  return (
    <div class="space-y-6">
      <Card padding="lg" class="rounded-xl border border-border shadow-sm">
        <div class="space-y-6">
          <div class="space-y-3">
            <p class="text-[11px] font-semibold uppercase tracking-[0.22em] text-blue-600 dark:text-blue-300">
              Infrastructure
            </p>
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div class="max-w-3xl space-y-2">
                <h2 class="text-[1.9rem] font-semibold leading-tight text-base-content">
                  Add and manage infrastructure from one workspace
                </h2>
                <p class="text-sm leading-6 text-muted">
                  Use the unified agent for normal host onboarding, fall back to direct Proxmox
                  connections only when needed, and keep the connected estate visible in one place.
                </p>
              </div>
              <div class="min-w-[220px] rounded-xl border border-border bg-gradient-to-br from-surface-alt to-surface px-5 py-4">
                <div class="text-[11px] uppercase tracking-[0.16em] text-muted">
                  Connected endpoints
                </div>
                <div class="mt-1 text-3xl font-semibold leading-none text-base-content">
                  {totalManagedEndpoints()}
                </div>
                <div class="mt-2 text-xs text-muted">
                  Agent-managed and direct infrastructure visible to Pulse.
                </div>
              </div>
            </div>
          </div>

          <div class="space-y-4">
            <div class="inline-flex w-full flex-col gap-2 rounded-xl border border-border bg-surface-alt p-2 lg:w-auto lg:flex-row">
              <button
                type="button"
                onClick={() => openView('install')}
                class={`inline-flex min-h-10 items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                  activeView() === 'install'
                    ? 'bg-surface text-base-content shadow-sm'
                    : 'text-muted hover:bg-surface hover:text-base-content'
                }`}
              >
                <Server class="h-4 w-4" />
                Install on a host
              </button>
              <button
                type="button"
                onClick={() => openView('direct')}
                class={`inline-flex min-h-10 items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                  activeView() === 'direct'
                    ? 'bg-surface text-base-content shadow-sm'
                    : 'text-muted hover:bg-surface hover:text-base-content'
                }`}
              >
                <Waypoints class="h-4 w-4" />
                Direct Proxmox
              </button>
              <button
                type="button"
                onClick={() => openView('inventory')}
                class={`inline-flex min-h-10 items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                  activeView() === 'inventory'
                    ? 'bg-surface text-base-content shadow-sm'
                    : 'text-muted hover:bg-surface hover:text-base-content'
                }`}
              >
                <Boxes class="h-4 w-4" />
                Inventory
              </button>
            </div>

            <div class={`rounded-xl border px-5 py-4 ${activeViewMeta().accentClass}`}>
              <div class="flex flex-col gap-1">
                <div class="text-[11px] font-semibold uppercase tracking-[0.16em] opacity-80">
                  {activeViewMeta().label}
                </div>
                <h3 class="text-lg font-semibold">{activeViewMeta().title}</h3>
                <p class="max-w-3xl text-sm leading-6 opacity-90">{activeViewMeta().description}</p>
              </div>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-xl border border-border bg-surface px-4 py-4">
              <div class="text-[11px] uppercase tracking-[0.16em] text-muted">Managed hosts</div>
              <div class="mt-2 text-2xl font-semibold text-base-content">{summary().managedHosts}</div>
              <div class="mt-1 text-xs text-muted">Unified agent installed</div>
            </div>
            <div class="rounded-xl border border-border bg-surface px-4 py-4">
              <div class="text-[11px] uppercase tracking-[0.16em] text-muted">Docker runtimes</div>
              <div class="mt-2 text-2xl font-semibold text-base-content">
                {summary().dockerRuntimes}
              </div>
              <div class="mt-1 text-xs text-muted">Discovered through agents</div>
            </div>
            <div class="rounded-xl border border-border bg-surface px-4 py-4">
              <div class="text-[11px] uppercase tracking-[0.16em] text-muted">
                Kubernetes clusters
              </div>
              <div class="mt-2 text-2xl font-semibold text-base-content">
                {summary().kubernetesClusters}
              </div>
              <div class="mt-1 text-xs text-muted">Managed through node installs</div>
            </div>
            <div class="rounded-xl border border-border bg-surface px-4 py-4">
              <div class="text-[11px] uppercase tracking-[0.16em] text-muted">Direct Proxmox</div>
              <div class="mt-2 text-2xl font-semibold text-base-content">
                {summary().proxmoxDirect}
              </div>
              <div class="mt-1 text-xs text-muted">Fallback direct connections</div>
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

            <div class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
              <Card padding="lg" class="rounded-xl border border-border shadow-sm">
                <div class="space-y-4">
                  <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <h3 class="text-base font-semibold text-base-content">
                        Direct Proxmox connections
                      </h3>
                      <p class="text-sm text-muted">
                        Review fallback direct coverage separately from agent-managed hosts.
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
                    <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
                      <div class="text-[11px] uppercase tracking-[0.16em] text-muted">PVE</div>
                      <div class="mt-2 text-2xl font-semibold text-base-content">
                        {props.pveNodes().length}
                      </div>
                    </div>
                    <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
                      <div class="text-[11px] uppercase tracking-[0.16em] text-muted">PBS</div>
                      <div class="mt-2 text-2xl font-semibold text-base-content">
                        {props.pbsNodes().length}
                      </div>
                    </div>
                    <div class="rounded-lg border border-border bg-surface-alt px-4 py-4">
                      <div class="text-[11px] uppercase tracking-[0.16em] text-muted">PMG</div>
                      <div class="mt-2 text-2xl font-semibold text-base-content">
                        {props.pmgNodes().length}
                      </div>
                    </div>
                  </div>

                  <div class="rounded-lg border border-border bg-surface-alt px-4 py-4 text-sm text-muted">
                    Keep direct connections limited to hosts where the unified agent cannot run.
                    Everything else should stay on the default install path.
                  </div>
                </div>
              </Card>

              <DockerRuntimeSettingsCard
                disableDockerUpdateActions={props.disableDockerUpdateActions}
                disableDockerUpdateActionsLocked={props.disableDockerUpdateActionsLocked}
                savingDockerUpdateActions={props.savingDockerUpdateActions}
                handleDisableDockerUpdateActionsChange={props.handleDisableDockerUpdateActionsChange}
              />
            </div>

            <AgentProfilesPanel />
          </div>
        </Match>
      </Switch>
    </div>
  );
};
