import { describe, expect, it } from 'vitest';
import { getProblemResourceStatusVariant } from '@/utils/problemResourcePresentation';

describe('problemResourcePresentation', () => {
  it('treats degraded dashboard problem resources as warning', () => {
    expect(getProblemResourceStatusVariant(149)).toBe('warning');
  });

  it('treats offline or critical dashboard problem resources as danger', () => {
    expect(getProblemResourceStatusVariant(150)).toBe('danger');
    expect(getProblemResourceStatusVariant(200)).toBe('danger');
  });
});
