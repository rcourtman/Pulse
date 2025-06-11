const EventEmitter = require('events');
const fs = require('fs').promises;
const path = require('path');
const nodemailer = require('nodemailer');
const axios = require('axios');
const customThresholdManager = require('./customThresholds');

class AlertManager extends EventEmitter {
    constructor() {
        super();
        this.alertRules = new Map();
        this.activeAlerts = new Map();
        this.alertHistory = [];
        this.acknowledgedAlerts = new Map();
        this.suppressedAlerts = new Map();
        this.notificationStatus = new Map();
        this.alertGroups = new Map();
        this.escalationRules = new Map();
        this.alertMetrics = {
            totalFired: 0,
            totalResolved: 0,
            totalAcknowledged: 0,
            averageResolutionTime: 0,
            falsePositiveRate: 0
        };
        
        this.maxHistorySize = 10000; // Increased for better analytics
        this.acknowledgementsFile = path.join(__dirname, '../data/acknowledgements.json');
        this.alertRulesFile = path.join(__dirname, '../data/alert-rules.json');
        this.activeAlertsFile = path.join(__dirname, '../data/active-alerts.json');
        this.notificationHistoryFile = path.join(__dirname, '../data/notification-history.json');
        
        // Add synchronization flags
        this.reloadingRules = false;
        this.processingMetrics = false;
        
        // Initialize alert groups
        this.initializeAlertGroups();
        
        // Load persisted state
        this.loadAcknowledgements();
        this.loadAlertRules();
        this.loadActiveAlerts();
        this.loadNotificationHistory();
        
        // Initialize custom threshold manager
        this.initializeCustomThresholds();
        
        // Initialize email transporter
        this.emailTransporter = null;
        this.emailConfig = null;
        this.initializeEmailTransporter();
        
        // Cleanup timer for resolved alerts
        this.cleanupInterval = setInterval(() => {
            this.cleanupResolvedAlerts();
            this.updateMetrics();
        }, 300000); // Every 5 minutes
        
        // Escalation check timer
        this.escalationInterval = setInterval(() => {
            this.checkEscalations();
        }, 60000); // Every minute
        
        // Watch alert rules file for changes
        this.setupAlertRulesWatcher();
    }
    
    setupAlertRulesWatcher() {
        const fs = require('fs');
        const path = require('path');
        
        try {
            console.log('[AlertManager] Setting up alert rules file watcher...');
            
            fs.watchFile(this.alertRulesFile, { interval: 1000 }, async (curr, prev) => {
                if (curr.mtime !== prev.mtime) {
                    console.log('[AlertManager] Alert rules file changed, reloading...');
                    
                    // Use a lock to prevent concurrent rule reloading
                    if (this.reloadingRules) {
                        console.log('[AlertManager] Rules already reloading, skipping...');
                        return;
                    }
                    
                    this.reloadingRules = true;
                    try {
                        await this.loadAlertRules();
                        console.log('[AlertManager] Alert rules reloaded successfully');
                        
                        // Safely evaluate current state with new rules
                        await this.evaluateCurrentState();
                    } catch (error) {
                        console.error('[AlertManager] Error during rule reload:', error);
                    } finally {
                        this.reloadingRules = false;
                    }
                }
            });
            
            console.log('[AlertManager] Alert rules file watcher active');
        } catch (error) {
            console.error('[AlertManager] Failed to setup alert rules watcher:', error);
        }
    }

    // Helper function to validate and parse environment variables
    parseEnvInt(envVar, defaultValue, min = 0, max = null) {
        const value = parseInt(process.env[envVar]);
        if (isNaN(value) || value < min || (max !== null && value > max)) {
            if (process.env[envVar]) {
                console.warn(`[AlertManager] Invalid value for ${envVar}: ${process.env[envVar]}, using default: ${defaultValue}`);
            }
            return defaultValue;
        }
        return value;
    }

    initializeAlertGroups() {
        this.alertGroups.set('system_performance', {
            id: 'system_performance',
            name: 'System Performance',
            description: 'CPU, Memory, and general performance alerts',
            color: '#f59e0b',
            priority: 2
        });

        this.alertGroups.set('critical_alerts', {
            id: 'critical_alerts',
            name: 'Critical Alerts',
            description: 'High-priority alerts requiring immediate attention',
            color: '#ef4444',
            priority: 1
        });

        this.alertGroups.set('storage_alerts', {
            id: 'storage_alerts',
            name: 'Storage Alerts',
            description: 'Disk space and storage-related alerts',
            color: '#8b5cf6',
            priority: 3
        });

        this.alertGroups.set('availability_alerts', {
            id: 'availability_alerts',
            name: 'Availability Alerts',
            description: 'Service and system availability alerts',
            color: '#ef4444',
            priority: 1
        });

        this.alertGroups.set('network_alerts', {
            id: 'network_alerts',
            name: 'Network Alerts',
            description: 'Network performance and anomaly alerts',
            color: '#10b981',
            priority: 4
        });
    }

    // Enhanced alert checking with custom conditions
    async checkMetrics(guests, metrics) {
        // Check if alerts are globally disabled
        if (process.env.ALERTS_ENABLED === 'false') {
            return;
        }
        
        if (this.processingMetrics || this.reloadingRules) {
            console.log('[AlertManager] Skipping metrics check - already processing or reloading rules');
            return;
        }
        
        this.processingMetrics = true;
        const timestamp = Date.now();
        const newlyTriggeredAlerts = [];
        
        try {
            // Validate inputs
            if (!Array.isArray(guests) || !Array.isArray(metrics)) {
                console.warn('[AlertManager] Invalid guests or metrics data provided');
                return;
            }
            
            guests.forEach(guest => {
                try {
                    // Evaluate all alert rules (both single-metric and compound threshold rules)
                    this.alertRules.forEach(rule => {
                        try {
                            if (!rule.enabled || this.isRuleSuppressed(rule.id, guest)) return;
                            
                            const alertKey = `${rule.id}_${guest.endpointId}_${guest.node}_${guest.vmid}`;
                            
                            if (rule.type === 'compound_threshold' && rule.thresholds) {
                                // Handle compound threshold rules
                                const triggered = this.evaluateCompoundThresholdRule(rule, guest, metrics, alertKey, timestamp);
                                if (triggered) newlyTriggeredAlerts.push(triggered);
                            } else {
                                // Handle single-metric rules
                                const triggered = this.evaluateRule(rule, guest, metrics, alertKey, timestamp);
                                if (triggered) newlyTriggeredAlerts.push(triggered);
                            }
                        } catch (ruleError) {
                            console.error(`[AlertManager] Error evaluating rule ${rule.id}:`, ruleError);
                        }
                    });
                } catch (guestError) {
                    console.error(`[AlertManager] Error processing guest ${guest.vmid}:`, guestError);
                }
            });
            
            // Emit newly triggered alerts
            newlyTriggeredAlerts.forEach(alert => {
                this.emit('alert', alert);
            });
            
        } catch (error) {
            console.error('[AlertManager] Error in checkMetrics:', error);
        } finally {
            this.processingMetrics = false;
        }
    }

    processMetrics(metricsData) {
        // Check if alerts are globally disabled
        if (process.env.ALERTS_ENABLED === 'false') {
            return;
        }
        
        const beforeAlertCount = this.activeAlerts.size;
        
        // Convert metricsData to guests format expected by checkMetrics
        const guests = metricsData.map(m => ({
            endpointId: m.endpointId,
            node: m.node || 'unknown',
            vmid: m.id,
            name: m.guest?.name || `Guest ${m.id}`,
            type: m.guest?.type || 'unknown',
            status: 'running' // Assume running if we have metrics
        }));
        
        // Process the metrics
        this.checkMetrics(guests, metricsData);
        
        // Return any new alerts that were triggered
        const newAlerts = [];
        for (const [key, alert] of this.activeAlerts) {
            if (alert.state === 'active' && alert.triggeredAt && alert.triggeredAt >= Date.now() - 5000) {
                newAlerts.push(alert);
            }
        }
        
        return newAlerts;
    }

    isRuleSuppressed(ruleId, guest) {
        const suppressKey = `${ruleId}_${guest.endpointId}_${guest.node}_${guest.vmid}`;
        const suppression = this.suppressedAlerts.get(suppressKey);
        
        if (!suppression) return false;
        
        if (Date.now() > suppression.expiresAt) {
            this.suppressedAlerts.delete(suppressKey);
            return false;
        }
        
        return true;
    }

    evaluateRule(rule, guest, metrics, alertKey, timestamp) {
        let isTriggered = false;
        let currentValue = null;
        let newlyTriggeredAlert = null;

        try {
            // Find metrics for this guest
            const guestMetrics = metrics.find(m => 
                m.endpointId === guest.endpointId &&
                m.node === guest.node &&
                m.id === guest.vmid
            );

            // Get effective threshold (custom or global)
            const effectiveThreshold = this.getEffectiveThreshold(rule, guest);

            // Enhanced condition evaluation
            if (rule.metric === 'status') {
                isTriggered = this.evaluateCondition(guest.status, rule.condition, effectiveThreshold);
                currentValue = guest.status;
            } else if (rule.metric === 'network_combined' && rule.condition === 'anomaly') {
                // Network anomaly detection
                isTriggered = this.detectNetworkAnomaly(guestMetrics, guest);
                currentValue = 'anomaly_detected';
            } else if (guestMetrics && guestMetrics.current) {
                const metricValue = this.getMetricValue(guestMetrics.current, rule.metric, guest);
                if (metricValue !== null) {
                    isTriggered = this.evaluateCondition(metricValue, rule.condition, effectiveThreshold);
                    currentValue = metricValue;
                }
            }

            const existingAlert = this.activeAlerts.get(alertKey);

            if (isTriggered) {
                if (!existingAlert) {
                    // Create new alert with permanent ID and safe copies of rule/guest objects
                    const newAlert = {
                        id: this.generateAlertId(), // Generate ID once when alert is created
                        rule: this.createSafeRuleCopy(rule),
                        guest: this.createSafeGuestCopy(guest),
                        startTime: timestamp,
                        lastUpdate: timestamp,
                        currentValue,
                        effectiveThreshold: effectiveThreshold,
                        state: 'pending',
                        escalated: false,
                        acknowledged: false
                    };
                    this.activeAlerts.set(alertKey, newAlert);
                } else if (existingAlert.state === 'pending') {
                    // Check if duration threshold is met
                    const duration = timestamp - existingAlert.startTime;
                    if (duration >= rule.duration) {
                        // Trigger alert
                        existingAlert.state = 'active';
                        existingAlert.triggeredAt = timestamp;
                        this.triggerAlert(existingAlert);
                        newlyTriggeredAlert = existingAlert;
                    }
                    existingAlert.lastUpdate = timestamp;
                    existingAlert.currentValue = currentValue;
                } else if (existingAlert.state === 'active' && !existingAlert.acknowledged) {
                    // Update existing active alert (only if not acknowledged)
                    existingAlert.lastUpdate = timestamp;
                    existingAlert.currentValue = currentValue;
                }
            } else {
                if (existingAlert && existingAlert.state === 'active' && !existingAlert.acknowledged) {
                    // Resolve alert (only if not acknowledged)
                    existingAlert.state = 'resolved';
                    existingAlert.resolvedAt = timestamp;
                    if (existingAlert.rule.autoResolve) {
                        this.resolveAlert(existingAlert);
                    }
                } else if (existingAlert && existingAlert.state === 'pending') {
                    // Remove pending alert that didn't trigger
                    this.activeAlerts.delete(alertKey);
                }
            }
        } catch (error) {
            console.error(`[AlertManager] Error in evaluateRule for ${alertKey}:`, error);
        }
        
        return newlyTriggeredAlert;
    }

