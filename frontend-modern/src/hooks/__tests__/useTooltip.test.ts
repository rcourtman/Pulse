import { createRoot } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useTooltip } from '@/hooks/useTooltip';

describe('useTooltip', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('prefers pointer coordinates when they are available', () => {
    createRoot((dispose) => {
      const tip = useTooltip();
      tip.onMouseEnter({
        clientX: 120,
        clientY: 80,
        currentTarget: {
          getBoundingClientRect: () => ({ left: 10, top: 20, width: 40, height: 16 }),
        },
      } as unknown as MouseEvent);

      expect(tip.pos()).toEqual({ x: 120, y: 80 });
      expect(tip.show()).toBe(true);
      dispose();
    });
  });

  it('falls back to trigger geometry when pointer coordinates are missing', () => {
    createRoot((dispose) => {
      const tip = useTooltip();
      tip.onMouseEnter({
        clientX: Number.NaN,
        clientY: Number.NaN,
        currentTarget: {
          getBoundingClientRect: () => ({ left: 10, top: 20, width: 40, height: 16 }),
        },
      } as unknown as MouseEvent);

      expect(tip.pos()).toEqual({ x: 30, y: 20 });
      expect(tip.show()).toBe(true);
      dispose();
    });
  });

  it('dismisses an active hover tooltip when the pointer activates the page', () => {
    createRoot((dispose) => {
      const tip = useTooltip();
      tip.onMouseEnter({
        clientX: 120,
        clientY: 80,
        currentTarget: {
          ownerDocument: document,
          getBoundingClientRect: () => ({ left: 10, top: 20, width: 40, height: 16 }),
        },
      } as unknown as MouseEvent);

      expect(tip.show()).toBe(true);
      document.dispatchEvent(new Event('pointerdown', { bubbles: true }));
      expect(tip.show()).toBe(false);
      dispose();
    });
  });

  it('re-arms pointer dismissal after a subsequent hover', () => {
    createRoot((dispose) => {
      const tip = useTooltip();
      const enter = () =>
        tip.onMouseEnter({
          clientX: 120,
          clientY: 80,
          currentTarget: {
            ownerDocument: document,
            getBoundingClientRect: () => ({ left: 10, top: 20, width: 40, height: 16 }),
          },
        } as unknown as MouseEvent);

      enter();
      document.dispatchEvent(new Event('pointerdown', { bubbles: true }));
      enter();
      expect(tip.show()).toBe(true);

      document.dispatchEvent(new Event('pointerdown', { bubbles: true }));
      expect(tip.show()).toBe(false);
      dispose();
    });
  });

  it('ignores synthesized mouse hover on a coarse touch pointer', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: false,
        media: '(hover: hover) and (pointer: fine)',
      }),
    );

    createRoot((dispose) => {
      const tip = useTooltip();
      tip.onMouseEnter({
        clientX: 120,
        clientY: 80,
        currentTarget: {
          getBoundingClientRect: () => ({ left: 10, top: 20, width: 40, height: 16 }),
        },
      } as unknown as MouseEvent);

      expect(tip.pos()).toEqual({ x: 0, y: 0 });
      expect(tip.show()).toBe(false);
      dispose();
    });
  });
});
