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
        this.alertGroups = new Map();
        this.escalationRules = new Map();
        this.notificationChannels = new Map();
        this.alertMetrics = {
            totalFired: 0,
            totalResolved: 0,
            totalAcknowledged: 0,
            averageResolutionTime: 0,
            falsePositiveRate: 0
        };
        
        this.maxHistorySize = 10000; // Increased for better analytics
        this.acknowledgementsFile = path.join(__dirname, '../data/acknowledgements.json');
        
        // Initialize default configuration
        this.initializeDefaultRules();
        this.initializeNotificationChannels();
        this.initializeAlertGroups();
        
        // Load persisted acknowledgements
        this.loadAcknowledgements();
        
        // Initialize custom threshold manager
        this.initializeCustomThresholds();
        
        // Initialize email transporter
        this.emailTransporter = null;
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
    }

    initializeDefaultRules() {
        const defaultRules = [
            {
                id: 'high_cpu',
                name: 'High CPU Usage',
                description: 'Triggers when CPU usage exceeds threshold for specified duration',
                metric: 'cpu',
                condition: 'greater_than',
                threshold: parseInt(process.env.ALERT_CPU_THRESHOLD) || 85,
                duration: parseInt(process.env.ALERT_CPU_DURATION) || 300000, // 5 minutes
                severity: 'warning',
                enabled: process.env.ALERT_CPU_ENABLED !== 'false',
                tags: ['performance', 'cpu'],
                group: 'system_performance',
                escalationTime: 900000, // 15 minutes
                autoResolve: true,
                suppressionTime: 300000, // 5 minutes
                notificationChannels: ['default']
            },
            {
                id: 'critical_cpu',
                name: 'Critical CPU Usage',
                description: 'Critical CPU usage requiring immediate attention',
                metric: 'cpu',
                condition: 'greater_than',
                threshold: parseInt(process.env.ALERT_CPU_CRITICAL_THRESHOLD) || 95,
                duration: parseInt(process.env.ALERT_CPU_CRITICAL_DURATION) || 60000, // 1 minute
                severity: 'critical',
                enabled: process.env.ALERT_CPU_CRITICAL_ENABLED !== 'false',
                tags: ['performance', 'cpu', 'critical'],
                group: 'critical_alerts',
                escalationTime: 300000, // 5 minutes
                autoResolve: true,
                suppressionTime: 0, // No suppression for critical
                notificationChannels: ['default', 'urgent']
            },
            {
                id: 'high_memory',
                name: 'High Memory Usage',
                description: 'Memory usage exceeds safe operating levels',
                metric: 'memory',
                condition: 'greater_than',
                threshold: parseInt(process.env.ALERT_MEMORY_THRESHOLD) || 90,
                duration: parseInt(process.env.ALERT_MEMORY_DURATION) || 300000, // 5 minutes
                severity: 'warning',
                enabled: process.env.ALERT_MEMORY_ENABLED !== 'false',
                tags: ['performance', 'memory'],
                group: 'system_performance',
                escalationTime: 900000, // 15 minutes
                autoResolve: true,
                suppressionTime: 300000,
                notificationChannels: ['default']
            },
            {
                id: 'critical_memory',
                name: 'Critical Memory Usage',
                description: 'Memory usage at critical levels - system stability at risk',
                metric: 'memory',
                condition: 'greater_than',
                threshold: parseInt(process.env.ALERT_MEMORY_CRITICAL_THRESHOLD) || 98,
                duration: parseInt(process.env.ALERT_MEMORY_CRITICAL_DURATION) || 120000, // 2 minutes
                severity: 'critical',
                enabled: process.env.ALERT_MEMORY_CRITICAL_ENABLED !== 'false',
                tags: ['performance', 'memory', 'critical'],
                group: 'critical_alerts',
                escalationTime: 300000, // 5 minutes
                autoResolve: true,
                suppressionTime: 0,
                notificationChannels: ['default', 'urgent']
            },
            {
                id: 'disk_space_warning',
                name: 'Low Disk Space',
                description: 'Disk usage approaching capacity limits',
                metric: 'disk',
                condition: 'greater_than',
                threshold: parseInt(process.env.ALERT_DISK_THRESHOLD) || 85,
                duration: parseInt(process.env.ALERT_DISK_DURATION) || 300000, // 5 minutes
                severity: 'warning',
                enabled: process.env.ALERT_DISK_ENABLED !== 'false',
                tags: ['storage', 'disk'],
                group: 'storage_alerts',
                escalationTime: 1800000, // 30 minutes
                autoResolve: true,
                suppressionTime: 600000, // 10 minutes
                notificationChannels: ['default']
            },
            {
                id: 'disk_space_critical',
                name: 'Critical Disk Space',
                description: 'Disk space critically low - immediate action required',
                metric: 'disk',
                condition: 'greater_than',
                threshold: parseInt(process.env.ALERT_DISK_CRITICAL_THRESHOLD) || 95,
                duration: parseInt(process.env.ALERT_DISK_CRITICAL_DURATION) || 60000, // 1 minute
                severity: 'critical',
                enabled: process.env.ALERT_DISK_CRITICAL_ENABLED !== 'false',
                tags: ['storage', 'disk', 'critical'],
                group: 'critical_alerts',
                escalationTime: 300000, // 5 minutes
                autoResolve: true,
                suppressionTime: 0,
                notificationChannels: ['default', 'urgent']
            },
            {
                id: 'guest_down',
                name: 'Guest System Down',
                description: 'Virtual machine or container has stopped unexpectedly',
                metric: 'status',
                condition: 'equals',
                threshold: 'stopped',
                duration: parseInt(process.env.ALERT_DOWN_DURATION) || 60000, // 1 minute
                severity: 'critical',
                enabled: process.env.ALERT_DOWN_ENABLED !== 'false',
                tags: ['availability', 'guest'],
                group: 'availability_alerts',
                escalationTime: 600000, // 10 minutes
                autoResolve: true,
                suppressionTime: 120000, // 2 minutes
                notificationChannels: ['default', 'urgent']
            },
            {
                id: 'network_anomaly',
                name: 'Network Traffic Anomaly',
                description: 'Unusual network traffic patterns detected',
                metric: 'network_combined',
                condition: 'anomaly',
                threshold: 'auto', // Machine learning based
                duration: 180000, // 3 minutes
                severity: 'info',
                enabled: process.env.ALERT_NETWORK_ANOMALY_ENABLED === 'true',
                tags: ['network', 'anomaly'],
                group: 'network_alerts',
                escalationTime: 1800000, // 30 minutes
                autoResolve: true,
                suppressionTime: 900000, // 15 minutes
                notificationChannels: ['default']
            }
        ];

        defaultRules.forEach(rule => {
            if (rule.enabled) {
                this.alertRules.set(rule.id, rule);
            }
        });
    }

    initializeNotificationChannels() {
        this.notificationChannels.set('default', {
            id: 'default',
            name: 'Default Channel',
            type: 'webhook',
            enabled: true,
            config: {
                url: process.env.WEBHOOK_URL || null,
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            }
        });

        this.notificationChannels.set('urgent', {
            id: 'urgent',
            name: 'Urgent Alerts',
            type: 'email',
            enabled: process.env.SMTP_HOST ? true : false,
            config: {
                smtp: {
                    host: process.env.SMTP_HOST || null,
                    port: parseInt(process.env.SMTP_PORT) || 587,
                    secure: process.env.SMTP_SECURE === 'true',
                    auth: {
                        user: process.env.SMTP_USER || null,
                        pass: process.env.SMTP_PASS || null
                    }
                },
                from: process.env.ALERT_FROM_EMAIL || 'alerts@pulse-monitoring.local',
                to: process.env.ALERT_TO_EMAIL ? process.env.ALERT_TO_EMAIL.split(',') : []
            }
        });
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
    checkMetrics(guests, metrics) {
        const timestamp = Date.now();
        
        guests.forEach(guest => {
            this.alertRules.forEach(rule => {
                if (this.isRuleSuppressed(rule.id, guest)) return;
                
                const alertKey = `${rule.id}_${guest.endpointId}_${guest.node}_${guest.vmid}`;
                this.evaluateRule(rule, guest, metrics, alertKey, timestamp);
            });
        });
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
                // Create new alert with permanent ID
                const newAlert = {
                    id: this.generateAlertId(), // Generate ID once when alert is created
                    rule,
                    guest,
                    startTime: timestamp,
                    lastUpdate: timestamp,
                    currentValue,
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
                existingAlert.currentValue = currentValue;
            } else if (existingAlert.state === 'active') {
                // Update existing active alert
                existingAlert.lastUpdate = timestamp;
                existingAlert.currentValue = currentValue;
            }
        } else {
            if (existingAlert && existingAlert.state === 'active') {
                // Resolve alert
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
            severity: this.escalateSeverity(alert.rule.severity),
            message: `ESCALATED: ${alert.rule.name}`,
            escalated: true
        };
        
        this.emit('alertEscalated', escalatedAlert);
        this.sendNotifications(escalatedAlert, ['urgent']);
        
        console.warn(`[ALERT ESCALATED] ${escalatedAlert.message}`);
    }

    escalateSeverity(currentSeverity) {
        const severityLevels = ['info', 'warning', 'critical'];
        const currentIndex = severityLevels.indexOf(currentSeverity);
        return severityLevels[Math.min(currentIndex + 1, severityLevels.length - 1)];
    }

    sendNotifications(alert, channelOverride = null) {
        const channels = channelOverride || alert.rule.notificationChannels || ['default'];
        
        channels.forEach(channelId => {
            const channel = this.notificationChannels.get(channelId);
            if (channel && channel.enabled) {
                this.sendToChannel(channel, alert);
            }
        });
    }

    async sendToChannel(channel, alert) {
        try {
            console.log(`[NOTIFICATION] Sending to ${channel.name}:`, {
                channel: channel.type,
                alert: alert.rule.name,
                severity: alert.rule.severity,
                guest: alert.guest.name
            });

            if (channel.type === 'email') {
                await this.sendEmailNotification(channel, alert);
            } else if (channel.type === 'webhook') {
                await this.sendWebhookNotification(channel, alert);
            }
            
            // Emit event for external handlers
            this.emit('notification', { channel, alert });
        } catch (error) {
            console.error(`[NOTIFICATION ERROR] Failed to send to ${channel.name}:`, error);
            this.emit('notificationError', { channel, alert, error });
        }
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
            if (alert.state === 'active' && this.matchesFilters(alert, filters)) {
                active.push(this.formatAlertForAPI(alert));
            }
        }
        return active.sort((a, b) => b.triggeredAt - a.triggeredAt);
    }

    getAlertHistory(limit = 100, filters = {}) {
        let filtered = this.alertHistory.filter(alert => this.matchesFilters(alert, filters));
        return filtered.slice(0, limit);
    }

    matchesFilters(alert, filters) {
        if (filters.severity && alert.rule.severity !== filters.severity) return false;
        if (filters.group && alert.rule.group !== filters.group) return false;
        if (filters.node && alert.guest.node !== filters.node) return false;
        if (filters.acknowledged !== undefined && alert.acknowledged !== filters.acknowledged) return false;
        return true;
    }

    formatAlertForAPI(alert) {
        return {
            id: alert.id, // Use the stored permanent ID
            ruleId: alert.rule.id,
            ruleName: alert.rule.name,
            description: alert.rule.description,
            severity: alert.rule.severity,
            group: alert.rule.group,
            tags: alert.rule.tags,
            guest: {
                name: alert.guest.name,
                vmid: alert.guest.vmid,
                node: alert.guest.node,
                type: alert.guest.type,
                endpointId: alert.guest.endpointId
            },
            metric: alert.rule.metric,
            threshold: alert.rule.threshold,
            currentValue: alert.currentValue,
            triggeredAt: alert.triggeredAt,
            duration: Date.now() - alert.triggeredAt,
            acknowledged: alert.acknowledged || false,
            acknowledgedBy: alert.acknowledgedBy,
            acknowledgedAt: alert.acknowledgedAt,
            escalated: alert.escalated || false,
            message: this.generateAlertMessage(alert)
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
            .filter(a => a.state === 'active').length;

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
            groups: Array.from(this.alertGroups.values()),
            channels: Array.from(this.notificationChannels.values()).map(c => ({
                id: c.id,
                name: c.name,
                type: c.type,
                enabled: c.enabled
            }))
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
        const alertInfo = this.formatAlertForAPI(alert);
        
        // Add to history
        this.addToHistory(alertInfo);
        
        // Send notifications
        this.sendNotifications(alert);
        
        // Emit event for external handling
        this.emit('alert', alertInfo);

        console.warn(`[ALERT] ${alertInfo.message}`);
    }

    resolveAlert(alert) {
        const alertInfo = {
            id: alert.id, // Use the stored alert ID
            ruleId: alert.rule.id,
            ruleName: alert.rule.name,
            severity: 'resolved',
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
    }

    generateAlertMessage(alert) {
        const { guest, rule, currentValue } = alert;
        let valueStr = '';
        
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
        
        return `${rule.name} - ${guest.name} (${guest.type.toUpperCase()} ${guest.vmid}) on ${guest.node} - ${valueStr} (threshold: ${thresholdStr})`;
    }

    generateResolvedMessage(alert) {
        const { guest, rule } = alert;
        const duration = Math.round((alert.resolvedAt - alert.triggeredAt) / 1000);
        return `${rule.name} RESOLVED - ${guest.name} (${guest.type.toUpperCase()} ${guest.vmid}) on ${guest.node} - Duration: ${duration}s`;
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
        
        for (const [key, alert] of this.activeAlerts) {
            if (alert.state === 'resolved' && alert.resolvedAt < cutoffTime) {
                this.activeAlerts.delete(key);
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
    }

    updateRule(ruleId, updates) {
        const rule = this.alertRules.get(ruleId);
        if (rule) {
            Object.assign(rule, updates);
            this.emit('ruleUpdated', { ruleId, updates });
            return true;
        }
        return false;
    }

    addRule(rule) {
        if (!rule.id || !rule.name || !rule.metric) {
            throw new Error('Rule must have id, name, and metric');
        }
        
        // Set defaults for new rules
        const fullRule = {
            condition: 'greater_than',
            duration: 300000,
            severity: 'warning',
            enabled: true,
            tags: [],
            group: 'custom',
            escalationTime: 900000,
            autoResolve: true,
            suppressionTime: 300000,
            notificationChannels: ['default'],
            ...rule
        };
        
        this.alertRules.set(rule.id, fullRule);
        this.emit('ruleAdded', fullRule);
        return fullRule;
    }

    removeRule(ruleId) {
        const success = this.alertRules.delete(ruleId);
        if (success) {
            this.emit('ruleRemoved', { ruleId });
        }
        return success;
    }

    /**
     * Refresh alert rules based on current environment variables
     * This should be called after configuration changes
     */
    refreshRules() {
        console.log('[AlertManager] Refreshing alert rules based on current environment variables');
        
        // Store currently disabled rule IDs to clean up their alerts
        const previouslyActiveRules = new Set(this.alertRules.keys());
        
        // Clear existing rules
        this.alertRules.clear();
        
        // Re-initialize rules with current environment variables
        this.initializeDefaultRules();
        
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
        
        this.emit('rulesRefreshed', { activeRules: nowActiveRules.size, disabledRules });
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
                    severity: 'resolved',
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
        if (filters.severity) {
            return rules.filter(rule => rule.severity === filters.severity);
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
                
                if (metricThresholds) {
                    // Determine which threshold to use based on rule severity
                    if (rule.severity === 'critical' && metricThresholds.critical !== undefined) {
                        console.log(`[AlertManager] Using custom critical ${rule.metric} threshold ${metricThresholds.critical}% for ${guest.endpointId}:${guest.node}:${guest.vmid}`);
                        return metricThresholds.critical;
                    } else if (rule.severity === 'warning' && metricThresholds.warning !== undefined) {
                        console.log(`[AlertManager] Using custom warning ${rule.metric} threshold ${metricThresholds.warning}% for ${guest.endpointId}:${guest.node}:${guest.vmid}`);
                        return metricThresholds.warning;
                    }
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

    /**
     * Initialize email transporter for sending notifications
     */
    initializeEmailTransporter() {
        if (process.env.SMTP_HOST) {
            try {
                this.emailTransporter = nodemailer.createTransport({
                    host: process.env.SMTP_HOST,
                    port: parseInt(process.env.SMTP_PORT) || 587,
                    secure: process.env.SMTP_SECURE === 'true', // true for 465, false for other ports
                    auth: {
                        user: process.env.SMTP_USER,
                        pass: process.env.SMTP_PASS
                    }
                });
                console.log('[AlertManager] Email transporter initialized');
            } catch (error) {
                console.error('[AlertManager] Failed to initialize email transporter:', error);
            }
        } else {
            console.log('[AlertManager] SMTP not configured, email notifications disabled');
        }
    }

    /**
     * Send email notification
     */
    async sendEmailNotification(channel, alert) {
        if (!this.emailTransporter) {
            throw new Error('Email transporter not configured');
        }

        const recipients = channel.config.to;
        if (!recipients || recipients.length === 0) {
            throw new Error('No email recipients configured');
        }

        const severityEmoji = {
            'info': '💙',
            'warning': '⚠️',
            'critical': '🚨'
        };

        const subject = `${severityEmoji[alert.rule.severity] || '📢'} Pulse Alert: ${alert.rule.name}`;
        
        const html = `
            <div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
                <div style="background: ${alert.rule.severity === 'critical' ? '#dc2626' : alert.rule.severity === 'warning' ? '#ea580c' : '#2563eb'}; color: white; padding: 20px; border-radius: 8px 8px 0 0;">
                    <h1 style="margin: 0; font-size: 24px;">${severityEmoji[alert.rule.severity] || '📢'} ${alert.rule.name}</h1>
                    <p style="margin: 5px 0 0 0; opacity: 0.9; font-size: 16px;">Severity: ${alert.rule.severity.toUpperCase()}</p>
                </div>
                
                <div style="background: #f9fafb; padding: 20px; border-left: 4px solid ${alert.rule.severity === 'critical' ? '#dc2626' : alert.rule.severity === 'warning' ? '#ea580c' : '#2563eb'};">
                    <h2 style="margin: 0 0 15px 0; color: #374151;">Alert Details</h2>
                    
                    <table style="width: 100%; border-collapse: collapse;">
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151; width: 120px;">VM/LXC:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.guest.name} (${alert.guest.type} ${alert.guest.id})</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Node:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.guest.node}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Metric:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.rule.metric.toUpperCase()}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Current Value:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.value}%</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Threshold:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.threshold}%</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Status:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${alert.guest.status}</td>
                        </tr>
                        <tr>
                            <td style="padding: 8px 0; font-weight: bold; color: #374151;">Time:</td>
                            <td style="padding: 8px 0; color: #6b7280;">${new Date(alert.timestamp).toLocaleString()}</td>
                        </tr>
                    </table>
                </div>
                
                <div style="background: white; padding: 20px; border-radius: 0 0 8px 8px; border-top: 1px solid #e5e7eb;">
                    <p style="margin: 0; color: #6b7280; font-size: 14px;">
                        <strong>Description:</strong> ${alert.rule.description}
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

Severity: ${alert.rule.severity.toUpperCase()}
VM/LXC: ${alert.guest.name} (${alert.guest.type} ${alert.guest.id})
Node: ${alert.guest.node}
Metric: ${alert.rule.metric.toUpperCase()}
Current Value: ${alert.value}%
Threshold: ${alert.threshold}%
Status: ${alert.guest.status}
Time: ${new Date(alert.timestamp).toLocaleString()}

Description: ${alert.rule.description}

This alert was generated by Pulse monitoring system.
        `;

        const mailOptions = {
            from: channel.config.from,
            to: recipients.join(', '),
            subject: subject,
            text: text,
            html: html
        };

        await this.emailTransporter.sendMail(mailOptions);
        console.log(`[EMAIL] Alert sent to: ${recipients.join(', ')}`);
    }

    /**
     * Send webhook notification
     */
    async sendWebhookNotification(channel, alert) {
        if (!channel.config.url) {
            throw new Error('Webhook URL not configured');
        }

        const severityEmoji = {
            'info': '💙',
            'warning': '⚠️',
            'critical': '🚨'
        };

        const payload = {
            timestamp: new Date(alert.triggeredAt || alert.lastUpdate || Date.now()).toISOString(),
            alert: {
                id: alert.id,
                rule: {
                    name: alert.rule.name,
                    description: alert.rule.description,
                    severity: alert.rule.severity,
                    metric: alert.rule.metric
                },
                guest: {
                    name: alert.guest.name,
                    id: alert.guest.id,
                    type: alert.guest.type,
                    node: alert.guest.node,
                    status: alert.guest.status
                },
                value: alert.value,
                threshold: alert.threshold,
                emoji: severityEmoji[alert.rule.severity] || '📢'
            },
            // Discord/Slack compatible format
            embeds: [{
                title: `${severityEmoji[alert.rule.severity] || '📢'} ${alert.rule.name}`,
                description: alert.rule.description,
                color: alert.rule.severity === 'critical' ? 15158332 : // Red
                       alert.rule.severity === 'warning' ? 15844367 : // Orange
                       3447003, // Blue
                fields: [
                    {
                        name: 'VM/LXC',
                        value: `${alert.guest.name} (${alert.guest.type} ${alert.guest.id})`,
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
                        value: `${alert.value}%`,
                        inline: true
                    },
                    {
                        name: 'Threshold',
                        value: `${alert.threshold}%`,
                        inline: true
                    }
                ],
                footer: {
                    text: 'Pulse Monitoring System'
                },
                timestamp: new Date(alert.timestamp).toISOString()
            }],
            // Slack compatible format
            text: `${severityEmoji[alert.rule.severity] || '📢'} *${alert.rule.name}*`,
            attachments: [{
                color: alert.rule.severity === 'critical' ? 'danger' :
                       alert.rule.severity === 'warning' ? 'warning' : 'good',
                fields: [
                    {
                        title: 'VM/LXC',
                        value: `${alert.guest.name} (${alert.guest.type} ${alert.guest.id})`,
                        short: true
                    },
                    {
                        title: 'Node',
                        value: alert.guest.node,
                        short: true
                    },
                    {
                        title: 'Metric',
                        value: `${alert.rule.metric.toUpperCase()}: ${alert.value}% (threshold: ${alert.threshold}%)`,
                        short: false
                    }
                ],
                footer: 'Pulse Monitoring',
                ts: Math.floor(alert.timestamp / 1000)
            }]
        };

        // Set appropriate headers
        const headers = {
            'Content-Type': 'application/json',
            'User-Agent': 'Pulse-Monitoring/1.0',
            ...channel.config.headers
        };

        try {
            const response = await axios.post(channel.config.url, payload, {
                headers,
                timeout: 10000, // 10 second timeout
                maxRedirects: 3
            });

            console.log(`[WEBHOOK] Alert sent to: ${channel.config.url} (${response.status})`);
        } catch (error) {
            if (error.response) {
                throw new Error(`Webhook failed: ${error.response.status} ${error.response.statusText}`);
            } else if (error.request) {
                throw new Error(`Webhook failed: No response from ${channel.config.url}`);
            } else {
                throw new Error(`Webhook failed: ${error.message}`);
            }
        }
    }

    destroy() {
        if (this.cleanupInterval) {
            clearInterval(this.cleanupInterval);
        }
        if (this.escalationInterval) {
            clearInterval(this.escalationInterval);
        }
        this.removeAllListeners();
        this.activeAlerts.clear();
        this.alertRules.clear();
        this.acknowledgedAlerts.clear();
        this.suppressedAlerts.clear();
        
        // Close email transporter
        if (this.emailTransporter) {
            this.emailTransporter.close();
        }
    }
}

module.exports = AlertManager; 