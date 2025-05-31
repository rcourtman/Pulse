const fs = require('fs').promises;
const path = require('path');

/**
 * Custom Threshold Manager
 * Handles per-VM/LXC threshold configurations
 */
class CustomThresholdManager {
    constructor() {
        this.configPath = path.join(__dirname, '../data/custom-thresholds.json');
        this.cache = new Map(); // endpointId:nodeId:vmid -> thresholds
        this.initialized = false;
    }

    /**
     * Initialize the threshold manager
     */
    async init() {
        try {
            await this.loadThresholds();
            this.initialized = true;
            console.log('[CustomThresholds] Initialized successfully');
        } catch (error) {
            console.error('[CustomThresholds] Initialization failed:', error);
        }
    }

    /**
     * Load thresholds from storage
     */
    async loadThresholds() {
        try {
            // Ensure data directory exists
            await fs.mkdir(path.dirname(this.configPath), { recursive: true });
            
            const data = await fs.readFile(this.configPath, 'utf8');
            const thresholds = JSON.parse(data);
            
            // Clear cache and rebuild
            this.cache.clear();
            
            Object.entries(thresholds).forEach(([key, config]) => {
                this.cache.set(key, config);
            });
            
            console.log(`[CustomThresholds] Loaded ${this.cache.size} threshold configurations`);
        } catch (error) {
            if (error.code === 'ENOENT') {
                // File doesn't exist yet, create empty structure
                await this.saveThresholds();
                console.log('[CustomThresholds] Created new threshold configuration file');
            } else {
                console.error('[CustomThresholds] Error loading thresholds:', error);
                throw error;
            }
        }
    }

    /**
     * Save thresholds to storage
     */
    async saveThresholds() {
        try {
            const data = Object.fromEntries(this.cache);
            await fs.writeFile(this.configPath, JSON.stringify(data, null, 2), 'utf8');
            console.log('[CustomThresholds] Saved threshold configurations');
        } catch (error) {
            console.error('[CustomThresholds] Error saving thresholds:', error);
            throw error;
        }
    }

    /**
     * Generate cache key for threshold lookup
     * Note: We use endpointId:vmid (without nodeId) to support VM migration within clusters
     */
    generateKey(endpointId, nodeId, vmid) {
        return `${endpointId}:${vmid}`;
    }

    /**
     * Get thresholds for a specific VM/LXC
     * Returns custom thresholds if configured, otherwise returns null
     */
    getThresholds(endpointId, nodeId, vmid) {
        const key = this.generateKey(endpointId, nodeId, vmid);
        return this.cache.get(key) || null;
    }

    /**
     * Set custom thresholds for a VM/LXC
     */
    async setThresholds(endpointId, nodeId, vmid, thresholds) {
        try {
            const key = this.generateKey(endpointId, nodeId, vmid);
            
            // Validate threshold structure
            const validatedThresholds = this.validateThresholds(thresholds);
            
            this.cache.set(key, {
                endpointId,
                nodeId,
                vmid,
                thresholds: validatedThresholds,
                enabled: true,
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            });
            
            await this.saveThresholds();
            return true;
        } catch (error) {
            console.error('[CustomThresholds] Error setting thresholds:', error);
            throw error;
        }
    }

    /**
     * Remove custom thresholds for a VM/LXC
     */
    async removeThresholds(endpointId, nodeId, vmid) {
        try {
            const key = this.generateKey(endpointId, nodeId, vmid);
            
            if (this.cache.has(key)) {
                this.cache.delete(key);
                await this.saveThresholds();
                return true;
            }
            return false;
        } catch (error) {
            console.error('[CustomThresholds] Error removing thresholds:', error);
            throw error;
        }
    }

    /**
     * Get all custom threshold configurations
     */
    getAllThresholds() {
        return Array.from(this.cache.values());
    }

    /**
     * Get thresholds for a specific endpoint
     */
    getThresholdsByEndpoint(endpointId) {
        return Array.from(this.cache.values()).filter(config => config.endpointId === endpointId);
    }

