import { createRoot } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useTooltip } from '@/hooks/useTooltip';

describe('useTooltip', () => {
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
});
