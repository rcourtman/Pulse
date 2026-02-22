import { describe, it, expect } from 'vitest';
import {
  BREAKPOINTS,
  PRIORITY_BREAKPOINTS,
  type Breakpoint,
  type ColumnPriority,
} from '../useBreakpoint';

const getBreakpointName = (width: number): Breakpoint => {
  if (width >= BREAKPOINTS['2xl']) return '2xl';
  if (width >= BREAKPOINTS.xl) return 'xl';
  if (width >= BREAKPOINTS.lg) return 'lg';
  if (width >= BREAKPOINTS.md) return 'md';
  if (width >= BREAKPOINTS.sm) return 'sm';
  return 'xs';
};

const isAtLeast = (width: number, bp: Breakpoint): boolean => {
  return width >= BREAKPOINTS[bp];
};

const isBelow = (width: number, bp: Breakpoint): boolean => {
  return width < BREAKPOINTS[bp];
};

const isVisible = (width: number, priority: ColumnPriority): boolean => {
  const minBreakpoint = PRIORITY_BREAKPOINTS[priority];
  return width >= BREAKPOINTS[minBreakpoint];
};

const isMobile = (width: number): boolean => width < BREAKPOINTS.md;
const isTablet = (width: number): boolean =>
  width >= BREAKPOINTS.md && width < BREAKPOINTS.xl;
const isDesktop = (width: number): boolean => width >= BREAKPOINTS.xl;

describe('useBreakpoint (pure functions)', () => {
  describe('getBreakpointName', () => {
    it('returns xs for width below sm', () => {
      expect(getBreakpointName(0)).toBe('xs');
      expect(getBreakpointName(399)).toBe('xs');
      expect(getBreakpointName(639)).toBe('xs');
    });

    it('returns sm for width between sm and md', () => {
      expect(getBreakpointName(640)).toBe('sm');
      expect(getBreakpointName(700)).toBe('sm');
      expect(getBreakpointName(767)).toBe('sm');
    });

    it('returns md for width between md and lg', () => {
      expect(getBreakpointName(768)).toBe('md');
      expect(getBreakpointName(900)).toBe('md');
      expect(getBreakpointName(1023)).toBe('md');
    });

    it('returns lg for width between lg and xl', () => {
      expect(getBreakpointName(1024)).toBe('lg');
      expect(getBreakpointName(1100)).toBe('lg');
      expect(getBreakpointName(1279)).toBe('lg');
    });

    it('returns xl for width between xl and 2xl', () => {
      expect(getBreakpointName(1280)).toBe('xl');
      expect(getBreakpointName(1400)).toBe('xl');
      expect(getBreakpointName(1535)).toBe('xl');
    });

    it('returns 2xl for width >= 2xl', () => {
      expect(getBreakpointName(1536)).toBe('2xl');
      expect(getBreakpointName(2000)).toBe('2xl');
    });
  });

  describe('isAtLeast', () => {
    it('returns true for current and smaller breakpoints', () => {
      expect(isAtLeast(768, 'xs')).toBe(true);
      expect(isAtLeast(768, 'sm')).toBe(true);
      expect(isAtLeast(768, 'md')).toBe(true);
      expect(isAtLeast(768, 'lg')).toBe(false);
      expect(isAtLeast(768, 'xl')).toBe(false);
      expect(isAtLeast(768, '2xl')).toBe(false);
    });

    it('handles edge cases at breakpoint values', () => {
      expect(isAtLeast(640, 'sm')).toBe(true);
      expect(isAtLeast(639, 'sm')).toBe(false);
    });
  });

  describe('isBelow', () => {
    it('returns true for smaller breakpoints', () => {
      expect(isBelow(768, 'xs')).toBe(false);
      expect(isBelow(768, 'sm')).toBe(false);
      expect(isBelow(768, 'md')).toBe(false);
      expect(isBelow(768, 'lg')).toBe(true);
      expect(isBelow(768, 'xl')).toBe(true);
      expect(isBelow(768, '2xl')).toBe(true);
    });
  });

  describe('isMobile', () => {
    it('returns true below md breakpoint', () => {
      expect(isMobile(400)).toBe(true);
      expect(isMobile(639)).toBe(true);
      expect(isMobile(767)).toBe(true);
      expect(isMobile(768)).toBe(false);
      expect(isMobile(1280)).toBe(false);
    });
  });

  describe('isTablet', () => {
    it('returns true between md and xl', () => {
      expect(isTablet(400)).toBe(false);
      expect(isTablet(768)).toBe(true);
      expect(isTablet(1024)).toBe(true);
      expect(isTablet(1279)).toBe(true);
      expect(isTablet(1280)).toBe(false);
    });
  });

  describe('isDesktop', () => {
    it('returns true at xl and above', () => {
      expect(isDesktop(400)).toBe(false);
      expect(isDesktop(1279)).toBe(false);
      expect(isDesktop(1280)).toBe(true);
      expect(isDesktop(1536)).toBe(true);
    });
  });

  describe('isVisible', () => {
    it('returns correct visibility for column priorities', () => {
      expect(isVisible(400, 'essential')).toBe(true);
      expect(isVisible(640, 'primary')).toBe(true);
      expect(isVisible(768, 'secondary')).toBe(true);
      expect(isVisible(1024, 'supplementary')).toBe(true);
      expect(isVisible(1280, 'detailed')).toBe(true);

      expect(isVisible(400, 'primary')).toBe(false);
      expect(isVisible(400, 'secondary')).toBe(false);
      expect(isVisible(400, 'supplementary')).toBe(false);
      expect(isVisible(400, 'detailed')).toBe(false);
    });
  });

  describe('BREAKPOINTS constant', () => {
    it('exports correct values', () => {
      expect(BREAKPOINTS.xs).toBe(400);
      expect(BREAKPOINTS.sm).toBe(640);
      expect(BREAKPOINTS.md).toBe(768);
      expect(BREAKPOINTS.lg).toBe(1024);
      expect(BREAKPOINTS.xl).toBe(1280);
      expect(BREAKPOINTS['2xl']).toBe(1536);
    });
  });

  describe('PRIORITY_BREAKPOINTS mapping', () => {
    it('exports correct values', () => {
      expect(PRIORITY_BREAKPOINTS.essential).toBe('xs');
      expect(PRIORITY_BREAKPOINTS.primary).toBe('sm');
      expect(PRIORITY_BREAKPOINTS.secondary).toBe('md');
      expect(PRIORITY_BREAKPOINTS.supplementary).toBe('lg');
      expect(PRIORITY_BREAKPOINTS.detailed).toBe('xl');
    });
  });
});