    /**
     * Validate threshold configuration
     */
    validateThresholds(thresholds) {
        const validated = {};
        
        // CPU thresholds
        if (thresholds.cpu) {
            validated.cpu = {
                warning: this.validateThresholdValue(thresholds.cpu.warning, 'cpu.warning'),
                critical: this.validateThresholdValue(thresholds.cpu.critical, 'cpu.critical')
            };
            
            // Ensure critical > warning
            if (validated.cpu.critical <= validated.cpu.warning) {
                throw new Error('CPU critical threshold must be greater than warning threshold');
            }
        }
        
        // Memory thresholds
        if (thresholds.memory) {
            validated.memory = {
                warning: this.validateThresholdValue(thresholds.memory.warning, 'memory.warning'),
                critical: this.validateThresholdValue(thresholds.memory.critical, 'memory.critical')
            };
            
            // Ensure critical > warning
            if (validated.memory.critical <= validated.memory.warning) {
                throw new Error('Memory critical threshold must be greater than warning threshold');
            }
        }
        
        // Disk thresholds
        if (thresholds.disk) {
            validated.disk = {
                warning: this.validateThresholdValue(thresholds.disk.warning, 'disk.warning'),
                critical: this.validateThresholdValue(thresholds.disk.critical, 'disk.critical')
            };
            
            // Ensure critical > warning
            if (validated.disk.critical <= validated.disk.warning) {
                throw new Error('Disk critical threshold must be greater than warning threshold');
            }
        }
        
        return validated;
    }

    /**
     * Validate individual threshold value
     */
    validateThresholdValue(value, fieldName) {
        if (typeof value !== 'number') {
            throw new Error(`${fieldName} must be a number`);
        }
        
        if (value < 0 || value > 100) {
            throw new Error(`${fieldName} must be between 0 and 100`);
        }
        
        return value;
    }

    /**
     * Bulk import thresholds from configuration object
     */
    async importThresholds(thresholdConfigs) {
        try {
            let imported = 0;
            
            for (const config of thresholdConfigs) {
                await this.setThresholds(
                    config.endpointId,
                    config.nodeId,
                    config.vmid,
                    config.thresholds
                );
                imported++;
            }
            
            console.log(`[CustomThresholds] Imported ${imported} threshold configurations`);
            return imported;
        } catch (error) {
            console.error('[CustomThresholds] Error importing thresholds:', error);
            throw error;
        }
    }

    /**
     * Export all thresholds for backup/migration
     */
    exportThresholds() {
        return {
            version: '1.0',
            exportedAt: new Date().toISOString(),
            thresholds: this.getAllThresholds()
        };
    }

    /**
     * Enable/disable custom thresholds for a VM/LXC
     */
    async toggleThresholds(endpointId, nodeId, vmid, enabled) {
        try {
            const key = this.generateKey(endpointId, nodeId, vmid);
            const config = this.cache.get(key);
            
            if (!config) {
                throw new Error('Threshold configuration not found');
            }
            
            config.enabled = enabled;
            config.updatedAt = new Date().toISOString();
            
            this.cache.set(key, config);
            await this.saveThresholds();
            
            return true;
        } catch (error) {
            console.error('[CustomThresholds] Error toggling thresholds:', error);
            throw error;
        }
    }

    /**
     * Get threshold statistics
     */
    getStatistics() {
        const configs = this.getAllThresholds();
        const enabled = configs.filter(c => c.enabled);
        
        return {
            total: configs.length,
            enabled: enabled.length,
            disabled: configs.length - enabled.length,
            byEndpoint: this.groupByEndpoint(configs),
            lastUpdated: configs.length > 0 ? Math.max(...configs.map(c => new Date(c.updatedAt).getTime())) : null
        };
    }

    /**
     * Group configurations by endpoint
     */
    groupByEndpoint(configs) {
        const grouped = {};
        
        configs.forEach(config => {
            if (!grouped[config.endpointId]) {
                grouped[config.endpointId] = {
                    total: 0,
                    enabled: 0
                };
            }
            
            grouped[config.endpointId].total++;
            if (config.enabled) {
                grouped[config.endpointId].enabled++;
            }
        });
        
        return grouped;
    }
}

// Create singleton instance
const customThresholdManager = new CustomThresholdManager();

module.exports = customThresholdManager;