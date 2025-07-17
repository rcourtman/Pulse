const EventEmitter = require('events');

class WebhookBatcher extends EventEmitter {
    constructor(options = {}) {
        super();
        this.queue = [];
        this.batchTimeout = null;
        this.processing = false;
        
        // Configuration
        this.config = {
            batchWindowMs: options.batchWindowMs || 5000, // 5 second window to collect alerts
            maxBatchSize: options.maxBatchSize || 10,
            summaryThreshold: options.summaryThreshold || 3, // Switch to summary mode after this many alerts
            priorityDelay: options.priorityDelay || 0, // Critical alerts bypass batching
            normalDelay: options.normalDelay || 10000, // 10 seconds between normal announcements
            ...options
        };
    }

    /**
     * Add an alert to the batch queue
     */
    async queueAlert(alert, webhookUrl) {
        const priority = this.calculatePriority(alert);
        
        // Critical alerts bypass the queue
        if (priority === 'critical' && this.config.priorityDelay === 0) {
            console.log(`[WebhookBatcher] Critical alert ${alert.id} - sending immediately`);
            return await this.sendSingleWebhook(alert, webhookUrl);
        }
        
        // Add to queue
        this.queue.push({
            alert,
            webhookUrl,
            priority,
            queuedAt: Date.now()
        });
        
        console.log(`[WebhookBatcher] Alert ${alert.id} queued (priority: ${priority}). Queue size: ${this.queue.length}`);
        
        // Start or reset batch timer
        this.startBatchTimer();
        
        // If queue is full, process immediately
        if (this.queue.length >= this.config.maxBatchSize) {
            console.log('[WebhookBatcher] Queue full - processing batch immediately');
            this.processBatch();
        }
    }

    /**
     * Calculate alert priority
     */
    calculatePriority(alert) {
        // Critical conditions
        if (alert.guest?.status === 'stopped' || alert.guest?.status === 'error') {
            return 'critical';
        }
        
        // Check metric values for critical thresholds
        const metric = alert.metric || alert.rule?.metric;
        const value = alert.currentValue;
        
        if (metric === 'cpu' && value > 95) return 'critical';
        if (metric === 'memory' && value > 95) return 'critical';
        if (metric === 'disk' && value > 95) return 'critical';
        if (alert.type === 'down_alert') return 'critical';
        
        // High priority
        if (value > 90) return 'high';
        
        // Normal priority
        return 'normal';
    }

    /**
     * Start or reset the batch timer
     */
    startBatchTimer() {
        if (this.batchTimeout) {
            clearTimeout(this.batchTimeout);
        }
        
        // If batch window is 0, process immediately
        if (this.config.batchWindowMs === 0) {
            this.processBatch();
            return;
        }
        
        this.batchTimeout = setTimeout(() => {
            this.processBatch();
        }, this.config.batchWindowMs);
    }

    /**
     * Process the current batch
     */
    async processBatch() {
        if (this.processing || this.queue.length === 0) {
            return;
        }
        
        this.processing = true;
        clearTimeout(this.batchTimeout);
        this.batchTimeout = null;
        
        // Sort by priority
        const batch = [...this.queue].sort((a, b) => {
            const priorityOrder = { critical: 0, high: 1, normal: 2 };
            return priorityOrder[a.priority] - priorityOrder[b.priority];
        });
        
        this.queue = []; // Clear queue
        
        console.log(`[WebhookBatcher] Processing batch of ${batch.length} alerts`);
        
        try {
            if (batch.length >= this.config.summaryThreshold) {
                // Send summary webhook
                await this.sendSummaryWebhook(batch);
            } else {
                // Send individual webhooks with delays
                for (let i = 0; i < batch.length; i++) {
                    const item = batch[i];
                    await this.sendSingleWebhook(item.alert, item.webhookUrl);
                    
                    if (i < batch.length - 1) {
                        const delay = item.priority === 'critical' ? 
                            this.config.priorityDelay : 
                            this.config.normalDelay;
                        
                        if (delay > 0) {
                            console.log(`[WebhookBatcher] Waiting ${delay/1000}s before next webhook...`);
                            await new Promise(resolve => setTimeout(resolve, delay));
                        }
                    }
                }
            }
        } catch (error) {
            console.error('[WebhookBatcher] Error processing batch:', error);
        } finally {
            this.processing = false;
            
            // If new alerts were queued while processing, start a new batch
            if (this.queue.length > 0) {
                this.startBatchTimer();
            }
        }
    }

