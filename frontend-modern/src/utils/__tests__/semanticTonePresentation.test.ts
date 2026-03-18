import { describe, expect, it } from 'vitest';
import { getSemanticTonePresentation } from '@/utils/semanticTonePresentation';

describe('semanticTonePresentation', () => {
  it('returns success presentation', () => {
    const presentation = getSemanticTonePresentation('success');
    expect(presentation.panelClass).toContain('border-green-200');
    expect(presentation.iconClass).toContain('text-green-600');
  });

  it('returns warning presentation', () => {
    const presentation = getSemanticTonePresentation('warning');
    expect(presentation.panelClass).toContain('border-amber-200');
    expect(presentation.iconClass).toContain('text-amber-600');
  });

  it('returns error presentation', () => {
    const presentation = getSemanticTonePresentation('error');
    expect(presentation.panelClass).toContain('border-red-200');
    expect(presentation.iconClass).toContain('text-red-600');
  });

  it('defaults to info presentation', () => {
    const presentation = getSemanticTonePresentation();
    expect(presentation.panelClass).toContain('border-blue-200');
    expect(presentation.iconClass).toContain('text-blue-600');
  });
});
