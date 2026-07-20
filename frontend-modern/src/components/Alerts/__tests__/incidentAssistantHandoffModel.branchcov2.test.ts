import { describe, expect, it } from 'vitest';

import type { Incident, IncidentEvent } from '@/types/api';

import { buildAlertIncidentAssistantHandoff } from '../incidentAssistantHandoffModel';

// `buildAlertIncidentAssistantHandoff` always populates briefing/context/handoffResources,
// but its declared return type borrows `AIChatContext`, where those fields are optional and
// `context` is `Record<string, unknown>`. The strict tsconfig therefore flags direct deep
// access as possibly-undefined. We mirror the concrete runtime shape in a local type and
// cast once so the assertions below stay precise (`toBe`/`toEqual`) and type-safe.
interface SanitizedIncidentEvent {
  id: string;
  type: string;
  timestamp: string;
  summary: string;
}

interface StrictHandoffContext {
  targetType: string;
  targetId: string;
  autonomousMode: boolean;
  handoffContext: string;
  handoffResources: { id: string; name: string; type: string; node?: string }[];
  briefing: {
    sourceLabel: string;
    title: string;
    subject: string;
    statusLabel: string;
    detailLines: string[];
    evidence: string[];
    actionLabel: string;
    safetyNote: string;
  };
  context: {
    alertIncidentId: string;
    alertIdentifier: string;
    alertType: string;
    alertLevel: string;
    alertStatus: string;
    alertMessage?: string;
    resourceName: string;
    resourceType?: string;
    node?: string;
    instance?: string;
    openedAt: string;
    closedAt?: string;
    acknowledged: boolean;
    eventCount: number;
    eventSummaries: SanitizedIncidentEvent[];
  };
}

interface StrictHandoff {
  context: StrictHandoffContext;
}

const DEFAULT_NOW = new Date('2026-03-20T10:05:00Z');

function buildHandoff(incident: Incident, now: Date = DEFAULT_NOW): StrictHandoff {
  return buildAlertIncidentAssistantHandoff({ incident, now }) as unknown as StrictHandoff;
}

function makeIncident(overrides: Partial<Incident> = {}): Incident {
  return {
    id: 'incident-1',
    alertIdentifier: 'docker:app-1::docker-container-health',
    alertType: 'docker-container-health',
    level: 'critical',
    resourceId: 'docker:app-1',
    resourceName: 'checkout-api',
    resourceType: 'docker-container',
    node: 'edge-1',
    message: 'Container health check is failing',
    status: 'open',
    openedAt: '2026-03-20T10:00:00Z',
    acknowledged: false,
    events: [
      {
        id: 'event-1',
        type: 'command',
        timestamp: '2026-03-20T10:02:00Z',
        summary: 'systemctl restart checkout-api',
      },
      {
        id: 'event-2',
        type: 'ai_analysis',
        timestamp: '2026-03-20T10:03:00Z',
        summary: 'Health check failure correlated with recent deployment',
      },
    ],
    ...overrides,
  };
}

function makeEvent(overrides: Partial<IncidentEvent> = {}): IncidentEvent {
  return {
    id: 'event-x',
    type: 'note',
    timestamp: '2026-03-20T10:04:00Z',
    summary: 'Something happened',
    ...overrides,
  };
}

// With level='critical' + status='open', formatIncidentLabel yields 'Critical' / 'Open', so the
// briefing statusLabel prefix is constant: `${levelLabel} incident · ${statusLabel} · `.
const STATUS_PREFIX = 'Critical incident · Open · ';

