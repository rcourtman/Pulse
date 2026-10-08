import { createSignal, Show } from 'solid-js';
import { A } from '@solidjs/router';

import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import type { Alert } from '@/types/api';
import {
  formatAlertSeverityLabel,
  getAlertSeverityBadgeClass,
} from '@/utils/alertSeverityPresentation';
import {
  getAlertOverviewAcknowledgedBadgeClass,
  getAlertOverviewAcknowledgedBadgeLabel,
  getAlertOverviewCardPresentation,
  getAlertOverviewNodeLabel,
  getAlertOverviewPrimaryActionLabel,
  getAlertOverviewPrimaryActionClass,
  getAlertOverviewSecondaryActionClass,
  getAlertOverviewStartedAtLabel,
  getAlertOverviewStartedAtClass,
  getAlertOverviewTimelineActionLabel,
  getAlertOverviewMoreActionsLabel,
  formatAlertOverviewStartedAgo,
  getAlertOverviewSnoozedUntilLabel,
  getAlertOverviewSnoozeLabel,
} from '@/utils/alertOverviewPresentation';

import { alertTypeDisplayLabel } from './helpers';
import { describeAlertDeliveryStatus } from './deliveryDiagnosisPresentation';
import { getCanonicalAlertId } from './identity';
import { getMetricAlertPresentation } from './metricAlertPresentation';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import type { AlertOverviewState } from './useAlertOverviewState';
import { ResourceMonitoringPolicyAction } from './ResourceMonitoringPolicyAction';
import { AlertSnoozeAction } from './AlertSnoozeAction';
import { isAlertSnoozed } from './useAlertSnoozeState';
import { isPulseSystemAlert } from '@/utils/alertScope';
import { formatTemperature } from '@/utils/temperature';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  normalizeSourcePlatformQueryValue,
  resolvePlatformTypeFromSources,
} from '@/utils/sourcePlatforms';
import {
  DOCKER_PATH,
  KUBERNETES_PATH,
  PROXMOX_PATH,
  STANDALONE_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildStandalonePath,
  buildTrueNASPath,
  buildVmwarePath,
} from '@/routing/resourceLinks';
import {
  PRIMARY_PLATFORM_NAV_IDS,
  PRIMARY_PLATFORM_NAV_SCOPE_IDS,
  type PrimaryPlatformNavId,
} from '@/features/platformNavigation/platformNavigationModel';

interface AlertOverviewAlertCardProps {
  alert: Alert;
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
  // True while the page states that delivery is off for every alert; the
  // per-alert line would only repeat that banner.
  deliveryPausedGlobally?: boolean;
}

// Delivery holds that apply to every alert at once. The overview states them
// in one banner instead of on each card.
const GLOBAL_DELIVERY_HOLD_REASONS = new Set(['notifications_inactive', 'notifications_disabled']);

// The page that lists each primary platform's resources, keyed by nav id so
// a new platform page cannot ship without an alert link.
const PLATFORM_PAGE_PATHS: Record<PrimaryPlatformNavId, string> = {
  proxmox: buildProxmoxPath(),
  docker: buildDockerPath(),
  kubernetes: buildKubernetesPath(),
  truenas: buildTrueNASPath(),
  vmware: buildVmwarePath(),
  standalone: buildStandalonePath(),
};

// Units of the threshold metrics, matching the evaluator's metric status unit.
const THRESHOLD_UNIT_BY_ALERT_TYPE: Record<string, string> = {
  cpu: '%',
  memory: '%',
  disk: '%',
  'disk-usage': '%',
  usage: '%',
  temperature: '°C',
  disk_temperature: '°C',
  diskTemperature: '°C',
  diskRead: ' MB/s',
  diskWrite: ' MB/s',
  networkIn: ' MB/s',
  networkOut: ' MB/s',
  'disk-wearout': '%',
};

const platformPagePath = (platform: string | undefined): string | undefined => {
  if (!platform) return undefined;
  const navId = PRIMARY_PLATFORM_NAV_IDS.find((id) =>
    PRIMARY_PLATFORM_NAV_SCOPE_IDS[id].includes(platform),
  );
  return navId ? PLATFORM_PAGE_PATHS[navId] : undefined;
};

