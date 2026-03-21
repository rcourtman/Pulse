import { Component, Show, For, Suspense } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { formatBytes, formatUptime } from '@/utils/format';
import { DiskList } from './DiskList';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import { isGuestDrawerVM, type GuestDrawerProps } from './guestDrawerModel';
import { useGuestDrawerState } from './useGuestDrawerState';

export const GuestDrawer: Component<GuestDrawerProps> = (props) => {
  const navigate = useNavigate();
  const {
    activeTab,
    agentLabel,
    agentTitle,
    backupPresentation,
    discoveryAgentId,
    discoveryLoadingState,
    discoveryResourceId,
    discoveryResourceType,
    guestId,
    hasAgentInfo,
    hasFilesystemDetails,
    hasNetworkInterfaces,
    hasOsInfo,
    infrastructureHref,
    ipAddresses,
    memoryExtraLines,
    networkInterfaces,
    normalizedTags,
    osName,
    osVersion,
    switchTab,
    webInterfaceTargetLabel,
  } = useGuestDrawerState(props);

  return (
    <div class="space-y-3">
      {/* Tabs */}
      <div class="flex items-center gap-6 border-b border-border px-1 mb-1">
        <button
          onClick={() => switchTab('overview')}
          class={`pb-2 text-sm font-medium transition-colors relative ${
            activeTab() === 'overview' ? 'text-blue-600 dark:text-blue-400' : ' hover:text-muted'
          }`}
        >
          Overview
          {activeTab() === 'overview' && (
            <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
          )}
        </button>
        <button
          onClick={() => switchTab('discovery')}
          class={`pb-2 text-sm font-medium transition-colors relative ${
            activeTab() === 'discovery' ? 'text-blue-600 dark:text-blue-400' : ' hover:text-muted'
          }`}
        >
          Discovery
          {activeTab() === 'discovery' && (
            <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
          )}
        </button>
      </div>
      <div class="flex justify-end">
        <button
          type="button"
          onClick={() => navigate(infrastructureHref())}
          class="inline-flex items-center rounded border border-border bg-surface-alt px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
        >
          Open related infrastructure
        </button>
      </div>

      {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
                 overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
      <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ 'overflow-anchor': 'none' }}>
        {/* Flex layout - items grow to fill space, max ~4 per row */}
        <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
          {/* System Info - always show */}
          <div class="rounded border border-border bg-surface p-3 shadow-sm">
            <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
              System
            </div>
            <div class="space-y-1.5 text-[11px]">
              <Show when={props.guest.cpus}>
                <div class="flex items-center justify-between">
                  <span class="text-muted">CPUs</span>
                  <span class="font-medium text-base-content">{props.guest.cpus}</span>
                </div>
              </Show>
              <Show when={props.guest.uptime > 0}>
                <div class="flex items-center justify-between">
                  <span class="text-muted">Uptime</span>
                  <span class="font-medium text-base-content">
                    {formatUptime(props.guest.uptime)}
                  </span>
                </div>
              </Show>
              <Show when={props.guest.node}>
                <div class="flex items-center justify-between">
                  <span class="text-muted">Node</span>
                  <span class="font-medium text-base-content">{props.guest.node}</span>
                </div>
              </Show>
              <Show when={hasAgentInfo()}>
                <div class="flex items-center justify-between">
                  <span class="text-muted">Agent</span>
                  <span class="font-medium text-base-content truncate ml-2" title={agentTitle()}>
                    {agentLabel()}
                  </span>
                </div>
              </Show>
            </div>
          </div>

          {/* Guest Info - OS and IPs */}
          <Show when={hasOsInfo() || ipAddresses().length > 0}>
            <div class="rounded border border-border bg-surface p-3 shadow-sm">
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Guest Info
              </div>
              <div class="space-y-2">
                <Show when={hasOsInfo()}>
                  <div
                    class="text-[11px] text-muted truncate"
                    title={`${osName()} ${osVersion()}`.trim()}
                  >
                    <Show when={osName().length > 0}>
                      <span class="font-medium">{osName()}</span>
                    </Show>
                    <Show when={osName().length > 0 && osVersion().length > 0}>
                      <span class="text-muted mx-1">•</span>
                    </Show>
                    <Show when={osVersion().length > 0}>
                      <span>{osVersion()}</span>
                    </Show>
                  </div>
                </Show>
                <Show when={ipAddresses().length > 0}>
                  <div class="flex flex-wrap gap-1">
                    <For each={ipAddresses()}>
                      {(ip) => (
                        <span
                          class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200 max-w-full truncate"
                          title={ip}
                        >
                          {ip}
                        </span>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            </div>
          </Show>

          {/* Memory Details */}
          <Show when={memoryExtraLines() && memoryExtraLines()!.length > 0}>
            <div class="rounded border border-border bg-surface p-3 shadow-sm">
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Memory
              </div>
              <div class="space-y-1 text-[11px] text-muted">
                <For each={memoryExtraLines()!}>{(line) => <div>{line}</div>}</For>
              </div>
            </div>
          </Show>

          {/* Backup Info */}
          <Show when={props.guest.lastBackup}>
            <div class="rounded border border-border bg-surface p-3 shadow-sm">
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Backup
              </div>
              <div class="space-y-1 text-[11px]">
                <Show when={backupPresentation()}>
                  {(presentation) => (
                    <>
                      <div class="flex items-center justify-between">
                        <span class="text-muted">Last Backup</span>
                        <span class={`font-medium ${presentation().ageClass}`}>
                          {presentation().ageLabel}
                        </span>
                      </div>
                      <div class="text-[10px] text-muted">{presentation().dateLabel}</div>
                    </>
                  )}
                </Show>
              </div>
            </div>
          </Show>

          {/* Tags */}
          <Show when={normalizedTags().length > 0}>
            <div class="rounded border border-border bg-surface p-3 shadow-sm">
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Tags
              </div>
              <div class="flex flex-wrap gap-1">
                <For each={normalizedTags()}>
                  {(tag) => (
                    <span class="inline-block rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                      {tag}
                    </span>
                  )}
                </For>
              </div>
            </div>
          </Show>

          {/* Filesystems */}
          <Show when={hasFilesystemDetails() && props.guest.disks && props.guest.disks.length > 0}>
            <div class="rounded border border-border bg-surface p-3 shadow-sm">
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Filesystems
              </div>
              <div class="text-[11px] text-muted">
                <DiskList
                  disks={props.guest.disks || []}
                  diskStatusReason={
                    isGuestDrawerVM(props.guest) ? (props.guest as any).diskStatusReason : undefined
                  }
                />
              </div>
            </div>
          </Show>

          {/* Network Interfaces */}
          <Show when={hasNetworkInterfaces()}>
            <div class="rounded border border-border bg-surface p-3 shadow-sm">
              <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
                Network
              </div>
              <div class="space-y-2">
                <For each={networkInterfaces().slice(0, 4)}>
                  {(iface) => {
                    const addresses = iface.addresses ?? [];
                    const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                    return (
                      <div class="rounded border border-dashed border-border p-2 overflow-hidden">
                        <div class="flex items-center gap-2 text-[11px] font-medium text-base-content min-w-0">
                          <span class="truncate min-w-0">{iface.name || 'interface'}</span>
                          <Show when={iface.mac}>
                            <span
                              class="text-[9px] text-muted font-normal truncate shrink-0 max-w-[100px]"
                              title={iface.mac}
                            >
                              {iface.mac}
                            </span>
                          </Show>
                        </div>
                        <Show when={addresses.length > 0}>
                          <div class="flex flex-wrap gap-1 mt-1">
                            <For each={addresses}>
                              {(ip) => (
                                <span
                                  class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200 max-w-full truncate"
                                  title={ip}
                                >
                                  {ip}
                                </span>
                              )}
                            </For>
                          </div>
                        </Show>
                        <Show when={hasTraffic}>
                          <div class="flex gap-3 mt-1 text-[10px] text-muted">
                            <span>RX {formatBytes(iface.rxBytes ?? 0)}</span>
                            <span>TX {formatBytes(iface.txBytes ?? 0)}</span>
                          </div>
                        </Show>
                      </div>
                    );
                  }}
                </For>
              </div>
            </div>
          </Show>
        </div>

        <div class="mt-3">
              <WebInterfaceUrlField
            metadataKind="guest"
            metadataId={guestId()}
            targetLabel={webInterfaceTargetLabel()}
            customUrl={props.customUrl}
            onCustomUrlChange={(url) => props.onCustomUrlChange?.(guestId(), url)}
          />
        </div>
      </div>

      {/* Always rendered, hidden via CSS. Wrapped in a local Suspense
                     so DiscoveryTab's createResource loading state doesn't bubble
                     up to the app-level Suspense and replace the entire page. */}
      <div
        class={activeTab() === 'discovery' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <Suspense
          fallback={
            <div class="flex items-center justify-center py-8">
              <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
              <span class="ml-2 text-sm text-muted">{discoveryLoadingState.text}</span>
            </div>
          }
        >
          <DiscoveryTab
            resourceType={discoveryResourceType()}
            agentId={discoveryAgentId()}
            resourceId={discoveryResourceId()}
            hostname={props.guest.name}
          />
        </Suspense>
      </div>
    </div>
  );
};