    evaluateCondition(value, condition, threshold) {
        switch (condition) {
            case 'greater_than':
                return value > threshold;
            case 'less_than':
                return value < threshold;
            case 'equals':
                return value === threshold;
            case 'not_equals':
                return value !== threshold;
            case 'greater_than_or_equal':
                return value >= threshold;
            case 'less_than_or_equal':
                return value <= threshold;
            case 'contains':
                return String(value).includes(String(threshold));
            case 'anomaly':
                // This would be handled by specific anomaly detection logic
                return false;
            default:
                return value >= threshold; // Default fallback
        }
    }

    detectNetworkAnomaly(guestMetrics, guest) {
        // Improved anomaly detection with multiple criteria
        if (!guestMetrics || !guestMetrics.current) return false;
        
        const { netin = 0, netout = 0 } = guestMetrics.current;
        const totalTraffic = netin + netout;
        
        // Skip anomaly detection for very low traffic (likely idle systems)
        if (totalTraffic < 1024 * 1024) { // Less than 1 MB/s
            return false;
        }
        
        // Different thresholds based on guest type and name
        let suspiciousThreshold = 100 * 1024 * 1024; // Default: 100 MB/s
        
        // Higher thresholds for media/backup services that legitimately use more bandwidth
        const highBandwidthServices = ['plex', 'jellyfin', 'emby', 'frigate', 'backup', 'syncthing', 'nextcloud'];
        const isHighBandwidthService = highBandwidthServices.some(service => 
            guest.name.toLowerCase().includes(service)
        );
        
        if (isHighBandwidthService) {
            suspiciousThreshold = 500 * 1024 * 1024; // 500 MB/s for media services
        }
        
        // Very high threshold for obvious backup/storage services
        const backupServices = ['proxmox-backup', 'backup', 'storage', 'nas'];
        const isBackupService = backupServices.some(service => 
            guest.name.toLowerCase().includes(service)
        );
        
        if (isBackupService) {
            suspiciousThreshold = 1024 * 1024 * 1024; // 1 GB/s for backup services
        }
        
        // Check for suspicious patterns
        const isSuspiciousVolume = totalTraffic > suspiciousThreshold;
        
        // Check for highly asymmetric traffic (could indicate data exfiltration)
        const maxTraffic = Math.max(netin, netout);
        const minTraffic = Math.min(netin, netout);
        const asymmetryRatio = minTraffic > 0 ? maxTraffic / minTraffic : maxTraffic;
        const isSuspiciousAsymmetry = asymmetryRatio > 50 && maxTraffic > 50 * 1024 * 1024; // 50:1 ratio with >50MB/s
        
        // Only trigger if we have suspicious volume OR suspicious asymmetry
        return isSuspiciousVolume || isSuspiciousAsymmetry;
    }

