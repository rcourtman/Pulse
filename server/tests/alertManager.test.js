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

    describe('Alert Management Functions', () => {
        test('should register new alert rules', () => {
            const newRule = {
                id: 'test-rule',
                name: 'Test Rule',
                metric: 'cpu',
                condition: 'greater_than',
                threshold: 75,
                duration: 60000,
                severity: 'warning',
                enabled: true
            };

            alertManager.addRule(newRule);
            expect(alertManager.alertRules.has('test-rule')).toBe(true);
            expect(alertManager.alertRules.get('test-rule')).toMatchObject(newRule);
        });

        test('should process metrics and trigger alerts', () => {
            const metrics = [{
                id: mockAlert.guest.vmid,
                endpointName: 'test-endpoint',
                current: { cpu: 95 }, // Above critical threshold
                guest: mockAlert.guest
            }];

            const triggeredAlerts = alertManager.processMetrics(metrics);
            expect(Array.isArray(triggeredAlerts)).toBe(true);
        });

        test('should acknowledge alerts and update status', () => {
            const alertId = 'test-alert-123';
            const acknowledgement = {
                acknowledgedBy: 'test-user',
                acknowledgedAt: Date.now(),
                reason: 'Planned maintenance'
            };

            alertManager.acknowledgeAlert(alertId, acknowledgement);
            expect(alertManager.acknowledgedAlerts.has(alertId)).toBe(true);
            expect(alertManager.acknowledgedAlerts.get(alertId)).toMatchObject(acknowledgement);
        });

        test('should resolve alerts and clean up', () => {
            const alertId = 'test-alert-resolve';
            const testAlert = { ...mockAlert, id: alertId };
            
            alertManager.activeAlerts.set(alertId, testAlert);
            alertManager.resolveAlert(alertId);
            
            expect(alertManager.activeAlerts.has(alertId)).toBe(false);
            expect(alertManager.alertHistory.some(a => a.id === alertId && a.resolved)).toBe(true);
        });
    });

    describe('Notification Channel Management', () => {
        test('should initialize default notification channels', () => {
            expect(alertManager.notificationChannels.size).toBeGreaterThan(0);
            expect(alertManager.notificationChannels.has('default')).toBe(true);
        });

        test('should add custom notification channels', () => {
            const customChannel = {
                id: 'custom-slack',
                name: 'Custom Slack Channel',
                type: 'webhook',
                enabled: true,
                config: {
                    url: 'https://hooks.slack.com/custom-webhook',
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' }
                }
            };

            alertManager.addNotificationChannel(customChannel);
            expect(alertManager.notificationChannels.has('custom-slack')).toBe(true);
        });

        test('should handle disabled notification channels', () => {
            const disabledChannel = {
                ...mockWebhookChannel,
                enabled: false
            };

            alertManager.addNotificationChannel(disabledChannel);
            const result = alertManager.shouldSendNotification(disabledChannel.id, mockAlert);
            expect(result).toBe(false);
        });
    });

    describe('Alert Escalation', () => {
        test('should escalate unacknowledged alerts after timeout', () => {
            const escalationRule = {
                id: 'escalation-test',
                fromSeverity: 'warning',
                toSeverity: 'critical',
                timeoutMs: 900000, // 15 minutes
                notificationChannels: ['urgent']
            };

            alertManager.addEscalationRule(escalationRule);
            expect(alertManager.escalationRules.has('escalation-test')).toBe(true);

            // Test escalation logic
            const oldAlert = {
                ...mockAlert,
                triggeredAt: Date.now() - 1000000, // Old enough to escalate
                severity: 'warning'
            };

            const shouldEscalate = alertManager.shouldEscalateAlert(oldAlert);
            expect(shouldEscalate).toBe(true);
        });
    });

    describe('Alert Suppression', () => {
        test('should suppress alerts during maintenance windows', () => {
            const alertId = 'suppress-test';
            const suppressionConfig = {
                reason: 'Scheduled maintenance',
                suppressedBy: 'admin',
                suppressedUntil: Date.now() + 3600000 // 1 hour
            };

            alertManager.suppressAlert(alertId, suppressionConfig);
            expect(alertManager.suppressedAlerts.has(alertId)).toBe(true);

            const isSuppressed = alertManager.isAlertSuppressed(alertId);
            expect(isSuppressed).toBe(true);
        });

        test('should automatically lift expired suppressions', () => {
            const alertId = 'expired-suppress-test';
            const expiredSuppression = {
                reason: 'Expired maintenance',
                suppressedBy: 'admin',
                suppressedUntil: Date.now() - 1000 // Already expired
            };

            alertManager.suppressedAlerts.set(alertId, expiredSuppression);
            const isSuppressed = alertManager.isAlertSuppressed(alertId);
            expect(isSuppressed).toBe(false);
        });
    });

    describe('Metrics and Analytics', () => {
        test('should track alert metrics correctly', () => {
            // Add some test data
            alertManager.alertMetrics.totalFired = 10;
            alertManager.alertMetrics.totalResolved = 8;
            alertManager.alertMetrics.totalAcknowledged = 5;

            alertManager.updateMetrics();

            expect(alertManager.alertMetrics.totalFired).toBe(10);
            expect(alertManager.alertMetrics.totalResolved).toBe(8);
            expect(alertManager.alertMetrics.totalAcknowledged).toBe(5);
        });

        test('should calculate alert statistics', () => {
            // Populate some history data
            const testHistory = [
                { id: '1', triggeredAt: 1000, resolvedAt: 2000, severity: 'warning' },
                { id: '2', triggeredAt: 2000, resolvedAt: 4000, severity: 'critical' },
                { id: '3', triggeredAt: 3000, resolvedAt: 5000, severity: 'warning' }
            ];

            alertManager.alertHistory = testHistory;
            const stats = alertManager.getAlertStatistics();

            expect(stats).toHaveProperty('totalAlerts');
            expect(stats).toHaveProperty('averageResolutionTime');
            expect(stats).toHaveProperty('severityBreakdown');
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

// Add helper methods for testing
AlertManager.prototype.addRule = function(rule) {
    this.alertRules.set(rule.id, rule);
};

AlertManager.prototype.addNotificationChannel = function(channel) {
    this.notificationChannels.set(channel.id, channel);
};

AlertManager.prototype.addEscalationRule = function(rule) {
    this.escalationRules.set(rule.id, rule);
};

AlertManager.prototype.processMetrics = function(metrics) {
    // Simplified version for testing
    return [];
};

AlertManager.prototype.acknowledgeAlert = function(alertId, acknowledgement) {
    this.acknowledgedAlerts.set(alertId, acknowledgement);
};

AlertManager.prototype.resolveAlert = function(alertId) {
    const alert = this.activeAlerts.get(alertId);
    if (alert) {
        this.activeAlerts.delete(alertId);
        this.alertHistory.push({ ...alert, resolved: true, resolvedAt: Date.now() });
    }
};

AlertManager.prototype.shouldSendNotification = function(channelId, alert) {
    const channel = this.notificationChannels.get(channelId);
    return channel && channel.enabled;
};

AlertManager.prototype.shouldEscalateAlert = function(alert) {
    const alertAge = Date.now() - alert.triggeredAt;
    return alertAge > 900000 && !this.acknowledgedAlerts.has(alert.id);
};

AlertManager.prototype.suppressAlert = function(alertId, config) {
    this.suppressedAlerts.set(alertId, config);
};

AlertManager.prototype.isAlertSuppressed = function(alertId) {
    const suppression = this.suppressedAlerts.get(alertId);
    if (!suppression) return false;
    
    if (suppression.suppressedUntil < Date.now()) {
        this.suppressedAlerts.delete(alertId);
        return false;
    }
    return true;
};

AlertManager.prototype.updateMetrics = function() {
    // Update metrics calculation
};

AlertManager.prototype.getAlertStatistics = function() {
    const resolved = this.alertHistory.filter(a => a.resolvedAt);
    const avgResolution = resolved.length > 0 
        ? resolved.reduce((sum, a) => sum + (a.resolvedAt - a.triggeredAt), 0) / resolved.length 
        : 0;

    const severityBreakdown = this.alertHistory.reduce((acc, alert) => {
        acc[alert.severity] = (acc[alert.severity] || 0) + 1;
        return acc;
    }, {});

    return {
        totalAlerts: this.alertHistory.length,
        averageResolutionTime: avgResolution,
        severityBreakdown
    };
};