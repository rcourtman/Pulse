import { Component, createEffect, createSignal, Match, Switch } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ProxmoxSettingsPanel, type ProxmoxSettingsPanelProps } from './ProxmoxSettingsPanel';
import { UnifiedAgents } from './UnifiedAgents';

type InfrastructureWorkspaceView = 'install' | 'direct' | 'inventory';

const inferViewFromPath = (pathname: string): InfrastructureWorkspaceView =>
  pathname.startsWith('/settings/infrastructure/proxmox') ? 'direct' : 'install';

export type InfrastructureWorkspaceProps = ProxmoxSettingsPanelProps;

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [activeView, setActiveView] = createSignal<InfrastructureWorkspaceView>(
    inferViewFromPath(location.pathname),
  );

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

  return (
    <div class="space-y-6">
      <div class="space-y-3">
        <div class="border-b border-border">
          <div class="flex flex-wrap items-center gap-6">
            <button
              type="button"
              onClick={() => openView('install')}
              class={`inline-flex min-h-10 items-center border-b-2 px-1 py-2 text-sm font-medium transition-colors ${
                activeView() === 'install'
                  ? 'border-blue-600 text-base-content'
                  : 'border-transparent text-muted hover:text-base-content'
              }`}
            >
              Install on a host
            </button>
            <button
              type="button"
              onClick={() => openView('direct')}
              class={`inline-flex min-h-10 items-center border-b-2 px-1 py-2 text-sm font-medium transition-colors ${
                activeView() === 'direct'
                  ? 'border-blue-600 text-base-content'
                  : 'border-transparent text-muted hover:text-base-content'
              }`}
            >
              Direct Proxmox
            </button>
            <button
              type="button"
              onClick={() => openView('inventory')}
              class={`inline-flex min-h-10 items-center border-b-2 px-1 py-2 text-sm font-medium transition-colors ${
                activeView() === 'inventory'
                  ? 'border-blue-600 text-base-content'
                  : 'border-transparent text-muted hover:text-base-content'
              }`}
            >
              Connected infrastructure
            </button>
          </div>
        </div>
      </div>

      <Switch>
        <Match when={activeView() === 'install'}>
          <UnifiedAgents embedded showInventory={false} />
        </Match>

        <Match when={activeView() === 'direct'}>
          <ProxmoxSettingsPanel {...props} embedded />
        </Match>

        <Match when={activeView() === 'inventory'}>
          <div class="space-y-6">
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
              </div>

              <AgentProfilesPanel />
            </div>
          </div>
        </Match>
      </Switch>
    </div>
  );
};