    /**
     * Send a single webhook
     */
    async sendSingleWebhook(alert, webhookUrl) {
        // This will be called by AlertManager's sendDirectWebhookNotification
        this.emit('send', alert, webhookUrl);
    }

    /**
     * Send a summary webhook for multiple alerts
     */
    async sendSummaryWebhook(batch) {
        const axios = require('axios');
        const webhookUrl = batch[0].webhookUrl; // All should have the same URL
        
        // Group alerts by type
        const summary = {
            total: batch.length,
            critical: batch.filter(b => b.priority === 'critical').length,
            byType: {},
            byNode: {},
            alerts: []
        };
        
        // Analyze the batch
        batch.forEach(item => {
            const alert = item.alert;
            const node = alert.guest?.node || 'unknown';
            
            // Handle bundled alerts differently
            if (alert.metric === 'bundled' && alert.exceededMetrics) {
                // Extract actual metrics from bundled alert
                alert.exceededMetrics.forEach(exceeded => {
                    const metricType = exceeded.metricType || 'unknown';
                    summary.byType[metricType] = (summary.byType[metricType] || 0) + 1;
                });
                
                // Include bundled alert details
                summary.alerts.push({
                    id: alert.id,
                    name: alert.guest?.name || 'Unknown',
                    metrics: alert.exceededMetrics.map(m => `${m.metricType}: ${m.currentValue}%`).join(', '),
                    priority: item.priority,
                    isBundled: true
                });
            } else {
                // Regular alert
                const metric = alert.metric || alert.rule?.metric || 'unknown';
                summary.byType[metric] = (summary.byType[metric] || 0) + 1;
                
                summary.alerts.push({
                    id: alert.id,
                    name: alert.guest?.name || 'Unknown',
                    metric: metric,
                    value: alert.currentValue || alert.value,
                    priority: item.priority
                });
            }
            
            // Count by node
            summary.byNode[node] = (summary.byNode[node] || 0) + 1;
        });
        
        // Create summary payload
        const summaryAlert = {
            id: `summary-${Date.now()}`,
            type: 'summary',
            rule: {
                name: 'Multiple Alerts Summary',
                description: `${summary.total} alerts triggered simultaneously`
            },
            guest: {
                name: 'System Summary',
                type: 'summary',
                node: 'multiple'
            },
            summary: summary,
            value: summary.total,
            threshold: this.config.summaryThreshold,
            isSummary: true
        };
        
        try {
            console.log(`[WebhookBatcher] Sending summary webhook for ${summary.total} alerts`);
            console.log(`[WebhookBatcher] Summary byType:`, JSON.stringify(summary.byType));
            console.log(`[WebhookBatcher] Summary alerts sample:`, JSON.stringify(summary.alerts.slice(0, 2)));
            
            const response = await axios.post(webhookUrl, {
                timestamp: new Date().toISOString(),
                alert: summaryAlert,
                isBatch: true,
                batchSize: summary.total
            }, {
                timeout: 10000,
                headers: {
                    'Content-Type': 'application/json',
                    'User-Agent': 'Pulse-Alert-System/1.0'
                }
            });
            
            console.log(`[WebhookBatcher] Summary webhook sent successfully (${response.status})`);
            
            // Mark all alerts as webhook sent
            batch.forEach(item => {
                this.emit('sent', item.alert.id);
            });
            
        } catch (error) {
            console.error('[WebhookBatcher] Failed to send summary webhook:', error.message);
            throw error;
        }
    }
}

module.exports = WebhookBatcher;