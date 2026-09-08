import type { AIChatContext } from '@/stores/aiChat';
import type { Incident, IncidentEvent } from '@/types/api';
import { formatIncidentEvidenceTime } from '@/utils/alertIncidentPresentation';
import { resolveAlertTargetType } from '@/utils/alertTargetTypes';

interface BuildAlertIncidentAssistantHandoffInput {
  incident: Incident;
}

interface AlertIncidentAssistantHandoff {
  context: AIChatContext;
}

interface SanitizedIncidentEvent {
  source?: string;
  evidence?: Pick<
    NonNullable<IncidentEvent['evidence']>,
    | 'id'
    | 'resourceId'
    | 'kind'
    | 'observedAt'
    | 'occurredAt'
    | 'sourceType'
    | 'sourceAdapter'
    | 'actor'
    | 'confidence'
  >;
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
}: BuildAlertIncidentAssistantHandoffInput): AlertIncidentAssistantHandoff {
  const resourceLabel = incident.resourceName || incident.resourceId || 'unknown resource';
  const targetType = resolveAlertTargetType({
    alertType: incident.alertType,
    resourceType: incident.resourceType,
    resourceId: incident.resourceId,
  });
  const levelLabel = formatIncidentLabel(incident.level);
  const statusLabel = formatIncidentLabel(incident.status);
  const durationText = formatIncidentDuration(incident.openedAt, incident.closedAt);
  const events = sanitizeIncidentEvents(incident.events ?? []);
  const eventCount = events.length;
  const eventCountLabel = `${eventCount} timeline event${eventCount === 1 ? '' : 's'}`;
  const handoffContext = buildIncidentAssistantModelContext({
    incident,
    events,
    resourceLabel,
    levelLabel,
    statusLabel,
    durationText,
    eventCountLabel,
  });

  return {
    context: {
      targetType,
      targetId: incident.resourceId,
      autonomousMode: false,
      handoffContext,
      handoffResources: [
        {
          id: incident.resourceId,
          name: resourceLabel,
          type: targetType,
          node: incident.node,
        },
      ],
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
          .slice(-MAX_BRIEFING_EVENTS)
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
        openedAt: formatIncidentEvidenceTime(incident.openedAt) ? incident.openedAt : undefined,
        closedAt: incident.closedAt,
        acknowledged: incident.acknowledged,
        eventCount,
        includedEventCount: Math.min(eventCount, MAX_CONTEXT_EVENTS),
        history: incident.history ? { ...incident.history } : undefined,
        eventSummaries: events.slice(-MAX_CONTEXT_EVENTS),
      },
    },
  };
}

function buildIncidentAssistantModelContext({
  incident,
  events,
  resourceLabel,
  levelLabel,
  statusLabel,
  durationText,
  eventCountLabel,
}: {
  incident: Incident;
  events: SanitizedIncidentEvent[];
  resourceLabel: string;
  levelLabel: string;
  statusLabel: string;
  durationText: string;
  eventCountLabel: string;
}): string {
  const eventLines = events
    .slice(-MAX_CONTEXT_EVENTS)
    .map((event, index) =>
      formatContextLine(
        `Timeline Event ${index + 1}`,
        `${event.timestamp} | ${formatIncidentLabel(event.type)} | ${event.summary}${event.evidence ? ` | evidence=${JSON.stringify(event.evidence)}` : event.source ? ` | source=${event.source}` : ''}`,
      ),
    );

  return [
    '[Alert Incident Context]',
    'Source: Pulse Alerts incident timeline',
    formatContextLine('Incident ID', incident.id),
    formatContextLine('Alert Identifier', incident.alertIdentifier),
    formatContextLine('Alert Type', incident.alertType),
    formatContextLine('Alert Level', levelLabel),
    formatContextLine('Incident Status', statusLabel),
    formatContextLine('Resource', resourceLabel),
    formatContextLine('Resource ID', incident.resourceId),
    formatContextLine('Resource Type', incident.resourceType),
    formatContextLine('Node', incident.node),
    formatContextLine('Instance', incident.instance),
    formatContextLine(
      'Opened At',
      formatIncidentEvidenceTime(incident.openedAt) ? incident.openedAt : 'unknown',
    ),
    formatContextLine('Closed At', incident.closedAt),
    formatContextLine('Duration', durationText),
    formatContextLine('Timeline Summary', eventCountLabel),
    formatContextLine(
      'Included Events',
      `Latest ${Math.min(events.length, MAX_CONTEXT_EVENTS)} of ${events.length} returned events`,
    ),
    formatContextLine(
      'History Coverage',
      incident.history ? JSON.stringify(incident.history) : 'not recorded',
    ),
    'History Boundary: Retained historical evidence does not establish current resource health.',
    formatContextLine('Message', incident.message),
    ...eventLines,
    'Timeline Boundary: Command events are summarized only. Raw command details and output stay in the incident or governed approval surface.',
    'Operator Boundary: This incident handoff is model-only context for explanation and review. Diagnostics, remediation, and any command execution require explicit operator approval.',
  ]
    .filter((line): line is string => Boolean(line))
    .join('\n');
}

function sanitizeIncidentEvents(events: IncidentEvent[]): SanitizedIncidentEvent[] {
  return events.map((event) => ({
    id: event.id,
    type: event.type,
    timestamp: event.timestamp,
    summary: sanitizeIncidentEventSummary(event),
    ...(event.source ? { source: event.source } : {}),
    ...(event.evidence
      ? {
          evidence: {
            id: event.evidence.id,
            resourceId: event.evidence.resourceId,
            kind: event.evidence.kind,
            observedAt: event.evidence.observedAt,
            occurredAt: event.evidence.occurredAt,
            sourceType: event.evidence.sourceType,
            sourceAdapter: event.evidence.sourceAdapter,
            actor: event.evidence.actor,
            confidence: event.evidence.confidence,
          },
        }
      : {}),
  }));
}

function sanitizeIncidentEventSummary(event: IncidentEvent): string {
  const normalizedType = event.type.toLowerCase();
  if (normalizedType.includes('command')) {
    return 'Command event recorded';
  }

  const summary = event.summary.trim();
  if (normalizedType === 'note' && typeof event.details?.note === 'string') {
    const note = event.details.note.trim();
    if (note) return `${summary || 'Operator note'}: ${note}`;
  }
  return summary.length > 0 ? summary : 'Timeline event recorded';
}

function formatContextLine(
  label: string,
  value?: string | number | boolean | null,
): string | undefined {
  if (value === undefined || value === null) return undefined;
  const text = String(value).trim();
  return text ? `${label}: ${text}` : undefined;
}

function formatIncidentDuration(openedAt: string, closedAt: string | undefined): string {
  if (!formatIncidentEvidenceTime(openedAt)) return 'unknown duration';
  const openedMs = new Date(openedAt).getTime();
  if (!closedAt) return 'closure not recorded';
  const closedMs = new Date(closedAt).getTime();
  if (!Number.isFinite(openedMs) || !Number.isFinite(closedMs) || closedMs < openedMs) {
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
