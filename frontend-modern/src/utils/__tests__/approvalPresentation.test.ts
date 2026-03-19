import { describe, expect, it } from 'vitest';

import { getResourceApprovalLevelLabel } from '@/utils/approvalPresentation';

describe('approvalPresentation', () => {
  it('formats canonical approval labels', () => {
    expect(getResourceApprovalLevelLabel('none')).toBe('None');
    expect(getResourceApprovalLevelLabel('dry_run_only')).toBe('Dry Run Only');
    expect(getResourceApprovalLevelLabel('admin')).toBe('Admin');
    expect(getResourceApprovalLevelLabel('mfa')).toBe('MFA');
  });

  it('falls back safely for unknown approval levels', () => {
    expect(getResourceApprovalLevelLabel('strict')).toBe('Strict');
    expect(getResourceApprovalLevelLabel('')).toBe('—');
    expect(getResourceApprovalLevelLabel(undefined)).toBe('—');
  });
});
