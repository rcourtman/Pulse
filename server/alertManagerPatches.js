const fs = require('fs').promises;
const path = require('path');
const fileLock = require('./utils/fileLock');

// Patches to be applied to AlertManager
module.exports = {
    // 1. Add file locking to prevent race conditions
    async loadAlertRules() {
        return await fileLock.withLock(this.alertRulesFile, async () => {
            try {
                const data = await fs.readFile(this.alertRulesFile, 'utf-8');
                const rules = JSON.parse(data);
                
                // Clear existing rules
                this.alertRules.clear();
                
                // Validate and load rules
                Object.entries(rules).forEach(([ruleId, rule]) => {
                    // Validate rule structure
                    if (this.validateRule(rule)) {
                        this.alertRules.set(ruleId, rule);
                    } else {
                        console.warn(`[AlertManager] Skipping invalid rule: ${ruleId}`);
                    }
                });
                
                console.log(`[AlertManager] Loaded ${this.alertRules.size} alert rules`);
            } catch (error) {
                if (error.code === 'ENOENT') {
                    console.log('[AlertManager] No alert rules file found, creating default rules');
                    await this.createDefaultAlertRules();
                } else {
                    console.error('[AlertManager] Error loading alert rules:', error);
                    throw error;
                }
            }
        });
    },

    async saveAlertRules() {
        return await fileLock.withLock(this.alertRulesFile, async () => {
            try {
                const dataDir = path.dirname(this.alertRulesFile);
                await fs.mkdir(dataDir, { recursive: true });
                
                const rulesToSave = {};
                for (const [ruleId, rule] of this.alertRules) {
                    rulesToSave[ruleId] = this.createSafeRuleCopy(rule);
                }
                
                await fs.writeFile(
                    this.alertRulesFile,
                    JSON.stringify(rulesToSave, null, 2),
                    'utf-8'
                );
            } catch (error) {
                console.error('[AlertManager] Error saving alert rules:', error);
                throw error;
            }
        });
    },

    async loadActiveAlerts() {
        return await fileLock.withLock(this.activeAlertsFile, async () => {
            try {
                const data = await fs.readFile(this.activeAlertsFile, 'utf-8');
                const persistedAlerts = JSON.parse(data);
                
                // Clear and restore alerts
                this.activeAlerts.clear();
                this.acknowledgedAlerts.clear();
                
                Object.entries(persistedAlerts).forEach(([key, alert]) => {
                    if (this.validateAlert(alert)) {
                        this.activeAlerts.set(key, alert);
                        if (alert.acknowledged) {
                            this.acknowledgedAlerts.set(key, alert);
                        }
                    }
                });
                
                console.log(`[AlertManager] Loaded ${this.activeAlerts.size} active alerts`);
            } catch (error) {
                if (error.code === 'ENOENT') {
                    // Create empty file
                    await this.saveActiveAlerts();
                } else {
                    console.error('[AlertManager] Error loading active alerts:', error);
                }
            }
        });
    },

    async saveActiveAlerts() {
        return await fileLock.withLock(this.activeAlertsFile, async () => {
            try {
                const dataDir = path.dirname(this.activeAlertsFile);
                await fs.mkdir(dataDir, { recursive: true });
                
                const alertsToSave = {};
                for (const [key, alert] of this.activeAlerts) {
                    alertsToSave[key] = this.createSafeAlertCopy(alert);
                }
                
                await fs.writeFile(
                    this.activeAlertsFile,
                    JSON.stringify(alertsToSave, null, 2),
                    'utf-8'
                );
            } catch (error) {
                console.error('[AlertManager] Error saving active alerts:', error);
                // Don't throw - we don't want to break the alert flow
            }
        });
    },

    // 2. Add validation methods
    validateRule(rule) {
        if (!rule || typeof rule !== 'object') return false;
        if (!rule.id || !rule.name) return false;
        
        // Validate thresholds are numbers
        if (rule.threshold !== undefined) {
            const threshold = typeof rule.threshold === 'string' ? 
                parseFloat(rule.threshold) : rule.threshold;
            if (isNaN(threshold) || threshold < 0) {
                console.warn(`[AlertManager] Invalid threshold ${rule.threshold} for rule ${rule.id}`);
                return false;
            }
            // Fix string thresholds
            rule.threshold = threshold;
        }
        
        // Validate duration
        if (rule.duration !== undefined) {
            const duration = typeof rule.duration === 'string' ? 
                parseInt(rule.duration) : rule.duration;
            if (isNaN(duration) || duration < 0) {
                console.warn(`[AlertManager] Invalid duration ${rule.duration} for rule ${rule.id}`);
                return false;
            }
            rule.duration = duration;
        }
        
        // Validate cooldowns
        if (rule.emailCooldowns) {
            this.validateCooldownConfig(rule.emailCooldowns, 'email', rule.id);
        }
        if (rule.webhookCooldowns) {
            this.validateCooldownConfig(rule.webhookCooldowns, 'webhook', rule.id);
        }
        
        return true;
    },

    validateCooldownConfig(config, type, ruleId) {
        const fields = ['cooldownMinutes', 'debounceMinutes', 'maxEmailsPerHour', 'maxCallsPerHour'];
        
        for (const field of fields) {
            if (config[field] !== undefined) {
                const value = typeof config[field] === 'string' ? 
                    parseInt(config[field]) : config[field];
                if (isNaN(value) || value < 0) {
                    console.warn(`[AlertManager] Invalid ${type} ${field} for rule ${ruleId}`);
                    config[field] = this[`${type}CooldownConfig`][field] || 15;
                } else {
                    config[field] = value;
                }
            }
        }
    },

    validateAlert(alert) {
        if (!alert || typeof alert !== 'object') return false;
        if (!alert.id || !alert.rule) return false;
        if (!alert.guest && !alert.nodeId) return false;
        
        // Validate timestamps
        if (alert.triggeredAt && isNaN(new Date(alert.triggeredAt).getTime())) {
            alert.triggeredAt = Date.now();
        }
        
        return true;
    },

    // 3. Fix memory leaks
    cleanupOldData() {
        const now = Date.now();
        const oneHourAgo = now - 3600000;
        const oneDayAgo = now - 86400000;
        const oneWeekAgo = now - 604800000;
        
        if (this.alertHistory.length > this.maxHistorySize) {
            this.alertHistory = this.alertHistory.slice(0, this.maxHistorySize);
        }
        
        // Clean up notification status for resolved alerts older than 1 hour
        for (const [alertId, status] of this.notificationStatus) {
            const alert = Array.from(this.activeAlerts.values())
                .find(a => a.id === alertId);
            
            if (!alert || (alert.state === 'resolved' && 
                alert.resolvedAt && alert.resolvedAt < oneHourAgo)) {
                this.notificationStatus.delete(alertId);
            }
        }
        
        // Clean up old cooldowns
        this.cleanupExpiredCooldowns();
        
        // Clean up webhook batcher listeners
        if (this.webhookBatcher) {
            const listeners = this.webhookBatcher.listenerCount('send');
            if (listeners > 10) {
                console.warn(`[AlertManager] Too many webhook batcher listeners: ${listeners}`);
                this.webhookBatcher.removeAllListeners('send');
                this.webhookBatcher.removeAllListeners('sent');
                this.setupWebhookBatcherListeners();
            }
        }
        
        console.log('[AlertManager] Cleanup completed - History: %d, Status: %d, Email CD: %d, Webhook CD: %d',
            this.alertHistory.length,
            this.notificationStatus.size,
            this.emailCooldowns.size,
            this.webhookCooldowns.size
        );
    },

    // 4. Add retry mechanism for notifications
    async sendNotificationWithRetry(type, sendFunc, retries = 3) {
        let lastError;
        
        for (let attempt = 1; attempt <= retries; attempt++) {
            try {
                return await sendFunc();
            } catch (error) {
                lastError = error;
                console.error(`[AlertManager] ${type} notification attempt ${attempt} failed:`, error.message);
                
                if (attempt < retries) {
                    // Exponential backoff: 1s, 2s, 4s
                    const delay = Math.pow(2, attempt - 1) * 1000;
                    await new Promise(resolve => setTimeout(resolve, delay));
                }
            }
        }
        
        throw new Error(`${type} notification failed after ${retries} attempts: ${lastError.message}`);
    },

    // 5. Safe copy methods to prevent circular references
    createSafeRuleCopy(rule) {
        if (!rule) return null;
        
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
            notifications: rule.notifications ? {...rule.notifications} : {},
            emailCooldowns: rule.emailCooldowns ? {...rule.emailCooldowns} : {},
            webhookCooldowns: rule.webhookCooldowns ? {...rule.webhookCooldowns} : {},
            globalThresholds: rule.globalThresholds ? {...rule.globalThresholds} : {},
            guestThresholds: rule.guestThresholds ? {...rule.guestThresholds} : {},
            nodeThresholds: rule.nodeThresholds ? {...rule.nodeThresholds} : {},
            createdAt: rule.createdAt,
            lastUpdated: rule.lastUpdated
        };
    },

    createSafeGuestCopy(guest) {
        if (!guest) return null;
        
        return {
            name: guest.name,
            vmid: guest.vmid,
            node: guest.node,
            type: guest.type,
            status: guest.status,
            endpointId: guest.endpointId
        };
    },

    createSafeAlertCopy(alert) {
        const copy = {
            id: alert.id,
            type: alert.type,
            rule: this.createSafeRuleCopy(alert.rule),
            startTime: alert.startTime,
            lastUpdate: alert.lastUpdate,
            triggeredAt: alert.triggeredAt,
            resolvedAt: alert.resolvedAt,
            currentValue: alert.currentValue,
            effectiveThreshold: alert.effectiveThreshold,
            state: alert.state,
            severity: alert.severity,
            message: alert.message,
            acknowledged: alert.acknowledged,
            acknowledgedBy: alert.acknowledgedBy,
            acknowledgedAt: alert.acknowledgedAt,
            acknowledgeNote: alert.acknowledgeNote,
            emailSent: alert.emailSent,
            webhookSent: alert.webhookSent,
            incidentType: alert.incidentType,
            notificationChannels: alert.notificationChannels,
            // Add bundled alert specific fields
            metric: alert.metric,
            exceededMetrics: alert.exceededMetrics,
            metricsCount: alert.metricsCount
        };
        
        if (alert.type === 'node_threshold') {
            copy.nodeId = alert.nodeId;
            copy.nodeName = alert.nodeName;
            copy.metric = alert.metric;
            copy.threshold = alert.threshold;
        } else {
            copy.guest = this.createSafeGuestCopy(alert.guest);
        }
        
        return copy;
    },

    // 6. Setup webhook batcher listeners properly
    setupWebhookBatcherListeners() {
        this.webhookBatcher.on('send', async (alert, webhookUrl) => {
            await this.sendDirectWebhookNotification(alert);
        });
        
        this.webhookBatcher.on('sent', (alertId) => {
            const status = this.notificationStatus.get(alertId) || {};
            status.webhookSent = true;
            this.notificationStatus.set(alertId, status);
        });
    },

    // 7. Enhanced validation for edge cases
    validateThresholdValue(value, metricType, context = '') {
        // Convert string to number if needed
        const numValue = typeof value === 'string' ? parseFloat(value) : value;
        
        // Check for invalid values
        if (isNaN(numValue) || numValue < 0) {
            console.warn(`[AlertManager] Invalid threshold value '${value}' for ${metricType} ${context}`);
            return this.getDefaultThreshold(metricType);
        }
        
        // Check for extreme values
        if (metricType === 'cpu' || metricType === 'memory' || metricType === 'disk') {
            if (numValue > 100) {
                console.warn(`[AlertManager] Threshold ${numValue}% exceeds 100% for ${metricType} ${context}`);
                return 100;
            }
        }
        
        return numValue;
    },
    
    getDefaultThreshold(metricType) {
        const defaults = {
            cpu: 85,
            memory: 90,
            disk: 90,
            diskread: 100 * 1024 * 1024, // 100 MB/s
            diskwrite: 100 * 1024 * 1024,
            netin: 100 * 1024 * 1024,
            netout: 100 * 1024 * 1024
        };
        return defaults[metricType] || 0;
    },
    
    // 8. State synchronization improvements
    broadcastStateUpdate(updateType, data) {
        try {
            // Emit events for WebSocket updates
            this.emit('stateUpdate', {
                type: updateType,
                data: data,
                timestamp: Date.now()
            });
            
            // Force save if critical update
            if (updateType === 'alert_triggered' || updateType === 'alert_resolved') {
                setImmediate(() => {
                    this.saveActiveAlerts().catch(err => 
                        console.error('[AlertManager] Failed to save after state update:', err)
                    );
                });
            }
        } catch (error) {
            console.error('[AlertManager] Error broadcasting state update:', error);
        }
    },
    
    // 9. Performance optimizations
    createThresholdCache() {
        this.thresholdCache = new Map();
        this.thresholdCacheTimeout = null;
    },
    
    getCachedThreshold(guestKey, metricType) {
        const cacheKey = `${guestKey}_${metricType}`;
        const cached = this.thresholdCache.get(cacheKey);
        
        if (cached && Date.now() - cached.timestamp < 60000) { // 1 minute cache
            return cached.value;
        }
        
        return null;
    },
    
    setCachedThreshold(guestKey, metricType, value) {
        const cacheKey = `${guestKey}_${metricType}`;
        this.thresholdCache.set(cacheKey, {
            value: value,
            timestamp: Date.now()
        });
        
        // Clean old entries periodically
        if (!this.thresholdCacheTimeout) {
            this.thresholdCacheTimeout = setTimeout(() => {
                this.cleanThresholdCache();
            }, 300000); // 5 minutes
        }
    },
    
    cleanThresholdCache() {
        const now = Date.now();
        const expiredKeys = [];
        
        for (const [key, data] of this.thresholdCache) {
            if (now - data.timestamp > 300000) { // 5 minutes
                expiredKeys.push(key);
            }
        }
        
        expiredKeys.forEach(key => this.thresholdCache.delete(key));
        this.thresholdCacheTimeout = null;
    },
    
    // 10. Initialize method to apply all patches
    applyPatches() {
        // Replace methods with patched versions
        this.loadAlertRules = module.exports.loadAlertRules.bind(this);
        this.saveAlertRules = module.exports.saveAlertRules.bind(this);
        this.loadActiveAlerts = module.exports.loadActiveAlerts.bind(this);
        this.saveActiveAlerts = module.exports.saveActiveAlerts.bind(this);
        this.validateRule = module.exports.validateRule.bind(this);
        this.validateCooldownConfig = module.exports.validateCooldownConfig.bind(this);
        this.validateAlert = module.exports.validateAlert.bind(this);
        this.cleanupOldData = module.exports.cleanupOldData.bind(this);
        this.sendNotificationWithRetry = module.exports.sendNotificationWithRetry.bind(this);
        this.createSafeRuleCopy = module.exports.createSafeRuleCopy.bind(this);
        this.createSafeGuestCopy = module.exports.createSafeGuestCopy.bind(this);
        this.createSafeAlertCopy = module.exports.createSafeAlertCopy.bind(this);
        this.setupWebhookBatcherListeners = module.exports.setupWebhookBatcherListeners.bind(this);
        this.validateThresholdValue = module.exports.validateThresholdValue.bind(this);
        this.getDefaultThreshold = module.exports.getDefaultThreshold.bind(this);
        this.broadcastStateUpdate = module.exports.broadcastStateUpdate.bind(this);
        this.createThresholdCache = module.exports.createThresholdCache.bind(this);
        this.getCachedThreshold = module.exports.getCachedThreshold.bind(this);
        this.setCachedThreshold = module.exports.setCachedThreshold.bind(this);
        this.cleanThresholdCache = module.exports.cleanThresholdCache.bind(this);
        
        // Initialize performance optimizations
        this.createThresholdCache();
        
        // Update cleanup interval to run more frequently
        clearInterval(this.cleanupInterval);
        this.cleanupInterval = setInterval(() => {
            this.cleanupOldData();
            this.cleanupResolvedAlerts();
        }, 300000); // Every 5 minutes
        
        // Clean up stale locks on startup
        fileLock.cleanupStaleLocks();
        
        // Add error boundaries for critical operations
        module.exports.wrapCriticalMethods.call(this);
        
        console.log('[AlertManager] Applied all patches for improved reliability');
    },
    
    // Wrap critical methods with error handling
    wrapCriticalMethods: function() {
        const criticalMethods = ['triggerAlert', 'resolveAlert', 'sendNotifications'];
        
        criticalMethods.forEach(methodName => {
            const originalMethod = this[methodName];
            if (originalMethod) {
                this[methodName] = async function(...args) {
                    try {
                        return await originalMethod.apply(this, args);
                    } catch (error) {
                        console.error(`[AlertManager] Critical error in ${methodName}:`, error);
                        // Don't let errors break the alert flow
                        this.emit('criticalError', {
                            method: methodName,
                            error: error.message,
                            stack: error.stack
                        });
                    }
                }.bind(this);
            }
        });
    }
};