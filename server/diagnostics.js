/**
 * Fixed diagnostic tool for troubleshooting Pulse configuration
 */

const fs = require('fs');
const path = require('path');

class DiagnosticTool {
    constructor(stateManager, metricsHistory, apiClients, pbsApiClients) {
        this.stateManager = stateManager;
        this.metricsHistory = metricsHistory;
        this.apiClients = apiClients || {};
        this.pbsApiClients = pbsApiClients || {};
    }

    async runDiagnostics() {
        const report = {
            timestamp: new Date().toISOString(),
            version: 'unknown',
            configuration: { proxmox: [], pbs: [] },
            state: {},
            permissions: { proxmox: [], pbs: [] },
            recommendations: []
        };

        try {
            report.version = this.getVersion();
        } catch (e) {
            console.error('Error getting version:', e);
        }

        try {
            report.configuration = this.getConfiguration();
        } catch (e) {
            console.error('Error getting configuration:', e);
            report.configuration = { proxmox: [], pbs: [] };
        }

        try {
            report.permissions = await this.checkPermissions();
        } catch (e) {
            console.error('Error checking permissions:', e);
            report.permissions = { proxmox: [], pbs: [] };
        }

        try {
            report.state = this.getStateInfo();
            
            // Check if we need to wait for data
            const state = this.stateManager.getState();
            const hasData = (state.vms && state.vms.length > 0) || (state.containers && state.containers.length > 0) || 
                           (state.nodes && state.nodes.length > 0);
            
            // If server has been running for more than 2 minutes, don't wait
            if (report.state.serverUptime > 120 || hasData) {
                console.log('[Diagnostics] Data already available or server has been running long enough');
                // Data should already be loaded, just use current state
            } else {
                // Only wait if server just started AND no data has loaded yet
                console.log('[Diagnostics] No data loaded yet, waiting for first discovery cycle...');
                
                const maxWaitTime = 30000; // Only wait up to 30 seconds
                const checkInterval = 500;
                const startTime = Date.now();
                
                while ((Date.now() - startTime) < maxWaitTime) {
                    const currentState = this.stateManager.getState();
                    const nowHasData = (currentState.vms && currentState.vms.length > 0) || 
                                      (currentState.containers && currentState.containers.length > 0) ||
                                      (currentState.nodes && currentState.nodes.length > 0);
                    if (nowHasData) {
                        console.log('[Diagnostics] Data loaded after', Math.floor((Date.now() - startTime) / 1000), 'seconds');
                        report.state = this.getStateInfo();
                        break;
                    }
                    await new Promise(resolve => setTimeout(resolve, checkInterval));
                }
                
                // If still no data after waiting
                const finalState = this.stateManager.getState();
                const finalHasData = (finalState.vms && finalState.vms.length > 0) || 
                                    (finalState.containers && finalState.containers.length > 0);
                if (!finalHasData) {
                    console.log('[Diagnostics] No data after waiting', Math.floor((Date.now() - startTime) / 1000), 'seconds');
                    report.state.loadTimeout = true;
                    report.state.waitTime = Math.floor((Date.now() - startTime) / 1000);
                }
            }
        } catch (e) {
            console.error('Error getting state:', e);
            report.state = { error: e.message };
        }

        // Generate recommendations
        try {
            this.generateRecommendations(report);
        } catch (e) {
            console.error('Error generating recommendations:', e);
        }

        // Return unsanitized report - sanitization will be done client-side for copy/download
        return report;
    }

