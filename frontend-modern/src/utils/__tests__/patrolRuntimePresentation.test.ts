import { describe, expect, it } from 'vitest';

import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';

describe('patrolRuntimePresentation', () => {
  it('keeps enabled state separate from a run-in-progress label', () => {
    expect(getPatrolRuntimePresentation('running')).toMatchObject({
      label: 'Patrol enabled',
      title: 'Patrol running',
      description: 'Patrol is checking your infrastructure now.',
      tone: 'info',
    });
  });
});
