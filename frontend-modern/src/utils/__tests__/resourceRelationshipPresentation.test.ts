import { describe, expect, it } from 'vitest';

import { describeResourceRelationship, formatResourceRelationshipType } from '@/utils/resourceRelationshipPresentation';

describe('resourceRelationshipPresentation', () => {
  it('formats canonical relationship type labels', () => {
    expect(formatResourceRelationshipType('runs_on')).toBe('Runs on');
    expect(formatResourceRelationshipType('depends_on')).toBe('Depends on');
    expect(formatResourceRelationshipType('mounted_to')).toBe('Mounted to');
    expect(formatResourceRelationshipType('exposed_by')).toBe('Exposed by');
    expect(formatResourceRelationshipType('owned_by')).toBe('Owned by');
    expect(formatResourceRelationshipType('custom_link')).toBe('Custom Link');
    expect(formatResourceRelationshipType('')).toBe('Related to');
  });

  it('describes canonical relationship context fragments', () => {
    const presentation = describeResourceRelationship({
      sourceId: 'node-1',
      targetId: 'vm-1',
      type: 'runs_on',
      confidence: 0.85,
      active: false,
      discoverer: 'proxmox_adapter',
      observedAt: '2026-03-18T12:00:00Z',
      lastSeenAt: '2026-03-18T12:05:00Z',
      metadata: {
        region: 'lab',
      },
    });

    expect(presentation).toMatchObject({
      typeLabel: 'Runs on',
      direction: 'node-1 → vm-1',
      provenance: 'proxmox_adapter',
      stateLabel: 'Historical',
      confidence: '85%',
      hasMetadata: true,
    });
  });
});
