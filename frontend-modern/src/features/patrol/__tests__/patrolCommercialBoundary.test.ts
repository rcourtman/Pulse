import { describe, expect, it } from 'vitest';
import patrolIntelligenceHeaderSource from '../PatrolIntelligenceHeader.tsx?raw';

describe('patrol commercial boundary', () => {
  it('keeps patrol configuration out of the operator header', () => {
    expect(patrolIntelligenceHeaderSource).toContain('Open Patrol settings');
  });

  it('keeps blocked provider readiness to one primary action', () => {
    expect(patrolIntelligenceHeaderSource).toContain('Fix setup');
  });
});
