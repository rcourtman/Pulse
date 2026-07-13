import { describe, expect, it } from 'vitest';
import { getActionApprovalBadgePresentation } from '../actionPresentation';

describe('getActionApprovalBadgePresentation', () => {
  it('returns null when nothing awaits approval', () => {
    expect(getActionApprovalBadgePresentation(0)).toBeNull();
    expect(getActionApprovalBadgePresentation(-2)).toBeNull();
    expect(getActionApprovalBadgePresentation(Number.NaN)).toBeNull();
  });

  it('labels a single pending approval', () => {
    expect(getActionApprovalBadgePresentation(1)).toEqual({
      count: 1,
      label: '1 action awaits approval',
    });
  });

  it('labels multiple pending approvals', () => {
    expect(getActionApprovalBadgePresentation(3)).toEqual({
      count: 3,
      label: '3 actions await approval',
    });
  });

  it('floors fractional counts from defensive callers', () => {
    expect(getActionApprovalBadgePresentation(2.9)).toEqual({
      count: 2,
      label: '2 actions await approval',
    });
  });
});
