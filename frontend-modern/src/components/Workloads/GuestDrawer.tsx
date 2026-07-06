import { Component, Show, Suspense, createMemo } from 'solid-js';
import CopyIcon from 'lucide-solid/icons/copy';
import MessageSquareIcon from 'lucide-solid/icons/message-square';
import XIcon from 'lucide-solid/icons/x';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import {
  DrawerHeaderActionButton,
  DrawerHeaderActionGroup,
  DrawerHeaderIconButton,
} from '@/components/shared/Button';
import { DiscoveryLoadingFallback } from '@/components/shared/DiscoveryLoadingFallback';
import { DrawerSubjectHeading } from '@/components/shared/DrawerSubjectHeading';
import { DiscoveryReadinessBadge } from '@/components/shared/DiscoveryReadinessBadge';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { getSimpleStatusIndicator } from '@/utils/status';
import { getGuestDrawerHistoryFallbackMetrics, type GuestDrawerProps } from './guestDrawerModel';
import { useGuestDrawerState } from './useGuestDrawerState';
import { GuestDrawerHistory, GuestDrawerHistoryRangeSelect } from './GuestDrawerHistory';
import { GuestDrawerOverview } from './GuestDrawerOverview';

export const GuestDrawer: Component<GuestDrawerProps> = (props) => {
  const {
    activeTab,
    agentLabel,
    agentTitle,
    agentContextCopied,
    backupPresentation,
    copyingAgentContext,
    discoveryAgentId,
    discoveryIdentifiedSummary,
    discoveryLoadingState,
    discoveryReadinessPresentation,
    discoveryResourceId,
    discoveryResourceType,
    diskThresholds,
    guestId,
    hasAgentInfo,
    hasDiscoverySupport,
    hasFilesystemDetails,
    hasHistorySupport,
    hasNetworkInterfaces,
    hasOsInfo,
    hasWorkloadActionAgent,
    historyRange,
    historyTarget,
    ipAddresses,
    guestOsSummary,
    networkInterfaces,
    normalizedTags,
    assistantAvailable,
    copyAgentContext,
    openAssistantForGuest,
    setHistoryRange,
    showInGuestAgentInstallCue,
    switchTab,
    webInterfaceMetadataId,
    webInterfaceTargetLabel,
    workloadActionAgentTitle,
  } = useGuestDrawerState(props);
  const headingId = () => `guest-drawer-heading-${guestId()}`;
  const historyFallbackMetrics = createMemo(() =>
    getGuestDrawerHistoryFallbackMetrics(props.guest),
  );

  const headerIndicator = createMemo(() => getSimpleStatusIndicator(props.guest.status));

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <div class="flex items-start justify-between gap-3">
        <DrawerSubjectHeading
          headingId={headingId()}
          title={props.guest.name}
          statusVariant={headerIndicator().variant}
          statusLabel={headerIndicator().label}
        />
        <DrawerHeaderActionGroup>
          <Show when={assistantAvailable()}>
            <DrawerHeaderActionButton
              onClick={() => openAssistantForGuest()}
              title={`Ask Pulse Assistant about ${props.guest.name}`}
              aria-label={`Ask Pulse Assistant about ${props.guest.name}`}
            >
              <MessageSquareIcon class="h-4 w-4" aria-hidden="true" />
              <span class="hidden sm:inline">Ask</span>
            </DrawerHeaderActionButton>
          </Show>
          <DrawerHeaderActionButton
            onClick={() => void copyAgentContext()}
            disabled={copyingAgentContext()}
            title={`Copy Pulse context for ${props.guest.name}`}
            aria-label={`Copy Pulse context for ${props.guest.name}`}
          >
            <CopyIcon class="h-4 w-4" aria-hidden="true" />
            <span class="hidden sm:inline">{agentContextCopied() ? 'Copied' : 'Copy'}</span>
          </DrawerHeaderActionButton>
          <DrawerHeaderIconButton onClick={() => props.onClose()} aria-label="Close guest drawer">
            <XIcon class="h-4 w-4" aria-hidden="true" />
          </DrawerHeaderIconButton>
        </DrawerHeaderActionGroup>
      </div>
      <Show when={discoveryReadinessPresentation()}>
        {(presentation) => (
          <div class="flex items-center gap-2 text-xs text-muted">
            <DiscoveryReadinessBadge presentation={presentation()} />
            <span class="truncate" title={presentation().detail || presentation().title}>
              {presentation().detail || presentation().statusLabel}
            </span>
          </div>
        )}
      </Show>
      <Subtabs
        class="mb-1"
        ariaLabel="Guest drawer sections"
        value={activeTab()}
        onChange={(value) => switchTab(value as Parameters<typeof switchTab>[0])}
        tabs={[
          { value: 'overview', label: 'Overview' },
          ...(hasHistorySupport()
            ? [{ value: 'history', label: 'History' } satisfies SubtabOption]
            : []),
          ...(hasDiscoverySupport()
            ? [{ value: 'discovery', label: 'Discovery' } satisfies SubtabOption]
            : []),
        ]}
        trailing={
          <Show when={hasHistorySupport() && activeTab() === 'history'}>
            <GuestDrawerHistoryRangeSelect range={historyRange()} onRangeChange={setHistoryRange} />
          </Show>
        }
      />

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
          networkInterfaces={networkInterfaces()}
          nestedWorkloadContext={props.nestedWorkloadContext}
          normalizedTags={normalizedTags()}
          onCustomUrlChange={props.onCustomUrlChange}
          customUrl={props.customUrl}
          backupPresentation={backupPresentation()}
          diskThresholds={diskThresholds()}
          discoveryIdentifiedSummary={discoveryIdentifiedSummary()}
          hasWorkloadActionAgent={hasWorkloadActionAgent()}
          showInGuestAgentInstallCue={showInGuestAgentInstallCue()}
          webInterfaceMetadataId={webInterfaceMetadataId()}
          webInterfaceTargetLabel={webInterfaceTargetLabel()}
          workloadActionAgentTitle={workloadActionAgentTitle()}
        />
      </div>

      {hasHistorySupport() && activeTab() === 'history' && (
        <div style={{ 'overflow-anchor': 'none' }}>
          <GuestDrawerHistory
            target={historyTarget()}
            range={historyRange()}
            fallbackMetrics={historyFallbackMetrics()}
          />
        </div>
      )}

      {/* Always rendered, hidden via CSS. Wrapped in a local Suspense
                     so DiscoveryTab's createResource loading state doesn't bubble
                     up to the app-level Suspense and replace the entire page. */}
      {hasDiscoverySupport() && (
        <div
          class={activeTab() === 'discovery' ? '' : 'hidden'}
          style={{ 'overflow-anchor': 'none' }}
        >
          <Suspense fallback={<DiscoveryLoadingFallback text={discoveryLoadingState.text} />}>
            <DiscoveryTab
              resourceType={discoveryResourceType()!}
              agentId={discoveryAgentId()}
              resourceId={discoveryResourceId()}
              hostname={props.guest.name}
              showManualRunAction
            />
          </Suspense>
        </div>
      )}
    </section>
  );
};
