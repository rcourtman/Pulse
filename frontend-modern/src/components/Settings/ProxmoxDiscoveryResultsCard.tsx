import type { Component } from 'solid-js';
import { For, Show } from 'solid-js';
import Loader from 'lucide-solid/icons/loader';
import { Card } from '@/components/shared/Card';
import { formatRelativeTime } from '@/utils/format';
import type { ProxmoxVariantPresentation } from '@/utils/proxmoxSettingsPresentation';
import type { DiscoveryScanStatus, DiscoveredServer } from './useInfrastructureSettingsState';

interface ProxmoxDiscoveryResultsCardProps {
  activeConfig: ProxmoxVariantPresentation;
  activeDiscoveredNodes: DiscoveredServer[];
  discoveryScanStatus: DiscoveryScanStatus;
  hasDiscoveryTimeouts: boolean;
  onOpenDiscoveredNode: (server: DiscoveredServer) => void;
}

export const ProxmoxDiscoveryResultsCard: Component<ProxmoxDiscoveryResultsCardProps> = (
  props,
) => {
  return (
    <Card padding="lg" class="rounded-xl border border-border shadow-sm">
      <div class="space-y-4">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h4 class="text-base font-semibold text-base-content">Discovery results</h4>
            <p class="text-sm text-muted">
              Found servers that can be turned into direct connections for this Proxmox type.
            </p>
          </div>
          <div class="text-xs text-muted">
            <Show when={props.discoveryScanStatus.scanning}>
              <span class="flex items-center gap-2">
                <Loader class="h-4 w-4 animate-spin" />
                <span>{props.activeConfig.scanningLabel}</span>
              </span>
            </Show>
            <Show
              when={
                !props.discoveryScanStatus.scanning &&
                (props.discoveryScanStatus.lastResultAt ||
                  props.discoveryScanStatus.lastScanStartedAt)
              }
            >
              <span>
                Last scan{' '}
                {formatRelativeTime(
                  props.discoveryScanStatus.lastResultAt ??
                    props.discoveryScanStatus.lastScanStartedAt,
                  { compact: true },
                )}
              </span>
            </Show>
          </div>
        </div>

        <Show
          when={
            props.discoveryScanStatus.errors && props.discoveryScanStatus.errors.length > 0
          }
        >
          <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-2">
            <span class="font-medium">Discovery issues:</span>
            <ul class="list-disc ml-4 mt-1 space-y-0.5">
              <For each={props.discoveryScanStatus.errors ?? []}>{(error) => <li>{error}</li>}</For>
            </ul>
            <Show when={props.hasDiscoveryTimeouts}>
              <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                Large networks can time out in auto mode. Switch to a custom subnet for faster,
                targeted scans.
              </p>
            </Show>
          </div>
        </Show>

        <Show when={props.discoveryScanStatus.scanning && props.activeDiscoveredNodes.length === 0}>
          <div class="flex items-center gap-2 text-xs text-muted">
            <Loader class="h-4 w-4 animate-spin" />
            <span>
              Waiting for responses… this can take up to a minute depending on your network size.
            </span>
          </div>
        </Show>

        <Show
          when={props.activeDiscoveredNodes.length > 0}
          fallback={
            <div class="rounded-md border border-dashed border-border px-4 py-6 text-sm text-muted">
              No discovery matches for this Proxmox type yet. You can still add a direct
              connection manually.
            </div>
          }
        >
          <div class="space-y-3">
            <For each={props.activeDiscoveredNodes}>
              {(server) => (
                <button
                  type="button"
                  class="w-full bg-surface-hover rounded-md p-4 border border-border text-left opacity-75 hover:opacity-100 hover:border-blue-300 dark:hover:border-blue-700 transition-all"
                  onClick={() => props.onOpenDiscoveredNode(server)}
                >
                  <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                    <div class="flex-1 min-w-0">
                      <div class="flex items-start gap-3">
                        <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                        <div class="flex-1 min-w-0">
                          <h4 class="font-medium text-base-content">
                            {props.activeConfig.titleFromServer(server)}
                          </h4>
                          <p class="text-sm mt-1">
                            {server.ip}:{server.port}
                          </p>
                          <div class="flex items-center gap-2 mt-2">
                            <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-400 rounded">
                              Discovered
                            </span>
                            <span class="text-xs text-muted">
                              Click to configure this connection
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="mt-1">
                      <path
                        d="M12 5v14m-7-7h14"
                        stroke="currentColor"
                        stroke-width="2"
                        stroke-linecap="round"
                      />
                    </svg>
                  </div>
                </button>
              )}
            </For>
          </div>
        </Show>
      </div>
    </Card>
  );
};
