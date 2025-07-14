
const express = require('express');
const axios = require('axios');
const stateManager = require('../state');
const ValidationMiddleware = require('../middleware/validation');

const router = express.Router();

// Test route to verify router is working
router.all('/test', (req, res) => {
    res.json({ 
        success: true, 
        message: 'Alerts router test endpoint working',
        method: req.method,
        path: req.path
    });
});

// Enhanced alerts endpoint with filtering
router.get('/', ValidationMiddleware.validateQuery({
    fields: {
        group: { type: 'string', maxLength: 100 },
        node: { type: 'string', maxLength: 255 },
        acknowledged: { type: 'boolean' }
    }
}), (req, res) => {
    try {
        // Use validatedQuery if available (when req.query is read-only), otherwise use req.query
        const query = req.validatedQuery || req.query || {};
        const filters = {
            group: query.group,
            node: query.node,
            acknowledged: query.acknowledged
        };
        
        // Get alert data with safe serialization
        let alertInfo;
        try {
            const activeAlerts = stateManager.alertManager.getActiveAlerts(filters);
            const stats = stateManager.alertManager.getEnhancedAlertStats();
            const rules = stateManager.alertManager.getRules();
            
            alertInfo = {
                active: activeAlerts,
                stats: stats,
                rules: rules
            };
            
            // Test serialization of each part to identify the issue
            JSON.stringify(activeAlerts);
            JSON.stringify(stats);
            JSON.stringify(rules);
            
        } catch (serializationError) {
            // Serialization error logged silently
            
            // Return empty data if serialization fails
            alertInfo = {
                active: [],
                stats: {
                    active: 0,
                    acknowledged: 0,
                    last24Hours: 0,
                    lastHour: 0,
                    totalRules: 0,
                    suppressedRules: 0,
                    metrics: { totalFired: 0, totalResolved: 0, totalAcknowledged: 0, averageResolutionTime: 0, falsePositiveRate: 0 },
                    groups: []
                },
                rules: []
            };
        }
        
        res.json(alertInfo);
    } catch (error) {
        console.error("Error in /api/alerts:", error);
        res.status(500).json({ error: "Failed to fetch alert information" });
    }
});

router.get('/active', (req, res) => {
    try {
        const filters = {
            group: req.query.group,
            node: req.query.node,
            acknowledged: req.query.acknowledged === 'true' ? true : 
                         req.query.acknowledged === 'false' ? false : undefined
        };
        
        const activeAlerts = stateManager.alertManager.getActiveAlerts(filters);
        
        res.json({
            success: true,
            alerts: activeAlerts
        });
    } catch (error) {
        console.error("Error in /api/alerts/active:", error);
        res.status(500).json({ success: false, error: "Failed to fetch active alerts" });
    }
});


// Alert acknowledgment endpoint
router.post('/:alertId/acknowledge', 
    ValidationMiddleware.validateParams({
        fields: {
            alertId: { type: 'string', maxLength: 100 }
        },
        required: ['alertId']
    }),
    ValidationMiddleware.validateBody({
        fields: {
            userId: { type: 'string', maxLength: 50, default: 'api-user' },
            note: { type: 'string', maxLength: 1000, default: '' }
        }
    }),
    (req, res) => {
    try {
        const alertId = req.params.alertId;
        const { userId = 'api-user', note = '' } = req.body;
        
        const success = stateManager.alertManager.acknowledgeAlert(alertId, userId, note);
        
        if (success) {
            res.json({ success: true, message: "Alert acknowledged successfully" });
        } else {
            res.status(404).json({ error: "Alert not found" });
        }
    } catch (error) {
        console.error("Error acknowledging alert:", error);
        res.status(400).json({ error: error.message });
    }
});

