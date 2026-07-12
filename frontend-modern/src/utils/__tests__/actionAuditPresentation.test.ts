import { describe, expect, it } from 'vitest';

import {
  formatActionApprovalPolicyLabel,
  formatActionCapabilityLabel,
  getActionAuditRecordStatePresentation,
  getActionAuditRefusalPresentation,
  getActionAuditResultPresentation,
  getActionAuditVerification,
  getActionAuditVerificationOutcomePresentation,
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
    expect(
      getActionAuditRecordStatePresentation({
        state: 'failed',
        result: {
          success: false,
          errorMessage: 'plan_drift: resource policy changed',
        },
      }),
    ).toMatchObject({ label: 'Refused' });
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

  it('formats refused action result prefixes without prefix-first operator copy', () => {
    expect(
      getActionAuditRefusalPresentation({
        result: {
          success: false,
          errorMessage: 'plan_drift: policy version changed',
        },
      }),
    ).toMatchObject({
      prefix: 'plan_drift:',
      label: 'Plan changed',
      recordedDetail: 'policy version changed',
    });
    expect(
      getActionAuditRefusalPresentation({
        result: {
          success: false,
          errorMessage: 'action_plan_expired: approval expired at 12:00Z',
        },
      }),
    ).toMatchObject({ label: 'Approval expired' });
    expect(
      getActionAuditRefusalPresentation({
        result: {
          success: false,
          errorMessage: 'action_dry_run_only: execute was requested for dry-run evidence',
        },
      }),
    ).toMatchObject({ label: 'Dry-run only' });
    expect(
      getActionAuditRefusalPresentation({
        result: {
          success: false,
          errorMessage: 'resource_remediation_locked: operator lock is active',
        },
      }),
    ).toMatchObject({ label: 'Resource remediation locked' });

    const presentation = getActionAuditResultPresentation({
      result: {
        success: false,
        errorMessage: 'resource_remediation_locked: operator lock is active',
      },
    });
    expect(presentation).toMatchObject({
      kind: 'refusal',
      label: 'Execution refused',
      reasonLabel: 'Resource remediation locked',
      recordedDetail: 'operator lock is active',
    });
    expect(presentation?.detail).not.toContain('resource_remediation_locked:');
  });

  it('keeps ordinary execution failures distinct from refused dispatches', () => {
    expect(
      getActionAuditResultPresentation({
        result: {
          success: false,
          errorMessage: 'executor exited with code 1',
        },
      }),
    ).toMatchObject({
      kind: 'failure',
      label: 'Execution failed',
      detail: 'executor exited with code 1',
    });
  });

  it('formats verification outcome statuses with bounded operator copy', () => {
    expect(
      getActionAuditVerificationOutcomePresentation({
        verificationOutcome: {
          status: 'verified',
          evidenceSummary: 'Readback matched the intended running state.',
        },
      }),
    ).toMatchObject({
      label: 'Legacy check passed (source unclassified)',
      evidenceSummary: 'Readback matched the intended running state.',
    });
    expect(
      getActionAuditVerificationOutcomePresentation({
        verificationOutcome: {
          status: 'unverified',
        },
      }),
    ).toMatchObject({
      label: 'Verification not confirmed',
      detail: 'Pulse did not receive verification evidence that confirmed the intended state.',
    });
    expect(
      getActionAuditVerificationOutcomePresentation({
        verificationOutcome: {
          status: 'failed',
        },
      }),
    ).toMatchObject({ label: 'Verification failed' });
    expect(
      getActionAuditVerificationOutcomePresentation({
        verificationOutcome: {
          status: 'unknown',
        },
      }),
    ).toMatchObject({ label: 'Verification unknown' });
    expect(
      getActionAuditVerificationOutcomePresentation({
        verificationOutcome: {
          status: 'needs_review' as never,
        },
      }),
    ).toMatchObject({ label: 'Verification outcome recorded' });
  });
});