describe('formatIncidentDuration (exercised via buildAlertIncidentAssistantHandoff)', () => {
  it('returns singular "1 min" for an exact 1-minute delta (durationMins === 1 true arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: '2026-03-20T10:01:00Z',
      }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}1 min`);
    expect(handoff.context.handoffContext).toContain('Duration: 1 min');
  });

  it('returns plural "5 mins" for a multi-minute sub-hour delta (durationMins === 1 false arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: '2026-03-20T10:05:00Z',
      }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}5 mins`);
  });

  it('returns "0 mins" when closedAt equals openedAt (zero delta, plural arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: '2026-03-20T10:00:00Z',
      }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}0 mins`);
  });

  it('clamps a negative (closedAt before openedAt) delta to "0 mins" via Math.max(0, ...)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:05:00Z',
        closedAt: '2026-03-20T10:00:00Z',
      }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}0 mins`);
  });

  it('formats a sub-day >= 60min delta as "Xh Ym" (durationMins >= 60, durationHours < 24 arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: '2026-03-20T11:05:00Z',
      }),
      new Date('2026-03-20T12:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}1h 5m`);
  });

  it('formats a multi-day delta as "Xd Yh" (durationHours >= 24 arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: '2026-03-21T14:00:00Z',
      }),
      new Date('2026-03-22T10:00:00Z'),
    );
    // 28h elapsed -> floor(28/24)=1 day, 28%24=4 hours
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}1d 4h`);
  });

  it('returns "unknown duration" when openedAt is unparseable (openedMs not finite)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: 'not-a-valid-date',
        closedAt: '2026-03-20T11:00:00Z',
      }),
      new Date('2026-03-20T12:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}unknown duration`);
    expect(handoff.context.handoffContext).toContain('Duration: unknown duration');
  });

  it('returns "unknown duration" when closedAt is truthy but unparseable (closedMs not finite)', () => {
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: 'not-a-valid-date',
      }),
      new Date('2026-03-20T12:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}unknown duration`);
  });

  it('falls back to `now` when closedAt is undefined (closedAt-falsy ternary arm, nonzero delta)', () => {
    const handoff = buildHandoff(
      makeIncident({ closedAt: undefined }),
      new Date('2026-03-20T10:05:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}5 mins`);
  });

  it('falls back to `now` yielding zero when closedAt is undefined and now === openedAt', () => {
    const handoff = buildHandoff(
      makeIncident({ closedAt: undefined }),
      new Date('2026-03-20T10:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}0 mins`);
  });

  it('uses closedAt (not now) when closedAt is present (closedAt-truthy ternary arm)', () => {
    // closedAt gives 5 mins; now is 1 hour after openedAt and would give 60 mins -> "1h 0m".
    const handoff = buildHandoff(
      makeIncident({
        openedAt: '2026-03-20T10:00:00Z',
        closedAt: '2026-03-20T10:05:00Z',
      }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.briefing.statusLabel).toBe(`${STATUS_PREFIX}5 mins`);
  });
});

describe('sanitizeIncidentEventSummary (exercised via buildAlertIncidentAssistantHandoff)', () => {
  it('redacts a lowercase "command" type to the canned summary (includes(command) true)', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'command', summary: 'rm -rf /' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Command event recorded');
    expect(handoff.context.briefing.evidence[0]).toBe('Command: Command event recorded');
    expect(JSON.stringify(handoff)).not.toContain('rm -rf');
  });

  it('redacts a title-cased "Command" type via toLowerCase() (case-insensitive command branch)', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'Command', summary: 'rm -rf /' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Command event recorded');
    expect(JSON.stringify(handoff)).not.toContain('rm -rf');
  });

  it('redacts on a substring match "pre_command" (includes("command") substring arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'pre_command', summary: 'secret-payload' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Command event recorded');
    expect(handoff.context.briefing.evidence[0]).toBe('Pre Command: Command event recorded');
    expect(JSON.stringify(handoff)).not.toContain('secret-payload');
  });

  it('passes through a non-command summary that is non-empty after trim (length > 0 true arm)', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'note', summary: 'Server rebooted' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Server rebooted');
    expect(handoff.context.briefing.evidence[0]).toBe('Note: Server rebooted');
  });

  it('trims surrounding whitespace on a non-command passthrough summary', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'note', summary: '  Server rebooted  ' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Server rebooted');
  });

  it('falls back to "Timeline event recorded" when a non-command summary is whitespace-only', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'note', summary: '   ' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Timeline event recorded');
    expect(handoff.context.briefing.evidence[0]).toBe('Note: Timeline event recorded');
  });

  it('falls back to "Timeline event recorded" when a non-command summary is empty', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [makeEvent({ id: 'e1', type: 'note', summary: '' })],
      }),
    );
    expect(handoff.context.context.eventSummaries[0].summary).toBe('Timeline event recorded');
  });

  it('preserves id/type/timestamp of a sanitized command event (sanitizeIncidentEvents mapping)', () => {
    const handoff = buildHandoff(
      makeIncident({
        events: [
          makeEvent({
            id: 'evt-99',
            type: 'command_executed',
            timestamp: '2026-03-20T10:08:00Z',
            summary: 'whoami',
          }),
        ],
      }),
    );
    expect(handoff.context.context.eventSummaries[0]).toEqual({
      id: 'evt-99',
      type: 'command_executed',
      timestamp: '2026-03-20T10:08:00Z',
      summary: 'Command event recorded',
    });
  });
});