// Alert suppression endpoint
router.post('/suppress',
    ValidationMiddleware.validateBody({
        fields: {
            ruleId: { type: 'string', maxLength: 100 },
            guestFilter: { type: 'object', default: {} },
            duration: { type: 'number', min: 0, max: 86400000, default: 3600000 },
            reason: { type: 'string', maxLength: 500, default: '' }
        },
        required: ['ruleId']
    }),
    (req, res) => {
    try {
        const { ruleId, guestFilter = {}, duration = 3600000, reason = '' } = req.body;
        
        const success = stateManager.alertManager.suppressAlert(ruleId, guestFilter, duration, reason);
        
        if (success) {
            res.json({ success: true, message: "Alert rule suppressed successfully" });
        } else {
            res.status(400).json({ error: "Failed to suppress alert rule" });
        }
    } catch (error) {
        console.error("Error suppressing alert:", error);
        res.status(400).json({ error: error.message });
    }
});

// Alert groups endpoint
router.get('/groups', (req, res) => {
    try {
        const stats = stateManager.alertManager.getEnhancedAlertStats();
        res.json({ groups: stats.groups });
    } catch (error) {
        console.error("Error in /api/alerts/groups:", error);
        res.status(500).json({ error: "Failed to fetch alert groups" });
    }
});



// Test email configuration
router.post('/test-email', async (req, res) => {
    try {
        // If body contains custom config, use sendTestEmailWithCustomTransporter
        if (req.body && Object.keys(req.body).length > 0) {
            const testResult = await stateManager.alertManager.sendTestEmailWithCustomTransporter(req.body);
            
            if (testResult.success) {
                res.json({ success: true, message: `Test email sent successfully via ${testResult.provider}` });
            } else {
                console.error('[Test Email] Failed to send test email:', testResult.error);
                res.status(400).json({ success: false, error: testResult.error || 'Failed to send test email' });
            }
        } else {
            // Use existing configuration
            const testResult = await stateManager.alertManager.sendTestEmail();
            
            if (testResult.success) {
                res.json({ success: true, message: 'Test email sent successfully' });
            } else {
                console.error('[Test Email] Failed to send test email:', testResult.error);
                res.status(400).json({ success: false, error: testResult.error || 'Failed to send test email' });
            }
        }
    } catch (error) {
        console.error('[Test Email] Error sending test email:', error);
        res.status(500).json({ success: false, error: 'Internal server error while sending test email' });
    }
});

// Test webhook endpoint
router.post('/test-webhook', async (req, res) => {
    try {
        
        // Get webhook URL from request body or environment
        const webhookUrl = req.body?.url || process.env.WEBHOOK_URL;
        
        if (!webhookUrl) {
            return res.status(400).json({ 
                success: false, 
                error: 'Webhook URL is required for testing' 
            });
        }
        
        // If URL is provided in body, test that specific URL
        if (req.body?.url) {
            // Direct webhook test with provided URL
            
            // Detect webhook type based on URL
            const isDiscord = webhookUrl.includes('discord.com/api/webhooks') || webhookUrl.includes('discordapp.com/api/webhooks');
            const isSlack = webhookUrl.includes('slack.com/') || webhookUrl.includes('hooks.slack.com');
            
            let testPayload;
            
            if (isDiscord) {
                // Discord-specific format
                testPayload = {
                    embeds: [{
                        title: '🧪 Webhook Test Alert',
                        description: 'This is a test alert to verify webhook configuration',
                        color: 3447003, // Blue
                        fields: [
                            {
                                name: 'Status',
                                value: '✅ Webhook configuration test successful!',
                                inline: false
                            }
                        ],
                        footer: {
                            text: 'Pulse Monitoring System'
                        },
                        timestamp: new Date().toISOString()
                    }]
                };
            } else if (isSlack) {
                // Slack-specific format
                testPayload = {
                    text: '🧪 *Webhook Test Alert*',
                    attachments: [{
                        color: 'good',
                        title: 'Test Alert',
                        text: 'This is a test alert from Pulse monitoring system',
                        fields: [
                            {
                                title: 'Status',
                                value: '✅ Webhook configuration test successful!',
                                short: false
                            }
                        ],
                        footer: 'Pulse Monitoring - Test',
                        ts: Math.floor(Date.now() / 1000)
                    }]
                };
            } else {
                // Generic webhook format
                testPayload = {
                    timestamp: new Date().toISOString(),
                    alert: {
                        id: 'test-' + Date.now(),
                        rule: {
                            name: 'Webhook Configuration Test',
                            description: 'This is a test webhook notification',
                            severity: 'info',
                            metric: 'webhook'
                        },
                        guest: {
                            name: 'Test VM',
                            node: 'test-node'
                        },
                        value: 100,
                        message: 'Webhook configuration test successful!'
                    },
                    test: true
                };
            }
            
            try {
                const response = await axios.post(webhookUrl, testPayload, {
                    timeout: 10000,
                    headers: {
                        'Content-Type': 'application/json',
                        'User-Agent': 'Pulse-Monitoring/1.0'
                    }
                });
                
                res.json({ 
                    success: true, 
                    message: 'Test webhook sent successfully!',
                    status: response.status
                });
            } catch (error) {
                console.error('[Test Webhook] Failed to send test webhook:', error);
                
                let errorMessage = 'Failed to send test webhook';
                if (error.response) {
                    errorMessage = `Webhook failed: ${error.response.status} ${error.response.statusText}`;
                } else if (error.request) {
                    errorMessage = `Webhook failed: No response from ${webhookUrl}`;
                } else {
                    errorMessage = `Webhook failed: ${error.message}`;
                }
                
                res.status(400).json({
                    success: false,
                    error: errorMessage
                });
            }
        } else {
            // Use the alert manager to send a test webhook with env URL
            const testResult = await stateManager.alertManager.sendTestWebhook();
            
            if (testResult.success) {
                res.json({ success: true, message: 'Test webhook sent successfully' });
            } else {
                console.error('[Test Webhook] Failed to send test webhook:', testResult.error);
                res.status(400).json({ success: false, error: testResult.error || 'Failed to send test webhook' });
            }
        }
    } catch (error) {
        console.error('[Test Webhook] Error sending test webhook:', error);
        res.status(500).json({ success: false, error: 'Internal server error while sending test webhook' });
    }
});