    // Enhanced alert management methods
    acknowledgeAlert(alertId, userId = 'system', note = '') {
        for (const [key, alert] of this.activeAlerts) {
            if (alert.id === alertId || key.includes(alertId)) {
                alert.acknowledged = true;
                alert.acknowledgedBy = userId;
                alert.acknowledgedAt = Date.now();
                alert.acknowledgeNote = note;
                
                this.acknowledgedAlerts.set(key, {
                    ...alert,
                    acknowledgedBy: userId,
                    acknowledgedAt: Date.now(),
                    note
                });
                
                // Add acknowledgment to history
                const ackInfo = {
                    id: `ack_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
                    type: 'acknowledgment',
                    alertId: alert.id,
                    ruleId: alert.rule.id,
                    ruleName: alert.rule.name,
                    guest: {
                        name: alert.guest.name,
                        vmid: alert.guest.vmid,
                        node: alert.guest.node,
                        type: alert.guest.type,
                        endpointId: alert.guest.endpointId
                    },
                    acknowledgedBy: userId,
                    acknowledgedAt: Date.now(),
                    note: note,
                    message: `Alert acknowledged by ${userId}: ${alert.rule.name} on ${alert.guest.name}`
                };
                this.addToHistory(ackInfo);
                
                this.emit('alertAcknowledged', alert);
                
                // Save acknowledgements to file
                this.saveAcknowledgements();
                
                return true;
            }
        }
        return false;
    }

    suppressAlert(ruleId, guestFilter = {}, duration = 3600000, reason = '') {
        const suppressKey = this.generateSuppressionKey(ruleId, guestFilter);
        const expiresAt = Date.now() + duration;
        
        this.suppressedAlerts.set(suppressKey, {
            ruleId,
            guestFilter,
            reason,
            suppressedAt: Date.now(),
            expiresAt,
            suppressedBy: 'user'
        });
        
        this.emit('alertSuppressed', { ruleId, guestFilter, duration, reason });
        return true;
    }

    generateSuppressionKey(ruleId, guestFilter) {
        return `${ruleId}_${guestFilter.endpointId || '*'}_${guestFilter.node || '*'}_${guestFilter.vmid || '*'}`;
    }

    checkEscalations() {
        const now = Date.now();
        
        for (const [key, alert] of this.activeAlerts) {
            if (alert.state === 'active' && !alert.escalated && !alert.acknowledged) {
                const alertAge = now - alert.triggeredAt;
                if (alertAge >= alert.rule.escalationTime) {
                    this.escalateAlert(alert);
                }
            }
        }
    }

    escalateAlert(alert) {
        alert.escalated = true;
        alert.escalatedAt = Date.now();
        
        const escalatedAlert = {
            ...alert,
            message: `ESCALATED: ${alert.rule.name}`,
            escalated: true
        };
        
        this.emit('alertEscalated', escalatedAlert);
        this.sendNotifications(escalatedAlert, true);
        
        console.warn(`[ALERT ESCALATED] ${escalatedAlert.message}`);
    }


    sendNotifications(alert, forceUrgent = false) {
        // Check if we've already sent notifications for this alert
        const existingStatus = this.notificationStatus.get(alert.id);
        if (existingStatus && (existingStatus.emailSent || existingStatus.webhookSent)) {
            console.log(`[AlertManager] Skipping notifications for alert ${alert.id} - already sent (email: ${existingStatus.emailSent}, webhook: ${existingStatus.webhookSent})`);
            return;
        }
        
        // Check global settings first - these act as master switches
        const globalEmailEnabled = process.env.GLOBAL_EMAIL_ENABLED === 'true';
        const globalWebhookEnabled = process.env.GLOBAL_WEBHOOK_ENABLED === 'true';
        
        console.log(`[AlertManager] Email notification check - GLOBAL_EMAIL_ENABLED: ${process.env.GLOBAL_EMAIL_ENABLED}, globalEmailEnabled: ${globalEmailEnabled}`);
        
        let sendEmail, sendWebhook;
        
        // Global email acts as master switch - if disabled, never send emails
        if (!globalEmailEnabled) {
            sendEmail = false;
            console.log(`[AlertManager] Email disabled globally - sendEmail set to false`);
        } else {
            // Global email is enabled - check individual rule preferences and transporter
            const ruleEmailEnabled = alert.rule && alert.rule.sendEmail !== false; // Default to true for system rules
            sendEmail = ruleEmailEnabled && !!this.emailTransporter;
            console.log(`[AlertManager] Email enabled globally - ruleEmailEnabled: ${ruleEmailEnabled}, hasTransporter: ${!!this.emailTransporter}, sendEmail: ${sendEmail}`);
        }
        
        // Global webhook acts as master switch - if disabled, never send webhooks  
        if (!globalWebhookEnabled) {
            sendWebhook = false;
        } else {
            // Global webhook is enabled - check individual rule preferences
            const ruleWebhookEnabled = alert.rule && alert.rule.sendWebhook === true;
            sendWebhook = ruleWebhookEnabled && process.env.WEBHOOK_URL;
        }
        
        if (sendEmail) {
            console.log(`[AlertManager] Sending email notification for alert ${alert.id}`);
            this.sendDirectEmailNotification(alert).catch(error => {
                console.error(`[EMAIL ERROR] Failed to send email:`, error);
                this.emit('notificationError', { type: 'email', alert, error });
            });
        } else {
            console.log(`[AlertManager] NOT sending email notification for alert ${alert.id} - sendEmail: ${sendEmail}`);
        }
        
        if (sendWebhook) {
            this.sendDirectWebhookNotification(alert).catch(error => {
                console.error(`[WEBHOOK ERROR] Failed to send webhook:`, error);
                this.emit('notificationError', { type: 'webhook', alert, error });
            });
        }
        
        // Store notification status separately to avoid any circular references
        // Don't attach anything to the alert object itself
        const alertId = alert.id;
        if (!this.notificationStatus) {
            this.notificationStatus = new Map();
        }
        this.notificationStatus.set(alertId, {
            emailSent: sendEmail,
            webhookSent: sendWebhook,
            channels: sendEmail || sendWebhook ? [
                ...(sendEmail ? ['email'] : []),
                ...(sendWebhook ? ['webhook'] : [])
            ] : []
        });
        
        // Emit event for external handlers (use safe subset of alert data)
        this.emit('notification', { 
            alertId: alert.id, 
            ruleId: alert.rule.id, 
            guest: alert.guest, 
            sentEmail: sendEmail, 
            sentWebhook: sendWebhook 
        });
    }

    updateMetrics() {
        // Calculate alert metrics for analytics
        const now = Date.now();
        const last24h = now - (24 * 60 * 60 * 1000);
        
        const recentAlerts = this.alertHistory.filter(a => 
            (a.triggeredAt || a.resolvedAt) >= last24h
        );
        
        this.alertMetrics.totalFired = recentAlerts.filter(a => a.triggeredAt).length;
        this.alertMetrics.totalResolved = recentAlerts.filter(a => a.resolvedAt).length;
        this.alertMetrics.totalAcknowledged = this.acknowledgedAlerts.size;
        
        // Calculate average resolution time
        const resolvedWithDuration = recentAlerts.filter(a => a.triggeredAt && a.resolvedAt);
        if (resolvedWithDuration.length > 0) {
            const totalDuration = resolvedWithDuration.reduce((sum, a) => 
                sum + (a.resolvedAt - a.triggeredAt), 0
            );
            this.alertMetrics.averageResolutionTime = totalDuration / resolvedWithDuration.length;
        }
    }

    // Enhanced getters with filtering and pagination
    getActiveAlerts(filters = {}) {
        const active = [];
        for (const alert of this.activeAlerts.values()) {
            // Include both 'active' alerts and 'resolved' alerts that have autoResolve=false
            if ((alert.state === 'active' || (alert.state === 'resolved' && !alert.rule.autoResolve)) 
                && this.matchesFilters(alert, filters)) {
                try {
                    const formattedAlert = this.formatAlertForAPI(alert);
                    // Test serialization before adding
                    JSON.stringify(formattedAlert);
                    active.push(formattedAlert);
                } catch (alertError) {
                    console.error(`[AlertManager] Skipping alert ${alert.id} due to serialization error:`, alertError.message);
                    // Find and remove the problematic alert from activeAlerts to prevent repeated errors
                    for (const [key, storedAlert] of this.activeAlerts) {
                        if (storedAlert.id === alert.id) {
                            console.warn(`[AlertManager] Removing corrupted alert ${alert.id} from activeAlerts`);
                            this.activeAlerts.delete(key);
                            break;
                        }
                    }
                    // Skip this alert but continue processing others
                }
            }
        }
        return active.sort((a, b) => b.triggeredAt - a.triggeredAt);
    }

    getAlertHistory(limit = 100, filters = {}) {
        let filtered = this.alertHistory.filter(alert => this.matchesFilters(alert, filters));
        return filtered.slice(0, limit);
    }

    matchesFilters(alert, filters) {
        if (filters.group && alert.rule.group !== filters.group) return false;
        if (filters.node && alert.guest.node !== filters.node) return false;
        if (filters.acknowledged !== undefined && alert.acknowledged !== filters.acknowledged) return false;
        return true;
    }

    formatAlertForAPI(alert) {
        try {
            // Create a safe, serializable alert object with no circular references
            const safeAlert = {
                id: alert.id, // Use the stored permanent ID
                ruleId: alert.rule?.id || 'unknown',
                ruleName: alert.rule?.name || 'Unknown Rule',
                description: alert.rule?.description || '',
                group: alert.rule?.group || 'unknown',
                tags: Array.isArray(alert.rule?.tags) ? [...alert.rule.tags] : [],
                guest: {
                    name: alert.guest?.name || 'Unknown',
                    vmid: alert.guest?.vmid || 'unknown',
                    node: alert.guest?.node || 'unknown',
                    type: alert.guest?.type || 'unknown',
                    endpointId: alert.guest?.endpointId || 'unknown'
                },
                metric: alert.rule?.metric || (alert.rule?.type === 'compound_threshold' ? 'compound' : null),
                threshold: alert.effectiveThreshold || alert.rule?.threshold || 0,
                currentValue: alert.currentValue || null,
                triggeredAt: alert.triggeredAt || Date.now(),
                duration: (alert.triggeredAt ? Date.now() - alert.triggeredAt : 0),
                acknowledged: alert.acknowledged || false,
                acknowledgedBy: alert.acknowledgedBy || null,
                acknowledgedAt: alert.acknowledgedAt || null,
                escalated: alert.escalated || false,
                message: this.generateAlertMessage(alert),
                type: alert.rule?.type || 'single_metric',
                thresholds: Array.isArray(alert.rule?.thresholds) ? 
                    alert.rule.thresholds.map(t => ({
                        metric: t.metric,
                        condition: t.condition,
                        threshold: t.threshold
                    })) : null,
                emailSent: this.notificationStatus?.get(alert.id)?.emailSent || false,
                webhookSent: this.notificationStatus?.get(alert.id)?.webhookSent || false,
                notificationChannels: this.notificationStatus?.get(alert.id)?.channels || []
            };
            
            // Test serialization
            JSON.stringify(safeAlert);
            return safeAlert;
        } catch (serializationError) {
            console.error(`[AlertManager] formatAlertForAPI serialization error for alert ${alert.id}:`, serializationError.message);
            console.error(`[AlertManager] Problematic alert keys:`, Object.keys(alert));
            
            // Try to identify which property is causing the issue
            const debugAlert = {};
            for (const [key, value] of Object.entries(alert)) {
                try {
                    JSON.stringify({ [key]: value });
                    debugAlert[key] = typeof value;
                } catch (keyError) {
                    console.error(`[AlertManager] Circular reference in alert.${key}:`, keyError.message);
                    if (typeof value === 'object' && value !== null) {
                        console.error(`[AlertManager] Problematic object keys for alert.${key}:`, Object.keys(value));
                        if (value.constructor) {
                            console.error(`[AlertManager] Object constructor: ${value.constructor.name}`);
                        }
                        
                        // Deep check each property
                        if (typeof value === 'object') {
                            for (const [subKey, subValue] of Object.entries(value)) {
                                try {
                                    JSON.stringify({ [subKey]: subValue });
                                } catch (subError) {
                                    console.error(`[AlertManager] Circular reference in alert.${key}.${subKey}:`, subError.message);
                                    if (subValue && subValue.constructor) {
                                        console.error(`[AlertManager] SubObject constructor: ${subValue.constructor.name}`);
                                    }
                                }
                            }
                        }
                    }
                }
            }
            console.error(`[AlertManager] Alert property types:`, debugAlert);
            
            // Return minimal but useful alert data
            return {
                id: alert.id || 'unknown',
                ruleId: alert.rule?.id || 'unknown',
                ruleName: alert.rule?.name || 'Unknown Rule',
                description: alert.rule?.description || '',
                group: alert.rule?.group || 'unknown',
                tags: [],
                guest: {
                    name: alert.guest?.name || 'Unknown',
                    vmid: alert.guest?.vmid || 'unknown',
                    node: alert.guest?.node || 'unknown',
                    type: alert.guest?.type || 'unknown',
                    endpointId: alert.guest?.endpointId || 'unknown'
                },
                metric: alert.rule?.metric || 'unknown',
                threshold: alert.effectiveThreshold || alert.rule?.threshold || 0,
                currentValue: alert.currentValue || null,
                triggeredAt: alert.triggeredAt || Date.now(),
                duration: (alert.triggeredAt ? Date.now() - alert.triggeredAt : 0),
                acknowledged: false,
                acknowledgedBy: null,
                acknowledgedAt: null,
                escalated: false,
                message: `${alert.rule?.name || 'Alert'} - ${alert.guest?.name || 'Unknown'} on ${alert.guest?.node || 'Unknown'}`,
                type: alert.rule?.type || 'single_metric',
                thresholds: null,
                emailSent: true, // We know it's true since you're getting emails
                webhookSent: false,
                notificationChannels: ['email']
            };
        }
    }

    // Helper function to sanitize currentValue for safe serialization
    sanitizeCurrentValue(currentValue) {
        if (currentValue === null || currentValue === undefined) {
            return null;
        }
        
        try {
            // If it's a simple value, return as-is
            if (typeof currentValue !== 'object') {
                return currentValue;
            }
            
            // For objects, create a clean copy with only safe values
            const sanitized = {};
            for (const [key, value] of Object.entries(currentValue)) {
                if (typeof value === 'number' || typeof value === 'string' || typeof value === 'boolean') {
                    sanitized[key] = value;
                }
            }
            return sanitized;
        } catch (error) {
            console.error('[AlertManager] Error sanitizing currentValue:', error);
            return null;
        }
    }

    // Helper function to create safe serializable alert data for WebSocket events
    createSafeAlertForEmit(alert) {
        try {
            // Create a safe alert with necessary data for frontend
            const safeAlert = {
                id: String(alert.id || 'unknown'),
                ruleId: String(alert.rule?.id || 'unknown'),
                ruleName: String(alert.rule?.name || 'Unknown Rule'),
                acknowledged: Boolean(alert.acknowledged),
                triggeredAt: Number(alert.triggeredAt || Date.now()),
                message: String(alert.rule?.name || 'Alert triggered'),
                guest: {
                    name: String(alert.guest?.name || 'Unknown'),
                    vmid: String(alert.guest?.vmid || 'unknown'),
                    node: String(alert.guest?.node || 'unknown'),
                    type: String(alert.guest?.type || 'unknown')
                }
            };
            
            // Test serialization
            JSON.stringify(safeAlert);
            return safeAlert;
        } catch (error) {
            console.error('[AlertManager] Even safe emit failed:', error);
            // Return absolute minimal data with guest fallback
            return {
                id: 'unknown',
                ruleId: 'unknown',
                message: 'Alert update',
                triggeredAt: Date.now(),
                guest: {
                    name: 'Unknown',
                    vmid: 'unknown',
                    node: 'unknown',
                    type: 'unknown'
                }
            };
        }
    }

    // Helper function to create a safe copy of a rule object without circular references
    createSafeRuleCopy(rule) {
        return {
            id: rule.id,
            name: rule.name,
            description: rule.description,
            metric: rule.metric,
            condition: rule.condition,
            threshold: rule.threshold,
            duration: rule.duration,
            enabled: rule.enabled,
            tags: Array.isArray(rule.tags) ? [...rule.tags] : [],
            group: rule.group,
            escalationTime: rule.escalationTime,
            autoResolve: rule.autoResolve,
            suppressionTime: rule.suppressionTime,
            type: rule.type,
            thresholds: Array.isArray(rule.thresholds) ? 
                rule.thresholds.map(t => ({
                    metric: t.metric,
                    condition: t.condition,
                    threshold: t.threshold
                })) : undefined,
            sendEmail: rule.sendEmail,
            sendWebhook: rule.sendWebhook
        };
    }

    // Helper function to create a safe copy of a guest object without circular references
    createSafeGuestCopy(guest) {
        return {
            endpointId: guest.endpointId,
            node: guest.node,
            vmid: guest.vmid,
            name: guest.name,
            type: guest.type,
            status: guest.status,
            maxmem: guest.maxmem,
            maxdisk: guest.maxdisk
        };
    }

    createSafeRuleCopy(rule) {
        return {
            id: rule.id,
            name: rule.name,
            description: rule.description,
            metric: rule.metric,
            condition: rule.condition,
            threshold: rule.threshold,
            duration: rule.duration,
            enabled: rule.enabled,
            tags: rule.tags ? [...rule.tags] : [],
            group: rule.group,
            escalationTime: rule.escalationTime,
            autoResolve: rule.autoResolve,
            suppressionTime: rule.suppressionTime,
            type: rule.type,
            thresholds: rule.thresholds ? rule.thresholds.map(t => ({
                metric: t.metric,
                condition: t.condition,
                threshold: t.threshold
            })) : [],
            sendEmail: rule.sendEmail,
            sendWebhook: rule.sendWebhook
        };
    }

    getEnhancedAlertStats() {
        const now = Date.now();
        const oneDayAgo = now - (24 * 60 * 60 * 1000);
        const oneHourAgo = now - (60 * 60 * 1000);

        const last24h = this.alertHistory.filter(a => 
            (a.triggeredAt || a.resolvedAt) >= oneDayAgo
        );
        const lastHour = this.alertHistory.filter(a => 
            (a.triggeredAt || a.resolvedAt) >= oneHourAgo
        );

        const activeCount = Array.from(this.activeAlerts.values())
            .filter(a => a.state === 'active' || (a.state === 'resolved' && !a.rule.autoResolve)).length;

        const acknowledgedCount = Array.from(this.activeAlerts.values())
            .filter(a => a.acknowledged).length;

        const escalatedCount = Array.from(this.activeAlerts.values())
            .filter(a => a.escalated).length;

        return {
            active: activeCount,
            acknowledged: acknowledgedCount,
            escalated: escalatedCount,
            last24Hours: last24h.length,
            lastHour: lastHour.length,
            totalRules: this.alertRules.size,
            suppressedRules: this.suppressedAlerts.size,
            metrics: this.alertMetrics,
            groups: Array.from(this.alertGroups.values())
        };
    }

    // Rest of the existing methods with enhancements...
    getMetricValue(metrics, metricName, guest) {
        switch (metricName) {
            case 'cpu':
                // CPU values from Proxmox VE API are typically decimals (0.0-1.0)
                // but in some processing they might already be converted to percentages
                const cpuValue = metrics.cpu;
                if (typeof cpuValue !== 'number' || isNaN(cpuValue)) {
                    return 0; // Invalid CPU value
                }
                
                // If value is > 1.0, assume it's already in percentage format
                // If value is <= 1.0, assume it's in decimal format and convert to percentage
                return cpuValue > 1.0 ? cpuValue : cpuValue * 100;
            case 'memory':
                if (guest.maxmem && metrics.mem) {
                    return (metrics.mem / guest.maxmem) * 100;
                }
                return null;
            case 'disk':
                if (guest.maxdisk && metrics.disk) {
                    return (metrics.disk / guest.maxdisk) * 100;
                }
                return null;
            default:
                return metrics[metricName] || null;
        }
    }

    triggerAlert(alert) {
        try {
            const alertInfo = this.formatAlertForAPI(alert);
            
            // Add to history
            this.addToHistory(alertInfo);
            
            // Send notifications
            this.sendNotifications(alert);
            
            // Save active alerts and notification history to disk
            this.saveActiveAlerts();
            this.saveNotificationHistory();
            
            // Emit event for external handling
            this.emit('alert', alertInfo);

            console.warn(`[ALERT] ${alertInfo.message}`);
        } catch (error) {
            console.error(`[AlertManager] Error in triggerAlert for ${alert.id}:`, error.message);
            // Remove the corrupted alert to prevent future issues
            for (const [key, storedAlert] of this.activeAlerts) {
                if (storedAlert.id === alert.id) {
                    console.warn(`[AlertManager] Removing corrupted alert ${alert.id} from activeAlerts in triggerAlert`);
                    this.activeAlerts.delete(key);
                    break;
                }
            }
        }
    }

    resolveAlert(alert) {
        const alertInfo = {
            id: alert.id, // Use the stored alert ID
            ruleId: alert.rule.id,
            ruleName: alert.rule.name,
            guest: {
                name: alert.guest.name,
                vmid: alert.guest.vmid,
                node: alert.guest.node,
                type: alert.guest.type,
                endpointId: alert.guest.endpointId
            },
            metric: alert.rule.metric,
            resolvedAt: alert.resolvedAt,
            duration: alert.resolvedAt - alert.triggeredAt,
            message: this.generateResolvedMessage(alert)
        };

        // Add to history
        this.addToHistory(alertInfo);

        // Apply suppression if configured
        if (alert.rule.suppressionTime > 0) {
            this.suppressAlert(alert.rule.id, {
                endpointId: alert.guest.endpointId,
                node: alert.guest.node,
                vmid: alert.guest.vmid
            }, alert.rule.suppressionTime, 'Auto-suppression after resolution');
        }

        // Emit event for external handling
        this.emit('alertResolved', alertInfo);

        console.info(`[ALERT RESOLVED] ${alertInfo.message}`);

        // Remove from active alerts
        const alertKey = this.findAlertKey(alert);
        if (alertKey) {
            this.activeAlerts.delete(alertKey);
        }
        
        // Save updated state to disk
        this.saveActiveAlerts();
    }

    generateAlertMessage(alert) {
        const { guest, rule, currentValue } = alert;
        let valueStr = '';
        
        // Validate guest object
        if (!guest) {
            console.error('[AlertManager] generateAlertMessage: guest is undefined', { alert });
            return `${rule.name} - Unknown guest - ${rule.type === 'compound_threshold' ? 'Compound threshold met' : 'Alert triggered'}`;
        }
        
        // Handle compound threshold rules
        if (rule.type === 'compound_threshold' && rule.thresholds) {
            // For compound rules, show all threshold values
            const conditions = rule.thresholds.map(threshold => {
                const value = currentValue && typeof currentValue === 'object' ? currentValue[threshold.metric] : null;
                const displayName = this.getThresholdDisplayName(threshold.metric);
                const unit = ['cpu', 'memory', 'disk'].includes(threshold.metric) ? '%' : ' bytes/s';
                const formattedValue = typeof value === 'number' ? Math.round(value * 10) / 10 : value;
                return `${displayName}: ${formattedValue}${unit}`;
            }).join(', ');
            
            return `${rule.name} - ${guest.name || 'Unknown'} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid || 'N/A'}) on ${guest.node || 'Unknown'} - ${conditions}`;
        }
        
        // Handle single-metric rules
        if (rule.metric === 'status') {
            valueStr = `Status: ${currentValue}`;
        } else if (rule.condition === 'anomaly') {
            valueStr = `Network anomaly detected`;
        } else if (rule.metric === 'network_combined') {
            valueStr = `Network anomaly detected`;
        } else {
            // Only add % for actual percentage metrics
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(rule.metric);
            const formattedValue = typeof currentValue === 'number' ? Math.round(currentValue) : currentValue;
            valueStr = `${rule.metric.toUpperCase()}: ${formattedValue}${isPercentageMetric ? '%' : ''}`;
        }
        
        // Format threshold display
        let thresholdStr = '';
        if (rule.condition === 'anomaly' || rule.threshold === 'auto') {
            thresholdStr = 'auto-detected';
        } else if (rule.metric === 'status') {
            thresholdStr = rule.threshold;
        } else {
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(rule.metric);
            thresholdStr = `${rule.threshold}${isPercentageMetric ? '%' : ''}`;
        }
        
        return `${rule.name} - ${guest.name || 'Unknown'} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid || 'N/A'}) on ${guest.node || 'Unknown'} - ${valueStr} (threshold: ${thresholdStr})`;
    }

    generateResolvedMessage(alert) {
        const { guest, rule } = alert;
        const duration = Math.round((alert.resolvedAt - alert.triggeredAt) / 1000);
        return `${rule.name} RESOLVED - ${guest.name || 'Unknown'} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid || 'N/A'}) on ${guest.node || 'Unknown'} - Duration: ${duration}s`;
    }

    findAlertKey(alert) {
        for (const [key, activeAlert] of this.activeAlerts) {
            if (activeAlert === alert) {
                return key;
            }
        }
        return null;
    }

    generateAlertId() {
        return `alert_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }

    addToHistory(alertInfo) {
        this.alertHistory.unshift(alertInfo);
        if (this.alertHistory.length > this.maxHistorySize) {
            this.alertHistory = this.alertHistory.slice(0, this.maxHistorySize);
        }
    }

    cleanupResolvedAlerts() {
        const cutoffTime = Date.now() - (24 * 60 * 60 * 1000); // 24 hours
        let alertsRemoved = false;
        
        for (const [key, alert] of this.activeAlerts) {
            if (alert.state === 'resolved' && alert.resolvedAt < cutoffTime) {
                this.activeAlerts.delete(key);
                // Also clean up notification history for this alert
                this.notificationStatus.delete(alert.id);
                alertsRemoved = true;
            }
        }

        // Clean up old suppressions
        for (const [key, suppression] of this.suppressedAlerts) {
            if (Date.now() > suppression.expiresAt) {
                this.suppressedAlerts.delete(key);
            }
        }

        // Clean up old acknowledgments
        const ackCutoff = Date.now() - (7 * 24 * 60 * 60 * 1000); // 1 week
        let acknowledgementsChanged = false;
        for (const [key, ack] of this.acknowledgedAlerts) {
            if (ack.acknowledgedAt < ackCutoff) {
                this.acknowledgedAlerts.delete(key);
                acknowledgementsChanged = true;
            }
        }
        
        // Save if acknowledgements were cleaned up
        if (acknowledgementsChanged) {
            this.saveAcknowledgements();
        }
        
        // Save if alerts or notification history were cleaned up
        if (alertsRemoved) {
            this.saveActiveAlerts();
            this.saveNotificationHistory();
        }
    }

    updateRule(ruleId, updates) {
        const rule = this.alertRules.get(ruleId);
        if (rule) {
            const wasEnabled = rule.enabled;
            Object.assign(rule, updates);
            
            // If rule was disabled, clean up any active alerts for this rule
            if (wasEnabled && updates.enabled === false) {
                const removedAlerts = this.cleanupAlertsForRule(ruleId);
                console.log(`[AlertManager] Rule ${ruleId} disabled - cleaned up ${removedAlerts} associated alerts`);
            }
            
            // Save all rules to JSON file
            this.saveAlertRules().catch(error => {
                console.error('[AlertManager] Failed to save alert rules after updating rule:', error);
                this.emit('ruleSaveError', { ruleId, error: error.message });
            });
            
            this.emit('ruleUpdated', { ruleId, updates });
            return true;
        }
        return false;
    }

    addRule(rule) {
        // Support both single-metric rules and compound threshold rules
        const isCompoundRule = rule.thresholds && Array.isArray(rule.thresholds) && rule.thresholds.length > 0;
        
        
        // Enhanced validation
        if (!rule.name || typeof rule.name !== 'string' || rule.name.trim().length === 0) {
            throw new Error('Rule must have a valid name (non-empty string)');
        }
        
        if (!isCompoundRule && !rule.metric) {
            throw new Error('Single-metric rule must have a metric. For compound threshold rules, provide thresholds array.');
        }
        
        if (isCompoundRule) {
            // Validate compound threshold rule structure
            if (!Array.isArray(rule.thresholds) || rule.thresholds.length === 0) {
                throw new Error('Compound threshold rule must have at least one threshold');
            }
            
            for (const threshold of rule.thresholds) {
                const foundProperties = Object.keys(threshold);
                const requiredProperties = ['metric', 'condition', 'threshold'];
                const missingProperties = requiredProperties.filter(prop => threshold[prop] === undefined);
                
                if (missingProperties.length > 0) {
                    throw new Error(`Threshold validation failed. Missing required properties: ${missingProperties.join(', ')}. Found properties: ${foundProperties.join(', ')}. Expected properties: ${requiredProperties.join(', ')}`);
                }
                
                const validConditions = ['greater_than', 'less_than', 'equals', 'not_equals', 'greater_than_or_equal', 'less_than_or_equal', 'contains', 'anomaly'];
                if (!validConditions.includes(threshold.condition)) {
                    throw new Error(`Invalid condition '${threshold.condition}' for metric '${threshold.metric}'. Valid conditions: ${validConditions.join(', ')}`);
                }
                
                if (typeof threshold.threshold !== 'number' && threshold.threshold !== 'stopped') {
                    throw new Error(`Invalid threshold value '${threshold.threshold}' for metric '${threshold.metric}'. Expected number or 'stopped' for status checks.`);
                }
            }
        } else {
            // Validate single-metric rule
            if (rule.threshold !== undefined && (typeof rule.threshold !== 'number' || rule.threshold < 0)) {
                throw new Error('Threshold must be a non-negative number');
            }
            
            if (rule.duration !== undefined && (typeof rule.duration !== 'number' || rule.duration < 0)) {
                throw new Error('Duration must be a non-negative number (milliseconds)');
            }
            
        }
        
        const ruleId = rule.id || (isCompoundRule ? 
            `compound_${Date.now()}_${Math.random().toString(36).substr(2, 9)}` : 
            `rule_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`);
        
        // Set defaults for new rules
        const fullRule = {
            id: ruleId,
            condition: 'greater_than',
            duration: 300000,
            enabled: false,
            tags: [],
            group: isCompoundRule ? 'compound_threshold' : 'custom',
            escalationTime: 900000,
            autoResolve: true,
            suppressionTime: 300000,
            type: isCompoundRule ? 'compound_threshold' : 'single_metric',
            ...rule
        };
        
        this.alertRules.set(ruleId, fullRule);
        console.log(`[AlertManager] Added ${isCompoundRule ? 'compound threshold' : 'single-metric'} rule: ${fullRule.name} (${ruleId})`);
        
        // Save all rules to disk
        this.saveAlertRules().catch(error => {
            console.error('[AlertManager] Failed to save alert rules after adding rule:', error);
            this.emit('ruleSaveError', { ruleId: fullRule.id, error: error.message });
        });
        
        this.emit('ruleAdded', fullRule);
        
        // Trigger immediate evaluation for the new rule
        this.evaluateCurrentState();
        
        return fullRule;
    }

    removeRule(ruleId) {
        const rule = this.alertRules.get(ruleId);
        const success = this.alertRules.delete(ruleId);
        
        if (success) {
            // Clean up any active alerts for this rule
            const removedAlerts = this.cleanupAlertsForRule(ruleId);
            console.log(`[AlertManager] Removed rule ${ruleId} and cleaned up ${removedAlerts} associated alerts`);
            
            // Save all rules to disk
            this.saveAlertRules().catch(error => {
                console.error('[AlertManager] Failed to save alert rules after removing rule:', error);
                this.emit('ruleSaveError', { ruleId, error: error.message });
            });
            
            this.emit('ruleRemoved', { ruleId });
        }
        return success;
    }

    /**
     * Refresh alert rules based on current environment variables
     * This should be called after configuration changes
     */
    async refreshRules() {
        console.log('[AlertManager] Refreshing alert rules from JSON file');
        
        // Store currently disabled rule IDs to clean up their alerts
        const previouslyActiveRules = new Set(this.alertRules.keys());
        
        // Reload all rules from JSON file
        await this.loadAlertRules();
        
        // Find rules that were disabled
        const nowActiveRules = new Set(this.alertRules.keys());
        const disabledRules = [...previouslyActiveRules].filter(ruleId => !nowActiveRules.has(ruleId));
        
        // Clean up alerts for disabled rules
        disabledRules.forEach(ruleId => {
            this.cleanupAlertsForRule(ruleId);
        });
        
        console.log(`[AlertManager] Rules refreshed. Active: ${this.alertRules.size}, Disabled: ${disabledRules.length}`);
        if (disabledRules.length > 0) {
            console.log(`[AlertManager] Cleaned up alerts for disabled rules: ${disabledRules.join(', ')}`);
        }
        
        // Trigger immediate evaluation of newly enabled rules against current state
        this.evaluateCurrentState();
        
        this.emit('rulesRefreshed', { activeRules: nowActiveRules.size, disabledRules });
    }

    /**
     * Evaluate current system state against all enabled rules
     * This should be called when rules are enabled to check for immediate alerts
     */
    evaluateCurrentState() {
        try {
            // Get current state from state manager
            const stateManager = require('./state');
            const currentState = stateManager.getState();
            
            if (!currentState) {
                return;
            }

            // Combine VMs and containers into guests array
            const allGuests = [...(currentState.vms || []), ...(currentState.containers || [])];
            const currentMetrics = currentState.metrics || [];

            const isDebugMode = process.env.ALERT_DEBUG === 'true';
            
            if (isDebugMode) {
                console.log(`[AlertDebug] evaluateCurrentState found ${allGuests.length} guests, ${currentMetrics.length} metrics`);
            }

            if (allGuests.length === 0) {
                if (isDebugMode) {
                    console.log(`[AlertDebug] No guests found, skipping evaluation`);
                }
                return;
            }

            // For immediate evaluation, we need to check existing conditions and create alerts immediately
            // This bypasses the normal duration-based pending state
            const timestamp = Date.now();
            
            allGuests.forEach(guest => {
                this.alertRules.forEach(rule => {
                    // Check if rule is enabled first
                    if (!rule.enabled || this.isRuleSuppressed(rule.id, guest)) {
                        return;
                    }
                    
                    const alertKey = `${rule.id}_${guest.endpointId}_${guest.node}_${guest.vmid}`;
                    const existingAlert = this.activeAlerts.get(alertKey);
                    
                    // Skip if alert already exists
                    if (existingAlert) {
                        return;
                    }
                    
                    // Handle different rule types
                    if (rule.metric === 'status') {
                        // Handle status-based rules (down alerts)
                        const effectiveThreshold = this.getEffectiveThreshold(rule, guest);
                        const isTriggered = this.evaluateCondition(guest.status, rule.condition, effectiveThreshold);
                        
                        if (isTriggered) {
                            // Create alert immediately without waiting for duration
                            const newAlert = {
                                id: this.generateAlertId(),
                                rule: this.createSafeRuleCopy(rule),
                                guest: this.createSafeGuestCopy(guest),
                                startTime: timestamp,
                                lastUpdate: timestamp,
                                triggeredAt: timestamp, // Set immediately for instant alerts
                                currentValue: guest.status,
                                effectiveThreshold: effectiveThreshold,
                                state: 'active', // Make it active immediately
                                escalated: false,
                                acknowledged: false
                            };
                            
                            this.activeAlerts.set(alertKey, newAlert);
                            this.triggerAlert(newAlert);
                        }
                    } else if (rule.type === 'compound_threshold' && rule.thresholds) {
                        // Handle compound threshold rules
                        // We need current metrics for compound threshold evaluation
                        // Use the regular compound threshold evaluation but bypass duration for immediate evaluation
                        this.evaluateCompoundThresholdRuleImmediate(rule, guest, alertKey, timestamp);
                    }
                });
            });
            

        } catch (error) {
            console.error('[AlertManager] Error evaluating current state:', error);
        }
    }

    evaluateCompoundThresholdRuleImmediate(rule, guest, alertKey, timestamp) {
        // Get current metrics from state manager
        const stateManager = require('./state');
        const currentState = stateManager.getState();
        
        const isDebugMode = process.env.ALERT_DEBUG === 'true';
        
        if (isDebugMode) {
            console.log(`[AlertDebug] Evaluating rule "${rule.name}" for guest ${guest.name} (${guest.vmid})`);
        }
        
        const metrics = currentState.metrics || [];
        
        // Find metrics for this guest (metrics is an array, not an object)
        const guestMetrics = metrics.find(m => 
            m.endpointId === guest.endpointId &&
            m.node === guest.node &&
            m.id === guest.vmid
        );
        
        if (isDebugMode) {
            console.log(`[AlertDebug] Guest metrics found for ${guest.name}:`, !!guestMetrics);
        }
        
        if (!guestMetrics || !guestMetrics.current) {
            if (isDebugMode) {
                console.log(`[AlertDebug] No metrics for guest ${guest.name}, skipping evaluation`);
            }
            return;
        }
        
        if (isDebugMode) {
            console.log(`[AlertDebug] Guest ${guest.name} raw disk: ${guestMetrics.current.disk} bytes, maxdisk: ${guest.maxdisk} bytes`);
            if (guest.maxdisk && guestMetrics.current.disk) {
                const diskPercentage = (guestMetrics.current.disk / guest.maxdisk) * 100;
                console.log(`[AlertDebug] Calculated disk percentage: ${diskPercentage.toFixed(2)}%`);
            }
        }

        // Check if ALL threshold conditions are met (AND logic)
        const thresholdsMet = rule.thresholds.every(threshold => {
            const result = this.evaluateThresholdCondition(threshold, guestMetrics.current, guest);
            if (isDebugMode) {
                console.log(`[AlertDebug] Threshold check for ${guest.name}: metric=${threshold.metric}, condition=${threshold.condition}, threshold=${threshold.threshold}, result=${result}`);
            }
            return result;
        });

        if (isDebugMode) {
            console.log(`[AlertDebug] All thresholds met for ${guest.name}: ${thresholdsMet}`);
        }

        if (thresholdsMet) {
            // Create alert immediately without waiting for duration
            const newAlert = {
                id: this.generateAlertId(),
                ruleId: rule.id,
                rule: this.createSafeRuleCopy(rule),
                guest: this.createSafeGuestCopy(guest),
                message: this.formatCompoundThresholdMessage(rule, guestMetrics.current, guest),
                startTime: timestamp,
                lastUpdate: timestamp,
                triggeredAt: timestamp, // Set immediately for instant alerts
                currentValue: this.getCurrentThresholdValues(rule.thresholds, guestMetrics.current, guest),
                state: 'active', // Make it active immediately
                escalated: false,
                acknowledged: false
            };
            
            this.activeAlerts.set(alertKey, newAlert);
            this.triggerAlert(newAlert);
        }
    }

    evaluateCompoundThresholdRule(rule, guest, metrics, alertKey, timestamp) {
        // Find metrics for this guest (metrics is an array, not an object)
        const guestMetrics = metrics.find(m => 
            m.endpointId === guest.endpointId &&
            m.node === guest.node &&
            m.id === guest.vmid
        );
        if (!guestMetrics || !guestMetrics.current) return;

        // Check if ALL threshold conditions are met (AND logic)
        const thresholdsMet = rule.thresholds.every(threshold => {
            return this.evaluateThresholdCondition(threshold, guestMetrics.current, guest);
        });

        const existingAlert = this.activeAlerts.get(alertKey);

        if (thresholdsMet) {
            if (!existingAlert) {
                // Create new alert with permanent ID and safe copies of rule/guest objects
                const newAlert = {
                    id: this.generateAlertId(), // Generate ID once when alert is created
                    rule: this.createSafeRuleCopy(rule),
                    guest: this.createSafeGuestCopy(guest),
                    startTime: timestamp,
                    lastUpdate: timestamp,
                    currentValue: this.getCurrentThresholdValues(rule.thresholds, guestMetrics.current, guest),
                    effectiveThreshold: Array.isArray(rule.thresholds) ? 
                        rule.thresholds.map(t => ({
                            metric: t.metric,
                            condition: t.condition,
                            threshold: t.threshold
                        })) : rule.thresholds,
                    state: 'pending',
                    escalated: false,
                    acknowledged: false
                };
                this.activeAlerts.set(alertKey, newAlert);
            } else if (existingAlert.state === 'pending') {
                // Check if duration threshold is met
                const duration = timestamp - existingAlert.startTime;
                if (duration >= rule.duration) {
                    // Trigger alert
                    existingAlert.state = 'active';
                    existingAlert.triggeredAt = timestamp;
                    this.triggerAlert(existingAlert);
                }
                existingAlert.lastUpdate = timestamp;
                existingAlert.currentValue = this.getCurrentThresholdValues(rule.thresholds, guestMetrics.current, guest);
            } else if (existingAlert.state === 'active' && !existingAlert.acknowledged) {
                // Update existing active alert (only if not acknowledged)
                existingAlert.lastUpdate = timestamp;
                existingAlert.currentValue = this.getCurrentThresholdValues(rule.thresholds, guestMetrics.current, guest);
            }
        } else {
            if (existingAlert && existingAlert.state === 'active' && !existingAlert.acknowledged) {
                // Resolve alert (only if not acknowledged)
                existingAlert.state = 'resolved';
                existingAlert.resolvedAt = timestamp;
                if (existingAlert.rule.autoResolve) {
                    this.resolveAlert(existingAlert);
                }
            } else if (existingAlert && existingAlert.state === 'pending') {
                // Remove pending alert that didn't trigger
                this.activeAlerts.delete(alertKey);
            }
        }
    }

    evaluateThresholdCondition(threshold, currentMetrics, guest) {
        let metricValue;

        switch (threshold.metric) {
            case 'cpu':
                metricValue = currentMetrics.cpu;
                // CPU values from Proxmox might be in decimal format (0.0-1.0)
                // Convert to percentage if needed
                if (metricValue !== undefined && metricValue !== null && metricValue <= 1.0) {
                    metricValue = metricValue * 100;
                }
                break;
            case 'memory':
                metricValue = currentMetrics.memory;
                break;
            case 'disk':
                // Calculate disk usage percentage like single-metric rules do
                if (guest.maxdisk && currentMetrics.disk) {
                    metricValue = (currentMetrics.disk / guest.maxdisk) * 100;
                } else {
                    metricValue = null;
                }
                break;
            case 'diskread':
                metricValue = currentMetrics.diskread;
                break;
            case 'diskwrite':
                metricValue = currentMetrics.diskwrite;
                break;
            case 'netin':
                metricValue = currentMetrics.netin;
                break;
            case 'netout':
                metricValue = currentMetrics.netout;
                break;
            default:
                return false;
        }

        if (metricValue === undefined || metricValue === null || isNaN(metricValue)) {
            return false;
        }

        // Apply the specified condition
        switch (threshold.condition) {
            case 'greater_than':
                return metricValue > threshold.threshold;
            case 'greater_than_or_equal':
                return metricValue >= threshold.threshold;
            case 'less_than':
                return metricValue < threshold.threshold;
            case 'less_than_or_equal':
                return metricValue <= threshold.threshold;
            case 'equals':
                return metricValue == threshold.threshold;
            case 'not_equals':
                return metricValue != threshold.threshold;
            default:
                // Default to >= for backward compatibility
                return metricValue >= threshold.threshold;
        }
    }

    formatCompoundThresholdMessage(rule, currentMetrics, guest) {
        const conditions = rule.thresholds.map(threshold => {
            const value = this.getThresholdCurrentValue(threshold, currentMetrics, guest);
            const displayName = this.getThresholdDisplayName(threshold.metric);
            const unit = ['cpu', 'memory', 'disk'].includes(threshold.metric) ? '%' : ' bytes/s';
            
            return `${displayName}: ${value}${unit} (≥ ${threshold.threshold}${unit})`;
        }).join(', ');

        return `Dynamic threshold rule "${rule.name}" triggered for ${guest.name}: ${conditions}`;
    }

    getCurrentThresholdValues(thresholds, currentMetrics, guest) {
        const values = {};
        thresholds.forEach(threshold => {
            values[threshold.metric] = this.getThresholdCurrentValue(threshold, currentMetrics, guest);
        });
        return values;
    }

    getThresholdCurrentValue(threshold, currentMetrics, guest) {
        switch (threshold.metric) {
            case 'cpu': 
                const cpuValue = currentMetrics.cpu || 0;
                // CPU values from Proxmox might be in decimal format (0.0-1.0)
                // Convert to percentage if needed
                if (cpuValue <= 1.0) {
                    return Math.round(cpuValue * 100 * 10) / 10; // Round to 1 decimal place
                }
                return Math.round(cpuValue * 10) / 10;
            case 'memory': return currentMetrics.memory || 0;
            case 'disk': 
                // Calculate disk usage percentage like single-metric rules do
                if (guest && guest.maxdisk && currentMetrics.disk) {
                    const diskPercentage = (currentMetrics.disk / guest.maxdisk) * 100;
                    return Math.round(diskPercentage * 10) / 10; // Round to 1 decimal place
                }
                return 0;
            case 'diskread': return currentMetrics.diskread || 0;
            case 'diskwrite': return currentMetrics.diskwrite || 0;
            case 'netin': return currentMetrics.netin || 0;
            case 'netout': return currentMetrics.netout || 0;
            default: return 0;
        }
    }

    getThresholdDisplayName(type) {
        const names = {
            'cpu': 'CPU',
            'memory': 'Memory',
            'disk': 'Disk',
            'diskread': 'Disk Read',
            'diskwrite': 'Disk Write',
            'netin': 'Network In',
            'netout': 'Network Out'
        };
        return names[type] || type;
    }

    /**
     * Clean up active alerts for a specific rule type
     */
    cleanupAlertsForRule(ruleId) {
        const alertsToRemove = [];
        
        // Find all active alerts for this rule
        for (const [alertKey, alert] of this.activeAlerts) {
            if (alert.rule.id === ruleId) {
                alertsToRemove.push(alertKey);
            }
        }
        
        // Remove the alerts
        alertsToRemove.forEach(alertKey => {
            const alert = this.activeAlerts.get(alertKey);
            if (alert) {
                // Mark as resolved due to rule disable
                const resolvedAlert = {
                    id: alert.id,
                    ruleId: alert.rule.id,
                    ruleName: alert.rule.name,
                    guest: {
                        name: alert.guest.name,
                        vmid: alert.guest.vmid,
                        node: alert.guest.node,
                        type: alert.guest.type,
                        endpointId: alert.guest.endpointId
                    },
                    metric: alert.rule.metric,
                    resolvedAt: Date.now(),
                    duration: alert.triggeredAt ? Date.now() - alert.triggeredAt : 0,
                    message: `${alert.rule.name} - Alert cleared due to rule type being disabled`,
                    resolvedReason: 'rule_disabled'
                };
                
                // Add to history
                this.addToHistory(resolvedAlert);
                
                // Emit event
                this.emit('alertResolved', resolvedAlert);
                
                console.info(`[ALERT CLEARED] ${resolvedAlert.message}`);
            }
            
            this.activeAlerts.delete(alertKey);
        });
        
        return alertsToRemove.length;
    }

    getRules(filters = {}) {
        const rules = Array.from(this.alertRules.values());
        if (filters.group) {
            return rules.filter(rule => rule.group === filters.group);
        }
        return rules;
    }

    /**
     * Get effective threshold for a rule, checking for custom thresholds first
     */
    getEffectiveThreshold(rule, guest) {
        try {
            // Check if custom thresholds are configured for this VM/LXC
            const customConfig = customThresholdManager.getThresholds(
                guest.endpointId, 
                guest.node, 
                guest.vmid
            );
            
            if (customConfig && customConfig.enabled && customConfig.thresholds) {
                const metricThresholds = customConfig.thresholds[rule.metric];
                
                if (metricThresholds && metricThresholds.threshold !== undefined) {
                    console.log(`[AlertManager] Using custom ${rule.metric} threshold ${metricThresholds.threshold}% for ${guest.endpointId}:${guest.node}:${guest.vmid}`);
                    return metricThresholds.threshold;
                }
            }
        } catch (error) {
            console.error('[AlertManager] Error getting custom thresholds:', error);
        }
        
        // Fall back to global threshold from rule
        return rule.threshold;
    }

    /**
     * Initialize custom threshold manager
     */
    async initializeCustomThresholds() {
        try {
            await customThresholdManager.init();
            console.log('[AlertManager] Custom threshold manager initialized');
        } catch (error) {
            console.error('[AlertManager] Failed to initialize custom threshold manager:', error);
        }
    }

    async loadAcknowledgements() {
        try {
            const data = await fs.readFile(this.acknowledgementsFile, 'utf-8');
            const acknowledgements = JSON.parse(data);
            
            // Restore acknowledgements to the map
            for (const [key, ack] of Object.entries(acknowledgements)) {
                this.acknowledgedAlerts.set(key, ack);
            }
            
            console.log(`Loaded ${this.acknowledgedAlerts.size} persisted acknowledgements`);
        } catch (error) {
            if (error.code !== 'ENOENT') {
                console.error('Error loading acknowledgements:', error);
            }
            // File doesn't exist yet, which is fine for first run
        }
    }
    
    async saveAcknowledgements() {
        try {
            // Ensure data directory exists
            const dataDir = path.dirname(this.acknowledgementsFile);
            await fs.mkdir(dataDir, { recursive: true });
            
            // Convert Map to plain object for JSON serialization
            const acknowledgements = {};
            for (const [key, ack] of this.acknowledgedAlerts) {
                acknowledgements[key] = ack;
            }
            
            await fs.writeFile(
                this.acknowledgementsFile, 
                JSON.stringify(acknowledgements, null, 2),
                'utf-8'
            );
        } catch (error) {
            console.error('Error saving acknowledgements:', error);
        }
    }

    async loadAlertRules() {
        try {
            const data = await fs.readFile(this.alertRulesFile, 'utf-8');
            const savedRules = JSON.parse(data);
            
            // Clear all existing rules
            this.alertRules.clear();
            
            // Load all rules from JSON
            for (const [key, rule] of Object.entries(savedRules)) {
                this.alertRules.set(key, rule);
            }
            
            console.log(`[AlertManager] Loaded ${Object.keys(savedRules).length} alert rules`);
            
            // Clear any active alerts for rules that no longer exist
            for (const [alertKey, alert] of this.activeAlerts) {
                if (!this.alertRules.has(alert.rule.id)) {
                    this.activeAlerts.delete(alertKey);
                }
            }
        } catch (error) {
            if (error.code === 'ENOENT') {
                // File doesn't exist - this is first run, create default template rules
                console.log('[AlertManager] No alert rules file found, creating default templates...');
                await this.createDefaultTemplateRules();
            } else {
                console.error('[AlertManager] Error loading alert rules:', error);
            }
        }
    }
    
    async saveAlertRules() {
        try {
            // Ensure data directory exists
            const dataDir = path.dirname(this.alertRulesFile);
            await fs.mkdir(dataDir, { recursive: true });
            
            // Convert Map to plain object for JSON serialization
            // Save ALL rules to JSON
            const rulesToSave = {};
            for (const [key, rule] of this.alertRules) {
                rulesToSave[key] = rule;
            }
            
            await fs.writeFile(
                this.alertRulesFile, 
                JSON.stringify(rulesToSave, null, 2),
                'utf-8'
            );
            
            console.log(`[AlertManager] Saved ${Object.keys(rulesToSave).length} alert rules to disk`);
        } catch (error) {
            console.error('[AlertManager] Error saving alert rules:', error);
        }
    }

    async createDefaultTemplateRules() {
        try {
            // Migrate any existing environment variables to rule configuration
            const defaultTemplates = [
                {
                    id: 'cpu',
                    name: 'CPU Usage',
                    description: 'Monitors CPU usage across all VMs and containers',
                    metric: 'cpu',
                    condition: 'greater_than',
                    threshold: this.parseEnvInt('ALERT_CPU_THRESHOLD', 85, 1, 100),
                    duration: this.parseEnvInt('ALERT_CPU_DURATION', 300000, 1000),
                    enabled: process.env.ALERT_CPU_ENABLED === 'true',
                    tags: ['performance', 'cpu'],
                    group: 'system_performance',
                    escalationTime: 900000, // 15 minutes
                    autoResolve: true,
                    suppressionTime: 300000, // 5 minutes
                },
                {
                    id: 'memory',
                    name: 'Memory Usage',
                    description: 'Monitors memory usage across all VMs and containers',
                    metric: 'memory',
                    condition: 'greater_than',
                    threshold: this.parseEnvInt('ALERT_MEMORY_THRESHOLD', 90, 1, 100),
                    duration: this.parseEnvInt('ALERT_MEMORY_DURATION', 300000, 1000),
                    enabled: process.env.ALERT_MEMORY_ENABLED === 'true',
                    tags: ['performance', 'memory'],
                    group: 'system_performance',
                    escalationTime: 900000, // 15 minutes
                    autoResolve: true,
                    suppressionTime: 300000,
                },
                {
                    id: 'disk',
                    name: 'Disk Usage',
                    description: 'Monitors disk space usage across all VMs and containers',
                    metric: 'disk',
                    condition: 'greater_than',
                    threshold: this.parseEnvInt('ALERT_DISK_THRESHOLD', 90, 1, 100),
                    duration: this.parseEnvInt('ALERT_DISK_DURATION', 300000, 1000),
                    enabled: process.env.ALERT_DISK_ENABLED === 'true',
                    tags: ['storage', 'disk'],
                    group: 'storage_alerts',
                    escalationTime: 1800000, // 30 minutes
                    autoResolve: true,
                    suppressionTime: 600000, // 10 minutes
                },
                {
                    id: 'down',
                    name: 'System Availability',
                    description: 'Monitors VM/container availability and uptime',
                    metric: 'status',
                    condition: 'equals',
                    threshold: 'stopped',
                    duration: this.parseEnvInt('ALERT_DOWN_DURATION', 60000, 1000),
                    enabled: process.env.ALERT_DOWN_ENABLED === 'true',
                    tags: ['availability', 'guest'],
                    group: 'availability_alerts',
                    escalationTime: 600000, // 10 minutes
                    autoResolve: true,
                    suppressionTime: 120000, // 2 minutes
                }
            ];

            // Add templates to rules map
            for (const template of defaultTemplates) {
                this.alertRules.set(template.id, template);
            }

            // Save to disk
            await this.saveAlertRules();
            
            console.log('[AlertManager] Created default template rules with environment variable settings');
            
        } catch (error) {
            console.error('[AlertManager] Error creating default template rules:', error);
        }
    }


    /**
     * Load persisted active alerts from disk
     * This ensures alerts survive service restarts
     */
    async loadActiveAlerts() {
        try {
            const data = await fs.readFile(this.activeAlertsFile, 'utf-8');
            const persistedAlerts = JSON.parse(data);
            
            // Restore alerts to the activeAlerts Map
            Object.entries(persistedAlerts).forEach(([key, alert]) => {
                // Validate the alert has required fields
                if (alert.id && alert.rule && alert.guest) {
                    this.activeAlerts.set(key, alert);
                }
            });
            
            console.log(`[AlertManager] Loaded ${this.activeAlerts.size} active alerts from disk`);
        } catch (error) {
            if (error.code !== 'ENOENT') {
                console.error('[AlertManager] Error loading active alerts:', error);
            }
            // File doesn't exist yet, which is fine for first run
        }
    }

    /**
     * Save active alerts to disk
     */
    async saveActiveAlerts() {
        try {
            // Ensure data directory exists
            const dataDir = path.dirname(this.activeAlertsFile);
            await fs.mkdir(dataDir, { recursive: true });
            
            // Convert Map to plain object for JSON serialization
            const alertsToSave = {};
            for (const [key, alert] of this.activeAlerts) {
                // Only save essential data, not circular references
                alertsToSave[key] = {
                    id: alert.id,
                    rule: this.createSafeRuleCopy(alert.rule),
                    guest: this.createSafeGuestCopy(alert.guest),
                    startTime: alert.startTime,
                    lastUpdate: alert.lastUpdate,
                    triggeredAt: alert.triggeredAt,
                    currentValue: alert.currentValue,
                    effectiveThreshold: alert.effectiveThreshold,
                    state: alert.state,
                    escalated: alert.escalated,
                    acknowledged: alert.acknowledged,
                    acknowledgedBy: alert.acknowledgedBy,
                    acknowledgedAt: alert.acknowledgedAt
                };
            }
            
            await fs.writeFile(
                this.activeAlertsFile,
                JSON.stringify(alertsToSave, null, 2),
                'utf-8'
            );
            
        } catch (error) {
            console.error('[AlertManager] Error saving active alerts:', error);
        }
    }

    /**
     * Load notification history from disk
     */
    async loadNotificationHistory() {
        try {
            const data = await fs.readFile(this.notificationHistoryFile, 'utf-8');
            const history = JSON.parse(data);
            
            // Restore notification status Map
            Object.entries(history).forEach(([alertId, status]) => {
                this.notificationStatus.set(alertId, status);
            });
            
            console.log(`[AlertManager] Loaded notification history for ${this.notificationStatus.size} alerts`);
        } catch (error) {
            if (error.code !== 'ENOENT') {
                console.error('[AlertManager] Error loading notification history:', error);
            }
            // File doesn't exist yet, which is fine for first run
        }
    }

    /**
     * Save notification history to disk
     */
    async saveNotificationHistory() {
        try {
            // Ensure data directory exists
            const dataDir = path.dirname(this.notificationHistoryFile);
            await fs.mkdir(dataDir, { recursive: true });
            
            // Convert Map to plain object with safe serialization
            const historyToSave = {};
            for (const [alertId, status] of this.notificationStatus) {
                // Create safe copy excluding any potential circular references
                historyToSave[alertId] = {
                    emailSent: Boolean(status.emailSent),
                    webhookSent: Boolean(status.webhookSent),
                    channels: Array.isArray(status.channels) ? status.channels.slice() : [],
                    timestamp: status.timestamp || Date.now()
                };
            }
            
            await fs.writeFile(
                this.notificationHistoryFile,
                JSON.stringify(historyToSave, null, 2),
                'utf-8'
            );
            
        } catch (error) {
            console.error('[AlertManager] Error saving notification history:', error);
        }
    }

    /**
     * Initialize email transporter for sending notifications
     */
    async initializeEmailTransporter() {
        try {
            // Try to load config from config API first
            const config = await this.loadEmailConfig();
            
            if (config.host || process.env.SMTP_HOST) {
                const transporter = nodemailer.createTransport({
                    host: config.host || process.env.SMTP_HOST,
                    port: parseInt(config.port || process.env.SMTP_PORT) || 587,
                    secure: config.secure || process.env.SMTP_SECURE === 'true', // true for 465, false for other ports
                    requireTLS: true, // Force TLS encryption
                    auth: {
                        user: config.user || process.env.SMTP_USER,
                        pass: config.pass || process.env.SMTP_PASS
                    },
                    tls: {
                        // Do not fail on invalid certs
                        rejectUnauthorized: false
                    }
                });
                
                // Make the transporter non-enumerable to prevent it from being serialized
                Object.defineProperty(this, 'emailTransporter', {
                    value: transporter,
                    writable: true,
                    enumerable: false,
                    configurable: true
                });
                
                // Store email config for use in notifications
                this.emailConfig = {
                    from: config.from || process.env.ALERT_FROM_EMAIL,
                    to: config.to || process.env.ALERT_TO_EMAIL
                };
                
                console.log('[AlertManager] Email transporter initialized with config:', {
                    host: config.host || process.env.SMTP_HOST,
                    hasAuth: !!(config.user || process.env.SMTP_USER),
                    from: this.emailConfig.from,
                    to: this.emailConfig.to
                });
            } else {
                console.log('[AlertManager] SMTP not configured, email notifications disabled');
            }
        } catch (error) {
            console.error('[AlertManager] Failed to initialize email transporter:', error);
        }
    }

    /**
     * Reload email configuration (call this when settings change)
     */
    async reloadEmailConfiguration() {
        console.log('[AlertManager] Reloading email configuration...');
        
        // Close existing transporter
        if (this.emailTransporter) {
            this.emailTransporter.close();
            this.emailTransporter = null;
        }
        
        // Reinitialize
        await this.initializeEmailTransporter();
    }

    /**
     * Get a valid timestamp from alert, falling back to current time if invalid
     */
    getValidTimestamp(alert) {
        const tryTimestamp = (timestamp) => {
            if (!timestamp) return null;
            const date = new Date(timestamp);
            return isNaN(date.getTime()) ? null : timestamp;
        };

        return tryTimestamp(alert.triggeredAt) || 
               tryTimestamp(alert.lastUpdate) || 
               Date.now();
    }

    /**
     * Send email notification using environment configuration
     */
    async sendDirectEmailNotification(alert) {
        if (!this.emailTransporter) {
            throw new Error('Email transporter not configured');
        }

        // Use stored email config or fall back to env vars
        const toEmail = this.emailConfig?.to || process.env.ALERT_TO_EMAIL;
        const recipients = toEmail ? toEmail.split(',') : [];
        if (!recipients || recipients.length === 0) {
            throw new Error('No email recipients configured (ALERT_TO_EMAIL)');
        }

        // Get the current value and effective threshold for this alert
        const currentValue = alert.currentValue;
        const effectiveThreshold = alert.effectiveThreshold || alert.rule.threshold;
        
        // Format values for display - handle both single values and compound objects
        let valueDisplay, thresholdDisplay;
        
        if (alert.rule.type === 'compound_threshold' && typeof currentValue === 'object' && currentValue !== null) {
            // Format compound threshold values
            const values = [];
            for (const [metric, value] of Object.entries(currentValue)) {
                const isPercentage = ['cpu', 'memory', 'disk'].includes(metric);
                const formattedValue = typeof value === 'number' ? Math.round(value * 10) / 10 : value;
                values.push(`${metric.toUpperCase()}: ${formattedValue}${isPercentage ? '%' : ''}`);
            }
            valueDisplay = values.join(', ');
            
            // Format compound thresholds
            if (Array.isArray(effectiveThreshold)) {
                const thresholds = effectiveThreshold.map(t => {
                    const isPercentage = ['cpu', 'memory', 'disk'].includes(t.metric);
                    return `${t.metric.toUpperCase()}: >${t.threshold}${isPercentage ? '%' : ''}`;
                });
                thresholdDisplay = thresholds.join(', ');
            } else {
                thresholdDisplay = 'Multiple thresholds';
            }
        } else {
            // Format single metric values
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(alert.rule.metric);
            valueDisplay = isPercentageMetric ? `${Math.round(currentValue || 0)}%` : (currentValue || 'N/A');
            thresholdDisplay = isPercentageMetric ? `${effectiveThreshold || 0}%` : (effectiveThreshold || 'N/A');
        }

        const subject = `🚨 Pulse Alert: ${alert.rule.name}`;
        
        const html = `
            <div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
                <div style="background: #dc2626; color: white; padding: 20px; border-radius: 8px 8px 0 0;">
                    <h1 style="margin: 0; font-size: 24px;">🚨 ${alert.rule.name}</h1>
                    <p style="margin: 5px 0 0 0; opacity: 0.9; font-size: 16px;">Alert Triggered</p>
                </div>
                
                <div style="background: #f9fafb; padding: 20px; border-left: 4px solid #dc2626;">
                    <h2 style="margin: 0 0 15px 0; color: #374151;">Alert Details</h2>
                    
                    <table style="width: 100%; border-collapse: collapse;">
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151; width: 120px;">VM/LXC:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Node:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.guest.node}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Metric:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.rule.metric ? alert.rule.metric.toUpperCase() : (alert.rule.type === 'compound_threshold' ? 'Multiple Thresholds' : 'N/A')}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Current Value:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${valueDisplay}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Threshold:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${thresholdDisplay}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Status:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.guest.status}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Time:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${new Date(this.getValidTimestamp(alert)).toLocaleString()}</td>
                        </tr>
                    </table>
                </div>
                
                <div style="background: white; padding: 20px; border-radius: 0 0 8px 8px; border-top: 1px solid #e5e7eb;">
                    <p style="margin: 0; color: #6b7280; font-size: 14px;">
                        <strong>Description:</strong> ${alert.rule.description || 'Alert triggered for the specified conditions'}
                    </p>
                    <p style="margin: 15px 0 0 0; color: #9ca3af; font-size: 12px;">
                        This alert was generated by Pulse monitoring system. 
                        Please check your Proxmox dashboard for more details.
                    </p>
                </div>
            </div>
        `;

        const text = `
PULSE ALERT: ${alert.rule.name}

VM/LXC: ${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})
Node: ${alert.guest.node}
Metric: ${alert.rule.metric ? alert.rule.metric.toUpperCase() : (alert.rule.type === 'compound_threshold' ? 'Multiple Thresholds' : 'N/A')}
Current Value: ${valueDisplay}
Threshold: ${thresholdDisplay}
Status: ${alert.guest.status}
Time: ${new Date(this.getValidTimestamp(alert)).toLocaleString()}

Description: ${alert.rule.description || 'Alert triggered for the specified conditions'}

This alert was generated by Pulse monitoring system.
        `;

        const mailOptions = {
            from: this.emailConfig?.from || process.env.ALERT_FROM_EMAIL || 'alerts@pulse-monitoring.local',
            to: recipients.join(', '),
            subject: subject,
            text: text,
            html: html
        };

        await this.emailTransporter.sendMail(mailOptions);
        console.log(`[EMAIL] Alert sent to: ${recipients.join(', ')}`);
    }

    /**
     * Send webhook notification using environment configuration
     */
    async sendDirectWebhookNotification(alert) {
        const webhookUrl = process.env.WEBHOOK_URL;
        if (!webhookUrl) {
            throw new Error('Webhook URL not configured (WEBHOOK_URL)');
        }

        const validTimestamp = this.getValidTimestamp(alert);
        
        // Get the current value and effective threshold for this alert
        const currentValue = alert.currentValue;
        const effectiveThreshold = alert.effectiveThreshold || alert.rule.threshold;
        
        // Format values for display (only add % for percentage metrics)
        const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(alert.rule.metric);
        const formattedValue = typeof currentValue === 'number' ? 
            (isPercentageMetric ? Math.round(currentValue) : currentValue) : (currentValue || 'N/A');
        const formattedThreshold = typeof effectiveThreshold === 'number' ? 
            effectiveThreshold : (effectiveThreshold || 'N/A');
        
        const valueDisplay = isPercentageMetric ? `${formattedValue}%` : formattedValue;
        const thresholdDisplay = isPercentageMetric ? `${formattedThreshold}%` : formattedThreshold;
        
        // Detect webhook type based on URL
        const isDiscord = webhookUrl.includes('discord.com/api/webhooks') || webhookUrl.includes('discordapp.com/api/webhooks');
        const isSlack = webhookUrl.includes('slack.com/') || webhookUrl.includes('hooks.slack.com');
        
        let payload;
        
        if (isDiscord) {
            // Discord-specific format
            payload = {
                embeds: [{
                title: `🚨 ${alert.rule.name}`,
                description: alert.rule.description,
                color: 15158332, // Red
                fields: [
                    {
                        name: 'VM/LXC',
                        value: `${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})`,
                        inline: true
                    },
                    {
                        name: 'Node',
                        value: alert.guest.node,
                        inline: true
                    },
                    {
                        name: 'Status',
                        value: alert.guest.status,
                        inline: true
                    },
                    {
                        name: 'Metric',
                        value: alert.rule.metric.toUpperCase(),
                        inline: true
                    },
                    {
                        name: 'Current Value',
                        value: valueDisplay,
                        inline: true
                    },
                    {
                        name: 'Threshold',
                        value: thresholdDisplay,
                        inline: true
                    }
                ],
                footer: {
                    text: 'Pulse Monitoring System'
                },
                timestamp: new Date(validTimestamp).toISOString()
                }]
            };
        } else if (isSlack) {
            // Slack-specific format
            payload = {
                text: `🚨 *${alert.rule.name}*`,
                attachments: [{
                    color: 'danger',
                    fields: [
                        {
                            title: 'VM/LXC',
                            value: `${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})`,
                            short: true
                        },
                        {
                            title: 'Node',
                            value: alert.guest.node,
                            short: true
                        },
                        {
                            title: 'Metric',
                            value: alert.rule.type === 'compound_threshold' ? 
                                `Compound Rule: ${valueDisplay}` : 
                                `${alert.rule.metric.toUpperCase()}: ${valueDisplay} (threshold: ${thresholdDisplay})`,
                            short: false
                        }
                    ],
                    footer: 'Pulse Monitoring',
                    ts: Math.floor(validTimestamp / 1000)
                }]
            };
        } else {
            // Generic webhook format with all fields (backward compatibility)
            payload = {
                timestamp: new Date(validTimestamp).toISOString(),
                alert: {
                    id: alert.id,
                    rule: {
                        name: alert.rule.name,
                        description: alert.rule.description,
                        metric: alert.rule.metric
                    },
                    guest: {
                        name: alert.guest.name,
                        id: alert.guest.vmid,
                        type: alert.guest.type,
                        node: alert.guest.node,
                        status: alert.guest.status
                    },
                    value: formattedValue,
                    threshold: formattedThreshold,
                    emoji: '🚨'
                },
                // Include both formats for generic webhooks
                embeds: [{
                    title: `🚨 ${alert.rule.name}`,
                    description: alert.rule.description,
                    color: 15158332, // Red color for all alerts
                    fields: [
                        {
                            name: 'VM/LXC',
                            value: `${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})`,
                            inline: true
                        },
                        {
                            name: 'Node',
                            value: alert.guest.node,
                            inline: true
                        },
                        {
                            name: 'Metric',
                            value: alert.rule.type === 'compound_threshold' ? 
                                `Compound Rule: ${valueDisplay}` : 
                                `${alert.rule.metric.toUpperCase()}: ${valueDisplay} (threshold: ${thresholdDisplay})`,
                            inline: true
                        }
                    ],
                    footer: {
                        text: 'Pulse Monitoring System'
                    },
                    timestamp: new Date(validTimestamp).toISOString()
                }],
                text: `🚨 *${alert.rule.name}*`,
                attachments: [{
                    color: 'danger',
                    fields: [
                        {
                            title: 'VM/LXC',
                            value: `${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})`,
                            short: true
                        },
                        {
                            title: 'Metric',
                            value: alert.rule.type === 'compound_threshold' ? 
                                `Compound Rule: ${valueDisplay}` : 
                                `${alert.rule.metric.toUpperCase()}: ${valueDisplay} (threshold: ${thresholdDisplay})`,
                            short: false
                        }
                    ],
                    footer: 'Pulse Monitoring',
                    ts: Math.floor(validTimestamp / 1000)
                }]
            };
        }

        // Set appropriate headers
        const headers = {
            'Content-Type': 'application/json',
            'User-Agent': 'Pulse-Monitoring/1.0'
        };

        const maxRetries = 3;
        let lastError;
        
        for (let attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                const response = await axios.post(webhookUrl, payload, {
                    headers,
                    timeout: 10000, // 10 second timeout
                    maxRedirects: 3
                });

                console.log(`[WEBHOOK] Alert sent to: ${webhookUrl} (${response.status}) - attempt ${attempt}`);
                return; // Success, exit retry loop
                
            } catch (error) {
                lastError = error;
                console.warn(`[WEBHOOK] Attempt ${attempt}/${maxRetries} failed for ${webhookUrl}:`, error.message);
                
                // Don't retry on 4xx client errors (likely permanent)
                if (error.response && error.response.status >= 400 && error.response.status < 500) {
                    console.error(`[WEBHOOK] Permanent client error ${error.response.status}, not retrying`);
                    break;
                }
                
                // Wait before retry (exponential backoff)
                if (attempt < maxRetries) {
                    const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000); // Max 5s delay
                    await new Promise(resolve => setTimeout(resolve, delay));
                }
            }
        }
        
        // All retries failed, throw final error
        if (lastError.response) {
            throw new Error(`Webhook failed after ${maxRetries} attempts: ${lastError.response.status} ${lastError.response.statusText}`);
        } else if (lastError.request) {
            throw new Error(`Webhook failed after ${maxRetries} attempts: No response from ${webhookUrl}`);
        } else {
            throw new Error(`Webhook failed after ${maxRetries} attempts: ${lastError.message}`);
        }
    }

    destroy() {
        if (this.cleanupInterval) {
            clearInterval(this.cleanupInterval);
        }
        if (this.escalationInterval) {
            clearInterval(this.escalationInterval);
        }
        
        // Stop watching alert rules file
        const fs = require('fs');
        fs.unwatchFile(this.alertRulesFile);
        
        this.removeAllListeners();
        this.activeAlerts.clear();
        this.alertRules.clear();
        this.acknowledgedAlerts.clear();
        this.suppressedAlerts.clear();
        this.notificationStatus.clear();
        
        // Close email transporter
        if (this.emailTransporter) {
            this.emailTransporter.close();
        }
    }

    async sendTestEmail() {
        try {
            console.log('[AlertManager] Sending test email...');
            
            if (!this.emailTransporter) {
                return { success: false, error: 'Email transporter not configured' };
            }

            const config = await this.loadEmailConfig();
            if (!config.to) {
                return { success: false, error: 'No recipient email address configured' };
            }

            return await this.sendTestEmailWithConfig({
                ALERT_FROM_EMAIL: config.from,
                ALERT_TO_EMAIL: config.to,
                SMTP_HOST: config.host,
                SMTP_PORT: config.port,
                SMTP_USER: config.user,
                SMTP_SECURE: config.secure
            });
            
        } catch (error) {
            console.error('[AlertManager] Error sending test email:', error);
            return { success: false, error: error.message };
        }
    }

    async sendTestEmailWithConfig(config) {
        try {
            console.log('[AlertManager] Sending test email with provided config...');
            
            if (!this.emailTransporter) {
                return { success: false, error: 'Email transporter not configured' };
            }

            if (!config.ALERT_TO_EMAIL) {
                return { success: false, error: 'No recipient email address configured' };
            }

            const testEmailOptions = {
                from: config.ALERT_FROM_EMAIL || 'noreply@pulse.local',
                to: config.ALERT_TO_EMAIL,
                subject: 'Pulse Alert System - Test Email',
                text: `This is a test email from your Pulse monitoring system.

Sent at: ${new Date().toISOString()}
From: ${require('os').hostname()}

If you received this email, your email configuration is working correctly!

Configuration used:
- SMTP Host: ${config.SMTP_HOST}
- SMTP Port: ${config.SMTP_PORT}
- From: ${config.ALERT_FROM_EMAIL}
- To: ${config.ALERT_TO_EMAIL}

Best regards,
Pulse Monitoring System`,
                html: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
    <h2 style="color: #3b82f6;">Pulse Alert System - Test Email</h2>
    <p>This is a test email from your Pulse monitoring system.</p>
    
    <div style="background-color: #f3f4f6; padding: 15px; border-radius: 5px; margin: 20px 0;">
        <strong>Test Details:</strong><br>
        <strong>Sent at:</strong> ${new Date().toISOString()}<br>
        <strong>From:</strong> ${require('os').hostname()}<br><br>
        <strong>Configuration used:</strong><br>
        <strong>SMTP Host:</strong> ${config.SMTP_HOST}<br>
        <strong>SMTP Port:</strong> ${config.SMTP_PORT}<br>
        <strong>From:</strong> ${config.ALERT_FROM_EMAIL}<br>
        <strong>To:</strong> ${config.ALERT_TO_EMAIL}
    </div>
    
    <p style="color: #16a34a;"><strong>✅ Success!</strong> If you received this email, your email configuration is working correctly!</p>
    
    <hr style="margin: 30px 0; border: none; border-top: 1px solid #e5e7eb;">
    <p style="color: #6b7280; font-size: 12px;">
        Best regards,<br>
        Pulse Monitoring System
    </p>
</div>`
            };

            await this.emailTransporter.sendMail(testEmailOptions);
            console.log('[AlertManager] Test email sent successfully');
            return { success: true };
            
        } catch (error) {
            console.error('[AlertManager] Error sending test email:', error);
            return { success: false, error: error.message };
        }
    }

    async loadEmailConfig() {
        try {
            // Load email configuration from config API
            const axios = require('axios');
            
            const response = await axios.get('http://localhost:7655/api/config');
            const config = response.data;
            
            console.log('[AlertManager] Loading email config from API, ALERT_TO_EMAIL:', config.ALERT_TO_EMAIL);
            
            return {
                from: config.ALERT_FROM_EMAIL,
                to: config.ALERT_TO_EMAIL,
                host: config.SMTP_HOST,
                port: config.SMTP_PORT,
                user: config.SMTP_USER,
                pass: config.SMTP_PASS,
                secure: config.SMTP_SECURE === 'true'
            };
        } catch (error) {
            console.error('[AlertManager] Error loading email config from API:', error);
            // Fallback to environment variables
            return {
                from: process.env.ALERT_FROM_EMAIL,
                to: process.env.ALERT_TO_EMAIL,
                host: process.env.SMTP_HOST,
                port: process.env.SMTP_PORT,
                user: process.env.SMTP_USER,
                pass: process.env.SMTP_PASS,
                secure: process.env.SMTP_SECURE === 'true'
            };
        }
    }
}

module.exports = AlertManager; 