    sanitizeReport(report) {
        // Deep clone the report to avoid modifying the original
        const sanitized = JSON.parse(JSON.stringify(report));
        
        // Sanitize configuration section
        if (sanitized.configuration) {
            if (sanitized.configuration.proxmox) {
                sanitized.configuration.proxmox = sanitized.configuration.proxmox.map(pve => ({
                    ...pve,
                    host: this.sanitizeUrl(pve.host),
                    // Remove potentially sensitive fields, keep only structure info
                    tokenConfigured: pve.tokenConfigured,
                    selfSignedCerts: pve.selfSignedCerts
                }));
            }
            
            if (sanitized.configuration.pbs) {
                sanitized.configuration.pbs = sanitized.configuration.pbs.map(pbs => ({
                    ...pbs,
                    host: this.sanitizeUrl(pbs.host),
                    // Remove potentially sensitive fields, keep only structure info
                    tokenConfigured: pbs.tokenConfigured,
                    selfSignedCerts: pbs.selfSignedCerts,
                    node_name: pbs.node_name
                }));
            }
        }
        
        // Sanitize permissions section
        if (sanitized.permissions) {
            if (sanitized.permissions.proxmox) {
                sanitized.permissions.proxmox = sanitized.permissions.proxmox.map(perm => ({
                    ...perm,
                    host: this.sanitizeUrl(perm.host),
                    name: this.sanitizeUrl(perm.name),
                    // Keep diagnostic info but sanitize error messages
                    errors: perm.errors ? perm.errors.map(err => this.sanitizeErrorMessage(err)) : []
                }));
            }
            
            if (sanitized.permissions.pbs) {
                sanitized.permissions.pbs = sanitized.permissions.pbs.map(perm => ({
                    ...perm,
                    host: this.sanitizeUrl(perm.host),
                    name: this.sanitizeUrl(perm.name),
                    // Keep diagnostic info but sanitize error messages
                    errors: perm.errors ? perm.errors.map(err => this.sanitizeErrorMessage(err)) : []
                }));
            }
        }
        
        // Sanitize state section
        if (sanitized.state) {
            // Remove potentially sensitive node names, keep only counts and structure
            if (sanitized.state.nodes && sanitized.state.nodes.names) {
                sanitized.state.nodes.names = sanitized.state.nodes.names.map((name, index) => `node-${index + 1}`);
            }
            
            // Remove specific backup IDs, keep only counts
            if (sanitized.state.pbs && sanitized.state.pbs.sampleBackupIds) {
                sanitized.state.pbs.sampleBackupIds = sanitized.state.pbs.sampleBackupIds.map((id, index) => `backup-${index + 1}`);
            }
        }
        
        // Sanitize recommendations
        if (sanitized.recommendations) {
            sanitized.recommendations = sanitized.recommendations.map(rec => ({
                ...rec,
                message: this.sanitizeRecommendationMessage(rec.message)
            }));
        }
        
        // Add notice about sanitization
        sanitized._sanitized = {
            notice: "This diagnostic report has been sanitized for safe sharing. Hostnames, IPs, node names, and backup IDs have been anonymized while preserving structural information needed for troubleshooting.",
            timestamp: new Date().toISOString()
        };
        
        return sanitized;
    }
    
