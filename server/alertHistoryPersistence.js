const fs = require('fs').promises;
const path = require('path');
const fileLock = require('./utils/fileLock');

class AlertHistoryPersistence {
    constructor() {
        this.historyFile = path.join(__dirname, '../data/alert-history.json');
        this.fileLock = fileLock;
        this.maxHistoryDays = 7; // Keep 7 days of history
        this.maxHistorySize = 5000; // Maximum number of alerts to keep
    }

    /**
     * Load alert history from disk
     */
    async loadHistory() {
        try {
            return await this.fileLock.withLock(this.historyFile, async () => {
                try {
                    const data = await fs.readFile(this.historyFile, 'utf-8');
                    const history = JSON.parse(data);
                    
                    // Validate and clean history
                    const validHistory = Array.isArray(history) ? 
                        history.filter(alert => this.validateHistoryAlert(alert)) : [];
                    
                    console.log(`[AlertHistory] Loaded ${validHistory.length} historical alerts`);
                    return validHistory;
                } catch (error) {
                    if (error.code === 'ENOENT') {
                        // File doesn't exist yet, return empty array
                        return [];
                    }
                    throw error;
                }
            });
        } catch (error) {
            console.error('[AlertHistory] Error loading history:', error);
            return [];
        }
    }

    /**
     * Save alert history to disk
     */
    async saveHistory(history) {
        try {
            return await this.fileLock.withLock(this.historyFile, async () => {
                // Ensure data directory exists
                const dataDir = path.dirname(this.historyFile);
                await fs.mkdir(dataDir, { recursive: true });
                
                // Clean old entries before saving
                const cleanedHistory = this.cleanOldEntries(history);
                
                // Limit size
                const limitedHistory = cleanedHistory.slice(0, this.maxHistorySize);
                
                await fs.writeFile(
                    this.historyFile,
                    JSON.stringify(limitedHistory, null, 2),
                    'utf-8'
                );
                
                return limitedHistory.length;
            });
        } catch (error) {
            console.error('[AlertHistory] Error saving history:', error);
            throw error;
        }
    }

    /**
     * Add an alert to history
     */
    async addToHistory(alert) {
        try {
            // Load current history
            const history = await this.loadHistory();
            
            // Create history entry
            const historyEntry = this.createHistoryEntry(alert);
            
            // Check if alert already exists (update instead of duplicate)
            const existingIndex = history.findIndex(h => h.id === historyEntry.id);
            if (existingIndex >= 0) {
                history[existingIndex] = historyEntry;
            } else {
                // Add to beginning
                history.unshift(historyEntry);
            }
            
            // Save updated history
            await this.saveHistory(history);
            
            return historyEntry;
        } catch (error) {
            console.error('[AlertHistory] Error adding to history:', error);
            throw error;
        }
    }

    /**
     * Update an alert in history (e.g., when resolved)
     */
    async updateInHistory(alertId, updates) {
        try {
            const history = await this.loadHistory();
            
            const index = history.findIndex(h => h.id === alertId);
            if (index >= 0) {
                history[index] = { ...history[index], ...updates };
                
                // If the alert is being resolved, ensure resolved flag is set
                if (updates.resolvedAt || updates.state === 'resolved') {
                    history[index].resolved = true;
                }
                
                await this.saveHistory(history);
                return history[index];
            }
            
            return null;
        } catch (error) {
            console.error('[AlertHistory] Error updating history:', error);
            throw error;
        }
    }

    /**
     * Create a history entry from an alert
     */
    createHistoryEntry(alert) {
        return {
            id: alert.id,
            ruleId: alert.rule?.id || alert.ruleId,
            ruleName: alert.rule?.name || 'Unknown',
            message: alert.message || this.generateMessage(alert),
            metric: alert.rule?.metric || alert.metric,
            threshold: alert.effectiveThreshold || alert.threshold,
            currentValue: alert.currentValue,
            guest: {
                name: alert.guest?.name || alert.guestName,
                vmid: alert.guest?.vmid,
                node: alert.guest?.node || alert.nodeId,
                type: alert.guest?.type,
                endpointId: alert.guest?.endpointId
            },
            triggeredAt: alert.triggeredAt || Date.now(),
            resolvedAt: alert.resolvedAt || null,
            resolved: alert.state === 'resolved' || alert.resolved || false,
            acknowledged: alert.acknowledged || false,
            acknowledgedAt: alert.acknowledgedAt || null,
            acknowledgedBy: alert.acknowledgedBy || null,
            duration: alert.duration || (alert.resolvedAt ? alert.resolvedAt - alert.triggeredAt : null),
            severity: alert.severity || 'warning'
        };
    }

    /**
     * Generate a message for alerts without one
     */
    generateMessage(alert) {
        if (alert.message) return alert.message;
        
        const guestName = alert.guest?.name || alert.guestName || 'Unknown';
        const metric = alert.rule?.metric || alert.metric || 'metric';
        const value = alert.currentValue || 'N/A';
        const threshold = alert.effectiveThreshold || alert.threshold || 'N/A';
        
        return `${guestName}: ${metric} ${value} exceeded threshold ${threshold}`;
    }

    /**
     * Validate a history alert entry
     */
    validateHistoryAlert(alert) {
        if (!alert || typeof alert !== 'object') return false;
        if (!alert.id) return false;
        if (!alert.triggeredAt || isNaN(new Date(alert.triggeredAt).getTime())) return false;
        return true;
    }

    /**
     * Clean entries older than maxHistoryDays
     */
    cleanOldEntries(history) {
        const cutoffTime = Date.now() - (this.maxHistoryDays * 24 * 60 * 60 * 1000);
        
        return history.filter(alert => {
            // Keep if triggered within the cutoff time
            if (alert.triggeredAt >= cutoffTime) return true;
            
            // Also keep if resolved within the cutoff time
            if (alert.resolvedAt && alert.resolvedAt >= cutoffTime) return true;
            
            return false;
        });
    }

    /**
     * Get statistics about the history
     */
    async getStats() {
        try {
            const history = await this.loadHistory();
            const now = Date.now();
            const oneDayAgo = now - 86400000;
            const oneWeekAgo = now - 604800000;
            
            return {
                total: history.length,
                resolved: history.filter(a => a.resolved).length,
                active: history.filter(a => !a.resolved).length,
                acknowledged: history.filter(a => a.acknowledged).length,
                last24h: history.filter(a => a.triggeredAt >= oneDayAgo).length,
                lastWeek: history.filter(a => a.triggeredAt >= oneWeekAgo).length,
                averageDuration: this.calculateAverageDuration(history)
            };
        } catch (error) {
            console.error('[AlertHistory] Error getting stats:', error);
            return null;
        }
    }

    /**
     * Calculate average alert duration
     */
    calculateAverageDuration(history) {
        const resolvedAlerts = history.filter(a => a.resolved && a.duration);
        if (resolvedAlerts.length === 0) return 0;
        
        const totalDuration = resolvedAlerts.reduce((sum, a) => sum + a.duration, 0);
        return Math.round(totalDuration / resolvedAlerts.length);
    }

    /**
     * Clean up and optimize history file
     */
    async cleanup() {
        try {
            const history = await this.loadHistory();
            const beforeCount = history.length;
            
            // This will clean old entries and enforce size limit
            const savedCount = await this.saveHistory(history);
            
            console.log(`[AlertHistory] Cleanup complete: ${beforeCount} -> ${savedCount} alerts`);
            return { before: beforeCount, after: savedCount };
        } catch (error) {
            console.error('[AlertHistory] Cleanup error:', error);
            return null;
        }
    }
}

module.exports = new AlertHistoryPersistence();