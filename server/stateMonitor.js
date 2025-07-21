const fs = require('fs');
const fsPromises = require('fs').promises;
const path = require('path');
const { createLogger } = require('./utils/logger');

const logger = createLogger('StateMonitor');

class StateMonitor {
    constructor(dataDir = '/data') {
        this.dataDir = dataDir;
        this.previousStates = new Map();
        // Initialize with default rules immediately
        this.stateRules = {
            vm_down: {
                enabled: true,
                from: ['running', 'online'],
                to: ['stopped', 'offline', 'paused'],
                notify: 'on_change',
                message: '{name} has stopped'
            },
            vm_up: {
                enabled: false,
                from: ['stopped', 'offline', 'paused'],
                to: ['running', 'online'],
                notify: 'on_change',
                message: '{name} has started'
            },
            node_down: {
                enabled: true,
                from: ['online'],
                to: ['offline', 'unknown'],
                notify: 'on_change',
                message: 'Node {name} is offline'
            },
            node_up: {
                enabled: false,
                from: ['offline', 'unknown'],
                to: ['online'],
                notify: 'on_change',
                message: 'Node {name} is back online'
            }
        };
        // Then try to load from file synchronously (will override defaults)
        this.loadStateRulesSync();
    }

    loadStateRulesSync() {
        try {
            const rulesPath = path.join(this.dataDir, 'alert-rules.json');
            console.log(`[StateMonitor] Loading state rules from: ${rulesPath}`);
            const rulesData = fs.readFileSync(rulesPath, 'utf8');
            const rules = JSON.parse(rulesData);
            
            // Extract state rules from the new format, or use defaults
            console.log(`[StateMonitor] Loaded rules object:`, JSON.stringify(rules, null, 2));
            this.stateRules = rules.states || {
                vm_down: {
                    enabled: true,
                    from: ['running', 'online'],
                    to: ['stopped', 'offline', 'paused'],
                    notify: 'on_change',
                    message: '{name} has stopped'
                },
                vm_up: {
                    enabled: false,
                    from: ['stopped', 'offline', 'paused'],
                    to: ['running', 'online'],
                    notify: 'on_change',
                    message: '{name} has started'
                }
            };
            
            logger.info('Loaded state rules', { rules: Object.keys(this.stateRules) });
            console.log(`[StateMonitor] Successfully loaded ${Object.keys(this.stateRules).length} state rules`);
        } catch (error) {
            logger.warn('Failed to load state rules, using defaults', { error: error.message });
            console.log(`[StateMonitor] Failed to load state rules: ${error.message}`);
            // Keep the default rules that were already set in constructor
        }
    }

    async loadStateRules() {
        try {
            const rulesPath = path.join(this.dataDir, 'alert-rules.json');
            const rulesData = await fsPromises.readFile(rulesPath, 'utf8');
            const rules = JSON.parse(rulesData);
            
            // Extract state rules from the new format, or use defaults
            this.stateRules = rules.states || this.stateRules;
            
            logger.info('Loaded state rules', { rules: Object.keys(this.stateRules) });
        } catch (error) {
            logger.warn('Failed to load state rules, using defaults', { error: error.message });
        }
    }

    checkTransitions(guests) {
        const alerts = [];
        
        for (const guest of guests) {
            const prevState = this.previousStates.get(guest.vmid);
            const currState = (guest.status || '').toLowerCase();
            
            // Skip if no previous state (first time seeing this guest)
            if (!prevState) {
                this.previousStates.set(guest.vmid, currState);
                continue;
            }
            
            // Check if state changed
            if (prevState !== currState) {
                console.log(`[StateMonitor] State transition detected for ${guest.name}: ${prevState} -> ${currState}`);
                logger.debug('State transition detected', { 
                    guest: guest.name, 
                    from: prevState, 
                    to: currState 
                });
                
                // Check each state rule
                for (const [ruleName, rule] of Object.entries(this.stateRules)) {
                    if (!rule.enabled) continue;
                    
                    // Check if this transition matches the rule
                    if (this.matchesTransition(prevState, currState, rule)) {
                        const alert = this.createStateAlert(guest, prevState, currState, ruleName, rule);
                        alerts.push(alert);
                    }
                }
            }
            
            this.previousStates.set(guest.vmid, currState);
        }
        
        return alerts;
    }

    checkNodeTransitions(nodes) {
        const alerts = [];
        
        for (const node of nodes) {
            const nodeId = `node-${node.node}`;
            const prevState = this.previousStates.get(nodeId);
            const currState = node.status === 'online' ? 'online' : 'offline';
            
            // Skip if no previous state (first time seeing this node)
            if (!prevState) {
                this.previousStates.set(nodeId, currState);
                continue;
            }
            
            // Check if state changed
            if (prevState !== currState) {
                logger.info('Node state transition detected', { 
                    node: node.node, 
                    from: prevState, 
                    to: currState 
                });
                
                const alert = {
                    id: `node-state-${node.node}-${Date.now()}`,
                    type: 'node_state_change',
                    rule: currState === 'offline' ? 'node_down' : 'node_up',
                    nodeId: node.node,
                    nodeName: node.node,
                    from: prevState,
                    to: currState,
                    message: `Node ${node.node} is now ${currState}`,
                    severity: currState === 'offline' ? 'critical' : 'info',
                    timestamp: Date.now(),
                    group: 'availability_alerts'
                };
                
                alerts.push(alert);
            }
            
            this.previousStates.set(nodeId, currState);
        }
        
        return alerts;
    }

    matchesTransition(fromState, toState, rule) {
        const fromMatches = rule.from.includes(fromState);
        const toMatches = rule.to.includes(toState);
        return fromMatches && toMatches;
    }

    createStateAlert(guest, fromState, toState, ruleName, rule) {
        const message = rule.message
            .replace('{name}', guest.name)
            .replace('{from}', fromState)
            .replace('{to}', toState)
            .replace('{type}', guest.type === 'lxc' ? 'Container' : 'VM');
            
        return {
            id: `state-${guest.vmid}-${ruleName}-${Date.now()}`,
            type: 'state_change',
            rule: ruleName,
            guest: {
                vmid: guest.vmid,
                name: guest.name,
                type: guest.type,
                node: guest.node,
                endpointId: guest.endpointId,
                status: guest.status
            },
            from: fromState,
            to: toState,
            message: message,
            severity: ruleName === 'vm_down' ? 'critical' : 'info',
            timestamp: Date.now(),
            group: 'availability_alerts'
        };
    }

    // Save state rules back to storage
    async saveStateRules(rules) {
        try {
            const rulesPath = path.join(this.dataDir, 'alert-rules.json');
            
            // Load existing rules to preserve threshold rules
            let existingRules = {};
            try {
                const data = await fsPromises.readFile(rulesPath, 'utf8');
                existingRules = JSON.parse(data);
            } catch (error) {
                // File might not exist yet
            }
            
            // Update state rules
            existingRules.states = rules;
            
            await fsPromises.writeFile(rulesPath, JSON.stringify(existingRules, null, 2));
            this.stateRules = rules;
            
            logger.info('Saved state rules');
        } catch (error) {
            logger.error('Failed to save state rules', { error: error.message });
            throw error;
        }
    }

    // Get current state rules for UI
    getStateRules() {
        return this.stateRules;
    }

    // Clear previous states (useful for testing)
    clearStates() {
        this.previousStates.clear();
    }
}

module.exports = StateMonitor;