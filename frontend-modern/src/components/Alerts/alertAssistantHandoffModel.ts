import type { Alert } from '@/types/api';
import type { AIChatContext } from '@/stores/aiChat';
import { getCanonicalAlertId } from '@/features/alerts/identity';
import { formatAlertValue } from '@/utils/alertFormatters';
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
  const currentValue = formatAlertValue(alert.value, alert.type);
  const thresholdValue = formatAlertValue(alert.threshold, alert.type);
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

  const prompt = `Investigate this ${alert.level.toUpperCase()} alert:

**Resource:** ${alert.resourceName}
**Alert Type:** ${alert.type}
**Current Value:** ${currentValue}
**Threshold:** ${thresholdValue}
**Duration:** ${durationText}
${nodeLabel ? `**Node:** ${nodeLabel}` : ''}

Please:
1. Identify the root cause
2. Check related metrics
3. Suggest specific remediation steps
4. Ask for operator approval before running any diagnostic command or change`;

  return {
    prompt,
    context: {
      targetType,
      targetId: alert.resourceId,
      autonomousMode: false,
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Alert investigation attached',
        subject: `${levelLabel} ${alert.type} on ${alert.resourceName}`,
        statusLabel: `${levelLabel} alert · Active ${durationText}`,
        detailLines: [
          `Current value ${currentValue}; threshold ${thresholdValue}`,
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