describe('formatContextLine (exercised via buildAlertIncidentAssistantHandoff)', () => {
  it('omits the "Node:" context line when node is undefined (value === undefined branch)', () => {
    const handoff = buildHandoff(makeIncident({ node: undefined }));
    expect(handoff.context.handoffContext).not.toContain('Node:');
    expect(handoff.context.briefing.detailLines).toEqual([
      '2 timeline events',
      'Message: Container health check is failing',
    ]);
  });

  it('omits the "Instance:" context line when instance is undefined', () => {
    const handoff = buildHandoff(makeIncident({ instance: undefined }));
    expect(handoff.context.handoffContext).not.toContain('Instance:');
  });

  it('omits the "Message:" context line and briefing detailLine when message is undefined', () => {
    const handoff = buildHandoff(makeIncident({ message: undefined }));
    expect(handoff.context.handoffContext).not.toContain('Message:');
    expect(handoff.context.briefing.detailLines).toEqual(['2 timeline events', 'Node: edge-1']);
  });

  it('omits the "Node:" context line when node is an empty string (trim -> empty -> undefined)', () => {
    const handoff = buildHandoff(makeIncident({ node: '' }));
    expect(handoff.context.handoffContext).not.toContain('Node:');
  });

  it('omits the "Node:" context line when node is whitespace-only (trim -> empty)', () => {
    const handoff = buildHandoff(makeIncident({ node: '   ' }));
    expect(handoff.context.handoffContext).not.toContain('Node:');
  });

  it('renders "Node: edge-1" for a non-empty node string (happy path)', () => {
    const handoff = buildHandoff(makeIncident({ node: 'edge-1' }));
    expect(handoff.context.handoffContext).toContain('Node: edge-1');
  });

  it('coerces a numeric value to a rendered "Node: 42" line (String(value) branch)', () => {
    const handoff = buildHandoff(makeIncident({ node: 42 as unknown as string }));
    expect(handoff.context.handoffContext).toContain('Node: 42');
    expect(handoff.context.briefing.detailLines).toContain('Node: 42');
  });

  it('coerces a boolean value to a rendered "Instance: true" line (String(value) branch)', () => {
    const handoff = buildHandoff(makeIncident({ instance: true as unknown as string }));
    expect(handoff.context.handoffContext).toContain('Instance: true');
  });

  it('omits the "Closed At:" context line when closedAt is null (value === null branch)', () => {
    const handoff = buildHandoff(makeIncident({ closedAt: null as unknown as string }));
    expect(handoff.context.handoffContext).not.toContain('Closed At:');
  });

  it('renders the "Closed At:" context line when closedAt is a present string', () => {
    const handoff = buildHandoff(
      makeIncident({ closedAt: '2026-03-20T10:05:00Z' }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.handoffContext).toContain('Closed At: 2026-03-20T10:05:00Z');
  });

  it('trims a whitespace-padded closedAt value to its inner text', () => {
    const handoff = buildHandoff(
      makeIncident({ closedAt: '  2026-03-20T10:05:00Z  ' as unknown as string }),
      new Date('2026-03-20T11:00:00Z'),
    );
    expect(handoff.context.handoffContext).toContain('Closed At: 2026-03-20T10:05:00Z');
    expect(handoff.context.handoffContext).not.toContain('Closed At:   2026-03-20T10:05:00Z');
  });
});

describe('buildAlertIncidentAssistantHandoff (resource-label, event-count & capping branches)', () => {
  it('uses resourceName first (|| chain 1st arm) and builds the full concrete handoff', () => {
    const handoff = buildHandoff(makeIncident(), new Date('2026-03-20T10:05:00Z'));
    // 1st arm of the resource-label || chain.
    expect(handoff.context.briefing.subject).toBe(
      'Critical docker-container-health on checkout-api',
    );
    expect(handoff.context.handoffResources[0]).toEqual({
      id: 'docker:app-1',
      name: 'checkout-api',
      type: 'app-container',
      node: 'edge-1',
    });
    // Both node + message detailLines present (briefing ternary true arms).
    expect(handoff.context.briefing.detailLines).toEqual([
      '2 timeline events',
      'Node: edge-1',
      'Message: Container health check is failing',
    ]);
    // actionLabel + safetyNote concrete values.
    expect(handoff.context.briefing.actionLabel).toBe('Discuss incident incident-1');
    expect(handoff.context.briefing.safetyNote).toBe(
      'Diagnostics and remediation require operator approval.',
    );
    // Top-level context scalars are concrete.
    expect(handoff.context.targetType).toBe('app-container');
    expect(handoff.context.targetId).toBe('docker:app-1');
    expect(handoff.context.autonomousMode).toBe(false);
    expect(handoff.context.context.alertIncidentId).toBe('incident-1');
    expect(handoff.context.context.acknowledged).toBe(false);
    expect(handoff.context.context.eventCount).toBe(2);
  });

  it('falls back to resourceId when resourceName is empty (|| chain, 2nd arm)', () => {
    const handoff = buildHandoff(makeIncident({ resourceName: '', resourceId: 'res-7' }));
    expect(handoff.context.briefing.subject).toBe('Critical docker-container-health on res-7');
    expect(handoff.context.handoffResources[0].name).toBe('res-7');
    expect(handoff.context.context.resourceName).toBe('res-7');
    expect(handoff.context.handoffContext).toContain('Resource: res-7');
  });

  it('falls back to "unknown resource" when both resourceName and resourceId are empty (|| chain, 3rd arm)', () => {
    const handoff = buildHandoff(makeIncident({ resourceName: '', resourceId: '' }));
    expect(handoff.context.briefing.subject).toBe(
      'Critical docker-container-health on unknown resource',
    );
    expect(handoff.context.handoffResources[0].name).toBe('unknown resource');
    expect(handoff.context.context.resourceName).toBe('unknown resource');
    expect(handoff.context.handoffContext).toContain('Resource: unknown resource');
  });

  it('treats undefined events as an empty array (?? [] branch) -> "0 timeline events"', () => {
    const handoff = buildHandoff(makeIncident({ events: undefined }));
    expect(handoff.context.context.eventCount).toBe(0);
    expect(handoff.context.briefing.detailLines).toContain('0 timeline events');
    expect(handoff.context.briefing.evidence).toEqual([]);
    expect(handoff.context.context.eventSummaries).toEqual([]);
    expect(handoff.context.handoffContext).toContain('Timeline Summary: 0 timeline events');
  });

  it('uses the singular "1 timeline event" label when exactly one event is present', () => {
    const handoff = buildHandoff(makeIncident({ events: [makeEvent({ id: 'solo' })] }));
    expect(handoff.context.context.eventCount).toBe(1);
    expect(handoff.context.briefing.detailLines).toContain('1 timeline event');
    expect(handoff.context.handoffContext).toContain('Timeline Summary: 1 timeline event');
  });

  it('uses the plural "N timeline events" label for multiple events', () => {
    const handoff = buildHandoff(makeIncident());
    expect(handoff.context.briefing.detailLines).toContain('2 timeline events');
  });

  it('omits the Node detailLine when node is falsy (briefing ternary false arm)', () => {
    const handoff = buildHandoff(makeIncident({ node: undefined, message: undefined }));
    expect(handoff.context.briefing.detailLines).toEqual(['2 timeline events']);
  });

  it('caps briefing evidence at MAX_BRIEFING_EVENTS (3) when more than 3 events exist', () => {
    const events = Array.from({ length: 5 }, (_, i) =>
      makeEvent({ id: `e${i + 1}`, timestamp: '2026-03-20T10:02:00Z' }),
    );
    const handoff = buildHandoff(makeIncident({ events }));
    expect(handoff.context.briefing.evidence).toHaveLength(3);
    expect(handoff.context.context.eventCount).toBe(5);
  });

  it('caps context.eventSummaries and timeline lines at MAX_CONTEXT_EVENTS (8) when >8 events', () => {
    const events = Array.from({ length: 10 }, (_, i) =>
      makeEvent({ id: `e${i + 1}`, timestamp: '2026-03-20T10:02:00Z' }),
    );
    const handoff = buildHandoff(makeIncident({ events }));
    expect(handoff.context.context.eventSummaries).toHaveLength(8);
    expect(handoff.context.context.eventCount).toBe(10);
    expect(handoff.context.handoffContext).toContain('Timeline Event 1');
    expect(handoff.context.handoffContext).toContain('Timeline Event 8');
    expect(handoff.context.handoffContext).not.toContain('Timeline Event 9');
  });

  it('renders the fixed boundary footer lines in handoffContext', () => {
    const handoff = buildHandoff(makeIncident());
    expect(handoff.context.handoffContext).toContain(
      'Timeline Boundary: Command events are summarized only; raw command details and output stay in the incident or governed approval surface.',
    );
    expect(handoff.context.handoffContext).toContain(
      'Operator Boundary: This incident handoff is model-only context for explanation and review. Diagnostics, remediation, and any command execution require explicit operator approval.',
    );
  });
});
