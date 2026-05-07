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
      now: new Date('2026-03-20T10:05:00Z'),
    });

    expect(handoff.prompt).toContain('Discuss this Critical alert incident from Pulse Alerts.');
    expect(handoff.prompt).toContain('Command details and output stay in the incident');
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
        statusLabel: 'Critical incident · Open · 5 mins',
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
});
