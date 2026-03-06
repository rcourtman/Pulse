import { Component, Show, createEffect, createMemo, createSignal, Match, Switch } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import Server from 'lucide-solid/icons/server';
import Boxes from 'lucide-solid/icons/boxes';
import Waypoints from 'lucide-solid/icons/waypoints';
import { Card } from '@/components/shared/Card';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { DockerRuntimeSettingsCard } from './DockerRuntimeSettingsCard';
import { ProxmoxSettingsPanel, type ProxmoxSettingsPanelProps } from './ProxmoxSettingsPanel';
import { UnifiedAgents } from './UnifiedAgents';

type InfrastructureWorkspaceView = 'install' | 'direct';

const VIEW_META: Record<
  InfrastructureWorkspaceView,
  {
    description: string;
  }
> = {
  install: {
    description:
      'Use one installer for hosts, Docker, Kubernetes, and agent-managed Proxmox. This is the normal onboarding path.',
  },
  direct: {
    description:
      'Use direct PVE, PBS, or PMG connections only when the unified agent cannot run on the target host.',
  },
};

const inferViewFromPath = (pathname: string): InfrastructureWorkspaceView =>
  pathname.startsWith('/settings/infrastructure/proxmox') ? 'direct' : 'install';

export interface InfrastructureWorkspaceProps extends ProxmoxSettingsPanelProps {
  disableDockerUpdateActions: () => boolean;
  disableDockerUpdateActionsLocked: () => boolean;
  savingDockerUpdateActions: () => boolean;
  handleDisableDockerUpdateActionsChange: (disabled: boolean) => Promise<void>;
}

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [activeView, setActiveView] = createSignal<InfrastructureWorkspaceView>(
    inferViewFromPath(location.pathname),
  );
  const [showInventory, setShowInventory] = createSignal(false);

  createEffect(() => {
    if (location.pathname.startsWith('/settings/infrastructure/proxmox')) {
      setActiveView('direct');
    }
  });

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
        <div class="space-y-4">
          <div class="space-y-2">
            <p class="text-[11px] font-semibold uppercase tracking-[0.22em] text-blue-600 dark:text-blue-300">
              Infrastructure
            </p>
            <h2 class="text-[1.75rem] font-semibold leading-tight text-base-content">
              Add and manage infrastructure
            </h2>
            <p class="max-w-3xl text-sm leading-6 text-muted">
              Start with the unified agent. Use direct Proxmox connections only when the agent
              cannot run on the target host.
            </p>
          </div>

          <div class="space-y-3">
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
            </div>

            <div class="text-sm text-muted">
              {activeViewMeta().description}
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
      </Switch>

      <Card padding="lg" class="rounded-xl border border-border shadow-sm">
        <div class="space-y-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="space-y-1">
              <div class="flex items-center gap-2 text-base font-semibold text-base-content">
                <Boxes class="h-4 w-4" />
                Connected infrastructure
              </div>
              <p class="text-sm text-muted">
                View installed agents, direct Proxmox links, Docker policy, and agent profiles.
              </p>
            </div>
            <button
              type="button"
              onClick={() => setShowInventory((current) => !current)}
              class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              {showInventory() ? 'Hide details' : 'Show details'}
            </button>
          </div>

          <Show when={showInventory()}>
            <div class="space-y-6 border-t border-border pt-4">
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
                      <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
                        <div class="text-sm font-medium text-base-content">PVE</div>
                        <div class="mt-1 text-xl font-semibold text-base-content">
                          {props.pveNodes().length}
                        </div>
                      </div>
                      <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
                        <div class="text-sm font-medium text-base-content">PBS</div>
                        <div class="mt-1 text-xl font-semibold text-base-content">
                          {props.pbsNodes().length}
                        </div>
                      </div>
                      <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
                        <div class="text-sm font-medium text-base-content">PMG</div>
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
              </div>

              <AgentProfilesPanel />
            </div>
          </Show>
        </div>
      </Card>
    </div>
  );
};
