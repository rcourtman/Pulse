import { describe, expect, it } from 'vitest';
import { isPulseSystemAlert } from '../alertScope';

describe('Pulse system-alert scope markers', () => {
  it('accepts the canonical ID even when metadata is absent', () => {
    expect(isPulseSystemAlert({ id: 'pulse-system-future-condition' })).toBe(true);
  });
  it('accepts explicit metadata on a retained legacy ID', () => {
    expect(isPulseSystemAlert({ id: 'legacy-id', metadata: { systemAlert: true } })).toBe(true);
  });
  it.each([undefined, false, 'true', 1])(
    'does not infer system scope from metadata %j',
    (marker) => {
      expect(isPulseSystemAlert({ id: 'vm-pulse', metadata: { systemAlert: marker } })).toBe(false);
    },
  );
});
