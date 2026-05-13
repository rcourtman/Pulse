import { describe, expect, it } from 'vitest';

import {
  formatActionApprovalPolicyLabel,
  formatActionCapabilityLabel,
  getActionAuditVerification,
  getActionAuditStatePresentation,
  shouldRenderActionAuditVerification,
} from '@/utils/actionAuditPresentation';

describe('actionAuditPresentation', () => {
  it('formats canonical action lifecycle states', () => {
    expect(getActionAuditStatePresentation('pending_approval')).toMatchObject({
      label: 'Pending approval',
    });
    expect(getActionAuditStatePresentation('completed')).toMatchObject({ label: 'Completed' });
    expect(getActionAuditStatePresentation('failed')).toMatchObject({ label: 'Failed' });
    expect(getActionAuditStatePresentation('unknown')).toMatchObject({ label: 'Unknown' });
  });

  it('formats capability and approval policy labels for operator history', () => {
    expect(formatActionCapabilityLabel('restart_service')).toBe('Restart Service');
    expect(formatActionCapabilityLabel('docker.update-container')).toBe('Docker Update Container');
    expect(formatActionCapabilityLabel('')).toBe('Action');
    expect(formatActionApprovalPolicyLabel('admin')).toBe('Admin approval');
    expect(formatActionApprovalPolicyLabel('dry_run_only')).toBe('Dry run only');
    expect(formatActionApprovalPolicyLabel('mfa')).toBe('MFA approval');
  });

  it('uses the canonical top-level verification field for rendering decisions', () => {
    expect(
      getActionAuditVerification({
        verification: {
          ran: true,
          success: true,
          command: "systemctl is-active 'nginx'",
        },
      }),
    ).toMatchObject({ ran: true, command: "systemctl is-active 'nginx'" });
    expect(
      getActionAuditVerification({
        result: {
          success: true,
          verification: {
            ran: false,
            success: false,
          },
        },
      }),
    ).toEqual({ ran: false, success: false });
    expect(
      shouldRenderActionAuditVerification({
        verification: {
          ran: true,
          success: true,
        },
      }),
    ).toBe(true);
    expect(
      shouldRenderActionAuditVerification({
        verification: {
          ran: false,
          success: false,
        },
      }),
    ).toBe(false);
  });
});