    sanitizeErrorMessage(errorMsg) {
        if (!errorMsg) return errorMsg;
        
        // Remove potential IP addresses, hostnames, and ports
        let sanitized = errorMsg
            .replace(/\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/g, '[IP-ADDRESS]')
            .replace(/https?:\/\/[^\/\s:]+(?::\d+)?/g, '[HOSTNAME]')
            .replace(/([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}/g, '[HOSTNAME]')
            .replace(/:\d{4,5}\b/g, ':[PORT]');
            
        return sanitized;
    }
    
    sanitizeRecommendationMessage(message) {
        if (!message) return message;
        
        // Remove potential hostnames and IPs from recommendation messages
        let sanitized = message
            .replace(/\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/g, '[IP-ADDRESS]')
            .replace(/https?:\/\/[^\/\s:]+(?::\d+)?/g, '[HOSTNAME]')
            .replace(/([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}/g, '[HOSTNAME]')
            .replace(/"[^"]*\.lan[^"]*"/g, '"[HOSTNAME]"')
            .replace(/"[^"]*\.local[^"]*"/g, '"[HOSTNAME]"');
            
        return sanitized;
    }

    getVersion() {
        try {
            const packagePath = path.join(__dirname, '..', 'package.json');
            const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
            return packageJson.version || 'unknown';
        } catch (error) {
            return 'unknown';
        }
    }

    async checkPermissions() {
        const permissions = {
            proxmox: [],
            pbs: []
        };

        // Check Proxmox permissions
        for (const [id, clientObj] of Object.entries(this.apiClients)) {
            if (!id.startsWith('pbs_') && clientObj && clientObj.client) {
                const permCheck = {
                    id: id,
                    name: clientObj.config?.name || id,
                    host: clientObj.config?.host,
                    canConnect: false,
                    canListNodes: false,
                    canListVMs: false,
                    canListContainers: false,
                    canGetNodeStats: false,
                    errors: []
                };

                try {
                    // Test basic connection and version endpoint
                    const versionData = await clientObj.client.get('/version');
                    if (versionData && versionData.data) {
                        permCheck.canConnect = true;
                        permCheck.version = versionData.data.version;
                    }
                } catch (error) {
                    permCheck.errors.push(`Connection failed: ${error.message}`);
                }

                if (permCheck.canConnect) {
                    // Test node listing permission
                    try {
                        const nodesData = await clientObj.client.get('/nodes');
                        if (nodesData && nodesData.data && Array.isArray(nodesData.data.data)) {
                            permCheck.canListNodes = true;
                            permCheck.nodeCount = nodesData.data.data.length;
                        }
                    } catch (error) {
                        permCheck.errors.push(`Cannot list nodes: ${error.message}`);
                    }

                    // Test VM listing permission using the same method as the actual app
                    if (permCheck.canListNodes && permCheck.nodeCount > 0) {
                        try {
                            const nodesData = await clientObj.client.get('/nodes');
                            let totalVMs = 0;
                            let vmCheckSuccessful = false;
                            
                            // Test VM listing on each node (same as the actual app)
                            for (const node of nodesData.data.data) {
                                if (node && node.node) {
                                    try {
                                        const vmData = await clientObj.client.get(`/nodes/${node.node}/qemu`);
                                        if (vmData && vmData.data) {
                                            vmCheckSuccessful = true;
                                            totalVMs += vmData.data.data ? vmData.data.data.length : 0;
                                        }
                                    } catch (nodeError) {
                                        permCheck.errors.push(`Cannot list VMs on node ${node.node}: ${nodeError.message}`);
                                    }
                                }
                            }
                            
                            if (vmCheckSuccessful) {
                                permCheck.canListVMs = true;
                                permCheck.vmCount = totalVMs;
                            }
                        } catch (error) {
                            permCheck.errors.push(`Cannot list VMs: ${error.message}`);
                        }
                    } else {
                        permCheck.errors.push('Cannot test VM listing: No nodes available');
                    }

                    // Test Container listing permission using the same method as the actual app
                    if (permCheck.canListNodes && permCheck.nodeCount > 0) {
                        try {
                            const nodesData = await clientObj.client.get('/nodes');
                            let totalContainers = 0;
                            let containerCheckSuccessful = false;
                            
                            // Test container listing on each node (same as the actual app)
                            for (const node of nodesData.data.data) {
                                if (node && node.node) {
                                    try {
                                        const lxcData = await clientObj.client.get(`/nodes/${node.node}/lxc`);
                                        if (lxcData && lxcData.data) {
                                            containerCheckSuccessful = true;
                                            totalContainers += lxcData.data.data ? lxcData.data.data.length : 0;
                                        }
                                    } catch (nodeError) {
                                        permCheck.errors.push(`Cannot list containers on node ${node.node}: ${nodeError.message}`);
                                    }
                                }
                            }
                            
                            if (containerCheckSuccessful) {
                                permCheck.canListContainers = true;
                                permCheck.containerCount = totalContainers;
                            }
                        } catch (error) {
                            permCheck.errors.push(`Cannot list containers: ${error.message}`);
                        }
                    } else {
                        permCheck.errors.push('Cannot test container listing: No nodes available');
                    }

                    // Test node stats permission (pick first node if available)
                    if (permCheck.canListNodes && permCheck.nodeCount > 0) {
                        try {
                            const nodesData = await clientObj.client.get('/nodes');
                            const firstNode = nodesData.data.data[0];
                            if (firstNode && firstNode.node) {
                                const statsData = await clientObj.client.get(`/nodes/${firstNode.node}/status`);
                                if (statsData && statsData.data) {
                                    permCheck.canGetNodeStats = true;
                                }
                            }
                        } catch (error) {
                            permCheck.errors.push(`Cannot get node stats: ${error.message}`);
                        }
                    }
                }

                permissions.proxmox.push(permCheck);
            }
        }

        // Check PBS permissions
        for (const [id, clientObj] of Object.entries(this.pbsApiClients)) {
            if (clientObj && clientObj.client) {
                const permCheck = {
                    id: id,
                    name: clientObj.config?.name || id,
                    host: clientObj.config?.host,
                    node_name: clientObj.config?.nodeName || clientObj.config?.node_name || 'NOT SET',
                    canConnect: false,
                    canListDatastores: false,
                    canListBackups: false,
                    errors: []
                };

                try {
                    // Test basic connection using the correct PBS API endpoint
                    const versionData = await clientObj.client.get('/version');
                    if (versionData && versionData.data) {
                        permCheck.canConnect = true;
                        permCheck.version = versionData.data.data?.version || versionData.data.version;
                    }
                } catch (error) {
                    permCheck.errors.push(`Connection failed: ${error.message}`);
                }

                if (permCheck.canConnect) {
                    // Test datastore listing permission using the primary endpoint the app uses
                    try {
                        const datastoreData = await clientObj.client.get('/status/datastore-usage');
                        if (datastoreData && datastoreData.data && Array.isArray(datastoreData.data.data)) {
                            permCheck.canListDatastores = true;
                            permCheck.datastoreCount = datastoreData.data.data.length;
                            
                            // Test backup listing on first datastore
                            const firstDatastore = datastoreData.data.data[0];
                            if (firstDatastore && firstDatastore.store) {
                                try {
                                    const backupData = await clientObj.client.get(`/admin/datastore/${firstDatastore.store}/groups`);
                                    if (backupData && backupData.data) {
                                        permCheck.canListBackups = true;
                                        permCheck.backupCount = backupData.data.data ? backupData.data.data.length : 0;
                                    }
                                } catch (error) {
                                    permCheck.errors.push(`Cannot list backup groups in datastore ${firstDatastore.store}: ${error.message}`);
                                }
                            }
                        }
                    } catch (error) {
                        // Try fallback endpoint
                        try {
                            const configData = await clientObj.client.get('/config/datastore');
                            if (configData && configData.data && Array.isArray(configData.data.data)) {
                                permCheck.canListDatastores = true;
                                permCheck.datastoreCount = configData.data.data.length;
                            }
                        } catch (fallbackError) {
                            permCheck.errors.push(`Cannot list datastores: ${error.message}`);
                        }
                    }
                }

                permissions.pbs.push(permCheck);
            }
        }

        return permissions;
    }

    getConfiguration() {
        const config = {
            proxmox: [],
            pbs: [],
            alerts: {
                cpu: {
                    enabled: process.env.ALERT_CPU_ENABLED !== 'false',
                    threshold: process.env.ALERT_CPU_THRESHOLD || '85'
                },
                memory: {
                    enabled: process.env.ALERT_MEMORY_ENABLED !== 'false',
                    threshold: process.env.ALERT_MEMORY_THRESHOLD || '90'
                },
                disk: {
                    enabled: process.env.ALERT_DISK_ENABLED !== 'false',
                    threshold: process.env.ALERT_DISK_THRESHOLD || '95'
                }
            }
        };

        // Get Proxmox configurations
        try {
            Object.entries(this.apiClients).forEach(([id, clientObj]) => {
                if (!id.startsWith('pbs_') && clientObj && clientObj.config) {
                    config.proxmox.push({
                        id: id,
                        host: clientObj.config.host,
                        name: clientObj.config.name || id,
                        port: clientObj.config.port || '8006',
                        tokenConfigured: !!clientObj.config.tokenId,
                        selfSignedCerts: clientObj.config.allowSelfSignedCerts || false
                    });
                }
            });
        } catch (e) {
            console.error('Error getting Proxmox config:', e);
        }

        // Get PBS configurations
        try {
            Object.entries(this.pbsApiClients).forEach(([id, clientObj]) => {
                if (clientObj && clientObj.config) {
                    const nodeName = clientObj.config.nodeName || clientObj.config.node_name;
                    config.pbs.push({
                        id: id,
                        host: clientObj.config.host,
                        name: clientObj.config.name || id,
                        port: clientObj.config.port || '8007',
                        node_name: nodeName || 'NOT SET',
                        tokenConfigured: !!clientObj.config.tokenId,
                        selfSignedCerts: clientObj.config.allowSelfSignedCerts || false
                    });
                }
            });
        } catch (e) {
            console.error('Error getting PBS config:', e);
        }

        return config;
    }

    getStateInfo() {
        try {
            const state = this.stateManager.getState();
            const stats = this.stateManager.getPerformanceStats ? this.stateManager.getPerformanceStats() : {};
            
            // Find the actual last update time
            const lastUpdateTime = state.lastUpdate || state.stats?.lastUpdated || null;
            
            const info = {
                lastUpdate: lastUpdateTime,
                serverUptime: process.uptime(),
                dataAge: lastUpdateTime ? Math.floor((Date.now() - new Date(lastUpdateTime).getTime()) / 1000) : null,
                nodes: {
                    count: state.nodes?.length || 0,
                    names: state.nodes?.map(n => n.node || n.name).slice(0, 5) || []
                },
                guests: {
                    total: (state.vms?.length || 0) + (state.containers?.length || 0),
                    vms: state.vms?.length || 0,
                    containers: state.containers?.length || 0,
                    running: ((state.vms?.filter(v => v.status === 'running') || []).length + 
                             (state.containers?.filter(c => c.status === 'running') || []).length),
                    stopped: ((state.vms?.filter(v => v.status === 'stopped') || []).length + 
                             (state.containers?.filter(c => c.status === 'stopped') || []).length)
                },
                pbs: {
                    instances: state.pbs?.length || 0,
                    totalBackups: 0,
                    datastores: 0,
                    sampleBackupIds: []
                },
                pveBackups: {
                    backupTasks: state.pveBackups?.backupTasks?.length || 0,
                    storageBackups: state.pveBackups?.storageBackups?.length || 0,
                    guestSnapshots: state.pveBackups?.guestSnapshots?.length || 0
                },
                performance: {
                    lastDiscoveryTime: stats.lastDiscoveryCycleTime || 'N/A',
                    lastMetricsTime: stats.lastMetricsCycleTime || 'N/A'
                },
                alerts: {
                    active: this.stateManager.alertManager?.getActiveAlerts ? 
                        this.stateManager.alertManager.getActiveAlerts().length : 0
                }
            };

            // Count PBS backups and get samples
            if (state.pbs && Array.isArray(state.pbs)) {
                state.pbs.forEach((pbsInstance, idx) => {
                    if (pbsInstance.datastores) {
                        info.pbs.datastores += pbsInstance.datastores.length;
                        pbsInstance.datastores.forEach(ds => {
                            if (ds.snapshots) {
                                info.pbs.totalBackups += ds.snapshots.length;
                                // Get unique backup IDs
                                ds.snapshots.forEach(snap => {
                                    const backupId = snap['backup-id'];
                                    if (backupId && !info.pbs.sampleBackupIds.includes(backupId)) {
                                        info.pbs.sampleBackupIds.push(backupId);
                                    }
                                });
                            }
                        });
                    }
                });
                // Limit sample backup IDs
                info.pbs.sampleBackupIds = info.pbs.sampleBackupIds.slice(0, 10);
            }

            return info;
        } catch (e) {
            console.error('Error getting state info:', e);
            return {
                error: e.message,
                lastUpdate: 'unknown',
                nodes: { count: 0 },
                guests: { total: 0 },
                pbs: { instances: 0 }
            };
        }
    }

    generateRecommendations(report) {
        // Check permission test results
        if (report.permissions) {
            // Check Proxmox permissions
            if (report.permissions.proxmox && Array.isArray(report.permissions.proxmox)) {
                report.permissions.proxmox.forEach(perm => {
                    if (!perm.canConnect) {
                        report.recommendations.push({
                            severity: 'critical',
                            category: 'Proxmox Connection',
                            message: `Cannot connect to Proxmox "${perm.name}" at ${perm.host}. Check your host, credentials, and network connectivity. Errors: ${perm.errors.join(', ')}`
                        });
                    } else {
                        // Check individual permissions
                        if (!perm.canListNodes) {
                            report.recommendations.push({
                                severity: 'critical',
                                category: 'Proxmox Permissions',
                                message: `Proxmox "${perm.name}": Token cannot list nodes. Ensure your API token has the 'Sys.Audit' permission on '/'.`
                            });
                        }
                        if (!perm.canListVMs) {
                            report.recommendations.push({
                                severity: 'critical',
                                category: 'Proxmox Permissions', 
                                message: `Proxmox "${perm.name}": Token cannot list VMs. Ensure your API token has the 'VM.Audit' permission on '/'.`
                            });
                        }
                        if (!perm.canListContainers) {
                            report.recommendations.push({
                                severity: 'critical',
                                category: 'Proxmox Permissions',
                                message: `Proxmox "${perm.name}": Token cannot list containers. Ensure your API token has the 'VM.Audit' permission on '/'.`
                            });
                        }
                        if (!perm.canGetNodeStats) {
                            report.recommendations.push({
                                severity: 'warning',
                                category: 'Proxmox Permissions',
                                message: `Proxmox "${perm.name}": Token cannot get node statistics. This may affect metrics collection. Ensure your API token has the 'Sys.Audit' permission on '/'.`
                            });
                        }
                    }
                });
            }

            // Check PBS permissions
            if (report.permissions.pbs && Array.isArray(report.permissions.pbs)) {
                report.permissions.pbs.forEach(perm => {
                    if (!perm.canConnect) {
                        report.recommendations.push({
                            severity: 'critical',
                            category: 'PBS Connection',
                            message: `Cannot connect to PBS "${perm.name}" at ${perm.host}. Check your host, credentials, and network connectivity. Errors: ${perm.errors.join(', ')}`
                        });
                    } else {
                        if (!perm.canListDatastores) {
                            report.recommendations.push({
                                severity: 'critical',
                                category: 'PBS Permissions',
                                message: `PBS "${perm.name}": Token cannot list datastores. Ensure your API token has the 'Datastore.Audit' permission.`
                            });
                        }
                        if (!perm.canListBackups && perm.canListDatastores) {
                            report.recommendations.push({
                                severity: 'warning',
                                category: 'PBS Permissions',
                                message: `PBS "${perm.name}": Token can list datastores but not backup snapshots. This may affect backup overview functionality.`
                            });
                        }
                    }
                    
                    if (perm.node_name === 'NOT SET') {
                        report.recommendations.push({
                            severity: 'critical',
                            category: 'PBS Configuration',
                            message: `PBS instance "${perm.name}" is missing PBS_NODE_NAME. This is required for the backups tab to work. SSH to your PBS server and run 'hostname' to get the correct value, then add PBS_NODE_NAME=<hostname> to your .env file.`
                        });
                    }
                });
            }
        }

        // Check PBS configuration (fallback if permissions not available)
        if (report.configuration && report.configuration.pbs && Array.isArray(report.configuration.pbs)) {
            report.configuration.pbs.forEach((pbs, index) => {
            if (pbs.node_name === 'NOT SET') {
                // Only add if we haven't already added this from permissions check
                const alreadyAdded = report.recommendations.some(r => 
                    r.category === 'PBS Configuration' && r.message.includes(pbs.name)
                );
                if (!alreadyAdded) {
                    report.recommendations.push({
                        severity: 'critical',
                        category: 'PBS Configuration',
                        message: `PBS instance "${pbs.name}" is missing PBS_NODE_NAME. This is required for the backups tab to work. SSH to your PBS server and run 'hostname' to get the correct value, then add PBS_NODE_NAME=<hostname> to your .env file.`
                    });
                }
            }
        });
        }

        // Check if there are backups but no guests
        if (report.state && report.state.pbs && report.state.guests) {
            if (report.state.pbs.totalBackups > 0 && report.state.guests.total === 0) {
                // Check if it's just a timing issue
                if (report.state.loadTimeout) {
                    report.recommendations.push({
                        severity: 'critical',
                        category: 'Discovery Issue',
                        message: `No data loaded after waiting ${report.state.waitTime}s. The discovery cycle is not completing. Check server logs for errors with Proxmox API connections.`
                    });
                } else if (report.state.dataAge === null) {
                    const uptime = Math.floor(report.state.serverUptime || 0);
                    // This shouldn't happen now since we wait for data
                    report.recommendations.push({
                        severity: 'warning',
                        category: 'Unexpected State',
                        message: `Data loading state is unclear (server uptime: ${uptime}s). Try running diagnostics again.`
                    });
                } else if (report.state.serverUptime < 60) {
                    report.recommendations.push({
                        severity: 'info',
                        category: 'Data Loading',
                        message: `Server recently started (${Math.floor(report.state.serverUptime || 0)}s ago). Data may still be loading. Please wait a moment and try again.`
                    });
                } else {
                    report.recommendations.push({
                        severity: 'critical',
                        category: 'Data Issue',
                        message: 'PBS has backups but no VMs/containers are detected. Check if your Proxmox API token has proper permissions to list VMs and containers.'
                    });
                }
            }

            // Check if PBS is configured but no backups found
            if (report.state.pbs.instances > 0 && report.state.pbs.totalBackups === 0) {
                report.recommendations.push({
                    severity: 'warning',
                    category: 'PBS Data',
                    message: 'PBS is configured but no backups were found. Verify that backups exist in your PBS datastores and that the API token has permission to read them.'
                });
            }
        }
        
        // Check PVE backups
        if (report.state && report.state.pveBackups) {
            const totalPveBackups = (report.state.pveBackups.backupTasks || 0) + 
                                  (report.state.pveBackups.storageBackups || 0);
            const totalPveSnapshots = report.state.pveBackups.guestSnapshots || 0;
            
            // If no PBS configured but PVE backups exist, that's fine
            if ((!report.state.pbs || report.state.pbs.instances === 0) && totalPveBackups > 0) {
                report.recommendations.push({
                    severity: 'info',
                    category: 'Backup Status',
                    message: `Found ${totalPveBackups} PVE backups and ${totalPveSnapshots} VM/CT snapshots. Note: PBS is not configured, showing only local PVE backups.`
                });
            }
        }

        // Check guest count
        if (report.state && report.state.guests && report.state.nodes) {
            if (report.state.guests.total === 0 && report.state.nodes.count > 0) {
                // Only add this recommendation if we haven't already identified it as a timing/loading issue
                const hasTimingRec = report.recommendations.some(r => 
                    r.category === 'Data Loading' || r.category === 'Discovery Issue'
                );
                
                if (!hasTimingRec) {
                    report.recommendations.push({
                        severity: 'warning',
                        category: 'Proxmox Data',
                        message: 'No VMs or containers found despite having Proxmox nodes. This could be a permissions issue with your Proxmox API token.'
                    });
                }
            }
        }

        // Add success message if everything looks good
        if (report.recommendations.length === 0) {
            report.recommendations.push({
                severity: 'info',
                category: 'Status',
                message: 'Configuration appears to be correct. If you\'re still experiencing issues, check the application logs for errors.'
            });
        }
    }

    sanitizeUrl(url) {
        if (!url) return 'Not configured';
        try {
            const parsed = new URL(url);
            // Anonymize hostname/IP but keep protocol and port structure
            const port = parsed.port || (parsed.protocol === 'https:' ? '443' : '80');
            
            // Check if hostname is an IP address
            const isIP = /^(?:\d{1,3}\.){3}\d{1,3}$/.test(parsed.hostname);
            const anonymizedHost = isIP ? '[IP-ADDRESS]' : '[HOSTNAME]';
            
            return `${parsed.protocol}//${anonymizedHost}:${port}`;
        } catch {
            // Fallback for malformed URLs
            return url
                .replace(/\/\/[^:]+:[^@]+@/, '//[USER]:[PASSWORD]@')
                .replace(/\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/g, '[IP-ADDRESS]')
                .replace(/([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}/g, '[HOSTNAME]');
        }
    }
}

module.exports = DiagnosticTool;