// Test alert notifications
router.post('/test', async (req, res) => {
    try {
        const { alertName, alertDescription, activeThresholds, notificationChannels, targetType, selectedVMs } = req.body;
        
        if (!alertName) {
            return res.status(400).json({ success: false, error: 'Alert name is required' });
        }
        
        if (!notificationChannels || notificationChannels.length === 0) {
            return res.status(400).json({ success: false, error: 'At least one notification channel is required' });
        }
        
        
        // Transform thresholds to match AlertManager format
        const transformedThresholds = (activeThresholds || []).map(threshold => ({
            metric: threshold.type,
            condition: 'greater_than_or_equal',
            threshold: threshold.value
        }));

        // Create a test alert object with realistic data
        const testAlert = {
            id: `test_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
            rule: {
                id: 'test-rule',
                name: alertName || 'Test Alert',
                description: alertDescription || 'This is a test alert to verify notification configuration',
                type: 'compound_threshold',
                thresholds: transformedThresholds,
                targetType: targetType || 'all',
                selectedVMs: selectedVMs || '[]'
            },
            guest: {
                name: 'test-vm',
                vmid: '100',
                node: 'node1',
                type: 'lxc',
                endpointId: 'primary'
            },
            message: `Test notification for alert rule "${alertName}"`,
            triggeredAt: Date.now(),
            details: {
                reason: 'Manual test triggered from dashboard',
                timestamp: new Date().toISOString(),
                notificationChannels: notificationChannels,
                activeThresholds: transformedThresholds,
                targetInfo: {
                    type: targetType || 'all',
                    selectedVMs: selectedVMs || '[]'
                }
            }
        };
        
        const results = {};
        
        // Test each requested notification channel
        for (const channel of notificationChannels) {
            try {
                switch (channel) {
                        
                    case 'email':
                        if (!process.env.ALERT_TO_EMAIL || !process.env.SMTP_HOST) {
                            results.email = { success: false, error: 'Email not configured' };
                        } else {
                            const emailResult = await stateManager.alertManager.sendTestAlertEmail({
                                alertName: alertName,
                                testAlert: testAlert,
                                config: {
                                    ALERT_TO_EMAIL: process.env.ALERT_TO_EMAIL,
                                    ALERT_FROM_EMAIL: process.env.ALERT_FROM_EMAIL,
                                    SMTP_HOST: process.env.SMTP_HOST,
                                    SMTP_PORT: process.env.SMTP_PORT,
                                    SMTP_USER: process.env.SMTP_USER,
                                    SMTP_SECURE: process.env.SMTP_SECURE
                                }
                            });
                            results.email = emailResult;
                            
                            // Update the test alert to show email was sent
                            if (emailResult.success) {
                                testAlert.emailSent = true;
                            }
                        }
                        break;
                        
                    case 'webhook':
                        if (!process.env.WEBHOOK_URL) {
                            results.webhook = { success: false, error: 'Webhook URL not configured' };
                        } else {
                            // Use the existing test webhook functionality
                            const webhookPayload = {
                                alert: testAlert,
                                type: 'test',
                                message: `Test notification: ${testAlert.message}`
                            };
                            
                            try {
                                await axios.post(process.env.WEBHOOK_URL, webhookPayload, {
                                    timeout: 10000,
                                    headers: { 'Content-Type': 'application/json' }
                                });
                                results.webhook = { success: true, message: 'Test webhook sent successfully' };
                                testAlert.webhookSent = true;
                            } catch (webhookError) {
                                results.webhook = { success: false, error: webhookError.message };
                            }
                        }
                        break;
                        
                    default:
                        results[channel] = { success: false, error: `Unknown notification channel: ${channel}` };
                }
            } catch (channelError) {
                results[channel] = { success: false, error: channelError.message };
            }
        }
        
        if (notificationChannels.includes('local')) {
            try {
                const alertManager = stateManager.alertManager;
                alertManager.addTestAlert(testAlert);
                
                // Force save the alert with updated notification flags
                alertManager.saveActiveAlerts();
                
                results.local = { success: true, message: 'Test alert added to dashboard' };
            } catch (localError) {
                results.local = { success: false, error: `Failed to add dashboard alert: ${localError.message}` };
            }
        }
        
        // Determine overall success
        const hasSuccess = Object.values(results).some(r => r.success);
        const hasFailure = Object.values(results).some(r => !r.success);
        
        const response = {
            success: hasSuccess,
            results: results,
            message: hasSuccess ? 
                (hasFailure ? 'Test completed with mixed results' : 'All test notifications sent successfully') :
                'All test notifications failed'
        };
        
        console.log('[Test Alert] Test completed successfully');
        res.json(response);
        
    } catch (error) {
        console.error('[Test Alert] Error sending test alert:', error);
        res.status(500).json({ success: false, error: 'Internal server error while testing alert' });
    }
});

// Alert rules management with filtering
router.get('/rules', (req, res) => {
    try {
        const filters = {
            group: req.query.group,
            severity: req.query.severity
        };
        
        const rules = stateManager.alertManager.getRules(filters);
        res.json({ rules });
    } catch (error) {
        console.error("Error in /api/alerts/rules:", error);
        res.status(500).json({ error: "Failed to fetch alert rules" });
    }
});

// Create new alert rule
router.post('/rules', (req, res) => {
    try {
        const rule = req.body;
        const newRule = stateManager.alertManager.addRule(rule);
        res.json({ success: true, message: "Rule added successfully", rule: newRule });
    } catch (error) {
        console.error("Error adding alert rule:", error);
        res.status(400).json({ error: error.message });
    }
});

// Update alert rule
router.put('/rules/:id', (req, res) => {
    try {
        const ruleId = req.params.id;
        const updates = req.body;
        const success = stateManager.alertManager.updateRule(ruleId, updates);
        
        if (success) {
            res.json({ success: true, message: "Rule updated successfully" });
        } else {
            res.status(404).json({ error: "Rule not found" });
        }
    } catch (error) {
        console.error("Error updating alert rule:", error);
        res.status(400).json({ error: error.message });
    }
});

// Delete alert rule
router.delete('/rules/:id', (req, res) => {
    try {
        const ruleId = req.params.id;
        const success = stateManager.alertManager.removeRule(ruleId);
        
        if (success) {
            res.json({ success: true, message: "Rule removed successfully" });
        } else {
            res.status(404).json({ error: "Rule not found" });
        }
    } catch (error) {
        console.error("Error removing alert rule:", error);
        res.status(400).json({ error: error.message });
    }
});

// Enhanced alerts/rules endpoints to handle compound threshold rules
router.get('/compound-rules', (req, res) => {
    try {
        const allRules = stateManager.alertManager.getRules();
        const compoundRules = allRules.filter(rule => rule.type === 'compound_threshold');
        res.json({ success: true, rules: compoundRules });
    } catch (error) {
        console.error("Error fetching compound threshold rules:", error);
        res.status(500).json({ error: "Failed to fetch compound threshold rules" });
    }
});

// Debug endpoint to manually reload alert rules
router.post('/rules/reload', async (req, res) => {
    try {
        await stateManager.alertManager.loadAlertRules();
        const allRules = stateManager.alertManager.getRules();
        res.json({ success: true, message: "Alert rules reloaded", rulesCount: allRules.length });
    } catch (error) {
        console.error("Error reloading alert rules:", error);
        res.status(500).json({ error: "Failed to reload alert rules" });
    }
});

// Endpoint to trigger immediate alert evaluation
router.post('/evaluate', async (req, res) => {
    try {
        
        stateManager.alertManager.evaluateCurrentState();
        res.json({ success: true, message: "Alert evaluation triggered" });
    } catch (error) {
        console.error("Error triggering alert evaluation:", error);
        res.status(500).json({ error: "Failed to trigger alert evaluation" });
    }
});

router.get('/debug', (req, res) => {
    try {
        const ruleId = req.query.ruleId;
        const currentState = stateManager.getState();
        const allGuests = [...(currentState.vms || []), ...(currentState.containers || [])];
        const metrics = currentState.metrics || [];
        
        const debugInfo = {
            timestamp: new Date().toISOString(),
            totalGuests: allGuests.length,
            totalMetrics: metrics.length,
            debugMode: process.env.ALERT_DEBUG === 'true',
            guests: []
        };
        
        // If specific rule requested, filter to that rule
        let rulesToCheck = stateManager.alertManager.getRules();
        if (ruleId) {
            rulesToCheck = rulesToCheck.filter(r => r.id === ruleId);
            if (rulesToCheck.length === 0) {
                return res.status(404).json({ error: `Rule '${ruleId}' not found` });
            }
        }
        
        // Evaluate each guest against each rule
        allGuests.forEach(guest => {
            const guestMetrics = metrics.find(m => 
                m.endpointId === guest.endpointId &&
                m.node === guest.node &&
                m.id === guest.vmid
            );
            
            const guestDebug = {
                name: guest.name,
                vmid: guest.vmid,
                node: guest.node,
                type: guest.type,
                hasMetrics: !!guestMetrics,
                rules: []
            };
            
            if (guestMetrics && guestMetrics.current) {
                // Calculate disk percentage if available
                if (guest.maxdisk && guestMetrics.current.disk) {
                    const diskPercentage = (guestMetrics.current.disk / guest.maxdisk) * 100;
                    guestDebug.diskUsage = {
                        raw: guestMetrics.current.disk,
                        max: guest.maxdisk,
                        percentage: Math.round(diskPercentage * 100) / 100
                    };
                }
                
                guestDebug.currentMetrics = {
                    cpu: guestMetrics.current.cpu,
                    memory: guestMetrics.current.memory,
                    disk: guestMetrics.current.disk
                };
            }
            
            // Check each rule
            rulesToCheck.forEach(rule => {
                if (rule.type === 'compound_threshold' && rule.thresholds) {
                    const ruleDebug = {
                        ruleId: rule.id,
                        ruleName: rule.name,
                        thresholds: [],
                        allThresholdsMet: true
                    };
                    
                    rule.thresholds.forEach(threshold => {
                        let metricValue = null;
                        let thresholdMet = false;
                        
                        if (guestMetrics && guestMetrics.current) {
                            metricValue = stateManager.alertManager.evaluateThresholdCondition(threshold, guestMetrics.current, guest) ? 
                                stateManager.alertManager.getThresholdCurrentValue(threshold, guestMetrics.current, guest) : null;
                            thresholdMet = stateManager.alertManager.evaluateThresholdCondition(threshold, guestMetrics.current, guest);
                        }
                        
                        ruleDebug.thresholds.push({
                            metric: threshold.metric,
                            condition: threshold.condition,
                            threshold: threshold.threshold,
                            currentValue: metricValue,
                            met: thresholdMet
                        });
                        
                        if (!thresholdMet) {
                            ruleDebug.allThresholdsMet = false;
                        }
                    });
                    
                    guestDebug.rules.push(ruleDebug);
                }
            });
            
            debugInfo.guests.push(guestDebug);
        });
        
        res.json(debugInfo);
    } catch (error) {
        console.error("Error in alert debug endpoint:", error);
        res.status(500).json({ error: "Failed to generate debug information" });
    }
});

// Simple endpoint to get just the alert enabled/disabled status
router.get('/status', (req, res) => {
    try {
        // Read the environment variables directly
        const alertStatus = {
            cpu: process.env.ALERT_CPU_ENABLED !== 'false',
            memory: process.env.ALERT_MEMORY_ENABLED !== 'false', 
            disk: process.env.ALERT_DISK_ENABLED !== 'false',
            down: process.env.ALERT_DOWN_ENABLED === 'true'
        };
        
        res.json({ success: true, alerts: alertStatus });
    } catch (error) {
        console.error("Error getting alert status:", error);
        res.status(500).json({ error: "Failed to get alert status" });
    }
});

// Test endpoint to debug the issue
router.post('/config-debug', (req, res) => {
    try {
        res.json({ 
            success: true, 
            message: 'Debug endpoint working',
            bodyReceived: !!req.body,
            bodyType: typeof req.body,
            bodyKeys: req.body ? Object.keys(req.body) : [],
            stateManager: !!stateManager,
            alertManager: !!(stateManager && stateManager.alertManager)
        });
    } catch (error) {
        res.status(500).json({ 
            success: false, 
            error: error.message,
            errorType: error.constructor.name
        });
    }
});

// Per-guest alert configuration endpoint
router.post('/config', (req, res) => {
    console.log('[API] /alerts/config endpoint called at', new Date().toISOString());
    
    // Wrap everything in try-catch to catch any error
    try {
        const alertConfig = req.body;
        
        console.log('[API] Request body type:', typeof req.body);
        console.log('[API] Request headers:', req.headers);
        
        if (!req.body) {
            console.error('[API] No request body received');
            return res.status(400).json({ 
                success: false, 
                error: 'No request body' 
            });
        }
        
        console.log('[API] Received alert config:', JSON.stringify(alertConfig, null, 2));
        console.log('[API] Alert config keys:', Object.keys(alertConfig));
        console.log('[API] Type of alertConfig:', typeof alertConfig);
        console.log('[API] Node thresholds in request:', {
            globalNodeThresholds: alertConfig.globalNodeThresholds,
            nodeThresholds: alertConfig.nodeThresholds
        });
        
        // Validate the alert configuration
        if (!alertConfig || typeof alertConfig !== 'object') {
            return res.status(400).json({ 
                success: false, 
                error: 'Invalid alert configuration' 
            });
        }
        
        // Create a rule from the per-guest configuration
        const rule = {
            id: 'per-guest-alerts',
            name: 'Per-Guest Alert Thresholds',
            description: 'Auto-generated rule from per-guest threshold configuration',
            type: 'per_guest_thresholds',
            globalThresholds: alertConfig.globalThresholds || {},
            guestThresholds: alertConfig.guestThresholds || {},
            globalNodeThresholds: alertConfig.globalNodeThresholds || {},
            nodeThresholds: alertConfig.nodeThresholds || {},
            alertLogic: alertConfig.alertLogic || 'and',
            duration: alertConfig.duration || 0,
            autoResolve: alertConfig.autoResolve !== false,
            enabled: alertConfig.enabled !== false,
            notifications: alertConfig.notifications || {
                dashboard: true,
                email: false,
                webhook: false
            },
            emailCooldowns: alertConfig.emailCooldowns || {},
            webhookCooldowns: alertConfig.webhookCooldowns || {},
            createdAt: alertConfig.lastUpdated || new Date().toISOString()
        };
        
        // Check if the rule already exists
        if (!stateManager || !stateManager.alertManager) {
            console.error('[API] Alert manager not initialized:', {
                stateManager: !!stateManager,
                alertManager: stateManager ? !!stateManager.alertManager : 'N/A'
            });
            throw new Error('Alert manager not initialized');
        }
        
        let allRules, existingRule;
        try {
            allRules = stateManager.alertManager.getRules();
            existingRule = allRules.find(r => r.id === 'per-guest-alerts');
        } catch (err) {
            console.error('[API] Error getting rules:', err);
            throw err;
        }
        
        if (existingRule) {
            // Update existing rule
            const success = stateManager.alertManager.updateRule('per-guest-alerts', {
                globalThresholds: rule.globalThresholds,
                guestThresholds: rule.guestThresholds,
                globalNodeThresholds: rule.globalNodeThresholds,
                nodeThresholds: rule.nodeThresholds,
                alertLogic: rule.alertLogic,
                duration: rule.duration,
                autoResolve: rule.autoResolve,
                notifications: rule.notifications,
                emailCooldowns: rule.emailCooldowns,
                webhookCooldowns: rule.webhookCooldowns,
                enabled: rule.enabled,
                lastUpdated: new Date().toISOString()
            });
            
            if (success) {
                res.json({ 
                    success: true, 
                    message: 'Alert configuration updated successfully' 
                });
            } else {
                res.status(500).json({ 
                    success: false, 
                    error: 'Failed to update alert configuration' 
                });
            }
        } else {
            // Create new rule
            try {
                const newRule = stateManager.alertManager.addRule(rule);
                res.json({ 
                    success: true, 
                    message: 'Alert configuration created successfully',
                    rule: newRule 
                });
            } catch (error) {
                res.status(400).json({ 
                    success: false, 
                    error: error.message 
                });
            }
        }
        
    } catch (error) {
        console.error('[API] Error in /alerts/config:', error);
        console.error('[API] Error type:', error.constructor.name);
        console.error('[API] Error message:', error.message);
        console.error('[API] Stack trace:', error.stack);
        
        // Make sure we haven't already sent a response
        if (!res.headersSent) {
            res.status(500).json({ 
                success: false, 
                error: error.message || 'Internal server error while saving alert configuration' 
            });
        } else {
            console.error('[API] Headers already sent, cannot send error response');
        }
    }
});

// Get per-guest alert configuration endpoint
router.get('/config', (req, res) => {
    try {
        const allRules = stateManager.alertManager.getRules();
        const existingRule = allRules.find(r => r.id === 'per-guest-alerts');
        
        if (existingRule && existingRule.type === 'per_guest_thresholds') {
            res.json({
                success: true,
                config: {
                    type: 'per_guest_thresholds',
                    globalThresholds: existingRule.globalThresholds || {},
                    guestThresholds: existingRule.guestThresholds || {},
                    alertLogic: existingRule.alertLogic || 'and',
                    duration: existingRule.duration || 0,
                    notifications: existingRule.notifications || {
                        dashboard: true,
                        email: false,
                        webhook: false
                    },
                    emailCooldowns: existingRule.emailCooldowns || {},
                    webhookCooldowns: existingRule.webhookCooldowns || {},
                    enabled: existingRule.enabled,
                    lastUpdated: existingRule.lastUpdated || existingRule.createdAt
                }
            });
        } else {
            // Return empty config if no per-guest rule exists
            res.json({
                success: true,
                config: {
                    type: 'per_guest_thresholds',
                    globalThresholds: {},
                    guestThresholds: {},
                    alertLogic: 'and',
                    duration: 0,
                    notifications: {
                        dashboard: true,
                        email: false,
                        webhook: false
                    },
                    emailCooldowns: {},
                    webhookCooldowns: {},
                    enabled: true,
                    lastUpdated: null
                }
            });
        }
    } catch (error) {
        console.error('Error loading alert configuration:', error);
        res.status(500).json({ 
            success: false, 
            error: 'Internal server error while loading alert configuration' 
        });
    }
});

module.exports = router;
