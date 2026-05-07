import type { AIChatContext } from '@/stores/aiChat';
import type { Incident, IncidentEvent } from '@/types/api';
import { resolveAlertTargetType } from '@/utils/alertTargetTypes';

interface BuildAlertIncidentAssistantHandoffInput {
  incident: Incident;
  now?: Date;
}

interface AlertIncidentAssistantHandoff {
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

interface SanitizedIncidentEvent {
  id: string;
  type: string;
  timestamp: string;
  summary: string;
}

const MAX_BRIEFING_EVENTS = 3;
const MAX_CONTEXT_EVENTS = 8;
const LABEL_INITIALISMS: Record<string, string> = {
  ai: 'AI',
  api: 'API',
  cpu: 'CPU',
  io: 'I/O',
  zfs: 'ZFS',
};

export function buildAlertIncidentAssistantHandoff({
  incident,
  now = new Date(),
}: BuildAlertIncidentAssistantHandoffInput): AlertIncidentAssistantHandoff {
  const resourceLabel = incident.resourceName || incident.resourceId || 'unknown resource';
  const targetType = resolveAlertTargetType({
    alertType: incident.alertType,
    resourceType: incident.resourceType,
    resourceId: incident.resourceId,
  });
  const levelLabel = formatIncidentLabel(incident.level);
  const statusLabel = formatIncidentLabel(incident.status);
  const durationText = formatIncidentDuration(incident.openedAt, incident.closedAt, now);
  const events = sanitizeIncidentEvents(incident.events ?? []);
  const eventCount = events.length;
  const eventCountLabel = `${eventCount} timeline event${eventCount === 1 ? '' : 's'}`;

  const prompt = [
    `Discuss this ${levelLabel} alert incident from Pulse Alerts.`,
    '',
    `**Resource:** ${resourceLabel}`,
    `**Alert Type:** ${incident.alertType}`,
    `**Status:** ${statusLabel}`,
    `**Duration:** ${durationText}`,
    incident.node ? `**Node:** ${incident.node}` : undefined,
    incident.message ? `**Message:** ${incident.message}` : undefined,
    '',
    'Use the attached sanitized incident timeline context. Command details and output stay in the incident or approval surface; do not infer, repeat, or execute raw command text from this chat handoff.',
    '',
    'Please:',
    '1. Explain what the incident record says happened',
    '2. Identify the likely cause and any uncertainty',
    '3. Call out related checks the operator should review',
    '4. Ask for approval before running diagnostics or remediation',
  ]
    .filter((line): line is string => line !== undefined)
    .join('\n');

  return {
    prompt,
    context: {
      targetType,
      targetId: incident.resourceId,
      autonomousMode: false,
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Incident timeline attached',
        subject: `${levelLabel} ${incident.alertType} on ${resourceLabel}`,
        statusLabel: `${levelLabel} incident · ${statusLabel} · ${durationText}`,
        detailLines: [
          eventCountLabel,
          incident.node ? `Node: ${incident.node}` : undefined,
          incident.message ? `Message: ${incident.message}` : undefined,
        ].filter((line): line is string => Boolean(line)),
        evidence: events
          .slice(0, MAX_BRIEFING_EVENTS)
          .map((event) => `${formatIncidentLabel(event.type)}: ${event.summary}`),
        actionLabel: `Discuss incident ${incident.id}`,
        safetyNote: 'Diagnostics and remediation require operator approval.',
      },
      context: {
        alertIncidentId: incident.id,
        alertIdentifier: incident.alertIdentifier,
        alertType: incident.alertType,
        alertLevel: incident.level,
        alertStatus: incident.status,
        alertMessage: incident.message,
        resourceName: resourceLabel,
        resourceType: incident.resourceType,
        node: incident.node,
        instance: incident.instance,
        openedAt: incident.openedAt,
        closedAt: incident.closedAt,
        acknowledged: incident.acknowledged,
        eventCount,
        eventSummaries: events.slice(0, MAX_CONTEXT_EVENTS),
      },
    },
  };
}

function sanitizeIncidentEvents(events: IncidentEvent[]): SanitizedIncidentEvent[] {
  return events.map((event) => ({
    id: event.id,
    type: event.type,
    timestamp: event.timestamp,
    summary: sanitizeIncidentEventSummary(event),
  }));
}

function sanitizeIncidentEventSummary(event: IncidentEvent): string {
  const normalizedType = event.type.toLowerCase();
  if (normalizedType.includes('command')) {
    return 'Command event recorded';
  }

  const summary = event.summary.trim();
  return summary.length > 0 ? summary : 'Timeline event recorded';
}

function formatIncidentDuration(openedAt: string, closedAt: string | undefined, now: Date): string {
  const openedMs = new Date(openedAt).getTime();
  const closedMs = closedAt ? new Date(closedAt).getTime() : now.getTime();
  if (!Number.isFinite(openedMs) || !Number.isFinite(closedMs)) {
    return 'unknown duration';
  }

  const durationMins = Math.floor(Math.max(0, closedMs - openedMs) / 60000);
  if (durationMins < 60) {
    return `${durationMins} min${durationMins === 1 ? '' : 's'}`;
  }

  const durationHours = Math.floor(durationMins / 60);
  if (durationHours < 24) {
    return `${durationHours}h ${durationMins % 60}m`;
  }

  return `${Math.floor(durationHours / 24)}d ${durationHours % 24}h`;
}

function formatIncidentLabel(value: string): string {
  return value
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map(
      (part) =>
        LABEL_INITIALISMS[part.toLowerCase()] || part.charAt(0).toUpperCase() + part.slice(1),
    )
    .join(' ');
}
