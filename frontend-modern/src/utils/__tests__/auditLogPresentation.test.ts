import { describe, expect, it } from 'vitest';
import {
  AUDIT_REFRESH_BUTTON_CLASS,
  AUDIT_TOOLBAR_BUTTON_CLASS,
  AUDIT_VERIFY_ALL_BUTTON_CLASS,
  AUDIT_VERIFY_ROW_BUTTON_CLASS,
  getAuditEventStatusPresentation,
  getAuditEventTypeBadgeClass,
  getAuditVerificationBadgePresentation,
} from '@/utils/auditLogPresentation';

describe('auditLogPresentation', () => {
  it('returns canonical event type badge classes', () => {
    expect(getAuditEventTypeBadgeClass('login')).toContain('bg-blue-100');
    expect(getAuditEventTypeBadgeClass('config_change')).toContain('bg-yellow-100');
    expect(getAuditEventTypeBadgeClass('startup')).toContain('bg-green-100');
    expect(getAuditEventTypeBadgeClass('logout')).toContain('bg-surface-alt');
  });

  it('returns canonical verification badge presentation', () => {
    expect(getAuditVerificationBadgePresentation(undefined)).toEqual({
      label: 'Not checked',
      className: 'bg-surface-alt text-base-content',
    });
    expect(getAuditVerificationBadgePresentation({ status: 'verified' })).toEqual({
      label: 'Verified',
      className: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    });
    expect(getAuditVerificationBadgePresentation({ status: 'failed' })).toEqual({
      label: 'Failed',
      className: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    });
  });

  it('returns canonical event status icon presentation', () => {
    expect(getAuditEventStatusPresentation(true)).toMatchObject({
      className: 'w-4 h-4 text-emerald-400',
    });
    expect(getAuditEventStatusPresentation(false)).toMatchObject({
      className: 'w-4 h-4 text-rose-400',
    });
  });

  it('exposes canonical audit action button classes', () => {
    expect(AUDIT_TOOLBAR_BUTTON_CLASS).toContain('border border-border');
    expect(AUDIT_REFRESH_BUTTON_CLASS).toContain('text-base-content');
    expect(AUDIT_VERIFY_ALL_BUTTON_CLASS).toContain('text-blue-700');
    expect(AUDIT_VERIFY_ROW_BUTTON_CLASS).toContain('text-blue-600');
  });
});
