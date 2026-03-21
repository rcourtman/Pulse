import type { Component, JSX } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { getSettingsConfigurationLoadingState } from '@/utils/settingsShellPresentation';
import type { ProxmoxVariantPresentation } from '@/utils/proxmoxSettingsPresentation';
import type { NodeConfigWithStatus } from '@/types/nodes';
import type { DiscoveredServer, NodeType } from './infrastructureSettingsModel';

interface ProxmoxDirectConnectionsCardProps {
  activeAgent: NodeType;
  activeConfig: ProxmoxVariantPresentation;
  activeConfiguredNodes: NodeConfigWithStatus[];
  activeDiscoveredNodes: DiscoveredServer[];
  configuredTable: JSX.Element;
  discoveryEnabled: boolean;
  envOverrides: Record<string, boolean>;
  initialLoadComplete: boolean;
  onDiscoveryToggle: (event: ToggleChangeEvent) => Promise<void>;
  onOpenCreateNode: (type: NodeType) => void;
  onRefreshDiscovery: () => Promise<void>;
  savingDiscoverySettings: boolean;
}

export const ProxmoxDirectConnectionsCard: Component<ProxmoxDirectConnectionsCardProps> = (
  props,
) => {
  return (
    <>
      <Show when={!props.initialLoadComplete}>
        <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
          {getSettingsConfigurationLoadingState().text}
        </div>
      </Show>

      <Show when={props.initialLoadComplete}>
        <Card padding="none" class="rounded-xl border border-border shadow-sm">
          <div class="px-3 py-4 sm:px-6 sm:py-6 space-y-4">
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
              <div class="space-y-2">
                <h4 class="text-base font-semibold text-base-content">{props.activeConfig.title}</h4>
                <div class="flex flex-wrap items-center gap-2 text-xs">
                  <span class="inline-flex items-center rounded-full bg-surface-alt px-2.5 py-1 font-medium text-base-content">
                    {props.activeConfiguredNodes.length} configured
                  </span>
                  <Show when={props.discoveryEnabled}>
                    <span class="inline-flex items-center rounded-full bg-blue-50 px-2.5 py-1 font-medium text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                      {props.activeDiscoveredNodes.length} discovered
                    </span>
                  </Show>
                </div>
              </div>
              <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                <div
                  class="flex items-center gap-2 sm:gap-3"
                  title="Enable automatic discovery of Proxmox servers on your network"
                >
                  <span class="text-xs sm:text-sm text-muted">Discovery</span>
                  <Toggle
                    checked={props.discoveryEnabled}
                    onChange={props.onDiscoveryToggle}
                    disabled={
                      props.envOverrides.discoveryEnabled || props.savingDiscoverySettings
                    }
                    containerClass="gap-2"
                    label={
                      <span class="text-xs font-medium text-muted">
                        {props.discoveryEnabled ? 'On' : 'Off'}
                      </span>
                    }
                  />
                </div>

                <Show when={props.discoveryEnabled}>
                  <button
                    type="button"
                    onClick={props.onRefreshDiscovery}
                    class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-md hover:bg-gray-700 transition-colors flex items-center gap-1"
                    title="Refresh discovered servers"
                  >
                    <svg
                      width="16"
                      height="16"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <polyline points="23 4 23 10 17 10"></polyline>
                      <polyline points="1 20 1 14 7 14"></polyline>
                      <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                    </svg>
                    <span class="hidden sm:inline">Refresh</span>
                  </button>
                </Show>

                <button
                  type="button"
                  onClick={() => props.onOpenCreateNode(props.activeAgent)}
                  class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-1"
                >
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <line x1="12" y1="5" x2="12" y2="19"></line>
                    <line x1="5" y1="12" x2="19" y2="12"></line>
                  </svg>
                  <span class="sm:hidden">Add</span>
                  <span class="hidden sm:inline">{props.activeConfig.addLabel}</span>
                </button>
              </div>
            </div>

            <Show when={props.activeConfiguredNodes.length > 0}>{props.configuredTable}</Show>

            <Show
              when={
                props.activeConfiguredNodes.length === 0 &&
                props.activeDiscoveredNodes.length === 0
              }
            >
              <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                <div class="rounded-full bg-surface-alt p-4 mb-4">
                  <Dynamic component={props.activeConfig.emptyIcon} class="h-8 w-8 text-muted" />
                </div>
                <p class="text-base font-medium text-base-content mb-1">
                  {props.activeConfig.emptyTitle}
                </p>
                <p class="text-sm text-muted">{props.activeConfig.emptyDescription}</p>
              </div>
            </Show>
          </div>
        </Card>
      </Show>
    </>
  );
};
