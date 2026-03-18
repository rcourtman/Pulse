import { describe, expect, it } from 'vitest';
import { DIAGNOSTICS_EMPTY_PBS_MESSAGE } from '@/utils/diagnosticsPresentation';

describe('diagnosticsPresentation', () => {
  it('exports canonical diagnostics empty-state copy', () => {
    expect(DIAGNOSTICS_EMPTY_PBS_MESSAGE).toBe('No PBS configured');
  });
});
