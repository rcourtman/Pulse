import { afterEach, describe, expect, it, vi } from 'vitest';
import { supportsHoverTooltips } from '@/components/shared/hoverCapability';

describe('supportsHoverTooltips', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('allows floating tooltips for a fine hover-capable pointer', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: true,
        media: '(hover: hover) and (pointer: fine)',
      }),
    );

    expect(supportsHoverTooltips()).toBe(true);
  });

  it('suppresses floating tooltips for a coarse touch pointer', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: false,
        media: '(hover: hover) and (pointer: fine)',
      }),
    );

    expect(supportsHoverTooltips()).toBe(false);
  });
});
