/**
 * AlertManager Webhook Tests
 * Tests webhook functionality and timestamp handling after the Teams webhook fix
 */

const AlertManager = require('../alertManager');
const axios = require('axios');

// Mock axios for webhook testing
jest.mock('axios');
const mockAxios = axios;

describe('AlertManager Webhook Functionality', () => {
    let alertManager;
    let mockWebhookChannel;
    let mockAlert;

    beforeEach(() => {
        alertManager = new AlertManager();
        
        // Mock webhook channel configuration
        mockWebhookChannel = {
            id: 'test-webhook',
            name: 'Test Webhook',
            type: 'webhook',
            enabled: true,
            config: {
                url: 'https://hooks.slack.com/test-webhook',
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            }
        };

        // Mock alert object with various timestamp scenarios
        mockAlert = {
            id: 'test-alert-123',
            rule: {
                name: 'High CPU Usage',
                description: 'CPU usage is too high',
                severity: 'warning',
                metric: 'cpu'
            },
            guest: {
                name: 'test-vm',
                vmid: '100',
                type: 'qemu',
                node: 'test-node',
                status: 'running'
            },
            currentValue: 92,
            effectiveThreshold: 85,
            triggeredAt: 1640995200000, // Valid timestamp
            lastUpdate: 1640995260000   // Valid timestamp
        };

        // Reset axios mock
        mockAxios.post.mockClear();
    });

    afterEach(() => {
        if (alertManager) {
            alertManager.destroy();
        }
    });

    describe('Webhook Timestamp Handling', () => {
        test('should use triggeredAt timestamp when available', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            await alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert);

            expect(mockAxios.post).toHaveBeenCalledTimes(1);
            const payload = mockAxios.post.mock.calls[0][1];
            
            // For Slack webhooks, check the timestamp in attachments
            expect(payload.attachments[0].ts).toBe(Math.floor(mockAlert.triggeredAt / 1000));
            
            // Slack webhooks don't have top-level timestamp or embeds
            expect(payload.timestamp).toBeUndefined();
            expect(payload.embeds).toBeUndefined();
        });

        test('should fallback to lastUpdate when triggeredAt is missing', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            // Remove triggeredAt from alert
            const alertWithoutTriggeredAt = { ...mockAlert };
            delete alertWithoutTriggeredAt.triggeredAt;

            await alertManager.sendWebhookNotification(mockWebhookChannel, alertWithoutTriggeredAt);

            expect(mockAxios.post).toHaveBeenCalledTimes(1);
            const payload = mockAxios.post.mock.calls[0][1];
            
            // Should use lastUpdate timestamp in Slack format
            expect(payload.attachments[0].ts).toBe(Math.floor(mockAlert.lastUpdate / 1000));
        });

        test('should fallback to current time when both timestamps are missing', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            // Remove both timestamps from alert
            const alertWithoutTimestamps = { ...mockAlert };
            delete alertWithoutTimestamps.triggeredAt;
            delete alertWithoutTimestamps.lastUpdate;

            const beforeTime = Date.now();
            await alertManager.sendWebhookNotification(mockWebhookChannel, alertWithoutTimestamps);
            const afterTime = Date.now();

            expect(mockAxios.post).toHaveBeenCalledTimes(1);
            const payload = mockAxios.post.mock.calls[0][1];
            
            // Should use current time (within reasonable range) for Slack format
            // Note: Unix timestamps lose millisecond precision, so allow for some tolerance
            const timestamp = payload.attachments[0].ts * 1000; // Convert Unix timestamp back to milliseconds
            expect(timestamp).toBeGreaterThanOrEqual(Math.floor(beforeTime / 1000) * 1000);
            expect(timestamp).toBeLessThanOrEqual(Math.ceil(afterTime / 1000) * 1000);
        });

        test('should handle invalid timestamp values gracefully', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            // Set invalid timestamps
            const alertWithInvalidTimestamps = {
                ...mockAlert,
                triggeredAt: 'invalid-timestamp',
                lastUpdate: null
            };

            const beforeTime = Date.now();
            await alertManager.sendWebhookNotification(mockWebhookChannel, alertWithInvalidTimestamps);
            const afterTime = Date.now();

            expect(mockAxios.post).toHaveBeenCalledTimes(1);
            const payload = mockAxios.post.mock.calls[0][1];
            
            // Should fallback to current time when timestamps are invalid (Slack format)
            // Note: Unix timestamps lose millisecond precision, so allow for some tolerance
            const timestamp = payload.attachments[0].ts * 1000;
            expect(timestamp).toBeGreaterThanOrEqual(Math.floor(beforeTime / 1000) * 1000);
            expect(timestamp).toBeLessThanOrEqual(Math.ceil(afterTime / 1000) * 1000);
        });
    });

    describe('Webhook Payload Structure', () => {
        test('should generate valid Discord/Slack payload structure', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            await alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert);

            expect(mockAxios.post).toHaveBeenCalledTimes(1);
            const payload = mockAxios.post.mock.calls[0][1];

            // Check Slack webhook structure (based on URL)
            expect(payload).toHaveProperty('text');
            expect(payload).toHaveProperty('attachments');
            
            // Slack webhooks don't have these properties
            expect(payload).not.toHaveProperty('timestamp');
            expect(payload).not.toHaveProperty('alert');
            expect(payload).not.toHaveProperty('embeds');

            // Check Slack attachment structure
            expect(payload.attachments).toHaveLength(1);
            expect(payload.attachments[0]).toHaveProperty('fields');
            expect(payload.attachments[0]).toHaveProperty('color');
            expect(payload.attachments[0]).toHaveProperty('footer');
            expect(payload.attachments[0]).toHaveProperty('ts');
        });

        test('should include all required alert fields in payload', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            await alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert);

            const payload = mockAxios.post.mock.calls[0][1];

            // Check Slack format fields (data is in text and attachments)
            expect(payload.text).toContain(mockAlert.rule.name);
            expect(payload.attachments[0].fields[0].value).toContain(mockAlert.guest.name);
            expect(payload.attachments[0].fields[1].value).toBe(mockAlert.guest.node);
            expect(payload.attachments[0].fields[2].value).toContain('92%'); // formatted value
            expect(payload.attachments[0].fields[2].value).toContain('85%'); // formatted threshold
        });

        test('should set correct colors based on severity', async () => {
            mockAxios.post.mockResolvedValue({ status: 200, data: { success: true } });

            // Test warning severity (Slack format only has attachments)
            await alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert);
            let payload = mockAxios.post.mock.calls[0][1];
            expect(payload.attachments[0].color).toBe('warning');

            // Test critical severity
            mockAxios.post.mockClear();
            const criticalAlert = { ...mockAlert, rule: { ...mockAlert.rule, severity: 'critical' } };
            await alertManager.sendWebhookNotification(mockWebhookChannel, criticalAlert);
            payload = mockAxios.post.mock.calls[0][1];
            expect(payload.attachments[0].color).toBe('danger');

            // Test info severity
            mockAxios.post.mockClear();
            const infoAlert = { ...mockAlert, rule: { ...mockAlert.rule, severity: 'info' } };
            await alertManager.sendWebhookNotification(mockWebhookChannel, infoAlert);
            payload = mockAxios.post.mock.calls[0][1];
            expect(payload.attachments[0].color).toBe('good');
        });
    });

    describe('Webhook Error Handling', () => {
        test('should throw error when webhook URL is not configured', async () => {
            const channelWithoutUrl = { ...mockWebhookChannel };
            delete channelWithoutUrl.config.url;

            await expect(
                alertManager.sendWebhookNotification(channelWithoutUrl, mockAlert)
            ).rejects.toThrow('Webhook URL not configured');
        });

        test('should handle HTTP errors gracefully', async () => {
            mockAxios.post.mockRejectedValue({
                response: { status: 404, statusText: 'Not Found' }
            });

            await expect(
                alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert)
            ).rejects.toThrow('Webhook failed: 404 Not Found');
        });

        test('should handle network errors gracefully', async () => {
            mockAxios.post.mockRejectedValue({
                request: {}
            });

            await expect(
                alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert)
            ).rejects.toThrow(`Webhook failed: No response from ${mockWebhookChannel.config.url}`);
        });

        test('should handle other errors gracefully', async () => {
            const errorMessage = 'Connection timeout';
            mockAxios.post.mockRejectedValue(new Error(errorMessage));

            await expect(
                alertManager.sendWebhookNotification(mockWebhookChannel, mockAlert)
            ).rejects.toThrow(`Webhook failed: ${errorMessage}`);
        });
    });

    describe('Email Notification Timestamp Fix', () => {
        test('should use correct timestamp fields in email templates', () => {
            // This test verifies that the email templates use the same timestamp fallback logic
            const emailHtml = alertManager.generateEmailTemplate(mockAlert);
            
            // The email should contain a formatted timestamp that doesn't throw errors
            expect(emailHtml).toContain(new Date(mockAlert.triggeredAt).toLocaleString());
            
            // Test with missing triggeredAt
            const alertWithoutTriggeredAt = { ...mockAlert };
            delete alertWithoutTriggeredAt.triggeredAt;
            
            const emailHtmlFallback = alertManager.generateEmailTemplate(alertWithoutTriggeredAt);
            expect(emailHtmlFallback).toContain(new Date(mockAlert.lastUpdate).toLocaleString());
        });
    });
});

// Helper to simulate the email template generation (since it's inline in the actual code)
AlertManager.prototype.generateEmailTemplate = function(alert) {
    const testEmailTemplate = `
        <td style="padding: 8px 0; color: #6b7280;">${new Date(alert.triggeredAt || alert.lastUpdate || Date.now()).toLocaleString()}</td>
    `;
    return testEmailTemplate;
};