import { describe, expect, it } from 'vitest';
import {
  ALERT_WEBHOOK_TEST_FAILURE,
  ALERT_WEBHOOK_TEST_SUCCESS,
  getAlertWebhookTestFailure,
  getAlertWebhookTestSuccess,
} from '@/utils/alertWebhookPresentation';

describe('alertWebhookPresentation', () => {
  it('returns canonical webhook test-result copy', () => {
    expect(ALERT_WEBHOOK_TEST_SUCCESS).toBe('Test webhook sent successfully!');
    expect(ALERT_WEBHOOK_TEST_FAILURE).toBe('Failed to send test webhook');
    expect(getAlertWebhookTestSuccess()).toBe('Test webhook sent successfully!');
    expect(getAlertWebhookTestFailure()).toBe('Failed to send test webhook');
  });
});
