import { describe, expect, it } from 'vitest';
import type { AgentWorkflowPrompt } from '@/api/agentCapabilities';
import { getAssistantWorkflowStarters } from '../workflowStarters';

const prompts = [
  {
    name: 'pulse_triage_fleet',
    label: 'Review the whole fleet',
    presentationKind: 'fleet',
    description: 'Triage the Pulse fleet.',
    arguments: [],
  },
  {
    name: 'pulse_operations_loop',
    label: 'Ask Patrol to handle an issue',
    presentationKind: 'workflow',
    description:
      'Have Patrol investigate active findings, follow the configured Patrol policy, take approved actions, verify the outcome, and record what happened.',
    arguments: [],
  },
  {
    name: 'pulse_investigate_resource',
    label: 'Inspect selected resource',
    presentationKind: 'resource',
    description: 'Investigate one Pulse resource.',
    arguments: [{ name: 'resourceId', required: true }],
  },
  {
    name: 'pulse_review_finding',
    label: 'Review selected finding',
    presentationKind: 'finding',
    description: 'Review one Patrol finding.',
    arguments: [{ name: 'finding_id', required: true }],
  },
] satisfies AgentWorkflowPrompt[];

describe('Assistant workflow starters', () => {
  it('keeps global starters available without scoped context', () => {
    expect(getAssistantWorkflowStarters(prompts, {})).toEqual([
      {
        id: 'pulse_triage_fleet',
        name: 'pulse_triage_fleet',
        label: 'Review the whole fleet',
        description: 'Triage the Pulse fleet.',
        kind: 'fleet',
        arguments: {},
      },
      {
        id: 'pulse_operations_loop',
        name: 'pulse_operations_loop',
        label: 'Ask Patrol to handle an issue',
        description:
          'Have Patrol investigate active findings, follow the configured Patrol policy, take approved actions, verify the outcome, and record what happened.',
        kind: 'workflow',
        arguments: {},
      },
    ]);
  });

  it('resolves resource workflow arguments from attached handoff resources first', () => {
    const starters = getAssistantWorkflowStarters(prompts, {
      targetType: 'vm',
      targetId: 'vm:old',
      handoffResources: [{ id: 'vm:101', name: 'web-101' }],
    });

    expect(starters.find((starter) => starter.name === 'pulse_investigate_resource')).toMatchObject(
      {
        label: 'Inspect selected resource',
        kind: 'resource',
        arguments: { resourceId: 'vm:101' },
      },
    );
  });

  it('does not treat Patrol run ids as resource workflow arguments', () => {
    const starters = getAssistantWorkflowStarters(prompts, {
      targetType: 'patrol-run',
      targetId: 'run-123',
      findingId: 'finding-1',
    });

    expect(starters.map((starter) => starter.name)).toEqual([
      'pulse_triage_fleet',
      'pulse_operations_loop',
      'pulse_review_finding',
    ]);
  });

  it('resolves finding workflow arguments from direct and action handoff context', () => {
    const direct = getAssistantWorkflowStarters(prompts, {
      findingId: 'finding-direct',
    });
    expect(direct.find((starter) => starter.name === 'pulse_review_finding')?.arguments).toEqual({
      finding_id: 'finding-direct',
    });

    const action = getAssistantWorkflowStarters(prompts, {
      handoffActions: [{ findingId: 'finding-action' }],
    });
    expect(action.find((starter) => starter.name === 'pulse_review_finding')?.arguments).toEqual({
      finding_id: 'finding-action',
    });
  });

  it('falls back to workflow presentation for undeclared kinds instead of inferring names', () => {
    const [starter] = getAssistantWorkflowStarters(
      [
        {
          name: 'pulse_triage_fleet',
          label: 'Legacy fleet prompt',
          presentationKind: 'legacy',
          arguments: [],
        },
      ],
      {},
    );

    expect(starter).toMatchObject({
      name: 'pulse_triage_fleet',
      kind: 'workflow',
    });
  });
});
