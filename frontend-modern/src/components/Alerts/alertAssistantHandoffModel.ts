import type { Alert } from '@/types/api';
import type { AIChatContext } from '@/stores/aiChat';
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
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

export function buildAlertAssistantHandoff({
  alert,
  resourceType,
  vmid,
  now = new Date(),
}: BuildAlertAssistantHandoffInput): AlertAssistantHandoff {
  const alertIdentifier = getCanonicalAlertId(alert);
  const durationText = formatAlertDuration(alert.startTime, now);
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
    durationText,
    nodeLabel,
    levelLabel,
  });

  const promptLines = [
    `Investigate this ${alert.level.toUpperCase()} alert:`,
    ``,
    `**Resource:** ${alert.resourceName}`,
    `**Alert Type:** ${alert.type}`,
  ];
  if (hasMetricValues) {
    promptLines.push(`**Current Value:** ${currentValue}`);
    promptLines.push(`**Threshold:** ${thresholdValue}`);
  }
  promptLines.push(`**Duration:** ${durationText}`);
  if (nodeLabel) promptLines.push(`**Node:** ${nodeLabel}`);
  if (alert.message) promptLines.push(`**Message:** ${alert.message}`);
  promptLines.push(``);
  promptLines.push(`Please:`);
  promptLines.push(`1. Identify the root cause`);
  if (hasMetricValues) {
    promptLines.push(`2. Check related metrics`);
    promptLines.push(`3. Suggest specific remediation steps`);
    promptLines.push(`4. Ask for operator approval before running any diagnostic command or change`);
  } else {
    promptLines.push(`2. Check what changed recently for this resource (state events, recent commands, related alerts)`);
    promptLines.push(`3. Suggest specific remediation steps`);
    promptLines.push(`4. Ask for operator approval before running any diagnostic command or change`);
  }
  const prompt = promptLines.join('\n');

  return {
    prompt,
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
        sourceLabel: 'Pulse Alerts',
        title: 'Alert investigation attached',
        subject: `${levelLabel} ${alert.type} on ${alert.resourceName}`,
        statusLabel: `${levelLabel} alert · Active ${durationText}`,
        detailLines: [
          hasMetricValues
            ? `Current value ${currentValue}; threshold ${thresholdValue}`
            : undefined,
          nodeLabel ? `Node: ${nodeLabel}` : undefined,
          alert.message ? `Message: ${alert.message}` : undefined,
        ].filter((line): line is string => Boolean(line)),
        actionLabel: `Investigate alert ${alertIdentifier}`,
        safetyNote: 'Diagnostics and remediation require operator approval.',
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

function formatAlertDuration(startTime: string, now: Date): string {
  const startedAt = new Date(startTime).getTime();
  const nowMs = now.getTime();
  if (!Number.isFinite(startedAt) || !Number.isFinite(nowMs)) {
    return 'unknown duration';
  }

  const durationMs = Math.max(0, nowMs - startedAt);
  const durationMins = Math.floor(durationMs / 60000);
  if (durationMins < 60) {
    return `${durationMins} min${durationMins !== 1 ? 's' : ''}`;
  }
  return `${Math.floor(durationMins / 60)}h ${durationMins % 60}m`;
}

function formatAlertLevel(level: Alert['level']): string {
  return level.charAt(0).toUpperCase() + level.slice(1);
}
