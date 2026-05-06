import { describe, expect, it } from 'vitest';
import {
  AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS,
  AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS,
  AUDIT_WEBHOOK_READONLY_NOTICE_CLASS,
  getAuditWebhookEmptyStateCopy,
  getAuditWebhookFeatureGateCopy,
  getAuditWebhookLoadingState,
} from '@/utils/auditWebhookPresentation';

describe('auditWebhookPresentation', () => {
  it('returns canonical feature gate copy', () => {
    expect(getAuditWebhookFeatureGateCopy()).toMatchObject({
      title: 'Audit Webhooks',
      body: expect.stringContaining('paid self-hosted and hosted plans'),
    });
    expect(getAuditWebhookFeatureGateCopy().body).not.toContain('Pro');
  });

  it('returns neutral feature gate copy when commercial prompts are hidden', () => {
    expect(getAuditWebhookFeatureGateCopy({ showCommercialCopy: false })).toMatchObject({
      title: 'Audit Webhooks',
      body: expect.not.stringContaining('Pro'),
    });
  });

  it('returns paid-runtime copy when the license is active on a community runtime', () => {
    expect(getAuditWebhookFeatureGateCopy({ paidRuntimeRequired: true })).toMatchObject({
      title: 'Pulse Pro runtime required',
      body: expect.stringContaining('private Pulse Pro runtime'),
    });
  });

  it('returns canonical empty state copy and shell classes', () => {
    expect(getAuditWebhookEmptyStateCopy()).toMatchObject({
      title: 'No audit webhooks configured yet.',
    });
    expect(getAuditWebhookLoadingState()).toEqual({
      text: 'Loading audit webhooks…',
    });
    expect(AUDIT_WEBHOOK_READONLY_NOTICE_CLASS).toContain('border-blue-200');
    expect(AUDIT_WEBHOOK_ENDPOINT_CARD_CLASS).toContain('bg-surface-alt');
    expect(AUDIT_WEBHOOK_ENDPOINT_ICON_CLASS).toContain('bg-blue-100');
  });
});
