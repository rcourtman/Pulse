import { describe, expect, it } from 'vitest';

import type { Incident } from '@/types/api';

import { buildAlertIncidentAssistantHandoff } from '../incidentAssistantHandoffModel';

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
        details: {
          command: 'systemctl restart checkout-api',
          output_excerpt: 'token=secret-value',
        },
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

describe('incidentAssistantHandoffModel', () => {
  it('builds an approval-required incident timeline handoff without raw command payloads', () => {
    const handoff = buildAlertIncidentAssistantHandoff({
      incident: makeIncident(),
    });

    expect(handoff).not.toHaveProperty('prompt');
    expect(handoff.context).toMatchObject({
      targetType: 'app-container',
      targetId: 'docker:app-1',
      autonomousMode: false,
      handoffResources: [
        {
          id: 'docker:app-1',
          name: 'checkout-api',
          type: 'app-container',
          node: 'edge-1',
        },
      ],
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Incident timeline attached',
        subject: 'Critical docker-container-health on checkout-api',
        statusLabel: 'Critical incident · Open · closure not recorded',
        detailLines: [
          '2 timeline events',
          'Node: edge-1',
          'Message: Container health check is failing',
        ],
        evidence: [
          'Command: Command event recorded',
          'AI Analysis: Health check failure correlated with recent deployment',
        ],
        actionLabel: 'Discuss incident incident-1',
        safetyNote: 'Diagnostics and remediation require operator approval.',
      },
      context: {
        alertIncidentId: 'incident-1',
        alertIdentifier: 'docker:app-1::docker-container-health',
        alertType: 'docker-container-health',
        alertLevel: 'critical',
        alertStatus: 'open',
        resourceName: 'checkout-api',
        resourceType: 'docker-container',
        eventCount: 2,
        eventSummaries: [
          {
            id: 'event-1',
            type: 'command',
            timestamp: '2026-03-20T10:02:00Z',
            summary: 'Command event recorded',
          },
          {
            id: 'event-2',
            type: 'ai_analysis',
            timestamp: '2026-03-20T10:03:00Z',
            summary: 'Health check failure correlated with recent deployment',
          },
        ],
      },
    });
    expect(handoff.context.handoffContext).toContain('[Alert Incident Context]');
    expect(handoff.context.handoffContext).toContain('Source: Pulse Alerts incident timeline');
    expect(handoff.context.handoffContext).toContain('Timeline Event 1:');
    expect(handoff.context.handoffContext).toContain('Command | Command event recorded');
    expect(handoff.context.handoffContext).toContain('Timeline Boundary:');
    expect(JSON.stringify(handoff)).not.toContain('systemctl');
    expect(JSON.stringify(handoff)).not.toContain('secret-value');
  });
  it('includes latest evidence and query bounds without exposing command payloads', () => {
    const events = Array.from({ length: 12 }, (_, index) => ({
      id: `event-${index}`,
      type: index === 11 ? 'alert_resolved' : 'command',
      timestamp: '2026-03-20T10:05:00Z',
      summary: index === 11 ? 'Independently resolved' : 'sensitive command',
      evidence: {
        id: `canonical-${index}`,
        resourceId: 'resource-a',
        kind: 'alert_resolved' as const,
        observedAt: '2026-03-20T10:06:00Z',
        occurredAt: '2026-03-20T10:05:00Z',
        sourceType: 'platform_event' as const,
        confidence: 'high' as const,
        metadata: { command: 'must not appear' },
      },
    }));
    const incident = makeIncident({
      events,
      history: {
        source: 'canonical_resource_history',
        observedSince: '2026-03-01T00:00:00Z',
        observedBefore: '2026-03-20T11:00:00Z',
        changeLimit: 12,
        hasMoreChanges: true,
        hasMoreIncidents: false,
      },
    });
    const handoff = buildAlertIncidentAssistantHandoff({ incident });
    expect(handoff.context.handoffContext).toContain('Latest 8 of 12 returned events');
    expect(handoff.context.handoffContext).toContain('Independently resolved');
    expect(handoff.context.handoffContext).toContain('canonical-11');
    expect(handoff.context.handoffContext).not.toContain('canonical-0"');
    expect(handoff.context.handoffContext).toContain('"hasMoreChanges":true');
    expect(handoff.context.handoffContext).toContain('2026-03-20T10:06:00Z');
    expect(handoff.context.handoffContext).not.toContain('must not appear');
    expect(handoff.context.handoffContext).not.toContain('sensitive command');
  });

  it('keeps an absent occurrence start unknown in Assistant context', () => {
    const handoff = buildAlertIncidentAssistantHandoff({
      incident: makeIncident({ openedAt: '0001-01-01T00:00:00Z', status: 'unknown' }),
    });
    expect(handoff.context.handoffContext).toContain('Opened At: unknown');
    expect(handoff.context.handoffContext).toContain('unknown duration');
    expect(handoff.context.handoffContext).not.toContain('0001-01-01');
  });
});

it('retains operator note content and attribution without forwarding command output', () => {
  const incident = makeIncident();
  incident.events!.push({
    id: 'operator-note',
    type: 'note',
    timestamp: '2026-03-20T10:04:00Z',
    summary: 'Note added by operator',
    source: 'operator_note',
    details: {
      note: 'Keep the old pool until its replacement is verified',
      output_excerpt: 'unrelated private output',
    },
  });
  const { context } = buildAlertIncidentAssistantHandoff({ incident });
  expect(context.handoffContext).toContain(
    'Note added by operator: Keep the old pool until its replacement is verified',
  );
  expect(context.handoffContext).toContain('source=operator_note');
  expect(context.handoffContext).not.toContain('unrelated private output');
  expect(context.handoffContext).not.toContain('token=secret-value');
});
