import { Show } from 'solid-js';
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
  getAlertOverviewSnoozedUntilLabel,
  getAlertOverviewSnoozeLabel,
} from '@/utils/alertOverviewPresentation';

import { alertTypeDisplayLabel } from './helpers';
import { describeAlertDeliveryStatus } from './deliveryDiagnosisPresentation';
import { getCanonicalAlertId } from './identity';
import type { AlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import type { AlertOverviewState } from './useAlertOverviewState';
import { ResourceMonitoringPolicyAction } from './ResourceMonitoringPolicyAction';
import { AlertSnoozeAction } from './AlertSnoozeAction';
import { isAlertSnoozed } from './useAlertSnoozeState';

interface AlertOverviewAlertCardProps {
  alert: Alert;
  state: AlertOverviewState;
  timelineState: AlertIncidentTimelineState;
}

export function AlertOverviewAlertCard(props: AlertOverviewAlertCardProps) {
  const alertKey = () => getCanonicalAlertId(props.alert);
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
  const deliveryStatusLine = () =>
    describeAlertDeliveryStatus(deliveryDiagnosis(), props.alert.acknowledged);

  const resourceLink = (): string => {
    const rid = props.alert.resourceId ?? '';
    const resourceType =
      typeof props.alert.metadata?.resourceType === 'string'
        ? (props.alert.metadata.resourceType as string)
        : '';
    if (rid.startsWith('agent:') || resourceType === 'agent') return '/machines';
    if (
      rid.includes('docker') ||
      resourceType === 'docker-container' ||
      resourceType === 'docker-host'
    )
      return '/docker/overview';
    if (resourceType === 'kubernetes' || resourceType.startsWith('k8s-'))
      return '/kubernetes/overview';
    if (resourceType.startsWith('vmware-') || props.alert.message?.toLowerCase().includes('vmware'))
      return '/vmware/overview';
    return '/proxmox/overview';
  };

  const platformTypeForPolicy = (): string | undefined => {
    const metadataPlatform = props.alert.metadata?.platformType;
    if (typeof metadataPlatform === 'string' && metadataPlatform.trim()) return metadataPlatform;
    const link = resourceLink();
    if (link.startsWith('/proxmox')) return 'proxmox';
    if (link.startsWith('/docker')) return 'docker';
    if (link.startsWith('/kubernetes')) return 'kubernetes';
    if (link.startsWith('/vmware')) return 'vmware';
    if (link.startsWith('/machines')) return 'agent';
    return undefined;
  };

  const resourceTypeForPolicy = (): string | undefined => {
    const metadataType = props.alert.metadata?.resourceType;
    if (typeof metadataType === 'string' && metadataType.trim()) return metadataType;
    const resourceId = (props.alert.resourceId || '').toLowerCase();
    if (resourceId.startsWith('agent:')) return 'agent';
    if (resourceId.includes('docker')) return 'app-container';
    if (resourceId.includes('k8s') || resourceId.includes('kubernetes')) return 'pod';
    if (resourceLink() === '/proxmox/overview') return 'vm';
    return undefined;
  };

  return (
    <div id={`alert-${alertKey()}`} class={alertCardPresentation().cardClassName}>
      <div class="flex flex-col sm:flex-row sm:items-start">
        <div class="flex items-start flex-1">
          <div class={alertCardPresentation().iconClassName}>
            {props.alert.acknowledged ? (
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            ) : (
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            )}
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <A
                href={resourceLink()}
                class={`${alertCardPresentation().resourceClassName} hover:underline cursor-pointer`}
                title="View resource"
              >
                {props.alert.resourceName}
              </A>
              <span class="text-xs text-muted">({alertTypeDisplayLabel(props.alert.type)})</span>
              <Show when={!props.alert.acknowledged}>
                <span class={getAlertSeverityBadgeClass(props.alert.level)}>
                  {formatAlertSeverityLabel(props.alert.level)}
                </span>
              </Show>
              <Show when={props.alert.node}>
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
                <span class="shrink-0 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
                  {getAlertOverviewSnoozeLabel()}
                </span>
              </Show>
            </div>
            <p class="text-sm text-base-content mt-1 break-words">{props.alert.message}</p>
            <div class="flex flex-wrap items-center gap-x-3 gap-y-0.5 mt-1">
              <p class={getAlertOverviewStartedAtClass()}>
                {getAlertOverviewStartedAtLabel(new Date(props.alert.startTime).toLocaleString())}
              </p>
              <Show when={props.alert.threshold > 0}>
                <span class="text-xs text-muted">
                  limit: {props.alert.threshold}
                  {props.alert.type === 'temperature' || props.alert.type === 'diskTemperature'
                    ? '°C'
                    : '%'}
                </span>
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
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-1.5 sm:gap-2 mt-3 sm:mt-0 sm:ml-4 self-end sm:self-start justify-end">
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
            {getAlertOverviewTimelineActionLabel(
              props.timelineState.expandedIncidents().has(alertKey()),
            )}
          </button>
          <Show when={(props.alert.resourceId || '').trim()}>
            <ResourceMonitoringPolicyAction
              resourceId={props.alert.resourceId}
              resourceName={props.alert.resourceName || props.alert.resourceId}
              resourceType={resourceTypeForPolicy()}
              platformType={platformTypeForPolicy()}
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
        </div>
      </div>
      <Show when={props.timelineState.expandedIncidents().has(alertKey())}>
        <div class="mt-3 border-t border-border pt-3">
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
