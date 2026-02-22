import { describe, expect, it } from 'vitest';
import { segmentedButtonClass } from '@/utils/segmentedButton';

describe('segmentedButtonClass', () => {
  it('returns selected classes when selected is true', () => {
    const result = segmentedButtonClass(true);
    expect(result).toContain('bg-surface');
    expect(result).toContain('shadow-sm');
    expect(result).toContain('ring-1');
  });

  it('returns unselected classes when selected is false', () => {
    const result = segmentedButtonClass(false);
    expect(result).toContain('hover:text-base-content');
    expect(result).toContain('hover:bg-surface-hover');
  });

  it('returns disabled classes when disabled is true', () => {
    const result = segmentedButtonClass(true, true);
    expect(result).toContain('cursor-not-allowed');
    expect(result).toContain('text-muted');
  });

  it('returns disabled classes with selected true but disabled true', () => {
    const result = segmentedButtonClass(true, true);
    expect(result).toContain('cursor-not-allowed');
  });

  it('includes transition classes for animation', () => {
    const result = segmentedButtonClass(false);
    expect(result).toContain('transition-all');
    expect(result).toContain('duration-150');
  });

  it('includes active scale transform', () => {
    const result = segmentedButtonClass(false);
    expect(result).toContain('active:scale-95');
  });

  it('includes flex layout', () => {
    const result = segmentedButtonClass(false);
    expect(result).toContain('inline-flex');
    expect(result).toContain('items-center');
    expect(result).toContain('gap-1.5');
  });

  it('includes text styling', () => {
    const result = segmentedButtonClass(false);
    expect(result).toContain('text-xs');
    expect(result).toContain('font-medium');
    expect(result).toContain('rounded-md');
  });
});
