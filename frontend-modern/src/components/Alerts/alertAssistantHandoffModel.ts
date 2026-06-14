import type { Alert } from '@/types/api';
import type { AIChatContext } from '@/stores/aiChat';
import { DEFAULT_LOCALE, t, type SupportedLocale } from '@/i18n';
import { getCanonicalAlertId } from '@/features/alerts/identity';
import { formatAlertValue } from '@/utils/alertFormatters';
import { isMetricAlertType } from '@/utils/alerts';
import { resolveAlertTargetType } from '@/utils/alertTargetTypes';

interface BuildAlertAssistantHandoffInput {
  alert: Alert;
  resourceType?: string;
  vmid?: number;
  now?: Date;
}

interface AlertAssistantHandoff {
  context: AIChatContext;
}

export function buildAlertAssistantHandoff({
  alert,
  resourceType,
  vmid,
  now = new Date(),
}: BuildAlertAssistantHandoffInput): AlertAssistantHandoff {
  const alertIdentifier = getCanonicalAlertId(alert);
  const durationText = formatAlertDuration(alert.startTime, now);
  const modelDurationText = formatAlertDuration(alert.startTime, now, DEFAULT_LOCALE);
  // State alerts (powered-off, unreachable, container-state, etc.) are
  // binary or enumerated conditions, not threshold crossings. Backend
  // sends value=0 and threshold=0 for those; rendering "current 0.0% /
  // threshold 0.0%" in operator-facing copy is misleading default-zero
  // noise. Suppress those fields and rely on alert.type + alert.message
  // to convey what's wrong.
  const hasMetricValues = isMetricAlertType(alert.type);
  const currentValue = hasMetricValues ? formatAlertValue(alert.value, alert.type) : '';
  const thresholdValue = hasMetricValues ? formatAlertValue(alert.threshold, alert.type) : '';
  const nodeLabel = alert.node ? alert.nodeDisplayName || alert.node : '';
  const levelLabel = formatAlertLevel(alert.level);
  const modelLevelLabel = formatAlertLevel(alert.level, DEFAULT_LOCALE);
  const targetType = resolveAlertTargetType({
    alertType: alert.type,
    resourceType,
    metadataResourceType:
      typeof alert.metadata?.resourceType === 'string'
        ? (alert.metadata.resourceType as string)
        : undefined,
    resourceId: alert.resourceId,
  });
  const handoffContext = buildAlertAssistantModelContext({
    alert,
    alertIdentifier,
    currentValue,
    thresholdValue,
    durationText: modelDurationText,
    nodeLabel,
    levelLabel: modelLevelLabel,
  });

  return {
    context: {
      targetType,
      targetId: alert.resourceId,
      autonomousMode: false,
      handoffContext,
      handoffResources: [
        {
          id: alert.resourceId,
          name: alert.resourceName,
          type: targetType,
          node: alert.node,
        },
      ],
      briefing: {
        sourceLabel: t('alerts.assistant.sourceLabel'),
        title: t('alerts.assistant.title'),
        subject: t('alerts.assistant.subject', {
          level: levelLabel,
          alertType: alert.type,
          resourceName: alert.resourceName,
        }),
        statusLabel: t('alerts.assistant.statusLabel', {
          level: levelLabel,
          duration: durationText,
        }),
        detailLines: [
          hasMetricValues
            ? t('alerts.assistant.detail.currentMetric', {
                currentValue,
                thresholdValue,
              })
            : undefined,
          nodeLabel ? t('alerts.assistant.detail.node', { node: nodeLabel }) : undefined,
          alert.message
            ? t('alerts.assistant.detail.message', { message: alert.message })
            : undefined,
        ].filter((line): line is string => Boolean(line)),
        actionLabel: t('alerts.assistant.action.investigate', { alertIdentifier }),
        safetyNote: t('alerts.assistant.safetyNote'),
      },
      context: {
        alertIdentifier,
        alertType: alert.type,
        alertLevel: alert.level,
        alertMessage: alert.message,
        guestName: alert.resourceName,
        node: alert.node,
        vmid,
      },
    },
  };
}

function buildAlertAssistantModelContext({
  alert,
  alertIdentifier,
  currentValue,
  thresholdValue,
  durationText,
  nodeLabel,
  levelLabel,
}: {
  alert: Alert;
  alertIdentifier: string;
  currentValue: string;
  thresholdValue: string;
  durationText: string;
  nodeLabel: string;
  levelLabel: string;
}): string {
  return [
    '[Alert Investigation Context]',
    'Source: Pulse Alerts active alert',
    formatContextLine('Alert Identifier', alertIdentifier),
    formatContextLine('Alert Type', alert.type),
    formatContextLine('Alert Level', levelLabel),
    formatContextLine('Alert Status', 'active'),
    formatContextLine('Resource', alert.resourceName),
    formatContextLine('Resource ID', alert.resourceId),
    formatContextLine('Current Value', currentValue),
    formatContextLine('Threshold', thresholdValue),
    formatContextLine('Duration', durationText),
    formatContextLine('Node', nodeLabel),
    formatContextLine('Message', alert.message),
    'Operator Boundary: This alert handoff is model-only context for explanation and review. Diagnostics, remediation, and any command execution require explicit operator approval.',
  ]
    .filter((line): line is string => Boolean(line))
    .join('\n');
}

function formatContextLine(label: string, value?: string | number | null): string | undefined {
  if (value === undefined || value === null) return undefined;
  const text = String(value).trim();
  return text ? `${label}: ${text}` : undefined;
}

function formatAlertDuration(startTime: string, now: Date, locale?: SupportedLocale): string {
  const startedAt = new Date(startTime).getTime();
  const nowMs = now.getTime();
  if (!Number.isFinite(startedAt) || !Number.isFinite(nowMs)) {
    return t('alerts.assistant.duration.unknown', {}, locale);
  }

  const durationMs = Math.max(0, nowMs - startedAt);
  const durationMins = Math.floor(durationMs / 60000);
  if (durationMins < 60) {
    return t(
      durationMins === 1 ? 'alerts.assistant.duration.minute' : 'alerts.assistant.duration.minutes',
      { count: durationMins },
      locale,
    );
  }
  return t(
    'alerts.assistant.duration.hoursMinutes',
    {
      hours: Math.floor(durationMins / 60),
      minutes: durationMins % 60,
    },
    locale,
  );
}

function formatAlertLevel(level: Alert['level'], locale?: SupportedLocale): string {
  const normalizedLevel = String(level);
  if (normalizedLevel === 'critical') {
    return t('alerts.assistant.level.critical', {}, locale);
  }
  if (normalizedLevel === 'info') {
    return t('alerts.assistant.level.info', {}, locale);
  }
  return t('alerts.assistant.level.warning', {}, locale);
}
