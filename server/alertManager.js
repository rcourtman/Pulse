const EventEmitter = require('events');
const fs = require('fs').promises;
const path = require('path');
const nodemailer = require('nodemailer');
const axios = require('axios');
const customThresholdManager = require('./customThresholds');
const WebhookBatcher = require('./webhookBatcher');
const alertPatches = require('./alertManagerPatches');
const DebounceHandler = require('./debounceHandler');
const EmailService = require('./emailService');
const alertHistory = require('./alertHistoryPersistence');

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
        this.guestStates = new Map(); // Track previous guest states for transition detection
        this.alertMetrics = {
            totalFired: 0,
            totalResolved: 0,
            totalAcknowledged: 0,
            averageResolutionTime: 0,
            falsePositiveRate: 0
        };
        
        // Email cooldown tracking
        this.emailCooldowns = new Map(); // Key: "ruleId-guestId-metric", Value: { lastSent, cooldownUntil }
        this.emailCooldownConfig = {
            defaultCooldownMinutes: parseInt(process.env.ALERT_EMAIL_COOLDOWN_MINUTES) || 15, // Default 15 minutes
            debounceDelayMinutes: parseInt(process.env.ALERT_EMAIL_DEBOUNCE_MINUTES) || 2, // Wait 2 minutes before first email
            recoveryDelayMinutes: parseInt(process.env.ALERT_RECOVERY_DELAY_MINUTES) || 5, // Delay recovery emails
            maxEmailsPerHour: parseInt(process.env.ALERT_MAX_EMAILS_PER_HOUR) || 4, // Max 4 emails per hour per alert
            enableFlappingDetection: process.env.ALERT_FLAPPING_DETECTION !== 'false' // Default enabled
        };
        
        // Webhook cooldown tracking
        this.webhookCooldowns = new Map(); // Key: "ruleId-guestId-metric", Value: { lastSent, cooldownUntil }
        this.webhookCooldownConfig = {
            defaultCooldownMinutes: parseInt(process.env.ALERT_WEBHOOK_COOLDOWN_MINUTES) || 5, // Default 5 minutes
            debounceDelayMinutes: parseInt(process.env.ALERT_WEBHOOK_DEBOUNCE_MINUTES) || 1, // Wait 1 minute before first webhook
            maxCallsPerHour: parseInt(process.env.ALERT_WEBHOOK_MAX_CALLS_PER_HOUR) || 10 // Max 10 webhooks per hour per alert
        };
        
        this.perGuestCooldownConfig = null; // Will be loaded from per-guest threshold rule if present
        
        // Email batching
        this.emailBatchEnabled = process.env.EMAIL_BATCH_ENABLED === 'true';
        this.emailBatchWindowMs = parseInt(process.env.EMAIL_BATCH_WINDOW_MS) || 30000; // 30 seconds
        this.emailQueue = [];
        this.emailBatchTimeout = null;
        
        // Initialize webhook batcher
        this.webhookBatcher = new WebhookBatcher({
            batchWindowMs: parseInt(process.env.WEBHOOK_BATCH_WINDOW_MS) || 5000,
            summaryThreshold: parseInt(process.env.WEBHOOK_SUMMARY_THRESHOLD) || 3,
            priorityDelay: parseInt(process.env.WEBHOOK_PRIORITY_DELAY) || 2000,
            normalDelay: parseInt(process.env.WEBHOOK_ANNOUNCEMENT_DELAY) || 10000
        });
        
        // Handle webhook batching events
        this.webhookBatcher.on('send', async (alert, webhookUrl) => {
            await this.sendDirectWebhookNotification(alert);
        });
        
        this.webhookBatcher.on('sent', (alertId) => {
            // Update notification status
            const status = this.notificationStatus.get(alertId) || {};
            status.webhookSent = true;
            this.notificationStatus.set(alertId, status);
            
            // Also update the alert object itself
            for (const [key, alert] of this.activeAlerts) {
                if (alert.id === alertId) {
                    alert.webhookSent = true;
                    this.saveActiveAlerts();
                    break;
                }
            }
        });
        
        this.maxHistorySize = 1000; // Limit history to prevent memory issues
        this.alertRulesFile = path.join(__dirname, '../data/alert-rules.json');
        this.activeAlertsFile = path.join(__dirname, '../data/active-alerts.json');
        this.notificationHistoryFile = path.join(__dirname, '../data/notification-history.json');
        
        // Add synchronization flags
        this.reloadingRules = false;
        this.processingMetrics = false;
        
        // Initialize alert groups
        this.initializeAlertGroups();
        
        // Load persisted state
        this.loadAlertRules();
        this.loadActiveAlerts();
        this.loadNotificationHistory();
        this.loadAlertHistory();
        
        // Initialize custom threshold manager
        this.initializeCustomThresholds();
        
        // Initialize email service
        this.emailService = new EmailService();
        this.emailTransporter = null;
        this.emailConfig = null;
        this.initializeEmailTransporter();
        
        // Apply patches for improved reliability
        alertPatches.applyPatches.call(this);
        
        // Initialize debounce handler
        this.debounceHandler = new DebounceHandler(this);
        this.debounceHandler.start();
        
        // Initialize alert batching for simultaneous triggers
        this.pendingAlertBatch = [];
        this.batchTimeout = null;
        
        // Initialize resolved alert batching for simultaneous resolutions
        this.pendingResolvedBatch = [];
        this.resolvedBatchTimeout = null;
        
        // Cleanup timer for resolved alerts and expired cooldowns
        this.cleanupInterval = setInterval(() => {
            this.cleanupResolvedAlerts();
            this.cleanupExpiredCooldowns();
            this.updateMetrics();
            // Also cleanup persistent history
            alertHistory.cleanup().catch(error => {
                console.error('[AlertManager] History cleanup error:', error);
            });
        }, 300000); // Every 5 minutes
        
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
                        
                        // Notify frontend that rules have been refreshed
                        this.emit('rulesRefreshed');
                        
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

    // Initialize guest states for transition detection
    initializeGuestStates(guests) {
        guests.forEach(guest => {
            const guestStateKey = `${guest.endpointId}_${guest.node}_${guest.vmid}`;
            if (!this.guestStates.has(guestStateKey)) {
                this.guestStates.set(guestStateKey, guest.status);
            }
        });
    }

    // Enhanced alert checking with custom conditions
    async checkNodeMetrics(nodes, nodeThresholds, globalNodeThresholds) {
        try {
            if (!nodeThresholds && !globalNodeThresholds) return;
            
            const timestamp = Date.now();
            const newlyTriggeredAlerts = [];
            
            // Get the per-guest threshold rule to access notification settings
            const perGuestRule = this.alertRules.get('per-guest-alerts');
            
            nodes.forEach(node => {
                try {
                    const nodeId = node.node;
                    if (!nodeId) return;
                    
                    // Get thresholds for this node
                    const nodeSpecificThresholds = nodeThresholds?.[nodeId] || {};
                    
                    // Check each metric
                    ['cpu', 'memory', 'disk'].forEach(metric => {
                        const threshold = nodeSpecificThresholds[metric] !== undefined 
                            ? nodeSpecificThresholds[metric] 
                            : globalNodeThresholds?.[metric];
                            
                        if (threshold === undefined || threshold === '') return;
                        
                        let currentValue = 0;
                        if (metric === 'cpu' && node.cpu !== undefined) {
                            currentValue = node.cpu * 100; // Convert to percentage
                        } else if (metric === 'memory' && node.mem !== undefined && node.maxmem) {
                            currentValue = (node.mem / node.maxmem) * 100;
                        } else if (metric === 'disk' && node.disk !== undefined && node.maxdisk) {
                            currentValue = (node.disk / node.maxdisk) * 100;
                        } else {
                            return; // Skip if metric not available
                        }
                        
                        const alertKey = `node_${nodeId}_${metric}`;
                        const existingAlert = this.activeAlerts.get(alertKey);
                        
                        if (currentValue > threshold) {
                            if (!existingAlert) {
                                // Create new alert
                                const alert = {
                                    id: `${alertKey}_${timestamp}`,
                                    type: 'node_threshold',
                                    nodeId: nodeId,
                                    nodeName: node.displayName || node.node,
                                    metric: metric,
                                    threshold: threshold,
                                    currentValue: currentValue,
                                    triggeredAt: timestamp,
                                    state: 'active',
                                    severity: currentValue > threshold + 10 ? 'critical' : 'warning',
                                    message: `Node ${node.displayName || node.node}: ${metric.toUpperCase()}: ${currentValue.toFixed(0)}% (≥${threshold}%)`,
                                    notificationChannels: this.determineNotificationChannels(perGuestRule?.notifications || { dashboard: true, email: false, webhook: false }),
                                    rule: {
                                        id: 'per_guest_thresholds',
                                        name: `Node ${metric} threshold`,
                                        type: 'node_threshold',
                                        group: 'node_alerts',
                                        autoResolve: perGuestRule?.autoResolve ?? true,
                                        notifications: perGuestRule?.notifications || { dashboard: true, email: false, webhook: false }
                                    }
                                };
                                
                                this.activeAlerts.set(alertKey, alert);
                                newlyTriggeredAlerts.push(alert);
                                console.log(`[AlertManager] Node alert triggered: ${alert.message}`);
                            }
                        } else if (existingAlert && existingAlert.type === 'node_threshold') {
                            // Resolve existing alert if value dropped below threshold
                            existingAlert.state = 'resolved';
                            existingAlert.resolvedAt = timestamp;
                            // Properly resolve the alert to update history
                            this.resolveAlert(existingAlert).catch(error => {
                                console.error(`[AlertManager] Error resolving node alert: ${error}`);
                            });
                            console.log(`[AlertManager] Node alert resolved: ${existingAlert.message}`);
                        }
                    });
                } catch (nodeError) {
                    console.error(`[AlertManager] Error processing node ${node.node}:`, nodeError);
                }
            });
            
            for (const alert of newlyTriggeredAlerts) {
                await this.triggerAlert(alert);
            }
            
        } catch (error) {
            console.error('[AlertManager] Error in checkNodeMetrics:', error);
        }
    }

    async checkMetrics(guests, metrics) {
        
        // Check if alerts are globally disabled
        if (process.env.ALERTS_ENABLED === 'false') {
            console.log('[AlertManager] Alerts are globally disabled');
            return;
        }
        
        // SIMPLIFIED ALERT SYSTEM - Use direct threshold checking instead of complex rules
        console.log('[AlertManager] checkMetrics called with', guests.length, 'guests');
        
        // Write to a debug file to confirm this is being called
        const fsSync = require('fs');
        fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] checkMetrics called with ${guests.length} guests\n`);
        
        try {
            fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] Entered try block\n`);
            // Convert guests array to metrics map format expected by simplified system
            const allGuestMetrics = new Map();
            
            fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] Building allGuestMetrics map from ${guests.length} guests\n`);
            
            guests.forEach(guest => {
                if (guest && guest.name) {
                    const guestKey = `${guest.endpointId || 'unknown'}-${guest.node}-${guest.vmid}`;
                    
                    // Debug first guest
                    if (guests.indexOf(guest) === 0) {
                        fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] First guest data: name=${guest.name}, vmid=${guest.vmid}, disk=${guest.disk}, maxdisk=${guest.maxdisk}\n`);
                    }
                    
                    const guestMetricsData = metrics.find(m => 
                        m.endpointId === guest.endpointId && 
                        m.node === guest.node && 
                        m.id === guest.vmid
                    );
                    
                    // Use metrics data if available (has calculated rates), fallback to guest data
                    let currentMetrics = guest;
                    
                    // If we have metrics data, merge it with guest data to get I/O rates
                    if (guestMetricsData && guestMetricsData.current) {
                        // Create a new object that combines guest properties with metrics data
                        currentMetrics = {
                            ...guest, // Start with all guest properties
                            ...guestMetricsData.current, // Override with metrics data
                            // Ensure critical guest properties are preserved
                            name: guest.name,
                            type: guest.type,
                            endpointId: guest.endpointId,
                            node: guest.node,
                            vmid: guest.vmid,
                            status: guest.status,
                            maxmem: guest.maxmem,
                            maxdisk: guest.maxdisk,
                            cpus: guest.cpus
                        };
                    }
                    
                    allGuestMetrics.set(guestKey, {
                        current: currentMetrics,
                        previous: null // Not needed for simple threshold checking
                    });
                    
                    // Debug: log what we're storing for disk usage for first 3 guests
                    if (allGuestMetrics.size < 3) {
                        fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] Guest ${currentMetrics.name} (${guestKey}): disk=${currentMetrics.disk}, maxdisk=${currentMetrics.maxdisk}\n`);
                    }
                }
            });
            
            fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] allGuestMetrics map has ${allGuestMetrics.size} entries\n`);
            
            // Use the new simplified evaluation
            await this.evaluateSimpleThresholds(allGuestMetrics);
            
        } catch (error) {
            console.error('[AlertManager] Error in simplified checkMetrics:', error);
            fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] ERROR in checkMetrics: ${error.message}\n${error.stack}\n`);
        }
        
        // Save state
        this.saveActiveAlerts();
        
        return;
        
        // === OLD COMPLEX RULE SYSTEM BELOW (DISABLED) ===
        // TODO: Remove this code block after testing simplified system
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
        
        // Initialize guest states on first run
        this.initializeGuestStates(guests);
        
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
        
        console.log(`[AlertManager] Rule ${ruleId} suppressed for ${guest.name} (${guest.vmid}) until ${new Date(suppression.expiresAt).toISOString()}`);
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

            // Track guest state for transition detection
            const guestStateKey = `${guest.endpointId}_${guest.node}_${guest.vmid}`;
            const previousState = this.guestStates.get(guestStateKey);
            const currentState = guest.status;
            this.guestStates.set(guestStateKey, currentState);

            // For status alerts, check for state transitions to create new alert instances
            const isStatusAlert = rule.metric === 'status';
            const hasStateChanged = previousState && previousState !== currentState;
            const shouldCreateNewIncident = isStatusAlert && hasStateChanged && isTriggered;

            if (isTriggered) {
                if (!existingAlert || shouldCreateNewIncident) {
                    // Resolve any existing acknowledged alert when creating new incident
                    if (existingAlert && existingAlert.acknowledged && shouldCreateNewIncident) {
                        existingAlert.state = 'resolved';
                        existingAlert.resolvedAt = timestamp;
                        existingAlert.resolveReason = 'New incident detected';
                        this.resolveAlert(existingAlert).catch(error => {
                            console.error('[AlertManager] Error resolving alert:', error);
                        });
                    }
                    
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
                        acknowledged: false,
                        previousState: previousState, // Track what state it came from
                        incidentType: shouldCreateNewIncident ? 'state_transition' : 'initial'
                    };
                    this.activeAlerts.set(alertKey, newAlert);
                } else if (existingAlert.state === 'pending') {
                    // Check if duration threshold is met
                    const duration = timestamp - existingAlert.startTime;
                    if (duration >= rule.duration) {
                        // Trigger alert
                        existingAlert.state = 'active';
                        existingAlert.triggeredAt = timestamp;
                        this.triggerAlert(existingAlert).catch(error => {
                            console.error(`[AlertManager] Error triggering alert ${existingAlert.id}:`, error);
                        });
                        newlyTriggeredAlert = existingAlert;
                    }
                    existingAlert.lastUpdate = timestamp;
                    existingAlert.currentValue = currentValue;
                } else if (existingAlert.state === 'active' && !existingAlert.acknowledged) {
                    existingAlert.lastUpdate = timestamp;
                    existingAlert.currentValue = currentValue;
                }
            } else {
                if (existingAlert && (existingAlert.state === 'active' || existingAlert.state === 'pending')) {
                    // Resolve alert regardless of acknowledgment status when condition clears
                    existingAlert.state = 'resolved';
                    existingAlert.resolvedAt = timestamp;
                    existingAlert.resolveReason = 'Condition cleared';
                    if (existingAlert.rule.autoResolve) {
                        this.resolveAlert(existingAlert).catch(error => {
                            console.error('[AlertManager] Error resolving alert:', error);
                        });
                    }
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
                
                // Update the alert in history with acknowledgment info
                alertHistory.updateInHistory(alert.id, {
                    acknowledged: true,
                    acknowledgedBy: userId,
                    acknowledgedAt: Date.now(),
                    acknowledgeNote: note
                });
                
                this.emit('alertAcknowledged', alert);
                
                this.saveActiveAlerts();
                
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
    
    getEmailCooldownKey(alert) {
        // Create a unique key for cooldown tracking per rule, guest, and metric
        const ruleId = alert.rule?.id || 'unknown';
        const guestId = `${alert.guest?.endpointId || 'unknown'}-${alert.guest?.node || 'unknown'}-${alert.guest?.vmid || 'unknown'}`;
        // Use alert.metric directly (for per-guest and node alerts) or fall back to rule.metric
        const metric = alert.metric || alert.rule?.metric || 'unknown';
        return `${ruleId}-${guestId}-${metric}`;
    }
    
    getWebhookCooldownKey(alert) {
        // Create a unique key for cooldown tracking per rule, guest, and metric
        const ruleId = alert.rule?.id || 'unknown';
        const guestId = `${alert.guest?.endpointId || 'unknown'}-${alert.guest?.node || 'unknown'}-${alert.guest?.vmid || 'unknown'}`;
        // Use alert.metric directly (for per-guest and node alerts) or fall back to rule.metric
        const metric = alert.metric || alert.rule?.metric || 'unknown';
        return `${ruleId}-${guestId}-${metric}`;
    }



    async sendNotifications(alert) {
        // Check if we've already sent notifications for this alert
        // Check both the notification status map and the alert's own flags
        const existingStatus = this.notificationStatus.get(alert.id);
        const alertHasEmailSent = alert.emailSent === true;
        const alertHasWebhookSent = alert.webhookSent === true;
        
        // Debug logging for individual alerts
        if (alert.metric) {
            console.log(`[AlertManager] sendNotifications for ${alert.metric} alert ${alert.id}:`, {
                metric: alert.metric,
                ruleType: alert.rule?.type,
                notifications: alert.rule?.notifications,
                existingStatus,
                alertHasEmailSent,
                alertHasWebhookSent
            });
        }
        
        if ((existingStatus && (existingStatus.emailSent || existingStatus.webhookSent)) || 
            (alertHasEmailSent || alertHasWebhookSent)) {
            return;
        }
        
        let sendEmail, sendWebhook;
        
        // For emails - check if email transporter exists and if rule has email enabled
        let ruleEmailEnabled = true; // Default to true for system rules
        if (alert.rule) {
            if (alert.rule.notifications && typeof alert.rule.notifications.email === 'boolean') {
                ruleEmailEnabled = alert.rule.notifications.email;
            } else if (typeof alert.rule.sendEmail === 'boolean') {
                ruleEmailEnabled = alert.rule.sendEmail;
            }
        }
        sendEmail = ruleEmailEnabled && !!this.emailTransporter;
        
        // For webhooks - check if webhook URL exists and if rule has webhooks enabled
        let ruleWebhookEnabled = false; // Default to false for webhooks
        if (alert.rule) {
            if (alert.rule.notifications && typeof alert.rule.notifications.webhook === 'boolean') {
                ruleWebhookEnabled = alert.rule.notifications.webhook;
            } else if (typeof alert.rule.sendWebhook === 'boolean') {
                ruleWebhookEnabled = alert.rule.sendWebhook;
            }
        }
        sendWebhook = ruleWebhookEnabled && !!process.env.WEBHOOK_URL;
        console.log(`[AlertManager] Webhook check - ruleEnabled: ${ruleWebhookEnabled}, hasURL: ${!!process.env.WEBHOOK_URL}, sendWebhook: ${sendWebhook}`);
        
        // Initialize notification status tracking for this alert
        const alertId = alert.id;
        if (!this.notificationStatus) {
            this.notificationStatus = new Map();
        }
        
        const statusUpdate = {
            emailSent: false,
            webhookSent: false,
            channels: []
        };
        
        // Send email notification with cooldown check
        if (sendEmail) {
            // Check cooldown before sending email
            const cooldownKey = this.getEmailCooldownKey(alert);
            const cooldownInfo = this.emailCooldowns.get(cooldownKey);
            const now = Date.now();
            
            console.log(`[AlertManager] Current cooldown info:`, cooldownInfo);
            
            // Use per-guest cooldown config if available, otherwise fall back to default
            let cooldownConfig = this.perGuestCooldownConfig || this.emailCooldownConfig;
            
            // Map cooldownMinutes to defaultCooldownMinutes if needed
            if (cooldownConfig.cooldownMinutes !== undefined && cooldownConfig.defaultCooldownMinutes === undefined) {
                cooldownConfig = {
                    defaultCooldownMinutes: cooldownConfig.cooldownMinutes,
                    debounceDelayMinutes: cooldownConfig.debounceMinutes !== undefined ? cooldownConfig.debounceMinutes : 2,
                    maxEmailsPerHour: cooldownConfig.maxEmailsPerHour || 4
                };
            }
            
            // Ensure cooldownConfig has valid values (allow 0 as valid)
            if (cooldownConfig.defaultCooldownMinutes === undefined || cooldownConfig.defaultCooldownMinutes === null || isNaN(cooldownConfig.defaultCooldownMinutes)) {
                cooldownConfig.defaultCooldownMinutes = 15; // Fallback to 15 minutes
            }
            
            if (cooldownInfo && now < cooldownInfo.cooldownUntil) {
                const remainingMinutes = Math.ceil((cooldownInfo.cooldownUntil - now) / 60000);
                statusUpdate.emailSkipped = true;
                statusUpdate.cooldownRemaining = remainingMinutes;
            } else {
                // Check if this is a new alert that should be debounced
                const isNewAlert = !cooldownInfo || !cooldownInfo.lastSent;
                if (isNewAlert && cooldownConfig.debounceDelayMinutes > 0) {
                    // For new alerts, wait for debounce period before sending first email
                    if (!cooldownInfo || !cooldownInfo.debounceStarted) {
                        // Start debounce timer
                        this.emailCooldowns.set(cooldownKey, {
                            debounceStarted: now,
                            debounceUntil: now + (cooldownConfig.debounceDelayMinutes * 60000)
                        });
                        console.log(`[AlertManager] Starting ${cooldownConfig.debounceDelayMinutes} minute debounce for alert ${alert.id}`);
                        statusUpdate.emailDebounced = true;
                        // Don't return early - still need to check webhook
                    } else if (now < cooldownInfo.debounceUntil) {
                        // Still in debounce period
                        const remainingSeconds = Math.ceil((cooldownInfo.debounceUntil - now) / 1000);
                        statusUpdate.emailDebounced = true;
                        // Don't return early - still need to check webhook
                    }
                }
                
                const oneHourAgo = now - 3600000;
                const recentEmails = cooldownInfo?.emailHistory?.filter(timestamp => timestamp > oneHourAgo) || [];
                if (recentEmails.length >= cooldownConfig.maxEmailsPerHour) {
                    statusUpdate.emailRateLimited = true;
                    // Don't return early - still need to check webhook
                }
                
                // Only send email if not debounced or rate limited
                if (!statusUpdate.emailDebounced && !statusUpdate.emailRateLimited) {
                    try {
                        if (this.emailBatchEnabled) {
                            // Add to email queue for batching
                            this.queueEmailNotification(alert);
                        } else {
                            // Send immediately
                            await this.sendDirectEmailNotification(alert);
                        }
                        
                        // Update cooldown tracking
                        const emailHistory = cooldownInfo?.emailHistory || [];
                        emailHistory.push(now);
                        // Keep only last 24 hours of history
                        const oneDayAgo = now - 86400000;
                        const recentHistory = emailHistory.filter(timestamp => timestamp > oneDayAgo);
                        
                        const cooldownData = {
                            lastSent: now,
                            cooldownUntil: now + (cooldownConfig.defaultCooldownMinutes * 60000),
                            emailHistory: recentHistory,
                            debounceStarted: cooldownInfo?.debounceStarted || now
                        };
                        this.emailCooldowns.set(cooldownKey, cooldownData);
                        
                        statusUpdate.emailSent = true;
                        statusUpdate.channels.push('email');
                    } catch (error) {
                        console.error(`[EMAIL ERROR] Failed to send email for alert ${alert.id}:`, error);
                        this.emit('notificationError', { type: 'email', alert, error });
                    }
                }
            }
        } else {
        }
        
        // Send webhook notification with cooldown check
        if (sendWebhook) {
            console.log(`[AlertManager] Webhook notification check for alert ${alert.id} - Rule: ${alert.rule?.name}, Guest: ${alert.guest?.name}`);
            
            // Check cooldown before sending webhook
            const cooldownKey = this.getWebhookCooldownKey(alert);
            const cooldownInfo = this.webhookCooldowns.get(cooldownKey);
            const now = Date.now();
            
            // Use per-guest cooldown config if available, otherwise fall back to default
            let cooldownConfig = this.perGuestCooldownConfig?.webhook || this.webhookCooldownConfig;
            
            // Map cooldownMinutes to defaultCooldownMinutes if needed
            if (cooldownConfig.cooldownMinutes !== undefined && cooldownConfig.defaultCooldownMinutes === undefined) {
                cooldownConfig = {
                    defaultCooldownMinutes: cooldownConfig.cooldownMinutes,
                    debounceDelayMinutes: cooldownConfig.debounceMinutes !== undefined ? cooldownConfig.debounceMinutes : 1,
                    maxCallsPerHour: cooldownConfig.maxCallsPerHour || 10
                };
            }
            
            // Ensure cooldownConfig has valid values (allow 0 as valid)
            if (cooldownConfig.defaultCooldownMinutes === undefined || cooldownConfig.defaultCooldownMinutes === null || isNaN(cooldownConfig.defaultCooldownMinutes)) {
                cooldownConfig.defaultCooldownMinutes = 5; // Fallback to 5 minutes for webhooks
            }
            if (cooldownConfig.debounceDelayMinutes === undefined || cooldownConfig.debounceDelayMinutes === null || isNaN(cooldownConfig.debounceDelayMinutes)) {
                cooldownConfig.debounceDelayMinutes = 1; // Fallback to 1 minute
            }
            
            console.log(`[AlertManager] Webhook cooldown config:`, cooldownConfig);
            console.log(`[AlertManager] Cooldown info for ${cooldownKey}:`, cooldownInfo);
            
            if (cooldownInfo && now < cooldownInfo.cooldownUntil) {
                const remainingMinutes = Math.ceil((cooldownInfo.cooldownUntil - now) / 60000);
                console.log(`[AlertManager] Webhook cooldown active for alert ${alert.id} - ${remainingMinutes} minutes remaining`);
                statusUpdate.webhookSkipped = true;
                statusUpdate.webhookCooldownRemaining = remainingMinutes;
            } else {
                // Check if this is a new alert that should be debounced
                const isNewAlert = !cooldownInfo || !cooldownInfo.lastSent;
                console.log(`[AlertManager] Is new alert: ${isNewAlert}, debounce delay: ${cooldownConfig.debounceDelayMinutes} minutes`);
                
                if (isNewAlert && cooldownConfig.debounceDelayMinutes > 0) {
                    // For new alerts, wait for debounce period before sending first webhook
                    if (!cooldownInfo || !cooldownInfo.debounceStarted) {
                        // Start debounce timer
                        this.webhookCooldowns.set(cooldownKey, {
                            debounceStarted: now,
                            debounceUntil: now + (cooldownConfig.debounceDelayMinutes * 60000)
                        });
                        console.log(`[AlertManager] Starting ${cooldownConfig.debounceDelayMinutes} minute debounce for webhook alert ${alert.id}`);
                        statusUpdate.webhookDebounced = true;
                    } else if (now < cooldownInfo.debounceUntil) {
                        // Still in debounce period
                        const remainingSeconds = Math.ceil((cooldownInfo.debounceUntil - now) / 1000);
                        console.log(`[AlertManager] Webhook debounce active for alert ${alert.id} - ${remainingSeconds} seconds remaining`);
                        statusUpdate.webhookDebounced = true;
                    }
                }
                
                const oneHourAgo = now - 3600000;
                const recentCalls = cooldownInfo?.callHistory?.filter(timestamp => timestamp > oneHourAgo) || [];
                if (recentCalls.length >= cooldownConfig.maxCallsPerHour) {
                    console.log(`[AlertManager] Webhook rate limit reached for alert ${alert.id} - ${recentCalls.length} calls made in last hour`);
                    statusUpdate.webhookRateLimited = true;
                }
                
                // Only send webhook if not debounced or rate limited
                if (!statusUpdate.webhookDebounced && !statusUpdate.webhookRateLimited) {
                    console.log(`[AlertManager] Queueing webhook notification for alert ${alert.id}`);
                    try {
                        // Use webhook batcher for intelligent batching
                        const webhookUrl = process.env.WEBHOOK_URL;
                        await this.webhookBatcher.queueAlert(alert, webhookUrl);
                        
                        // Note: The actual sending and status update is handled by the batcher
                        
                        // Update cooldown tracking
                        const callHistory = cooldownInfo?.callHistory || [];
                        callHistory.push(now);
                        // Keep only last 24 hours of history
                        const oneDayAgo = now - 86400000;
                        const recentHistory = callHistory.filter(timestamp => timestamp > oneDayAgo);
                        
                        this.webhookCooldowns.set(cooldownKey, {
                            lastSent: now,
                            cooldownUntil: now + (cooldownConfig.defaultCooldownMinutes * 60000),
                            callHistory: recentHistory,
                            debounceStarted: cooldownInfo?.debounceStarted || now
                        });
                        
                        statusUpdate.webhookSent = true;
                        statusUpdate.channels.push('webhook');
                    } catch (error) {
                        console.error(`[WEBHOOK ERROR] Failed to send webhook for alert ${alert.id}:`, error);
                        this.emit('notificationError', { type: 'webhook', alert, error });
                    }
                }
            }
        }
        
        // Only update notification status after actual delivery attempts
        this.notificationStatus.set(alertId, statusUpdate);
        
        // Also update the alert object itself with notification status
        if (statusUpdate.emailSent) {
            alert.emailSent = true;
        }
        if (statusUpdate.webhookSent) {
            alert.webhookSent = true;
        }
        
        // Save the updated alerts to persist the notification status
        this.saveActiveAlerts();
        
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

    async clearAlertHistory(permanent = false) {
        console.log('[AlertManager] Clearing alert history', permanent ? '(permanent)' : '(cache only)');
        this.alertHistory = [];
        
        if (permanent) {
            // Clear the history file on disk
            try {
                await alertHistory.saveHistory([]);
                console.log('[AlertManager] Alert history file cleared');
            } catch (error) {
                console.error('[AlertManager] Error clearing history file:', error);
                throw error;
            }
        } else {
            // Reload from disk to ensure we have the latest data
            await this.loadAlertHistory();
        }
        
        console.log(`[AlertManager] Alert history ${permanent ? 'cleared' : 'reloaded'}: ${this.alertHistory.length} alerts`);
    }

    formatAlertForAPI(alert) {
        try {
            // Create a safe, serializable alert object with no circular references
            // Use primitive type conversion to ensure no object references are kept
            const safeAlert = {
                id: String(alert.id || 'unknown'), // Use the stored permanent ID
                ruleId: String(alert.rule?.id || 'unknown'),
                ruleName: String(alert.rule?.name || 'Unknown Rule'),
                description: String(alert.rule?.description || ''),
                group: String(alert.rule?.group || 'unknown'),
                tags: Array.isArray(alert.rule?.tags) ? alert.rule.tags.map(t => String(t)) : [],
                metric: String(alert.metric || alert.rule?.metric || (alert.rule?.type === 'compound_threshold' ? 'compound' : 'unknown')),
                threshold: Number(alert.threshold || alert.effectiveThreshold || alert.rule?.threshold || 0),
                currentValue: alert.currentValue != null ? Number(alert.currentValue) : null,
                triggeredAt: Number(alert.triggeredAt || Date.now()),
                duration: Number(alert.triggeredAt ? Date.now() - alert.triggeredAt : 0),
                acknowledged: Boolean(alert.acknowledged),
                acknowledgedBy: alert.acknowledgedBy ? String(alert.acknowledgedBy) : null,
                acknowledgedAt: alert.acknowledgedAt ? Number(alert.acknowledgedAt) : null,
                type: String(alert.rule?.type || 'single_metric'),
                thresholds: Array.isArray(alert.rule?.thresholds) ? 
                    alert.rule.thresholds.map(t => ({
                        metric: String(t.metric || ''),
                        condition: String(t.condition || '>'),
                        threshold: Number(t.threshold || 0)
                    })) : null,
                emailSent: Boolean(alert.emailSent || this.notificationStatus?.get(alert.id)?.emailSent),
                webhookSent: Boolean(alert.webhookSent || this.notificationStatus?.get(alert.id)?.webhookSent),
                notificationChannels: this.getEnabledNotificationChannels(alert),
                currentValue: alert.currentValue || null,
                threshold: alert.threshold || null
            };
            
            // Handle node alerts differently - they don't have guest property
            if (alert.type === 'node_threshold') {
                safeAlert.guest = {
                    name: String(alert.nodeName || alert.nodeId || 'Unknown Node'),
                    vmid: 'node',
                    node: String(alert.nodeId || 'unknown'),
                    type: 'node',
                    endpointId: String(alert.nodeId || 'unknown')
                };
                safeAlert.message = String(alert.message || `Node ${alert.nodeName || alert.nodeId} ${alert.metric} alert`);
            } else {
                // Regular guest alerts
                safeAlert.guest = {
                    name: String(alert.guest?.name || 'Unknown'),
                    vmid: String(alert.guest?.vmid || 'unknown'),
                    node: String(alert.guest?.node || 'unknown'),
                    type: String(alert.guest?.type || 'unknown'),
                    endpointId: String(alert.guest?.endpointId || 'unknown')
                };
                safeAlert.message = String(alert.message || this.generateAlertMessage(alert));
            }
            
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

    // Helper function to get enabled notification channels based on rule settings
    getEnabledNotificationChannels(alert) {
        const channels = [];
        
        // Dashboard is always included
        channels.push('dashboard');
        
        // Check rule notifications settings
        if (alert.rule?.notifications) {
            // Check email - enabled if email is true AND email transporter is configured
            if (alert.rule.notifications.email && this.emailTransporter) {
                channels.push('email');
            }
            
            // Check webhook - enabled if webhook is true AND webhook URL is configured
            if (alert.rule.notifications.webhook && process.env.WEBHOOK_URL) {
                channels.push('webhook');
            }
        }
        
        return channels;
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


        return {
            active: activeCount,
            acknowledged: acknowledgedCount,
            last24Hours: last24h.length,
            lastHour: lastHour.length,
            totalRules: this.alertRules.size,
            suppressedRules: this.suppressedAlerts.size,
            metrics: this.alertMetrics,
            groups: Array.from(this.alertGroups.values())
        };
    }

    // Rest of the existing methods with enhancements...
    calculateIORate(metrics, metricName, guest) {
        // Calculate I/O rate from cumulative counters using metrics history
        const guestId = `${guest.endpointId}-${guest.node}-${guest.vmid}`;
        const currentValue = metrics[metricName] || 0;
        const timestamp = Date.now();
        
        
        
        
        // Get or initialize metrics history for this guest
        if (!this.guestMetricsHistory) {
            this.guestMetricsHistory = new Map();
        }
        
        const guestHistory = this.guestMetricsHistory.get(guestId) || [];
        const currentEntry = { timestamp, [metricName]: currentValue };
        
        // Add current entry to history
        guestHistory.push(currentEntry);
        
        if (guestHistory.length > 10) {
            guestHistory.shift();
        }
        
        this.guestMetricsHistory.set(guestId, guestHistory);
        
        // Need at least 2 data points to calculate rate
        if (guestHistory.length < 2) {
            return 0; // No rate can be calculated yet
        }
        
        // Calculate rate using the same logic as dashboard
        const validHistory = guestHistory.filter(entry =>
            typeof entry.timestamp === 'number' && !isNaN(entry.timestamp) &&
            typeof entry[metricName] === 'number' && !isNaN(entry[metricName])
        );
        
        if (validHistory.length < 2) {
            return 0;
        }
        
        const oldest = validHistory[0];
        const newest = validHistory[validHistory.length - 1];
        const valueDiff = newest[metricName] - oldest[metricName];
        const timeDiffSeconds = (newest.timestamp - oldest.timestamp) / 1000;
        
        if (timeDiffSeconds <= 0 || valueDiff < 0) {
            return 0;
        }
        
        // Return rate in bytes per second
        const rate = valueDiff / timeDiffSeconds;
        
        // Debug logging to see actual rates
        if (guest && guest.name && rate > 0) {
            console.log(`[AlertManager] ${guest.name} ${metricName} rate: ${Math.round(rate/1024/1024*100)/100} MB/s`);
        }
        
        return rate;
    }

    getMetricValue(metrics, metricName, guest) {
        switch (metricName) {
            case 'cpu':
                // but in some processing they might already be converted to percentages
                const cpuValue = metrics.cpu;
                if (typeof cpuValue !== 'number' || isNaN(cpuValue)) {
                    return 0; // Invalid CPU value
                }
                
                // If value is > 1.0, assume it's already in percentage format
                // If value is <= 1.0, assume it's in decimal format and convert to percentage
                return cpuValue > 1.0 ? cpuValue : cpuValue * 100;
            case 'memory':
                if (metrics.maxmem) {
                    const mem = metrics.mem || 0; // Treat undefined/null mem as 0
                    const memoryPercent = (mem / metrics.maxmem) * 100;
                    console.log(`[AlertManager] Memory calculation: ${mem}/${metrics.maxmem} = ${memoryPercent}%`);
                    return memoryPercent;
                }
                console.log(`[AlertManager] Memory calculation failed: missing maxmem (maxmem=${metrics.maxmem}, mem=${metrics.mem})`);
                return null;
            case 'disk':
                if (metrics.maxdisk) {
                    const disk = metrics.disk || 0; // Treat undefined/null disk as 0
                    return (disk / metrics.maxdisk) * 100;
                }
                return null;
            case 'diskread':
            case 'diskwrite':
            case 'netin':
            case 'netout':
                // I/O metrics are cumulative counters, need to calculate rate
                return this.calculateIORate(metrics, metricName, guest);
            default:
                return metrics[metricName] || null;
        }
    }

    async triggerAlert(alert) {
        try {
            const alertInfo = this.formatAlertForAPI(alert);
            
            // Add to history
            this.addToHistory(alertInfo);
            
            // For node alerts, ensure guest property is set
            if (alert.type === 'node_threshold' && !alert.guest) {
                alert.guest = {
                    name: alert.nodeName || alert.nodeId || 'Unknown Node',
                    vmid: 'node',
                    node: alert.nodeId || 'unknown',
                    type: 'node',
                    endpointId: alert.nodeId || 'unknown'
                };
            }
            
            // Add to batch instead of sending immediately
            this.pendingAlertBatch.push(alert);
            
            // Clear any existing timeout
            if (this.batchTimeout) {
                clearTimeout(this.batchTimeout);
            }
            
            // Set a very short timeout (10ms) to batch alerts from the same evaluation cycle
            this.batchTimeout = setTimeout(() => {
                this.processPendingAlertBatch();
            }, 10);
            
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

    /**
     * Process pending alert batch - send individual or grouped notifications
     */
    async processPendingAlertBatch() {
        const alerts = [...this.pendingAlertBatch];
        this.pendingAlertBatch = [];
        this.batchTimeout = null;
        
        if (alerts.length === 0) return;
        
        console.log(`[AlertManager] Processing batch of ${alerts.length} alerts`);
        
        // If only one alert, send it normally
        if (alerts.length === 1) {
            await this.sendNotifications(alerts[0]);
            return;
        }
        
        // Group alerts by notification type
        const emailAlerts = alerts.filter(a => a.notificationChannels?.includes('email'));
        const webhookAlerts = alerts.filter(a => a.notificationChannels?.includes('webhook'));
        
        // Send grouped email if multiple email alerts
        if (emailAlerts.length > 1) {
            console.log(`[AlertManager] Sending grouped email for ${emailAlerts.length} alerts`);
            await this.sendGroupedEmailNotification(emailAlerts);
        } else if (emailAlerts.length === 1) {
            await this.sendEmailNotification(emailAlerts[0]);
        }
        
        // Send grouped webhook if multiple webhook alerts
        if (webhookAlerts.length > 1) {
            console.log(`[AlertManager] Sending grouped webhook for ${webhookAlerts.length} alerts`);
            await this.sendGroupedWebhookNotification(webhookAlerts);
        } else if (webhookAlerts.length === 1) {
            await this.sendWebhookNotification(webhookAlerts[0]);
        }
        
        // Mark alerts as sent
        for (const alert of alerts) {
            const status = this.notificationStatus.get(alert.id) || {};
            
            if (alert.notificationChannels?.includes('email')) {
                alert.emailSent = true;
                status.emailSent = true;
            }
            if (alert.notificationChannels?.includes('webhook')) {
                alert.webhookSent = true;
                status.webhookSent = true;
            }
            
            this.notificationStatus.set(alert.id, status);
        }
    }
    
    /**
     * Process pending resolved alert batch - emit as individual or grouped events
     */
    async processPendingResolvedBatch() {
        const resolvedAlerts = [...this.pendingResolvedBatch];
        this.pendingResolvedBatch = [];
        this.resolvedBatchTimeout = null;
        
        if (resolvedAlerts.length === 0) return;
        
        console.log(`[AlertManager] Processing batch of ${resolvedAlerts.length} resolved alerts`);
        
        // Emit each resolved alert individually
        // The frontend will handle grouping them for display
        for (const alertInfo of resolvedAlerts) {
            this.emit('alertResolved', alertInfo);
        }
    }
    
    /**
     * Send a grouped email notification for multiple alerts
     */
    async sendGroupedEmailNotification(alerts) {
        if (!this.emailTransporter) {
            console.log('[AlertManager] Email not configured, skipping grouped email notification');
            return;
        }
        
        try {
            const subject = `🚨 Pulse Alert: ${alerts.length} alerts triggered`;
            
            // Group alerts by metric type
            const alertsByMetric = {};
            for (const alert of alerts) {
                const metric = alert.metric || 'unknown';
                if (!alertsByMetric[metric]) {
                    alertsByMetric[metric] = [];
                }
                alertsByMetric[metric].push(alert);
            }
            
            // Prepare email configuration
            const config = await this.loadEmailConfig();
            const fromEmail = config.from || process.env.ALERT_FROM_EMAIL || 'alerts@pulse-monitoring.local';
            const toEmail = config.to || process.env.ALERT_TO_EMAIL;
            const smtpHost = config.host || process.env.SMTP_HOST || 'localhost';
            const smtpPort = config.port || process.env.SMTP_PORT || '587';
            
            // Prepare alert data for the template
            const alertDetails = [];
            for (const [metric, metricAlerts] of Object.entries(alertsByMetric)) {
                for (const alert of metricAlerts) {
                    const value = alert.currentValue || 0;
                    const threshold = alert.threshold || 0;
                    const displayValue = value < 1 ? value.toFixed(1) : Math.round(value);
                    const displayThreshold = threshold < 1 ? threshold.toFixed(1) : Math.round(threshold);
                    
                    alertDetails.push({
                        guestName: alert.guest?.name || 'Unknown',
                        guestType: alert.guest?.type || 'unknown',
                        guestId: alert.guest?.vmid || 'N/A',
                        node: alert.guest?.node || 'unknown',
                        metric: metric.toUpperCase(),
                        currentValue: `${displayValue}%`,
                        threshold: `${displayThreshold}%`,
                        status: alert.guest?.status || 'unknown'
                    });
                }
            }
            
            // Group by node for summary
            const alertsByNode = {};
            for (const alert of alerts) {
                const node = alert.guest?.node || 'unknown';
                if (!alertsByNode[node]) alertsByNode[node] = 0;
                alertsByNode[node]++;
            }
            
            // Prepare alertsByType in the format expected by the template
            const alertsByType = {};
            for (const [metric, metricAlerts] of Object.entries(alertsByMetric)) {
                alertsByType[metric] = metricAlerts.map(alert => ({
                    guest: alert.guest?.name || 'Unknown',
                    value: alert.currentValue || 0,
                    threshold: alert.threshold || 0
                }));
            }
            
            // Use the unified template with summary type
            const html = this.generateEmailTemplate({
                type: 'summary',
                data: {
                    title: `Alert Summary: ${alerts.length} alerts`,
                    subtitle: 'Multiple alerts triggered',
                    fromEmail: fromEmail,
                    toEmail: toEmail,
                    smtpHost: smtpHost,
                    smtpPort: smtpPort,
                    alerts: alerts,
                    alertsByType: alertsByType,
                    alertsByNode: alertsByNode,
                    totalCount: alerts.length
                }
            });
            
            // Generate plain text version
            let text = `PULSE ALERT: ${alerts.length} alerts triggered\n\n`;
            for (const [metric, metricAlerts] of Object.entries(alertsByMetric)) {
                text += `${metric.toUpperCase()} ALERTS (${metricAlerts.length}):\n`;
                for (const alert of metricAlerts) {
                    const value = alert.currentValue || 0;
                    const threshold = alert.threshold || 0;
                    const displayValue = value < 1 ? value.toFixed(1) : Math.round(value);
                    const displayThreshold = threshold < 1 ? threshold.toFixed(1) : Math.round(threshold);
                    text += `  - ${alert.guest?.name}: ${displayValue}% (threshold: ${displayThreshold}%)\n`;
                }
                text += '\n';
            }
            text += `\nTime: ${new Date().toLocaleString()}\n`;
            text += '---\nThis alert was generated by Pulse monitoring system.';
            
            const mailOptions = {
                from: fromEmail,
                to: toEmail,
                subject: subject,
                html: html
            };
            
            await this.emailTransporter.sendMail(mailOptions);
            console.log('[AlertManager] Grouped email sent successfully');
        } catch (error) {
            console.error('[AlertManager] Failed to send grouped email:', error);
        }
    }
    
    /**
     * Send a grouped webhook notification for multiple alerts
     */
    async sendGroupedWebhookNotification(alerts) {
        const webhookUrl = process.env.WEBHOOK_URL;
        if (!webhookUrl) {
            console.log('[AlertManager] Webhook not configured, skipping grouped webhook notification');
            return;
        }
        
        try {
            // Create a summary payload
            const summary = {
                type: 'grouped_alerts',
                count: alerts.length,
                alerts: alerts.map(alert => ({
                    guest: alert.guest?.name || 'Unknown',
                    metric: alert.metric,
                    value: alert.currentValue,
                    threshold: alert.threshold,
                    message: alert.message
                })),
                triggeredAt: Date.now()
            };
            
            const NotificationService = require('./notificationServices');
            const notificationService = new NotificationService();
            await notificationService.send(webhookUrl, summary);
            console.log('[AlertManager] Grouped webhook sent successfully');
        } catch (error) {
            console.error('[AlertManager] Failed to send grouped webhook:', error);
        }
    }
    
    /**
     * Check alerts in history and resolve any that no longer meet trigger conditions
     */
    async checkAndResolveHistoricalAlerts(allGuests, currentMetrics) {
        try {
            let resolvedCount = 0;
            const now = Date.now();
            
            // Load persistent history to check
            const persistentHistory = await alertHistory.loadHistory();
            console.log(`[AlertManager] Checking ${persistentHistory.length} historical alerts for resolution`);
            
            // Check each unresolved alert in persistent history
            for (const histAlert of persistentHistory) {
                if (histAlert.resolved) continue;
                
                // For individual metric alerts, check if conditions still exist
                if (histAlert.metric && histAlert.guest && histAlert.metric !== 'bundled') {
                    const guest = allGuests.find(g => 
                        g.vmid === histAlert.guest.vmid && 
                        g.node === histAlert.guest.node
                    );
                    
                    if (guest) {
                        // Re-evaluate the guest to see if alert conditions still exist
                        const globalThresholds = this.getGlobalThresholds();
                        const guestThresholds = this.getGuestSpecificThresholds(guest);
                        const guestMetrics = currentMetrics[`${guest.node}_${guest.vmid}`];
                        
                        // Check if this specific metric still exceeds threshold
                        const metricResult = this.evaluateMetricForGuest(guest, histAlert.metric, guestThresholds, globalThresholds, guestMetrics);
                        
                        // If metric no longer exceeds threshold, check if we should resolve
                        if (!metricResult || !metricResult.isExceeded) {
                            // Get the rule configuration to check autoResolve setting
                            const rule = Array.from(this.alertRules.values()).find(r => r.type === 'per_guest_thresholds');
                            
                            // Check if auto-resolve is disabled
                            if (rule?.autoResolve === false) {
                                console.log(`[AlertManager] Auto-resolve disabled for ${histAlert.metric} alert ${histAlert.id} for ${guest.name}, keeping alert active`);
                                continue;
                            }
                            
                            histAlert.resolved = true;
                            histAlert.resolvedAt = now;
                            histAlert.duration = histAlert.duration || (now - histAlert.triggeredAt);
                            
                            console.log(`[AlertManager] Resolving historical ${histAlert.metric} alert ${histAlert.id} for ${guest.name} - threshold no longer exceeded`);
                            
                            // Update in persistent history
                            try {
                                await alertHistory.updateInHistory(histAlert.id, {
                                    resolved: true,
                                    resolvedAt: histAlert.resolvedAt,
                                    duration: histAlert.duration
                                });
                                
                                // Also update in-memory history
                                const memIndex = this.alertHistory.findIndex(h => h.id === histAlert.id);
                                if (memIndex >= 0) {
                                    this.alertHistory[memIndex].resolved = true;
                                    this.alertHistory[memIndex].resolvedAt = histAlert.resolvedAt;
                                    this.alertHistory[memIndex].duration = histAlert.duration;
                                }
                                
                                resolvedCount++;
                            } catch (error) {
                                console.error(`[AlertManager] Failed to update resolved status for alert ${histAlert.id}:`, error);
                            }
                            
                            continue;
                        }
                    }
                }
                
                // Check if this alert is still in activeAlerts
                const isStillActive = Array.from(this.activeAlerts.values()).some(
                    active => active.id === histAlert.id
                );
                
                if (!isStillActive) {
                    // Alert is not active anymore, mark it as resolved
                    histAlert.resolved = true;
                    histAlert.resolvedAt = histAlert.resolvedAt || now;
                    histAlert.duration = histAlert.duration || (histAlert.resolvedAt - histAlert.triggeredAt);
                    
                    // Update in persistent history
                    try {
                        await alertHistory.updateInHistory(histAlert.id, {
                            resolved: true,
                            resolvedAt: histAlert.resolvedAt,
                            duration: histAlert.duration
                        });
                        
                        // Also update in-memory history
                        const memIndex = this.alertHistory.findIndex(h => h.id === histAlert.id);
                        if (memIndex >= 0) {
                            this.alertHistory[memIndex].resolved = true;
                            this.alertHistory[memIndex].resolvedAt = histAlert.resolvedAt;
                            this.alertHistory[memIndex].duration = histAlert.duration;
                        }
                        
                        resolvedCount++;
                    } catch (error) {
                        console.error(`[AlertManager] Failed to update resolved status for alert ${histAlert.id}:`, error);
                    }
                }
            }
            
            if (resolvedCount > 0) {
                console.log(`[AlertManager] Auto-resolved ${resolvedCount} historical alerts that are no longer active`);
            }
        } catch (error) {
            console.error('[AlertManager] Error checking historical alerts:', error);
        }
    }

    async resolveAlert(alert) {
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
            metric: alert.metric || alert.rule.metric,
            resolvedAt: alert.resolvedAt,
            resolved: true,
            duration: alert.resolvedAt - alert.triggeredAt,
            message: this.generateResolvedMessage(alert),
            triggeredAt: alert.triggeredAt,
            currentValue: alert.currentValue,
            threshold: alert.threshold
        };

        // Add to history
        await this.addToHistory(alertInfo);
        
        // Update in persistent history
        try {
            console.log(`[AlertManager] Updating alert ${alert.id} as resolved in persistent history`);
            const updated = await alertHistory.updateInHistory(alert.id, {
                resolved: true,
                resolvedAt: alert.resolvedAt,
                duration: alert.resolvedAt - alert.triggeredAt
            });
            if (updated) {
                console.log(`[AlertManager] Successfully marked alert ${alert.id} as resolved in history`);
            } else {
                console.log(`[AlertManager] Alert ${alert.id} not found in history to update`);
            }
        } catch (error) {
            console.error('[AlertManager] Failed to update resolved alert in history:', error);
        }

        // Apply suppression if configured
        if (alert.rule.suppressionTime > 0) {
            this.suppressAlert(alert.rule.id, {
                endpointId: alert.guest.endpointId,
                node: alert.guest.node,
                vmid: alert.guest.vmid
            }, alert.rule.suppressionTime, 'Auto-suppression after resolution');
        }

        // Add to batch instead of emitting immediately
        this.pendingResolvedBatch.push(alertInfo);
        
        // Clear any existing timeout
        if (this.resolvedBatchTimeout) {
            clearTimeout(this.resolvedBatchTimeout);
        }
        
        // Set a longer timeout (50ms) to ensure all resolutions from the same evaluation cycle are batched
        this.resolvedBatchTimeout = setTimeout(() => {
            this.processPendingResolvedBatch();
        }, 50);

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
        
        if (rule.metric === 'combined_thresholds') {
            return `${guest.name || 'Unknown'} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid || 'N/A'}) on ${guest.node || 'Unknown'} - ${rule.name}`;
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
            const metric = rule.metric || 'unknown';
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(metric);
            const formattedValue = typeof currentValue === 'number' ? Math.round(currentValue) : currentValue;
            valueStr = metric !== 'unknown' ? `${metric.toUpperCase()}: ${formattedValue}${isPercentageMetric ? '%' : ''}` : `Alert: ${formattedValue}`;
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
        
        // For per-guest threshold alerts, use cleaner format without rule name prefix
        if (rule.metric === 'combined_thresholds') {
            return `${guest.name || 'Unknown'} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid || 'N/A'}) on ${guest.node || 'Unknown'} - ${valueStr} (threshold: ${thresholdStr})`;
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

    async loadAlertHistory() {
        try {
            const persistedHistory = await alertHistory.loadHistory();
            // Merge with in-memory history, avoiding duplicates
            const historyMap = new Map();
            
            // Add persisted history first
            persistedHistory.forEach(alert => {
                historyMap.set(alert.id, alert);
            });
            
            // Add current in-memory history (may have newer data)
            this.alertHistory.forEach(alert => {
                if (!historyMap.has(alert.id)) {
                    historyMap.set(alert.id, alert);
                }
            });
            
            this.alertHistory = Array.from(historyMap.values())
                .sort((a, b) => (b.triggeredAt || 0) - (a.triggeredAt || 0));
            
            console.log(`[AlertManager] Loaded ${this.alertHistory.length} alerts from history`);
        } catch (error) {
            console.error('[AlertManager] Failed to load alert history:', error);
        }
    }

    async addToHistory(alertInfo) {
        this.alertHistory.unshift(alertInfo);
        if (this.alertHistory.length > this.maxHistorySize) {
            this.alertHistory = this.alertHistory.slice(0, this.maxHistorySize);
        }
        
        // Save to persistent history
        try {
            await alertHistory.addToHistory(alertInfo);
        } catch (error) {
            console.error('[AlertManager] Failed to persist alert to history:', error);
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
            // No need to save separately - acknowledgements are part of active alerts
        }
        
        // Save if alerts or notification history were cleaned up
        if (alertsRemoved || acknowledgementsChanged) {
            this.saveActiveAlerts();
            this.saveNotificationHistory();
        }
    }
    
    cleanupExpiredCooldowns() {
        const now = Date.now();
        const oneDayAgo = now - 86400000; // 24 hours
        let emailCooldownsRemoved = 0;
        let webhookCooldownsRemoved = 0;
        
        // Clean up expired email cooldowns
        for (const [key, cooldownInfo] of this.emailCooldowns) {
            // Remove cooldowns that have expired and haven't been used in 24 hours
            if (cooldownInfo.cooldownUntil < now && 
                (!cooldownInfo.lastSent || cooldownInfo.lastSent < oneDayAgo)) {
                this.emailCooldowns.delete(key);
                emailCooldownsRemoved++;
            }
        }
        
        // Clean up expired webhook cooldowns
        for (const [key, cooldownInfo] of this.webhookCooldowns) {
            // Remove cooldowns that have expired and haven't been used in 24 hours
            if (cooldownInfo.cooldownUntil < now && 
                (!cooldownInfo.lastSent || cooldownInfo.lastSent < oneDayAgo)) {
                this.webhookCooldowns.delete(key);
                webhookCooldownsRemoved++;
            }
        }
        
        if (emailCooldownsRemoved > 0) {
        }
        if (webhookCooldownsRemoved > 0) {
            console.log(`[AlertManager] Cleaned up ${webhookCooldownsRemoved} expired webhook cooldowns`);
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
            
            // Trigger immediate evaluation if rule was enabled
            if (rule.enabled) {
                this.evaluateCurrentState().catch(error => {
                    console.error('[AlertManager] Error evaluating state after rule update:', error);
                });
            }
            
            return true;
        }
        return false;
    }

    addRule(rule) {
        // Support single-metric rules, compound threshold rules, and per-guest threshold rules
        const isCompoundRule = rule.thresholds && Array.isArray(rule.thresholds) && rule.thresholds.length > 0;
        const isPerGuestRule = rule.type === 'per_guest_thresholds' && rule.guestThresholds;
        
        
        // Enhanced validation
        if (!rule.name || typeof rule.name !== 'string' || rule.name.trim().length === 0) {
            throw new Error('Rule must have a valid name (non-empty string)');
        }
        
        if (!isCompoundRule && !isPerGuestRule && !rule.metric) {
            throw new Error('Single-metric rule must have a metric. For compound threshold rules, provide thresholds array. For per-guest rules, provide guestThresholds object.');
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
        } else if (isPerGuestRule) {
            // Validate per-guest threshold rule
            if (!rule.guestThresholds || typeof rule.guestThresholds !== 'object') {
                throw new Error('Per-guest rule must have a guestThresholds object');
            }
            
            // Validate each guest's thresholds
            for (const [guestId, guestThresholds] of Object.entries(rule.guestThresholds)) {
                if (typeof guestThresholds !== 'object') {
                    throw new Error(`Guest ${guestId} thresholds must be an object`);
                }
                
                // Validate each metric threshold for this guest
                for (const [metric, threshold] of Object.entries(guestThresholds)) {
                    const validMetrics = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
                    if (!validMetrics.includes(metric)) {
                        throw new Error(`Invalid metric '${metric}' for guest ${guestId}. Valid metrics: ${validMetrics.join(', ')}`);
                    }
                    
                    if (typeof threshold !== 'string' && typeof threshold !== 'number') {
                        throw new Error(`Invalid threshold value '${threshold}' for metric '${metric}' of guest ${guestId}. Expected string or number.`);
                    }
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
            isPerGuestRule ?
            `per_guest_${Date.now()}_${Math.random().toString(36).substr(2, 9)}` :
            `rule_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`);
        
        // Set defaults for new rules
        const fullRule = {
            id: ruleId,
            condition: 'greater_than',
            duration: 300000,
            enabled: false,
            tags: [],
            group: isCompoundRule ? 'compound_threshold' : isPerGuestRule ? 'per_guest' : 'custom',
            autoResolve: true,
            suppressionTime: 300000,
            type: isCompoundRule ? 'compound_threshold' : isPerGuestRule ? 'per_guest_thresholds' : 'single_metric',
            ...rule
        };
        
        this.alertRules.set(ruleId, fullRule);
        console.log(`[AlertManager] Added ${isCompoundRule ? 'compound threshold' : isPerGuestRule ? 'per-guest threshold' : 'single-metric'} rule: ${fullRule.name} (${ruleId})`);
        
        // Save all rules to disk
        this.saveAlertRules().catch(error => {
            console.error('[AlertManager] Failed to save alert rules after adding rule:', error);
            this.emit('ruleSaveError', { ruleId: fullRule.id, error: error.message });
        });
        
        this.emit('ruleAdded', fullRule);
        
        // Trigger immediate evaluation for the new rule
        this.evaluateCurrentState().catch(error => {
            console.error('[AlertManager] Error evaluating state after adding rule:', error);
        });
        
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
        this.evaluateCurrentState().catch(error => {
            console.error('[AlertManager] Error evaluating state after refreshing rules:', error);
        });
        
        this.emit('rulesRefreshed', { activeRules: nowActiveRules.size, disabledRules });
    }

    /**
     * Evaluate current system state against all enabled rules
     * This should be called when rules are enabled to check for immediate alerts
     */
    async evaluateCurrentState() {
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

            // Check and resolve any alerts in history that no longer meet conditions
            await this.checkAndResolveHistoricalAlerts(allGuests, currentMetrics);

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
                                        acknowledged: false
                            };
                            
                            this.activeAlerts.set(alertKey, newAlert);
                            this.triggerAlert(newAlert).catch(error => {
                                console.error(`[AlertManager] Error triggering alert ${newAlert.id}:`, error);
                            });
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
            this.triggerAlert(newAlert).catch(error => {
                console.error(`[AlertManager] Error triggering alert ${newAlert.id}:`, error);
            });
        }
    }

    evaluateCompoundThresholdRule(rule, guest, metrics, alertKey, timestamp) {
        const guestMetrics = metrics.find(m => 
            m.endpointId === guest.endpointId &&
            m.node === guest.node &&
            m.id === guest.vmid
        );
        if (!guestMetrics || !guestMetrics.current) return;

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
                    this.triggerAlert(existingAlert).catch(error => {
                        console.error(`[AlertManager] Error triggering alert ${existingAlert.id}:`, error);
                    });
                }
                existingAlert.lastUpdate = timestamp;
                existingAlert.currentValue = this.getCurrentThresholdValues(rule.thresholds, guestMetrics.current, guest);
            } else if (existingAlert.state === 'active' && !existingAlert.acknowledged) {
                existingAlert.lastUpdate = timestamp;
                existingAlert.currentValue = this.getCurrentThresholdValues(rule.thresholds, guestMetrics.current, guest);
            }
        } else {
            if (existingAlert && existingAlert.state === 'active' && !existingAlert.acknowledged) {
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
                metricValue = this.calculateIORate(currentMetrics, 'diskread', guest);
                break;
            case 'diskwrite':
                metricValue = this.calculateIORate(currentMetrics, 'diskwrite', guest);
                break;
            case 'netin':
                metricValue = this.calculateIORate(currentMetrics, 'netin', guest);
                break;
            case 'netout':
                metricValue = this.calculateIORate(currentMetrics, 'netout', guest);
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
            case 'diskread': 
                return this.calculateIORate(currentMetrics, 'diskread', guest);
            case 'diskwrite': return this.calculateIORate(currentMetrics, 'diskwrite', guest);
            case 'netin': return this.calculateIORate(currentMetrics, 'netin', guest);
            case 'netout': return this.calculateIORate(currentMetrics, 'netout', guest);
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
     * Evaluate per-guest threshold rules - handles individual guest thresholds and global fallbacks
     */
    async evaluatePerGuestThresholdRule(rule, guest, metrics, timestamp) {
        const triggeredAlerts = [];
        
        // Find metrics for this guest
        const guestMetrics = metrics.find(m => 
            m.endpointId === guest.endpointId &&
            m.node === guest.node &&
            m.id === guest.vmid
        );
        
        if (!guestMetrics || !guestMetrics.current) {
            console.log(`[AlertManager] No metrics found for guest ${guest.name} (${guest.endpointId}-${guest.node}-${guest.vmid})`);
            return triggeredAlerts;
        }
        
        console.log(`[AlertManager] Found metrics for guest ${guest.name}: CPU=${guestMetrics.current.cpu}%, Memory=${guestMetrics.current.mem}/${guestMetrics.current.maxmem}`);
        console.log(`[AlertManager] Guest ${guest.name} properties:`, Object.keys(guest));
        console.log(`[AlertManager] Metrics properties:`, Object.keys(guestMetrics.current));
        
        // Create a guest identifier for threshold lookup
        const guestId = `${guest.endpointId}-${guest.node}-${guest.vmid}`;
        
        const guestThresholds = rule.guestThresholds || {};
        const globalThresholds = rule.globalThresholds || {};
        const thresholdConfigForGuest = guestThresholds[guestId] || {};
        
        // Metrics to check for alerts
        const metricsToCheck = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
        
        // Collect all configured thresholds and their current values
        const configuredThresholds = {};
        const exceededThresholds = {};
        let hasConfiguredThresholds = false;
        
        // First pass: collect all configured thresholds and check if they're exceeded
        for (const metricType of metricsToCheck) {
            // Determine the effective threshold for this metric and guest
            let effectiveThreshold = null;
            
            // First priority: guest-specific threshold
            if (thresholdConfigForGuest[metricType] !== undefined && thresholdConfigForGuest[metricType] !== '') {
                effectiveThreshold = parseFloat(thresholdConfigForGuest[metricType]);
            } 
            // Second priority: global threshold
            else if (globalThresholds[metricType] !== undefined && globalThresholds[metricType] !== '') {
                effectiveThreshold = parseFloat(globalThresholds[metricType]);
            }
            
            if (effectiveThreshold === null || isNaN(effectiveThreshold)) {
                continue;
            }
            
            // Accept 0 as a valid threshold, only skip negative values
            if (effectiveThreshold < 0) {
                continue;
            }
            
            hasConfiguredThresholds = true;
            configuredThresholds[metricType] = effectiveThreshold;
            
            // Get current metric value
            const currentValue = this.getMetricValue(guestMetrics.current, metricType, guest);
            if (currentValue === null) {
                continue;
            }
            
            
            const condition = rule.condition || 'greater_than';
            const isThresholdExceeded = this.evaluateCondition(currentValue, condition, effectiveThreshold);
            
            if (isThresholdExceeded) {
                exceededThresholds[metricType] = {
                    currentValue: currentValue,
                    threshold: effectiveThreshold,
                    condition: condition
                };
            }
        }
        
        // If no thresholds are configured, return early
        if (!hasConfiguredThresholds) {
            console.log(`[AlertManager] No valid thresholds configured for guest ${guest.name}`);
            return triggeredAlerts;
        }
        
        const alertLogic = (rule.alertLogic || 'and').toLowerCase();
        
        // Check if alert conditions are met based on logic mode
        const configuredMetrics = Object.keys(configuredThresholds);
        const exceededMetrics = Object.keys(exceededThresholds);
        
        let alertConditionMet = false;
        let alertDescription = '';
        
        if (alertLogic === 'or') {
            // OR logic: Any configured threshold exceeded triggers alert
            alertConditionMet = exceededMetrics.length > 0;
            if (alertConditionMet) {
                // Create detailed description of exceeded thresholds
                const exceededDetails = exceededMetrics.map(metric => {
                    const data = exceededThresholds[metric];
                    const currentFormatted = this.formatMetricValue(data.currentValue, metric);
                    const thresholdFormatted = this.formatMetricValue(data.threshold, metric);
                    const metricName = this.getReadableMetricName(metric);
                    return `${metricName}: ${currentFormatted} (≥${thresholdFormatted})`;
                }).join(', ');
                alertDescription = exceededDetails;
            } else {
                alertDescription = `No Thresholds Exceeded (${exceededMetrics.length}/${configuredMetrics.length})`;
            }
        } else {
            // AND logic: All configured thresholds must be exceeded
            alertConditionMet = configuredMetrics.every(metric => exceededMetrics.includes(metric));
            if (alertConditionMet) {
                // Create detailed description of all exceeded thresholds
                const exceededDetails = exceededMetrics.map(metric => {
                    const data = exceededThresholds[metric];
                    const currentFormatted = this.formatMetricValue(data.currentValue, metric);
                    const thresholdFormatted = this.formatMetricValue(data.threshold, metric);
                    const metricName = this.getReadableMetricName(metric);
                    return `${metricName}: ${currentFormatted} (≥${thresholdFormatted})`;
                }).join(', ');
                alertDescription = exceededDetails;
            } else {
                alertDescription = `Not All Thresholds Exceeded (${exceededMetrics.length}/${configuredMetrics.length})`;
            }
        }
        
        console.log(`[AlertManager] Guest ${guest.name}: Logic=${alertLogic.toUpperCase()}, Configured: [${configuredMetrics.join(', ')}], Exceeded: [${exceededMetrics.join(', ')}], Condition met: ${alertConditionMet}`);
        
        const alertKey = `${rule.id}_${guest.endpointId}_${guest.node}_${guest.vmid}`;
        const existingAlert = this.activeAlerts.get(alertKey);
        
        if (alertConditionMet) {
            // Alert conditions are met - create or update single alert
            if (!existingAlert) {
                // Create new consolidated alert
                const newAlert = {
                    id: this.generateAlertId(),
                    rule: this.createSafeRuleCopy({
                        ...rule,
                        metric: 'combined_thresholds',
                        name: alertDescription
                    }),
                    guest: this.createSafeGuestCopy(guest),
                    startTime: timestamp,
                    lastUpdate: timestamp,
                    state: 'pending',
                    acknowledged: false,
                    exceededThresholds: exceededThresholds, // Include all exceeded threshold data
                    configuredThresholds: configuredThresholds, // Include all configured thresholds
                    alertLogic: alertLogic // Include logic mode for reference
                };
                this.activeAlerts.set(alertKey, newAlert);
                console.log(`[AlertManager] Created new ${alertLogic.toUpperCase()} alert for guest ${guest.name} with ${exceededMetrics.length} exceeded thresholds`);
            } else if (existingAlert.state === 'pending') {
                // Check if duration threshold is met
                const duration = timestamp - existingAlert.startTime;
                if (duration >= rule.duration) {
                    // Trigger the consolidated alert
                    existingAlert.state = 'active';
                    existingAlert.triggeredAt = timestamp;
                    existingAlert.exceededThresholds = exceededThresholds; // Update with current exceeded thresholds
                    existingAlert.alertLogic = alertLogic; // Update logic mode
                    this.triggerAlert(existingAlert).catch(error => {
                        console.error(`[AlertManager] Error triggering combined per-guest alert ${existingAlert.id}:`, error);
                    });
                    triggeredAlerts.push(existingAlert);
                    console.log(`[AlertManager] Triggered ${alertLogic.toUpperCase()} alert for guest ${guest.name} after ${duration}ms duration`);
                }
                existingAlert.lastUpdate = timestamp;
                existingAlert.exceededThresholds = exceededThresholds; // Always update with current data
                existingAlert.alertLogic = alertLogic; // Update logic mode
            } else if (existingAlert.state === 'active' && !existingAlert.acknowledged) {
                // Update existing active alert with current threshold data
                existingAlert.lastUpdate = timestamp;
                existingAlert.exceededThresholds = exceededThresholds;
                existingAlert.alertLogic = alertLogic; // Update logic mode
                console.log(`[AlertManager] Updated active ${alertLogic.toUpperCase()} alert for guest ${guest.name}`);
            }
        } else {
            // Alert conditions are not met - resolve any existing alert
            if (existingAlert && (existingAlert.state === 'active' || existingAlert.state === 'pending')) {
                existingAlert.state = 'resolved';
                existingAlert.resolvedAt = timestamp;
                existingAlert.resolveReason = alertDescription;
                if (existingAlert.rule.autoResolve !== false) {
                    this.resolveAlert(existingAlert);
                }
                console.log(`[AlertManager] Resolved ${alertLogic.toUpperCase()} alert for guest ${guest.name} - ${alertDescription.toLowerCase()}`);
            }
        }
        
        return triggeredAlerts;
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

    // Deprecated: acknowledgements are now loaded from active alerts
    async loadAcknowledgements() {
        // This method is kept for backward compatibility but does nothing
    }
    
    // Deprecated: acknowledgements are now saved with active alerts
    async saveAcknowledgements() {
        // This method is kept for backward compatibility but does nothing
    }
    
    // Old saveAcknowledgements code removed - acknowledgements are now part of active alerts

    async loadAlertRules() {
        try {
            const data = await fs.readFile(this.alertRulesFile, 'utf-8');
            const savedRules = JSON.parse(data);
            
            // Clear all existing rules
            this.alertRules.clear();
            
            // Load all rules from JSON
            for (const [key, rule] of Object.entries(savedRules)) {
                this.alertRules.set(key, rule);
                
                // Check if this is a per-guest thresholds rule with cooldown settings
                if (rule.type === 'per_guest_thresholds') {
                    if (rule.emailCooldowns) {
                        this.perGuestCooldownConfig = rule.emailCooldowns;
                    }
                    if (rule.webhookCooldowns) {
                        // Store webhook cooldowns in the perGuestCooldownConfig object
                        if (!this.perGuestCooldownConfig) {
                            this.perGuestCooldownConfig = {};
                        }
                        this.perGuestCooldownConfig.webhook = rule.webhookCooldowns;
                        console.log(`[AlertManager] Loaded webhook cooldown settings from per-guest thresholds rule:`, this.perGuestCooldownConfig.webhook);
                    }
                }
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
                // Node alerts have nodeId instead of guest
                if (alert.id && alert.rule && (alert.guest || alert.nodeId)) {
                    this.activeAlerts.set(key, alert);
                    
                    // If this alert is acknowledged, also add it to acknowledgedAlerts
                    if (alert.acknowledged) {
                        this.acknowledgedAlerts.set(key, alert);
                    }
                }
            });
            
            console.log(`[AlertManager] Loaded ${this.activeAlerts.size} active alerts from disk`);
            console.log(`[AlertManager] Loaded ${this.acknowledgedAlerts.size} acknowledged alerts from active alerts`);
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
                // Handle both guest alerts and node alerts
                if (alert.type === 'node_threshold') {
                    // Node alert
                    alertsToSave[key] = {
                        id: alert.id,
                        type: alert.type,
                        rule: this.createSafeRuleCopy(alert.rule),
                        nodeId: alert.nodeId,
                        nodeName: alert.nodeName,
                        metric: alert.metric,
                        threshold: alert.threshold,
                        currentValue: alert.currentValue,
                        triggeredAt: alert.triggeredAt,
                        state: alert.state,
                        severity: alert.severity,
                        message: alert.message,
                        notificationChannels: alert.notificationChannels
                    };
                } else {
                    // Guest alert
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
                        acknowledged: alert.acknowledged,
                        acknowledgedBy: alert.acknowledgedBy,
                        acknowledgedAt: alert.acknowledgedAt,
                        acknowledgeNote: alert.acknowledgeNote,
                        emailSent: alert.emailSent,
                        webhookSent: alert.webhookSent,
                        incidentType: alert.incidentType
                    };
                }
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
            
            // Initialize the new email service
            const emailConfig = {
                emailProvider: config.emailProvider || process.env.EMAIL_PROVIDER || 'smtp',
                sendgridApiKey: config.sendgridApiKey || process.env.SENDGRID_API_KEY,
                sendgridFromEmail: config.sendgridFromEmail || process.env.SENDGRID_FROM_EMAIL,
                alertToEmail: config.to || process.env.ALERT_TO_EMAIL || process.env.SENDGRID_TO_EMAIL,
                smtpHost: config.host || process.env.SMTP_HOST,
                smtpPort: config.port || process.env.SMTP_PORT,
                smtpSecure: config.secure || process.env.SMTP_SECURE === 'true',
                smtpUser: config.user || process.env.SMTP_USER,
                smtpPass: config.pass || process.env.SMTP_PASS,
                alertFromEmail: config.from || process.env.ALERT_FROM_EMAIL
            };
            
            await this.emailService.initialize(emailConfig);
            
            // Store email config for use in notifications
            this.emailConfig = this.emailService.getConfig();
            
            // For backward compatibility, create emailTransporter property that delegates to emailService
            if (this.emailService.getProvider()) {
                Object.defineProperty(this, 'emailTransporter', {
                    value: {
                        sendMail: async (options) => {
                            await this.emailService.sendMail(options);
                        },
                        close: () => {
                            this.emailService.close();
                        }
                    },
                    writable: true,
                    enumerable: false,
                    configurable: true
                });
                
            } else {
            }
        } catch (error) {
            console.error('[AlertManager] Failed to initialize email service:', error);
        }
    }

    /**
     * Reload email configuration (call this when settings change)
     */
    async reloadEmailConfiguration() {
        
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
    /**
     * Generate unified email HTML template
     * @param {Object} options - Email template options
     * @param {string} options.type - Type of email: 'alert', 'test', or 'test-alert'
     * @param {Object} options.data - Data for the email template
     * @returns {string} HTML email content
     */
    generateEmailTemplate(options) {
        const { type, data } = options;
        const timestamp = new Date();
        
        // Common header for all emails
        const header = `
            <div style="background: linear-gradient(135deg, #1f2937, #111827); color: white; padding: 24px; border-bottom: 3px solid #2563eb;">
                <div style="margin-bottom: 12px;">
                    <h1 style="margin: 0; font-size: 20px; font-weight: 600; letter-spacing: -0.025em;">Pulse</h1>
                    <div style="font-size: 12px; opacity: 0.7; margin-top: 2px;">Alert Notification System</div>
                </div>
                <div style="background: rgba(59, 130, 246, 0.1); border: 1px solid rgba(59, 130, 246, 0.2); border-radius: 6px; padding: 12px;">
                    <h2 style="margin: 0; font-size: 16px; font-weight: 500;">${data.title}</h2>
                    <div style="font-size: 13px; opacity: 0.8; margin-top: 4px;">${data.subtitle}</div>
                </div>
            </div>
        `;
        
        // Common footer for all emails
        const footer = `
            <div style="background: linear-gradient(135deg, #f8fafc, #f1f5f9); padding: 16px; border-top: 1px solid #e5e7eb;">
                <div style="display: flex; justify-content: space-between; align-items: center; font-size: 12px; color: #6b7280;">
                    <div>
                        <div style="color: #374151; font-weight: 600;">Pulse Monitoring System</div>
                        <div style="margin-top: 1px;">${data.fromEmail} → ${data.toEmail}</div>
                    </div>
                    <div style="text-align: right; font-family: monospace; font-size: 11px;">
                        <div style="color: #2563eb; font-weight: 500;">SMTP: ${data.smtpHost}:${data.smtpPort}</div>
                        <div style="margin-top: 2px; color: #94a3b8;">${timestamp.toISOString().split('T')[0]} • ${timestamp.toISOString().split('T')[1].split('.')[0]}</div>
                    </div>
                </div>
            </div>
        `;
        
        let content = '';
        
        if (type === 'alert') {
            // Real alert content
            const alert = data.alert;
            const statusColor = alert.type === 'triggered' ? '#dc2626' : '#10b981';
            const statusIcon = alert.type === 'triggered' ? '🚨' : '✅';
            const statusText = alert.type === 'triggered' ? 'Alert Triggered' : 'Alert Resolved';
            
            content = `
                <!-- Alert Status Banner -->
                <div style="background: ${alert.type === 'triggered' ? '#fef2f2' : '#f0fdf4'}; border-left: 4px solid ${statusColor}; padding: 12px 16px; margin-bottom: 24px;">
                    <p style="margin: 0; color: #374151; font-size: 14px;">
                        <strong>${statusIcon} ${statusText}:</strong> ${alert.description || 'Monitoring threshold exceeded'}
                    </p>
                </div>
                
                <!-- System and Timestamp Grid -->
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 24px;">
                    <div>
                        <h3 style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Target System</h3>
                        <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 12px;">
                            <div style="font-weight: 600; color: #111827; margin-bottom: 4px;">${alert.guestName}</div>
                            <div style="font-size: 13px; color: #6b7280;">${alert.guestType.toUpperCase()} ${alert.guestId} • ${alert.node}</div>
                        </div>
                    </div>
                    <div>
                        <h3 style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Timestamp</h3>
                        <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 12px;">
                            <div style="font-weight: 600; color: #111827; margin-bottom: 4px;">${timestamp.toLocaleDateString()}</div>
                            <div style="font-size: 13px; color: #6b7280;">${timestamp.toLocaleTimeString()}</div>
                        </div>
                    </div>
                </div>
                
                <!-- Metrics Details -->
                <div style="margin-bottom: 24px;">
                    <h3 style="margin: 0 0 12px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Alert Details</h3>
                    <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px;">
                        ${alert.metrics && alert.metrics.length > 0 ? 
                            // For bundled alerts with multiple metrics
                            alert.metrics.map((m, index) => `
                                <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0;${index < alert.metrics.length - 1 ? ' border-bottom: 1px solid #e5e7eb;' : ''}">
                                    <span style="font-weight: 500; color: #374151; min-width: 100px;">${m.name}</span>
                                    <div style="text-align: right;">
                                        <span style="font-family: monospace; color: ${alert.type === 'triggered' ? '#dc2626' : '#10b981'}; font-weight: 600; margin-right: 8px;">${m.value}</span>
                                        <span style="font-family: monospace; color: #6b7280; font-size: 13px;">(threshold: ${m.threshold})</span>
                                    </div>
                                </div>
                            `).join('') :
                            // For single metric alerts
                            `<div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #e5e7eb;">
                                <span style="font-weight: 500; color: #374151;">Metric</span>
                                <span style="font-family: monospace; color: #6b7280;">${alert.metric}</span>
                            </div>
                            <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #e5e7eb;">
                                <span style="font-weight: 500; color: #374151;">Current Value</span>
                                <span style="font-family: monospace; color: ${alert.type === 'triggered' ? '#dc2626' : '#10b981'}; font-weight: 600;">${alert.currentValue}</span>
                            </div>
                            <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0;">
                                <span style="font-weight: 500; color: #374151;">Threshold</span>
                                <span style="font-family: monospace; color: #6b7280;">${alert.threshold}</span>
                            </div>`
                        }
                    </div>
                </div>
                
                <!-- Additional Info -->
                ${alert.additionalInfo ? `
                <div style="background: #eff6ff; border: 1px solid #dbeafe; border-radius: 6px; padding: 16px; margin-bottom: 24px;">
                    <p style="margin: 0; color: #1e40af; font-size: 14px;">${alert.additionalInfo}</p>
                </div>
                ` : ''}
            `;
        } else if (type === 'test') {
            // Simple test email content
            content = `
                <!-- Test Notice -->
                <div style="background: #f0fdf4; border-left: 4px solid #10b981; padding: 12px 16px; margin-bottom: 24px;">
                    <p style="margin: 0; color: #374151; font-size: 14px;">
                        <strong>✅ Test Successful:</strong> Email configuration is working correctly
                    </p>
                </div>
                
                <!-- Configuration Details -->
                <div style="margin-bottom: 24px;">
                    <h3 style="margin: 0 0 12px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Configuration Details</h3>
                    <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px;">
                        <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #e5e7eb;">
                            <span style="font-weight: 500; color: #374151;">SMTP Host</span>
                            <span style="font-family: monospace; color: #6b7280;">${data.smtpHost}</span>
                        </div>
                        <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #e5e7eb;">
                            <span style="font-weight: 500; color: #374151;">SMTP Port</span>
                            <span style="font-family: monospace; color: #6b7280;">${data.smtpPort}</span>
                        </div>
                        <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #e5e7eb;">
                            <span style="font-weight: 500; color: #374151;">From Address</span>
                            <span style="font-family: monospace; color: #6b7280;">${data.fromEmail}</span>
                        </div>
                        <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid #e5e7eb;">
                            <span style="font-weight: 500; color: #374151;">To Address</span>
                            <span style="font-family: monospace; color: #6b7280;">${data.toEmail}</span>
                        </div>
                        ${data.smtpSecure !== undefined ? `
                        <div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0;">
                            <span style="font-weight: 500; color: #374151;">Security</span>
                            <span style="font-family: monospace; color: #6b7280;">${data.smtpSecure ? 'SSL/TLS' : 'STARTTLS'}</span>
                        </div>
                        ` : ''}
                    </div>
                </div>
                
                <!-- Success Message -->
                <div style="background: linear-gradient(135deg, #10b981, #059669); border-radius: 6px; padding: 16px; color: white;">
                    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px;">
                        <div style="width: 6px; height: 6px; background: #34d399; border-radius: 50%;"></div>
                        <span style="font-weight: 600;">Email System Verified</span>
                    </div>
                    <p style="margin: 0; opacity: 0.9; font-size: 14px;">
                        Your email notifications are configured correctly. You will receive alerts at this address when monitoring thresholds are exceeded.
                    </p>
                </div>
            `;
        } else if (type === 'test-alert') {
            // Test alert email content
            const testAlert = data.testAlert;
            content = `
                <!-- Test Notice -->
                <div style="background: #f8fafc; border-left: 4px solid #f59e0b; padding: 12px 16px; margin-bottom: 24px;">
                    <p style="margin: 0; color: #374151; font-size: 14px;">
                        <strong>Test Notification:</strong> Verifying alert configuration for ${testAlert.description || 'monitoring rule'}
                    </p>
                </div>
                
                <!-- Alert Overview -->
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 24px;">
                    <div>
                        <h3 style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Target System</h3>
                        <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 12px;">
                            <div style="font-weight: 600; color: #111827; margin-bottom: 4px;">${testAlert.guestName}</div>
                            <div style="font-size: 13px; color: #6b7280;">${testAlert.guestType.toUpperCase()} ${testAlert.guestId} • ${testAlert.node}</div>
                        </div>
                    </div>
                    <div>
                        <h3 style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Timestamp</h3>
                        <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 12px;">
                            <div style="font-weight: 600; color: #111827; margin-bottom: 4px;">${timestamp.toLocaleDateString()}</div>
                            <div style="font-size: 13px; color: #6b7280;">${timestamp.toLocaleTimeString()}</div>
                        </div>
                    </div>
                </div>
                
                <!-- Thresholds -->
                <div style="margin-bottom: 24px;">
                    <h3 style="margin: 0 0 12px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Alert Thresholds</h3>
                    <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px;">
                        ${testAlert.thresholds && testAlert.thresholds.length > 0 ? 
                            testAlert.thresholds.map((t, index) => 
                                `<div style="display: flex; justify-content: space-between; align-items: center; padding: 8px 0;${index < testAlert.thresholds.length - 1 ? ' border-bottom: 1px solid #e5e7eb;' : ''}">
                                    <span style="font-weight: 500; color: #374151;">${(t.metric || t.type || 'unknown').charAt(0).toUpperCase() + (t.metric || t.type || 'unknown').slice(1)}</span>
                                    <span style="font-family: monospace; color: #6b7280;">≥ ${t.threshold || t.value}%</span>
                                </div>`
                            ).join('') :
                            '<div style="color: #6b7280; text-align: center; padding: 16px;">No thresholds configured</div>'
                        }
                    </div>
                </div>
                
                <!-- Status -->
                <div style="background: linear-gradient(135deg, #10b981, #059669); border-radius: 6px; padding: 16px; color: white;">
                    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px;">
                        <div style="width: 6px; height: 6px; background: #34d399; border-radius: 50%;"></div>
                        <span style="font-weight: 600;">Email Configuration Verified</span>
                    </div>
                    <p style="margin: 0; opacity: 0.9; font-size: 14px;">
                        Alert notifications are working correctly. Real alerts will be delivered using this configuration when thresholds are exceeded.
                    </p>
                </div>
            `;
        } else if (type === 'summary') {
            // Summary alert content for multiple alerts
            const { alertsByType, alertsByNode, totalCount } = data;
            
            content = `
                <!-- Summary Banner -->
                <div style="background: #fef3c7; border-left: 4px solid #f59e0b; padding: 12px 16px; margin-bottom: 24px;">
                    <p style="margin: 0; color: #374151; font-size: 14px;">
                        <strong>🚨 Multiple Alerts:</strong> ${totalCount} alerts triggered simultaneously
                    </p>
                </div>
                
                <!-- Alerts by Type -->
                <div style="margin-bottom: 24px;">
                    <h3 style="margin: 0 0 12px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Alerts by Metric</h3>
                    <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px;">
                        ${Object.entries(alertsByType).map(([metric, items]) => `
                            <div style="margin-bottom: 16px;">
                                <div style="font-weight: 600; color: #111827; margin-bottom: 8px; text-transform: uppercase;">
                                    ${metric} (${items.length} alert${items.length > 1 ? 's' : ''})
                                </div>
                                <div style="border-left: 3px solid #e5e7eb; padding-left: 12px;">
                                    ${items.map(item => {
                                        const value = typeof item.value === 'number' ? Math.round(item.value) : item.value;
                                        const threshold = typeof item.threshold === 'number' ? Math.round(item.threshold) : item.threshold;
                                        return `
                                            <div style="font-size: 13px; padding: 4px 0; color: #6b7280;">
                                                <span style="font-weight: 500; color: #374151;">${item.guest}:</span>
                                                <span style="font-family: monospace;">${value}%</span>
                                                <span style="color: #9ca3af;">(threshold: ${threshold}%)</span>
                                            </div>
                                        `;
                                    }).join('')}
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
                
                <!-- Alerts by Node -->
                <div style="margin-bottom: 24px;">
                    <h3 style="margin: 0 0 12px 0; font-size: 14px; font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Distribution by Node</h3>
                    <div style="background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px;">
                        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px;">
                            ${Object.entries(alertsByNode).map(([node, count]) => `
                                <div style="background: white; border: 1px solid #e5e7eb; border-radius: 4px; padding: 8px 12px; text-align: center;">
                                    <div style="font-weight: 600; color: #111827; font-size: 20px;">${count}</div>
                                    <div style="font-size: 12px; color: #6b7280; margin-top: 2px;">${node}</div>
                                </div>
                            `).join('')}
                        </div>
                    </div>
                </div>
                
                <!-- Action Required -->
                <div style="background: linear-gradient(135deg, #f59e0b, #d97706); border-radius: 6px; padding: 16px; color: white;">
                    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px;">
                        <div style="width: 6px; height: 6px; background: #fbbf24; border-radius: 50%;"></div>
                        <span style="font-weight: 600;">Action Required</span>
                    </div>
                    <p style="margin: 0; opacity: 0.9; font-size: 14px;">
                        Multiple systems are experiencing issues. Please review the alerts in your Pulse dashboard for detailed information and take appropriate action.
                    </p>
                </div>
            `;
        }
        
        // Combine all parts
        return `
            <div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; max-width: 600px; margin: 0 auto; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; background: #ffffff;">
                ${header}
                <div style="padding: 24px;">
                    ${content}
                </div>
                ${footer}
            </div>
        `;
    }

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
        const effectiveThreshold = alert.effectiveThreshold || alert.threshold || alert.rule.threshold;
        
        // Format values for display - handle both single values and compound objects
        let valueDisplay, thresholdDisplay, metricDisplay;
        let metricsArray = null; // For bundled alerts
        
        console.log(`[AlertManager] Alert object keys:`, Object.keys(alert));
        console.log(`[AlertManager] Checking conditions - metric === 'bundled': ${alert.metric === 'bundled'}, exceededMetrics truthy: ${!!alert.exceededMetrics}`);
        
        if (alert.metric === 'bundled' && alert.exceededMetrics) {
            console.log(`[AlertManager] Exceeded metrics:`, JSON.stringify(alert.exceededMetrics));
            metricsArray = alert.exceededMetrics.map(m => {
                const isPercentage = ['cpu', 'memory', 'disk'].includes(m.metricType);
                const formattedValue = typeof m.currentValue === 'number' ? Math.round(m.currentValue * 10) / 10 : m.currentValue;
                const formattedThreshold = typeof m.threshold === 'number' ? Math.round(m.threshold * 10) / 10 : m.threshold;
                return {
                    name: m.metricType.toUpperCase(),
                    value: `${formattedValue}${isPercentage ? '%' : ''}`,
                    threshold: `${formattedThreshold}${isPercentage ? '%' : ''}`
                };
            });
            
            metricDisplay = metricsArray.map(m => m.name).join(', ');
            valueDisplay = metricsArray.map(m => m.value).join(', ');
            thresholdDisplay = metricsArray.map(m => `>${m.threshold}`).join(', ');
            console.log(`[AlertManager] Bundled alert formatted - metricDisplay: ${metricDisplay}, valueDisplay: ${valueDisplay}, thresholdDisplay: ${thresholdDisplay}`);
        } else if (alert.rule.type === 'compound_threshold' && typeof currentValue === 'object' && currentValue !== null) {
            // Format compound threshold values
            const values = [];
            for (const [metric, value] of Object.entries(currentValue)) {
                const isPercentage = ['cpu', 'memory', 'disk'].includes(metric);
                const formattedValue = typeof value === 'number' ? Math.round(value * 10) / 10 : value;
                values.push(`${metric.toUpperCase()}: ${formattedValue}${isPercentage ? '%' : ''}`);
            }
            valueDisplay = values.join(', ');
            metricDisplay = 'Multiple Thresholds';
            
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
            // Use alert.metric if available (for node alerts), otherwise fall back to alert.rule.metric
            const metric = alert.metric || alert.rule.metric;
            const isPercentageMetric = ['cpu', 'memory', 'disk'].includes(metric);
            valueDisplay = isPercentageMetric ? `${Math.round(currentValue || 0)}%` : (currentValue || 'N/A');
            thresholdDisplay = isPercentageMetric ? `${effectiveThreshold || 0}%` : (effectiveThreshold || 'N/A');
            metricDisplay = metric ? metric.toUpperCase() : 'N/A';
        }

        // Get email configuration
        const config = await this.loadEmailConfig();
        const fromEmail = this.emailConfig?.from || process.env.ALERT_FROM_EMAIL || 'alerts@pulse-monitoring.local';
        const toEmailAddresses = recipients.join(', ');
        const smtpHost = config.host || process.env.SMTP_HOST || 'localhost';
        const smtpPort = config.port || process.env.SMTP_PORT || '587';
        
        const subject = `🚨 Pulse Alert: ${alert.rule.name}`;
        
        
        // Handle node vs guest alert data
        let alertData;
        if (alert.type === 'node_threshold') {
            alertData = {
                type: 'triggered',
                description: alert.rule.description,
                guestName: alert.nodeName || alert.nodeId || 'Unknown Node',
                guestType: 'NODE',
                guestId: 'N/A',
                node: alert.nodeId,
                metric: metricDisplay,
                currentValue: valueDisplay,
                threshold: thresholdDisplay,
                metrics: null, // Individual alerts only
                status: 'active',
                timestamp: this.getValidTimestamp(alert)
            };
        } else {
            alertData = {
                type: 'triggered',
                description: alert.rule.description,
                guestName: alert.guest.name,
                guestType: alert.guest.type,
                guestId: alert.guest.vmid,
                node: alert.guest.node,
                metric: metricDisplay,
                currentValue: valueDisplay,
                threshold: thresholdDisplay,
                metrics: null, // Individual alerts only
                status: alert.guest.status,
                timestamp: this.getValidTimestamp(alert)
            };
        }
        
        // Use the unified template
        const html = this.generateEmailTemplate({
            type: 'alert',
            data: {
                title: alert.rule.name,
                subtitle: 'Alert notification',
                fromEmail: fromEmail,
                toEmail: toEmailAddresses,
                smtpHost: smtpHost,
                smtpPort: smtpPort,
                alert: alertData
            }
        });

        let text;
        if (alert.type === 'node_threshold') {
            text = `
PULSE ALERT: ${alert.rule.name}

Alert Details
Node:          ${alert.nodeName || alert.nodeId || 'Unknown Node'}
Type:          NODE
Metric:        ${metricDisplay}
Current Value: ${valueDisplay}
Threshold:     ${thresholdDisplay}
Time:          ${new Date(this.getValidTimestamp(alert)).toLocaleString()}

${alert.rule.description || 'Alert triggered for the specified conditions'}

---
This alert was generated by Pulse monitoring system.
        `;
        } else {
            text = `
PULSE ALERT: ${alert.rule.name}

Alert Details
VM/LXC:        ${alert.guest.name} (${alert.guest.type} ${alert.guest.vmid})
Node:          ${alert.guest.node}
Metric:        ${metricDisplay}
Current Value: ${valueDisplay}
Threshold:     ${thresholdDisplay}
Status:        ${alert.guest.status}
Time:          ${new Date(this.getValidTimestamp(alert)).toLocaleString()}

${alert.rule.description || 'Alert triggered for the specified conditions'}

---
This alert was generated by Pulse monitoring system.
        `;
        }

        const mailOptions = {
            from: this.emailConfig?.from || process.env.ALERT_FROM_EMAIL || 'alerts@pulse-monitoring.local',
            to: recipients.join(', '),
            subject: subject,
            text: text,
            html: html
        };

        await this.emailTransporter.sendMail(mailOptions);
    }
    
    cleanup() {
        if (this.cleanupInterval) {
            clearInterval(this.cleanupInterval);
        }
        
        // Stop debounce handler
        if (this.debounceHandler) {
            this.debounceHandler.stop();
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

    /**
     * Queue email notification for batching
     */
    queueEmailNotification(alert) {
        this.emailQueue.push(alert);
        
        // Start or reset batch timer
        if (this.emailBatchTimeout) {
            clearTimeout(this.emailBatchTimeout);
        }
        
        this.emailBatchTimeout = setTimeout(() => {
            this.sendBatchedEmails();
        }, this.emailBatchWindowMs);
    }
    
    /**
     * Send batched email notifications
     */
    async sendBatchedEmails() {
        if (this.emailQueue.length === 0) return;
        
        
        try {
            if (this.emailQueue.length === 1) {
                // Single alert - send normally
                await this.sendDirectEmailNotification(this.emailQueue[0]);
            } else {
                // Multiple alerts - send summary email
                await this.sendSummaryEmail(this.emailQueue);
            }
            
            // Clear queue
            this.emailQueue = [];
            this.emailBatchTimeout = null;
        } catch (error) {
            console.error('[AlertManager] Error sending batched emails:', error);
            // Clear queue even on error to prevent infinite retries
            this.emailQueue = [];
            this.emailBatchTimeout = null;
        }
    }
    
    /**
     * Send summary email for multiple alerts
     */
    async sendSummaryEmail(alerts) {
        if (!this.emailTransporter) {
            throw new Error('Email transporter not configured');
        }
        
        const toEmail = this.emailConfig?.to || process.env.ALERT_TO_EMAIL;
        const recipients = toEmail ? toEmail.split(',') : [];
        if (!recipients || recipients.length === 0) {
            throw new Error('No email recipients configured');
        }
        
        // Group alerts by type
        const alertsByType = {};
        const alertsByNode = {};
        
        alerts.forEach(alert => {
            // Group by metric type
            if (alert.metric === 'bundled' && alert.exceededMetrics) {
                alert.exceededMetrics.forEach(m => {
                    const metric = m.metricType || 'unknown';
                    if (!alertsByType[metric]) alertsByType[metric] = [];
                    alertsByType[metric].push({
                        guest: alert.guest.name,
                        value: m.currentValue,
                        threshold: m.threshold
                    });
                });
            } else {
                const metric = alert.metric || alert.rule?.metric || 'unknown';
                if (!alertsByType[metric]) alertsByType[metric] = [];
                alertsByType[metric].push({
                    guest: alert.guest?.name || 'Unknown',
                    value: alert.currentValue || alert.value,
                    threshold: alert.threshold || alert.rule?.threshold
                });
            }
            
            // Group by node
            const node = alert.guest?.node || 'unknown';
            if (!alertsByNode[node]) alertsByNode[node] = 0;
            alertsByNode[node]++;
        });
        
        // Create summary text
        let summaryText = `PULSE ALERT SUMMARY: ${alerts.length} alerts triggered\n\n`;
        
        Object.entries(alertsByType).forEach(([metric, items]) => {
            summaryText += `${metric.toUpperCase()} ALERTS (${items.length}):\n`;
            items.forEach(item => {
                const value = typeof item.value === 'number' ? Math.round(item.value) : item.value;
                const threshold = typeof item.threshold === 'number' ? Math.round(item.threshold) : item.threshold;
                summaryText += `  - ${item.guest}: ${value}% (threshold: ${threshold}%)\n`;
            });
            summaryText += '\n';
        });
        
        summaryText += 'BY NODE:\n';
        Object.entries(alertsByNode).forEach(([node, count]) => {
            summaryText += `  - ${node}: ${count} alert${count > 1 ? 's' : ''}\n`;
        });
        
        // Create HTML version with the email template
        const html = this.generateEmailTemplate({
            type: 'summary',
            data: {
                title: `Alert Summary: ${alerts.length} alerts`,
                subtitle: 'Multiple alerts triggered',
                fromEmail: this.emailConfig?.from || process.env.ALERT_FROM_EMAIL,
                toEmail: recipients.join(', '),
                alerts: alerts,
                alertsByType: alertsByType,
                alertsByNode: alertsByNode,
                totalCount: alerts.length
            }
        });
        
        const mailOptions = {
            from: this.emailConfig?.from || process.env.ALERT_FROM_EMAIL || 'alerts@pulse-monitoring.local',
            to: recipients.join(', '),
            subject: `🚨 Pulse Alert Summary: ${alerts.length} alerts`,
            text: summaryText,
            html: html
        };
        
        await this.emailTransporter.sendMail(mailOptions);
    }

    async sendTestEmail(customConfig = null) {
        try {
            
            // If custom config is provided, use it to create a temporary transporter
            if (customConfig) {
                return await this.sendTestEmailWithCustomTransporter(customConfig);
            }
            
            // Otherwise use the existing transporter
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

    async sendTestEmailWithCustomTransporter(config) {
        try {
            let { emailProvider, sendgridApiKey, host, port, user, pass, from, to, secure } = config;
            
            // Validate based on provider
            if (emailProvider === 'sendgrid') {
                if (!sendgridApiKey || !from || !to) {
                    return {
                        success: false,
                        error: 'SendGrid API key and email addresses are required'
                    };
                }
            } else {
                // For SMTP, allow empty password (will use stored one)
                if (!host || !port || !user || !from || !to) {
                    return {
                        success: false,
                        error: 'All SMTP fields are required for testing (except password which can use stored value)'
                    };
                }
                // If password is empty, we'll use the stored one
                if (!pass) {
                    pass = process.env.SMTP_PASS;
                }
            }
            
            // Use the email service for testing
            const testConfig = {
                emailProvider: emailProvider || 'smtp',
                sendgridApiKey: sendgridApiKey,
                from: from,
                to: to,
                host: host,
                port: port,
                secure: secure === true,
                user: user,
                pass: pass
            };
            
            // Generate the proper test email HTML template
            const html = this.generateEmailTemplate({
                type: 'test',
                data: {
                    title: 'Test Email',
                    subtitle: 'Email configuration test',
                    fromEmail: from,
                    toEmail: to,
                    smtpHost: host,
                    smtpPort: port,
                    smtpSecure: secure,
                    provider: emailProvider,
                    configSource: 'custom'
                }
            });
            
            
            // Send using email service with the template
            if (emailProvider === 'sendgrid') {
                const testSgMail = require('@sendgrid/mail');
                testSgMail.setApiKey(sendgridApiKey);
                
                const response = await testSgMail.send({
                    to: to,
                    from: from,
                    subject: '🧪 Pulse Alert System - Test Email',
                    html: html,
                    text: 'This is a test email from Pulse monitoring system. Your email configuration is working correctly!'
                });
                
                return { success: true, provider: 'sendgrid' };
            } else {
                // For SMTP, send directly with the template
                const nodemailer = require('nodemailer');
                const portNum = parseInt(port) || 587;
                const isSecure = secure !== undefined ? secure === true : portNum === 465;
                
                const testTransporter = nodemailer.createTransport({
                    host: host,
                    port: portNum,
                    secure: isSecure,
                    auth: {
                        user: user,
                        pass: pass
                    },
                    tls: {
                        rejectUnauthorized: false
                    }
                });

                await testTransporter.sendMail({
                    from: from,
                    to: to,
                    subject: '🧪 Pulse Alert System - Test Email',
                    html: html,
                    text: 'This is a test email from Pulse monitoring system. Your email configuration is working correctly!'
                });

                testTransporter.close();
                return { success: true, provider: 'smtp' };
            }
            
        } catch (error) {
            console.error('[AlertManager] Error sending test email:', error);
            if (error.response && emailProvider === 'sendgrid') {
                console.error('[AlertManager] SendGrid error response:', error.response.body);
                return { success: false, error: `SendGrid error: ${JSON.stringify(error.response.body)}` };
            }
            return { success: false, error: error.message || 'Unknown error occurred' };
        }
    }

    async sendTestEmailWithConfig(config) {
        try {
            
            if (!this.emailTransporter) {
                return { success: false, error: 'Email transporter not configured' };
            }

            if (!config.ALERT_TO_EMAIL) {
                return { success: false, error: 'No recipient email address configured' };
            }

            // Use the unified template
            const html = this.generateEmailTemplate({
                type: 'test',
                data: {
                    title: 'Test Email',
                    subtitle: 'Email configuration test',
                    fromEmail: config.ALERT_FROM_EMAIL || 'noreply@pulse.local',
                    toEmail: config.ALERT_TO_EMAIL,
                    smtpHost: config.SMTP_HOST || 'localhost',
                    smtpPort: config.SMTP_PORT || '587'
                }
            });
            
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
                html: html
            };

            await this.emailTransporter.sendMail(testEmailOptions);
            return { success: true };
            
        } catch (error) {
            console.error('[AlertManager] Error sending test email:', error);
            return { success: false, error: error.message };
        }
    }

    async sendTestAlertEmail({ alertName, testAlert, config }) {
        try {
            
            if (!this.emailTransporter) {
                return { success: false, error: 'Email transporter not configured' };
            }

            if (!config.ALERT_TO_EMAIL) {
                return { success: false, error: 'No recipient email address configured' };
            }

            const testAlertEmailOptions = {
                from: config.ALERT_FROM_EMAIL || 'noreply@pulse.local',
                to: config.ALERT_TO_EMAIL,
                subject: `Pulse Alert Test: ${alertName}`,
                text: `TEST ALERT NOTIFICATION

Alert Rule: ${alertName}
Description: ${testAlert.rule.description}

This is a test alert to verify your notification configuration is working correctly.

Test Details:
- VM/Container: ${testAlert.guest.name} (${testAlert.guest.vmid})
- Node: ${testAlert.guest.node}
- Type: ${testAlert.guest.type.toUpperCase()}
- Triggered At: ${new Date().toLocaleString()}
- Reason: ${testAlert.details.reason}

Alert Configuration:
${testAlert.rule.thresholds && testAlert.rule.thresholds.length > 0 ? 
    testAlert.rule.thresholds.map(t => `- ${(t.metric || t.type || 'unknown')}: ≥ ${t.threshold || t.value}%`).join('\n') :
    '- No thresholds configured yet'
}
- Target: ${testAlert.rule.targetType === 'all' ? 'All VMs and LXCs' : 'Specific VMs/LXCs only'}

If this were a real alert, you would receive this email when your configured thresholds are exceeded.

Configuration used:
- SMTP Host: ${config.SMTP_HOST}
- SMTP Port: ${config.SMTP_PORT}
- From: ${config.ALERT_FROM_EMAIL}
- To: ${config.ALERT_TO_EMAIL}

✅ Email notifications are working correctly!

Best regards,
Pulse Monitoring System`,
                html: this.generateEmailTemplate({
                    type: 'test-alert',
                    data: {
                        title: alertName,
                        subtitle: 'Test notification • Configuration verification',
                        fromEmail: config.ALERT_FROM_EMAIL || 'noreply@pulse.local',
                        toEmail: config.ALERT_TO_EMAIL,
                        smtpHost: config.SMTP_HOST || 'localhost',
                        smtpPort: config.SMTP_PORT || '587',
                        testAlert: {
                            description: testAlert.rule.description,
                            guestName: testAlert.guest.name,
                            guestType: testAlert.guest.type,
                            guestId: testAlert.guest.vmid,
                            node: testAlert.guest.node,
                            thresholds: testAlert.rule.thresholds
                        }
                    }
                })
            };

            await this.emailTransporter.sendMail(testAlertEmailOptions);
            return { success: true };
            
        } catch (error) {
            console.error('[AlertManager] Error sending test alert email:', error);
            return { success: false, error: error.message };
        }
    }

    async sendDirectWebhookNotification(alert) {
        try {
            const webhookUrl = process.env.WEBHOOK_URL;
            if (!webhookUrl) {
                throw new Error('No webhook URL configured');
            }
            
            console.log(`[AlertManager] Sending direct webhook for alert ${alert.id} - ${alert.rule?.name}`);
            
            // Use the new notification service
            const NotificationService = require('./notificationServices');
            const notificationService = new NotificationService();
            
            const response = await notificationService.send(webhookUrl, alert);
            
            console.log(`[AlertManager] Webhook sent successfully for alert ${alert.id} (${response.status})`);
            return { success: true };
            
        } catch (error) {
            console.error(`[AlertManager] Failed to send webhook for alert ${alert.id}:`, error.message);
            throw error;
        }
    }

    async loadEmailConfig() {
        try {
            // Load email configuration from config API
            const axios = require('axios');
            
            const port = process.env.PORT || 7655;
            const response = await axios.get(`http://localhost:${port}/api/config`);
            const config = response.data;
            
            
            return {
                emailProvider: config.EMAIL_PROVIDER,
                sendgridApiKey: config.SENDGRID_API_KEY,
                sendgridFromEmail: config.SENDGRID_FROM_EMAIL,
                from: config.ALERT_FROM_EMAIL,
                to: config.ALERT_TO_EMAIL || config.SENDGRID_TO_EMAIL,
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

    // Add a test alert to the dashboard
    addTestAlert(testAlert) {
        try {
            const alertKey = `${testAlert.rule.id}_${testAlert.guest.endpointId}_${testAlert.guest.node}_${testAlert.guest.vmid}`;
            
            // Create a properly formatted alert object
            const formattedAlert = {
                ...testAlert,
                startTime: testAlert.triggeredAt,
                state: 'active',  // Test alerts are immediately active
                acknowledged: false,
                incidentType: 'test',
                // Add thresholds at the alert level for compatibility with the modal
                thresholds: testAlert.rule.thresholds || [],
                ruleName: testAlert.rule.name,
                // Include notification status flags
                emailSent: testAlert.emailSent || false,
                webhookSent: testAlert.webhookSent || false
            };
            
            this.activeAlerts.set(alertKey, formattedAlert);
            
            // Add to history
            const historyEntry = {
                id: testAlert.id,
                type: 'test_alert',
                ruleId: testAlert.rule.id,
                ruleName: testAlert.rule.name,
                guest: testAlert.guest,
                triggeredAt: testAlert.triggeredAt,
                message: testAlert.message,
                details: testAlert.details
            };
            
            this.alertHistory.unshift(historyEntry);
            if (this.alertHistory.length > this.maxHistorySize) {
                this.alertHistory = this.alertHistory.slice(0, this.maxHistorySize);
            }
            
            // Save state
            this.saveActiveAlerts();
            
            console.log(`[AlertManager] Test alert added: ${testAlert.rule.name} for ${testAlert.guest.name}`);
            return true;
        } catch (error) {
            console.error('[AlertManager] Error adding test alert:', error);
            throw error;
        }
    }

    /**
     * Format metric values with appropriate units
     */
    formatMetricValue(value, metricType) {
        if (metricType === 'cpu' || metricType === 'memory' || metricType === 'disk') {
            // Show decimal places for values < 1%
            if (value < 1) {
                return `${value.toFixed(1)}%`;
            }
            return `${Math.round(value)}%`;
        } else if (metricType === 'diskread' || metricType === 'diskwrite' || metricType === 'netin' || metricType === 'netout') {
            // Format as MB/s or similar
            if (value >= 1024 * 1024 * 1024) {
                return `${Math.round(value / (1024 * 1024 * 1024) * 100) / 100} GB/s`;
            } else if (value >= 1024 * 1024) {
                return `${Math.round(value / (1024 * 1024) * 100) / 100} MB/s`;
            } else if (value >= 1024) {
                return `${Math.round(value / 1024 * 100) / 100} KB/s`;
            } else {
                return `${Math.round(value * 100) / 100} B/s`;
            }
        }
        return value.toString();
    }

    /**
     * Get readable metric names
     */
    getReadableMetricName(metricType) {
        const names = {
            'cpu': 'CPU',
            'memory': 'Memory',
            'disk': 'Disk',
            'diskread': 'Disk Read',
            'diskwrite': 'Disk Write',
            'netin': 'Network In',
            'netout': 'Network Out'
        };
        return names[metricType] || metricType.toUpperCase();
    }

    /**
     * SIMPLIFIED ALERT SYSTEM - Direct threshold checking with guest-level bundling
     * Creates one alert per guest containing all exceeded metrics
     */
    async evaluateSimpleThresholds(allGuestMetrics) {
        if (this.processingMetrics) {
            console.log('[AlertManager] Already processing metrics, skipping...');
            return;
        }
        
        this.processingMetrics = true;
        
        try {
            const thresholdConfig = Array.from(this.alertRules.values()).find(rule => rule.type === 'per_guest_thresholds');
            if (!thresholdConfig) {
                console.log('[AlertManager] No threshold configuration found');
                return;
            }
            
            if (!thresholdConfig.enabled) {
                console.log('[AlertManager] Threshold config is disabled, skipping evaluation');
                return;
            }
            
            const globalThresholds = thresholdConfig.globalThresholds || {};
            const guestThresholds = thresholdConfig.guestThresholds || {};
            const alertDuration = thresholdConfig.duration || 0; // Get duration from config
            const timestamp = Date.now();
            
            console.log('[AlertManager] Evaluating simple thresholds:', {
                globalThresholds,
                guestCount: allGuestMetrics.size,
                alertDuration
            });
            
            const fsSync = require('fs');
            fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] evaluateSimpleThresholds called, globalThresholds: ${JSON.stringify(globalThresholds)}\n`);
            
            // Process each guest and bundle all their metric alerts
            for (const [guestKey, guestMetrics] of allGuestMetrics.entries()) {
                fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] Processing guest ${guestKey}\n`);
                const guest = guestMetrics.current;
                if (!guest || !guest.name) {
                    console.log(`[AlertManager] Skipping invalid guest: ${guestKey}`);
                    continue;
                }
                
                // Debug: log guest being evaluated
                console.log(`[AlertManager] Evaluating guest ${guest.name}: disk=${guest.disk}, maxdisk=${guest.maxdisk}`);
                
                // Replace bundled alerts with individual metric alerts
                try {
                    console.log(`[AlertManager] About to call evaluateGuestIndividualAlerts for ${guest.name}`);
                    await this.evaluateGuestIndividualAlerts(guest, globalThresholds, guestThresholds, timestamp, guestMetrics, alertDuration);
                    console.log(`[AlertManager] Finished evaluateGuestIndividualAlerts for ${guest.name}`);
                } catch (error) {
                    console.error(`[AlertManager] Error evaluating alerts for ${guest.name}:`, error);
                    console.error(`[AlertManager] Stack trace:`, error.stack);
                    fsSync.appendFileSync('/opt/pulse/alert-debug.log', `[${new Date().toISOString()}] ERROR evaluating ${guest.name}: ${error.message}\n${error.stack}\n`);
                }
            }
            
        } catch (error) {
            console.error('[AlertManager] Error in simplified threshold evaluation:', error);
        } finally {
            this.processingMetrics = false;
        }
    }

    /**
     * Evaluate all metrics for a guest and create individual alerts for each exceeded metric
     */
    async evaluateGuestIndividualAlerts(guest, globalThresholds, guestThresholds, timestamp, guestMetrics = null, alertDuration = 0) {
        // Get the threshold rule configuration
        const thresholdConfig = Array.from(this.alertRules.values()).find(rule => rule.type === 'per_guest_thresholds');
        console.log(`[AlertManager] Individual alerts for ${guest.name} - thresholdConfig found: ${!!thresholdConfig}`);
        
        // Get threshold for this guest
        const guestKey = `${guest.endpointId || 'unknown'}-${guest.node}-${guest.vmid}`;
        const guestSpecificThresholds = guestThresholds[guestKey] || {};
        
        // Check all metrics for this guest
        const metrics = ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'];
        
        // Process each metric individually
        for (const metricType of metrics) {
            const metricResult = this.evaluateMetricForGuest(guest, metricType, guestSpecificThresholds, globalThresholds, guestMetrics);
            if (metricResult) {
                console.log(`[AlertManager] ${guest.name} - ${metricType}: value=${metricResult.currentValue}, threshold=${metricResult.threshold}, exceeded=${metricResult.isExceeded}`);
                if (metricResult.isExceeded) {
                    // Create an individual alert for this metric
                    await this.createIndividualMetricAlert(guest, metricType, metricResult, thresholdConfig, timestamp, alertDuration);
                } else {
                    // Metric is below threshold, resolve any existing alert
                    await this.resolveIndividualMetricAlert(guest, metricType, timestamp);
                }
            } else {
                // No metric data available, check if there's an existing alert that should be resolved
                await this.resolveIndividualMetricAlert(guest, metricType, timestamp);
            }
        }
    }
    
    /**
     * Create an individual alert for a specific metric
     */
    async createIndividualMetricAlert(guest, metricType, metricResult, thresholdConfig, timestamp, alertDuration) {
        const alertKey = `${guest.endpointId || 'unknown'}_${guest.node}_${guest.vmid}_${metricType}`;
        const existingAlert = this.activeAlerts.get(alertKey);
        
        const alertName = `${guest.name} - ${this.getReadableMetricName(metricType)}`;
        const metricDetail = `${this.formatMetricValue(metricResult.currentValue, metricType)} (≥${this.formatMetricValue(metricResult.threshold, metricType)})`;
        
        if (!existingAlert) {
            // Create new individual alert
            const notificationSettings = thresholdConfig?.notifications || { dashboard: true, email: true, webhook: true };
            console.log(`[AlertManager] Creating individual ${metricType} alert for ${guest.name}`);
            
            const newAlert = {
                id: this.generateAlertId(),
                rule: {
                    id: `guest-${metricType}`,
                    name: alertName,
                    description: `${this.getReadableMetricName(metricType)} threshold exceeded`,
                    group: 'guest_threshold',
                    tags: [metricType],
                    type: 'guest_threshold',
                        autoResolve: thresholdConfig?.autoResolve !== false,
                        duration: alertDuration,
                        notifications: notificationSettings
                    },
                    guest: this.createSafeGuestCopy(guest),
                metric: metricType,
                currentValue: metricResult.currentValue,
                threshold: metricResult.threshold,
                    startTime: timestamp,
                    lastUpdate: timestamp,
                    state: alertDuration === 0 ? 'active' : 'pending', // If duration is 0, trigger immediately
                    acknowledged: false,
                    message: `${alertName}: ${metricDetail}`,
                    notificationChannels: this.determineNotificationChannels(notificationSettings),
                    emailSent: false,
                    webhookSent: false,
                    triggeredAt: alertDuration === 0 ? timestamp : undefined // Set triggeredAt if immediate
                };
                
                this.activeAlerts.set(alertKey, newAlert);
                
                if (alertDuration === 0) {
                    console.log(`[AlertManager] Created active individual ${metricType} alert for ${guest.name} (immediate trigger)`);
                    this.triggerAlert(newAlert).catch(error => {
                        console.error(`[AlertManager] Error triggering alert ${newAlert.id}:`, error);
                    });
                } else {
                    console.log(`[AlertManager] Created pending individual ${metricType} alert for ${guest.name}`);
                }
            } else if (existingAlert.state === 'pending') {
                // Check if duration threshold is met
                const elapsedTime = timestamp - existingAlert.startTime;
                if (elapsedTime >= alertDuration) {
                    // Trigger the alert
                    existingAlert.state = 'active';
                    existingAlert.triggeredAt = timestamp;
                existingAlert.currentValue = metricResult.currentValue;
                existingAlert.threshold = metricResult.threshold;
                existingAlert.message = `${alertName}: ${metricDetail}`;
                    // Update notification channels based on current rule settings
                    existingAlert.notificationChannels = this.determineNotificationChannels(existingAlert.rule.notifications);
                    
                console.log(`[AlertManager] Triggered ${metricType} alert for ${guest.name} after ${elapsedTime}ms`);
                    
                    await this.triggerAlert(existingAlert);
                } else {
                    // Update pending alert with current data
                    existingAlert.lastUpdate = timestamp;
                existingAlert.currentValue = metricResult.currentValue;
                existingAlert.threshold = metricResult.threshold;
                existingAlert.message = `${alertName}: ${metricDetail}`;
                }
            } else if (existingAlert.state === 'active') {
                // Update existing active alert
                existingAlert.rule.name = alertName;
                existingAlert.currentValue = metricResult.currentValue;
                existingAlert.threshold = metricResult.threshold;
                existingAlert.lastUpdate = timestamp;
                existingAlert.message = `${alertName}: ${metricDetail}`;
            }
        }
    
    /**
     * Resolve an individual metric alert if it exists
     */
    async resolveIndividualMetricAlert(guest, metricType, timestamp) {
        const alertKey = `${guest.endpointId || 'unknown'}_${guest.node}_${guest.vmid}_${metricType}`;
        const existingAlert = this.activeAlerts.get(alertKey);
        
        if (existingAlert && (existingAlert.state === 'active' || existingAlert.state === 'pending')) {
            // Check if auto-resolve is enabled
            if (existingAlert.rule?.autoResolve === false) {
                console.log(`[AlertManager] Auto-resolve disabled for ${metricType} alert for ${guest.name}, keeping alert active`);
                return;
            }
            
            const wasActive = existingAlert.state === 'active';
            existingAlert.state = 'resolved';
            existingAlert.resolvedAt = timestamp;
            
            console.log(`[AlertManager] Resolved ${metricType} alert for ${guest.name}`);
            
            if (wasActive) {
                // Properly resolve the alert
                this.resolveAlert(existingAlert).catch(error => {
                    console.error(`[AlertManager] Error resolving ${metricType} alert: ${error}`);
                });
            } else {
                // Just remove if it was pending
                this.activeAlerts.delete(alertKey);
            }
        }
    }

    /**
     * Evaluate a single metric for a guest and return result
     */
    evaluateMetricForGuest(guest, metricType, guestSpecificThresholds, globalThresholds, guestMetrics = null) {
        let threshold = guestSpecificThresholds[metricType] || globalThresholds[metricType];
        
        console.log(`[AlertManager] evaluateMetricForGuest: ${guest.name} - ${metricType}, threshold=${threshold}`);
        
        // Skip if no threshold set or empty
        if (threshold === undefined || threshold === null || threshold === '') {
            return null;
        }
        
        threshold = parseFloat(threshold);
        if (isNaN(threshold) || threshold < 0) {
            return null;
        }
        
        // Get current metric value
        let currentValue = this.getMetricValueForGuest(guest, metricType, guestMetrics);
        if (currentValue === null) {
            return null;
        }
        
        
        // Check if threshold is exceeded
        const isExceeded = currentValue >= threshold;
        
        return {
            metricType,
            currentValue,
            threshold,
            isExceeded
        };
    }

    /**
     * Get metric value for a guest (extracted from old method)
     */
    getMetricValueForGuest(guest, metricType, guestMetrics = null) {
        let currentValue = null;
        
        if (metricType === 'cpu') {
            // CPU value is already in percentage format when it reaches the alert system
            currentValue = guest.cpu || 0;
        } else if (metricType === 'memory') {
            // Memory percentage calculation
            if (guest.mem && guest.maxmem) {
                currentValue = (guest.mem / guest.maxmem) * 100;
            }
        } else if (metricType === 'disk') {
            // Disk percentage calculation  
            if (guest.disk && guest.maxdisk) {
                currentValue = (guest.disk / guest.maxdisk) * 100;
                // Debug logging for disk calculation
                if (guest.name === 'homepage' || guest.name === 'homeassistant') {
                    console.log(`[AlertManager] Disk calculation for ${guest.name}: disk=${guest.disk}, maxdisk=${guest.maxdisk}, percentage=${currentValue}%`);
                }
            }
        } else if (metricType === 'diskread') {
            // Use guestMetrics if available, otherwise fallback to guest data
            const metricsData = guestMetrics?.current || guest;
            currentValue = this.calculateIORate(metricsData, 'diskread', guest);
        } else if (metricType === 'diskwrite') {
            const metricsData = guestMetrics?.current || guest;
            currentValue = this.calculateIORate(metricsData, 'diskwrite', guest);
        } else if (metricType === 'netin') {
            const metricsData = guestMetrics?.current || guest;
            currentValue = this.calculateIORate(metricsData, 'netin', guest);
        } else if (metricType === 'netout') {
            const metricsData = guestMetrics?.current || guest;
            currentValue = this.calculateIORate(metricsData, 'netout', guest);
        }
        
        return currentValue;
    }

    /**
     * Evaluate a single metric threshold for a guest (OLD METHOD - DISABLED)
     * This method is replaced by evaluateGuestBundledAlerts for cleaner UI
     */
    evaluateMetricThreshold(guest, metricType, globalThresholds, guestThresholds, timestamp) {
        // DISABLED - Using bundled alerts instead
        return;
        // Get threshold for this guest/metric  
        const guestKey = `${guest.endpointId || 'unknown'}-${guest.node}-${guest.vmid}`;
        const guestSpecificThresholds = guestThresholds[guestKey] || {};
        
        let threshold = guestSpecificThresholds[metricType] || globalThresholds[metricType];
        
        // Skip if no threshold set or empty
        if (threshold === undefined || threshold === null || threshold === '') {
            return;
        }
        
        threshold = parseFloat(threshold);
        if (isNaN(threshold) || threshold < 0) {
            return;
        }
        
        let currentValue = null;
        
        if (metricType === 'cpu') {
            // CPU value is already in percentage format when it reaches the alert system
            currentValue = guest.cpu || 0;
        } else if (metricType === 'memory') {
            // Memory percentage calculation
            if (guest.mem && guest.maxmem) {
                currentValue = (guest.mem / guest.maxmem) * 100;
            }
        } else if (metricType === 'disk') {
            // Disk percentage calculation  
            if (guest.disk && guest.maxdisk) {
                currentValue = (guest.disk / guest.maxdisk) * 100;
                // Debug logging for disk calculation
                if (guest.name === 'homepage' || guest.name === 'homeassistant') {
                    console.log(`[AlertManager] Disk calculation for ${guest.name}: disk=${guest.disk}, maxdisk=${guest.maxdisk}, percentage=${currentValue}%`);
                }
            }
        } else if (metricType === 'diskread') {
            currentValue = this.calculateIORate(guest, 'diskread', guest);
        } else if (metricType === 'diskwrite') {
            currentValue = this.calculateIORate(guest, 'diskwrite', guest);
        } else if (metricType === 'netin') {
            currentValue = this.calculateIORate(guest, 'netin', guest);
        } else if (metricType === 'netout') {
            currentValue = this.calculateIORate(guest, 'netout', guest);
        }
        
        
        if (currentValue === null) {
            return;
        }
        
        // Check if threshold is exceeded
        const isExceeded = currentValue >= threshold;
        
        // Create unique alert key for this guest + metric
        const alertKey = `${guest.endpointId || 'unknown'}_${guest.node}_${guest.vmid}_${metricType}`;
        const existingAlert = this.activeAlerts.get(alertKey);
        
        if (isExceeded) {
            const alertName = `${this.getReadableMetricName(metricType)}: ${this.formatMetricValue(currentValue, metricType)} (≥${this.formatMetricValue(threshold, metricType)})`;
            
            if (!existingAlert) {
                // Create new alert for this specific metric
                const newAlert = {
                    id: this.generateAlertId(),
                    rule: {
                        id: `simple-${metricType}`,
                        name: alertName,
                        description: `Simplified ${metricType} threshold alert`,
                        group: 'simple_thresholds',
                        tags: [metricType],
                        type: 'simple_threshold',
                        autoResolve: true
                    },
                    guest: this.createSafeGuestCopy(guest),
                    metric: metricType,
                    currentValue: currentValue,
                    threshold: threshold,
                    triggeredAt: timestamp,
                    state: 'active',
                    acknowledged: false,
                    message: `${guest.name} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid}) on ${guest.node} - ${alertName}`,
                    notificationChannels: this.determineNotificationChannels(matchingRule?.notifications || { dashboard: true, email: true, webhook: true }),
                    emailSent: false,
                    webhookSent: false
                };
                
                this.activeAlerts.set(alertKey, newAlert);
                console.log(`[AlertManager] Created simple ${metricType.toUpperCase()} alert for ${guest.name}: ${this.formatMetricValue(currentValue, metricType)} > ${this.formatMetricValue(threshold, metricType)}`);
                
                // Emit alert event
                this.emit('alert', newAlert);
            } else {
                // Always update existing alert with current values and fresh timestamp
                existingAlert.rule.name = alertName;
                existingAlert.currentValue = currentValue;
                existingAlert.threshold = threshold;
                existingAlert.triggeredAt = timestamp; // Always use current timestamp
                existingAlert.message = `${guest.name} (${(guest.type || 'unknown').toUpperCase()} ${guest.vmid}) on ${guest.node} - ${alertName}`;
            }
        } else {
            // Threshold not exceeded - resolve alert if it exists
            if (existingAlert && existingAlert.state === 'active') {
                existingAlert.state = 'resolved';
                existingAlert.resolvedAt = timestamp;
                
                console.log(`[AlertManager] Resolved simple ${metricType.toUpperCase()} alert for ${guest.name}`);
                
                // Properly resolve the alert
                this.resolveAlert(existingAlert).catch(error => {
                    console.error(`[AlertManager] Error resolving simple ${metricType} alert: ${error}`);
                });
            }
        }
    }

    /**
     * Determine which notification channels should be enabled for an alert
     * @param {Object} notifications - The notifications object from the rule
     * @returns {Array} Array of enabled notification channels
     */
    determineNotificationChannels(notifications) {
        const channels = ['dashboard']; // Dashboard is always included
        
        if (!notifications) {
            return channels;
        }
        
        // Check if email is enabled and configured
        if (notifications.email && this.emailTransporter) {
            channels.push('email');
        }
        
        // Check if webhook is enabled and configured  
        if (notifications.webhook && process.env.WEBHOOK_URL) {
            channels.push('webhook');
        }
        
        return channels;
    }
}

module.exports = AlertManager; 