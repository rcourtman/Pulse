import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import { buildAlertAssistantHandoff } from '../alertAssistantHandoffModel';

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    type: 'cpu',
    level: 'warning',
    resourceId: 'vm-101',
    resourceName: 'app-vm',
    node: 'pve1',
    nodeDisplayName: 'PVE Node 1',
    instance: '',
    message: 'CPU usage is high',
    value: 82.5,
    threshold: 80,
    startTime: '2026-05-07T10:00:00.000Z',
    acknowledged: false,
    ...overrides,
  };
}

describe('alertAssistantHandoffModel', () => {
  it('builds a visible approval-required alert handoff without command payloads', () => {
    const handoff = buildAlertAssistantHandoff({
      alert: makeAlert(),
      now: new Date('2026-05-07T10:05:00.000Z'),
      vmid: 101,
    });

    expect(handoff.prompt).toContain('Investigate this WARNING alert');
    expect(handoff.prompt).toContain(
      'Ask for operator approval before running any diagnostic command',
    );
    expect(handoff.context).toMatchObject({
      targetType: 'vm',
      targetId: 'vm-101',
      autonomousMode: false,
      handoffResources: [
        {
          id: 'vm-101',
          name: 'app-vm',
          type: 'vm',
          node: 'pve1',
        },
      ],
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Alert investigation attached',
        subject: 'Warning cpu on app-vm',
        statusLabel: 'Warning alert · Active 5 mins',
        detailLines: [
          'Current value 82.5%; threshold 80.0%',
          'Node: PVE Node 1',
          'Message: CPU usage is high',
        ],
        actionLabel: 'Investigate alert alert-1',
        safetyNote: 'Diagnostics and remediation require operator approval.',
      },
      context: {
        alertIdentifier: 'alert-1',
        alertType: 'cpu',
        alertLevel: 'warning',
        alertMessage: 'CPU usage is high',
        guestName: 'app-vm',
        node: 'pve1',
        vmid: 101,
      },
    });
    expect(handoff.context.handoffContext).toContain('[Alert Investigation Context]');
    expect(handoff.context.handoffContext).toContain('Source: Pulse Alerts active alert');
    expect(handoff.context.handoffContext).toContain('Alert Identifier: alert-1');
    expect(handoff.context.handoffContext).toContain('Operator Boundary:');
    expect(JSON.stringify(handoff)).not.toContain('systemctl');
  });

  it('uses canonical target inference for alert resources', () => {
    const handoff = buildAlertAssistantHandoff({
      alert: makeAlert({
        type: 'docker_cpu',
        resourceId: 'container-1',
        metadata: { resourceType: 'container' },
      }),
      now: new Date('2026-05-07T10:05:00.000Z'),
    });

    expect(handoff.context.targetType).toBe('app-container');
  });
});
