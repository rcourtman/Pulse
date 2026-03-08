import { describe, expect, it } from 'vitest';
import {
  getPatrolRunStatusPresentation,
  getToolCallResultBadgeClass,
  getToolCallResultTextClass,
} from '@/utils/patrolRunPresentation';

describe('patrolRunPresentation', () => {
  it('maps critical and error runs to danger styling', () => {
    expect(getPatrolRunStatusPresentation('critical').badgeClass).toContain('red-100');
    expect(getPatrolRunStatusPresentation('error').badgeClass).toContain('red-100');
  });

  it('maps issues found to warning styling', () => {
    const presentation = getPatrolRunStatusPresentation('issues_found');
    expect(presentation.badgeClass).toContain('amber-100');
    expect(presentation.label).toBe('issues found');
  });

  it('maps healthy runs to success styling', () => {
    expect(getPatrolRunStatusPresentation('healthy').badgeClass).toContain('green-100');
  });

  it('normalizes unknown status labels safely', () => {
    const presentation = getPatrolRunStatusPresentation(' Needs Review ');
    expect(presentation.label).toBe('needs review');
    expect(presentation.badgeClass).toContain('bg-surface-alt');
  });

  it('maps tool call success and failure to canonical colors', () => {
    expect(getToolCallResultBadgeClass(true)).toContain('green-100');
    expect(getToolCallResultBadgeClass(false)).toContain('red-100');
    expect(getToolCallResultTextClass(true)).toContain('text-emerald-600');
    expect(getToolCallResultTextClass(false)).toContain('text-red-600');
  });
});
