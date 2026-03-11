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
      title: 'Audit Webhooks (Pro)',
      body: expect.stringContaining('require Pro'),
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