export function AlertOverviewAlertCard(props: AlertOverviewAlertCardProps) {
  // The started age and the live reading's stale cut-off and ages measure
  // from the wall clock, so a card that mounts between ticks is current.
  const now = useRelativeTimeNow();
  const alertKey = () => getCanonicalAlertId(props.alert);
  const hasResource = () =>
    !isPulseSystemAlert(props.alert) && Boolean(props.alert.resourceId?.trim());
  const processing = () =>
    props.state.processingAlerts().has(alertKey()) ||
    props.state.snoozeProcessingAlerts().has(alertKey());
  const alertCardPresentation = () =>
    getAlertOverviewCardPresentation(
      props.alert.level ?? 'warning',
      props.alert.acknowledged,
      processing(),
    );

  const deliveryDiagnosis = () => props.state.deliveryDiagnoses()[alertKey()];
  const deliveryStatusLine = () => {
    const diagnosis = deliveryDiagnosis();
    if (
      props.deliveryPausedGlobally &&
      GLOBAL_DELIVERY_HOLD_REASONS.has((diagnosis?.reason || '').split(':')[0])
    ) {
      return null;
    }
    return describeAlertDeliveryStatus(diagnosis, props.alert.acknowledged);
  };
  const [moreOpen, setMoreOpen] = createSignal(false);
  // A threshold alert can stay open below its trigger, so the card leads with
  // the reading Pulse is evaluating now; the message keeps the last breach.
  const metricPresentation = () =>
    isPulseSystemAlert(props.alert) ? null : getMetricAlertPresentation(props.alert, now());
  const alertLevels = (): { alert: string; clear?: string } | null => {
    const presentation = metricPresentation();
    if (presentation) {
      return {
        alert: presentation.alertLevel,
        clear:
          presentation.clearLevel !== presentation.alertLevel ? presentation.clearLevel : undefined,
      };
    }
    if (isPulseSystemAlert(props.alert) || !(props.alert.threshold > 0)) return null;
    // Without a live status only the metric types carry a known unit; other
    // thresholds (queue ages, counts) are stated in their own message.
    const unit = THRESHOLD_UNIT_BY_ALERT_TYPE[props.alert.type];
    if (!unit) return null;
    return {
      alert:
        unit === '°C'
          ? formatTemperature(props.alert.threshold)
          : `${props.alert.threshold}${unit}`,
    };
  };
  const hasMetaLine = () =>
    Boolean(alertLevels()) ||
    Boolean(deliveryStatusLine()) ||
    (isAlertSnoozed(props.alert) && Boolean(props.alert.operationalRecord?.suppression?.expiresAt));
  const timelineOpen = () => props.timelineState.expandedIncidents().has(alertKey());

  // Incident alerts land on canonical agent/vm/storage/network resources, so
  // the resource type alone cannot name the platform. Their metadata carries
  // the incident's provider and the resource's sources, which do, whatever
  // the message says.
  const metadataPlatform = (): string | undefined => {
    const metadata = props.alert.metadata;
    const explicit =
      typeof metadata?.platformType === 'string'
        ? normalizeSourcePlatformQueryValue(metadata.platformType)
        : '';
    if (explicit) return explicit;
    const provider =
      typeof metadata?.incidentProvider === 'string'
        ? resolvePlatformTypeFromSources([metadata.incidentProvider])
        : undefined;
    if (provider) return provider;
    const sources = Array.isArray(metadata?.resourceSources)
      ? metadata.resourceSources.filter((source): source is string => typeof source === 'string')
      : [];
    return resolvePlatformTypeFromSources(sources);
  };

  const resourceLink = (): string => {
    const platformPage = platformPagePath(metadataPlatform());
    if (platformPage) return platformPage;
    const rid = props.alert.resourceId ?? '';
    const resourceType =
      typeof props.alert.metadata?.resourceType === 'string'
        ? (props.alert.metadata.resourceType as string)
        : '';
    // Ids and messages carry names the user chose: a Proxmox guest id embeds
    // its cluster and node names (docker-01:docker-01:100, agent:pve1:100) and
    // a powered-off message names the guest. They may only decide the page
    // when the alert names no resource type at all.
    const typeless = !resourceType;
    if (resourceType === 'agent' || (typeless && rid.startsWith('agent:')))
      return buildStandalonePath();
    if (
      resourceType.startsWith('docker-') ||
      resourceType === 'app-container' ||
      (typeless && rid.includes('docker'))
    )
      return buildDockerPath();
    if (resourceType === 'kubernetes' || resourceType.startsWith('k8s-'))
      return buildKubernetesPath();
    if (resourceType.startsWith('truenas-')) return buildTrueNASPath();
    if (
      resourceType.startsWith('vmware-') ||
      (typeless && props.alert.message?.toLowerCase().includes('vmware'))
    )
      return buildVmwarePath();
    return buildProxmoxPath();
  };

  const platformTypeForPolicy = (): string | undefined => {
    const platform = metadataPlatform();
    if (platform) return platform;
    const link = resourceLink();
    if (link.startsWith(PROXMOX_PATH)) return 'proxmox';
    if (link.startsWith(DOCKER_PATH)) return 'docker';
    if (link.startsWith(KUBERNETES_PATH)) return 'kubernetes';
    if (link.startsWith(TRUENAS_PATH)) return 'truenas';
    if (link.startsWith(VMWARE_PATH)) return 'vmware';
    if (link.startsWith(STANDALONE_PATH)) return 'agent';
    return undefined;
  };

  const resourceTypeForPolicy = (): string | undefined => {
    const metadataType = props.alert.metadata?.resourceType;
    if (typeof metadataType === 'string' && metadataType.trim()) return metadataType;
    const resourceId = (props.alert.resourceId || '').toLowerCase();
    if (resourceId.startsWith('agent:')) return 'agent';
    if (resourceId.includes('docker')) return 'app-container';
    if (resourceId.includes('k8s') || resourceId.includes('kubernetes')) return 'pod';
    if (resourceLink() === buildProxmoxPath()) return 'vm';
    return undefined;
  };

  return (
    <div id={`alert-${alertKey()}`} class={alertCardPresentation().cardClassName}>
      <div class="flex flex-col gap-2 sm:flex-row sm:items-start">
        {/* Without a floor the text column shrinks to its longest word and the
            actions keep one row, squeezing the reading to ~110px beside them. */}
        <div class="flex items-start flex-1 sm:min-w-64">
          <div class={alertCardPresentation().iconClassName}>
            {props.alert.acknowledged ? (
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            ) : (
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            )}
          </div>
          <div class="flex-1 min-w-0 wrap-anywhere">
            <div class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
              <Show
                when={hasResource()}
                fallback={
                  <span class={alertCardPresentation().resourceClassName}>
                    {props.alert.resourceName}
                  </span>
                }
              >
                <A
                  href={resourceLink()}
                  class={`${alertCardPresentation().resourceClassName} hover:underline cursor-pointer`}
                  title="View resource"
                >
                  {props.alert.resourceName}
                </A>
              </Show>
              <span class="text-xs text-muted">({alertTypeDisplayLabel(props.alert.type)})</span>
              <Show when={!props.alert.acknowledged}>
                <span class={getAlertSeverityBadgeClass(props.alert.level)}>
                  {formatAlertSeverityLabel(props.alert.level)}
                </span>
              </Show>
              <Show when={!isPulseSystemAlert(props.alert) && props.alert.node}>
                <span class="text-xs text-muted">
                  {getAlertOverviewNodeLabel(props.alert.nodeDisplayName || props.alert.node)}
                </span>
              </Show>
              <Show when={props.alert.acknowledged}>
                <span class={getAlertOverviewAcknowledgedBadgeClass()}>
                  {getAlertOverviewAcknowledgedBadgeLabel()}
                </span>
              </Show>
              <Show when={isAlertSnoozed(props.alert)}>
                <span class="shrink-0 rounded-sm bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
                  {getAlertOverviewSnoozeLabel()}
                </span>
              </Show>
              <span
                class={getAlertOverviewStartedAtClass()}
                title={getAlertOverviewStartedAtLabel(
                  new Date(props.alert.startTime).toLocaleString(),
                )}
              >
                {formatAlertOverviewStartedAgo(props.alert.startTime, now())}
              </span>
            </div>
            <Show
              when={metricPresentation()}
              fallback={
                <p class="text-sm text-base-content mt-0.5 wrap-break-word">
                  {props.alert.message}
                </p>
              }
            >
              {(presentation) => (
                <>
                  <p
                    class="text-sm text-base-content mt-0.5 wrap-break-word"
                    title={presentation().lastBreach}
                  >
                    {presentation().summary}
                  </p>
                  <p class="text-xs text-muted wrap-break-word">{presentation().detail}</p>
                </>
              )}
            </Show>
            <Show when={hasMetaLine()}>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-0.5 mt-0.5">
                <Show when={alertLevels()}>
                  {(levels) => (
                    <>
                      <span class="text-xs text-muted">Alert level {levels().alert}</span>
                      <Show when={levels().clear}>
                        <span class="text-xs text-muted">Clear level {levels().clear}</span>
                      </Show>
                    </>
                  )}
                </Show>
                <Show when={deliveryStatusLine()}>
                  <span
                    class={
                      deliveryStatusLine()?.tone === 'attention'
                        ? 'text-xs text-amber-600 dark:text-amber-400'
                        : 'text-xs text-muted'
                    }
                    title={deliveryDiagnosis()?.message}
                  >
                    {deliveryStatusLine()?.label}
                  </span>
                </Show>
                <Show
                  when={
                    isAlertSnoozed(props.alert) &&
                    props.alert.operationalRecord?.suppression?.expiresAt
                  }
                >
                  <span class="text-xs text-blue-600 dark:text-blue-400">
                    {getAlertOverviewSnoozedUntilLabel(
                      new Date(
                        props.alert.operationalRecord!.suppression!.expiresAt!,
                      ).toLocaleString(),
                    )}
                  </span>
                </Show>
              </div>
            </Show>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-1.5 sm:ml-4 self-end sm:self-start justify-end">
          <button
            class={getAlertOverviewPrimaryActionClass(props.alert.acknowledged)}
            disabled={processing()}
            onClick={async (e) => {
              e.preventDefault();
              e.stopPropagation();
              await props.state.handleAlertAcknowledgement(props.alert);
            }}
          >
            {getAlertOverviewPrimaryActionLabel({
              acknowledged: props.alert.acknowledged,
              processing: processing(),
            })}
          </button>
          <Show when={!props.alert.acknowledged || isAlertSnoozed(props.alert)}>
            <AlertSnoozeAction
              alert={props.alert}
              state={props.state}
              timelineState={props.timelineState}
            />
          </Show>
          <InvestigateAlertButton
            alert={props.alert}
            resourceType={
              typeof props.alert.metadata?.resourceType === 'string'
                ? (props.alert.metadata.resourceType as string)
                : undefined
            }
            variant="text"
            size="sm"
            patrolOption
          />
          <button
            type="button"
            class={getAlertOverviewSecondaryActionClass()}
            aria-expanded={moreOpen() || timelineOpen()}
            onClick={() => {
              if (moreOpen() || timelineOpen()) {
                // Less closes everything the disclosure opened, timeline included.
                setMoreOpen(false);
                if (timelineOpen()) {
                  void props.timelineState.toggleIncidentTimeline(
                    alertKey(),
                    alertKey(),
                    props.alert.startTime,
                  );
                }
                return;
              }
              setMoreOpen(true);
            }}
          >
            {getAlertOverviewMoreActionsLabel(moreOpen() || timelineOpen())}
          </button>
        </div>
      </div>
      <Show when={moreOpen() || timelineOpen()}>
        <div class="mt-2 flex flex-wrap items-center justify-end gap-1.5 border-t border-border pt-2">
          <button
            class={getAlertOverviewSecondaryActionClass()}
            onClick={() => {
              void props.timelineState.toggleIncidentTimeline(
                alertKey(),
                alertKey(),
                props.alert.startTime,
              );
            }}
          >
            {getAlertOverviewTimelineActionLabel(timelineOpen())}
          </button>
          <Show when={hasResource()}>
            <ResourceMonitoringPolicyAction
              resourceId={props.alert.resourceId}
              resourceName={props.alert.resourceName || props.alert.resourceId}
              resourceType={resourceTypeForPolicy()}
              platformType={platformTypeForPolicy()}
            />
          </Show>
        </div>
      </Show>
      <Show when={timelineOpen()}>
        <div class="mt-2 border-t border-border pt-3">
          <IncidentTimelinePanel
            loading={() => props.timelineState.incidentLoading()[alertKey()]}
            error={() => props.timelineState.incidentErrors()[alertKey()]}
            timeline={() => props.timelineState.incidentTimelines()[alertKey()]}
            filters={props.timelineState.eventFilters}
            setFilters={props.timelineState.setEventFilters}
            filterVariant="panel"
            eventCardVariant="alt"
            noteDraft={() => props.timelineState.incidentNoteDrafts()[alertKey()] || ''}
            onNoteDraftChange={(value) =>
              props.timelineState.setIncidentNoteDraft(alertKey(), value)
            }
            noteSaving={() => props.timelineState.incidentNoteSaving().has(alertKey())}
            onSaveNote={() => {
              void props.timelineState.saveIncidentNote(
                alertKey(),
                alertKey(),
                props.alert.startTime,
              );
            }}
            onRetry={() => {
              void props.timelineState.loadIncidentTimeline(
                alertKey(),
                alertKey(),
                props.alert.startTime,
              );
            }}
          />
        </div>
      </Show>
    </div>
  );
}
