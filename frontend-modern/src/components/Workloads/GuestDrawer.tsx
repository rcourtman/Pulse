import { Component, Suspense } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import type { GuestDrawerProps } from './guestDrawerModel';
import { useGuestDrawerState } from './useGuestDrawerState';
import { GuestDrawerOverview } from './GuestDrawerOverview';

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
    hasDiscoverySupport,
    hasFilesystemDetails,
    hasNetworkInterfaces,
    hasOsInfo,
    infrastructureHref,
    ipAddresses,
    guestOsSummary,
    memoryExtraLines,
    networkInterfaces,
    normalizedTags,
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
        {hasDiscoverySupport() && (
          <button
            onClick={() => switchTab('discovery')}
            class={`pb-2 text-sm font-medium transition-colors relative ${
              activeTab() === 'discovery'
                ? 'text-blue-600 dark:text-blue-400'
                : ' hover:text-muted'
            }`}
          >
            Discovery
            {activeTab() === 'discovery' && (
              <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
            )}
          </button>
        )}
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
        <GuestDrawerOverview
          guest={props.guest}
          guestId={guestId()}
          guestOsSummary={guestOsSummary()}
          agentLabel={agentLabel()}
          agentTitle={agentTitle()}
          hasAgentInfo={hasAgentInfo()}
          hasFilesystemDetails={hasFilesystemDetails()}
          hasNetworkInterfaces={hasNetworkInterfaces()}
          hasOsInfo={hasOsInfo()}
          ipAddresses={ipAddresses()}
          memoryExtraLines={memoryExtraLines()}
          networkInterfaces={networkInterfaces()}
          normalizedTags={normalizedTags()}
          onCustomUrlChange={props.onCustomUrlChange}
          customUrl={props.customUrl}
          backupPresentation={backupPresentation()}
          webInterfaceTargetLabel={webInterfaceTargetLabel()}
        />
      </div>

      {/* Always rendered, hidden via CSS. Wrapped in a local Suspense
                     so DiscoveryTab's createResource loading state doesn't bubble
                     up to the app-level Suspense and replace the entire page. */}
      {hasDiscoverySupport() && (
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
              resourceType={discoveryResourceType()!}
              agentId={discoveryAgentId()}
              resourceId={discoveryResourceId()}
              hostname={props.guest.name}
            />
          </Suspense>
        </div>
      )}
    </div>
  );
};
