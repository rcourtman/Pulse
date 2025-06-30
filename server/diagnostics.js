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

        // Add summary for UI
        report.summary = {
            hasIssues: report.recommendations.some(r => r.severity === 'critical' || r.severity === 'warning'),
            criticalIssues: report.recommendations.filter(r => r.severity === 'critical').length,
            warnings: report.recommendations.filter(r => r.severity === 'warning').length,
            info: report.recommendations.filter(r => r.severity === 'info').length,
            isTimingIssue: report.state.loadTimeout || (report.state.serverUptime < 60 && (!report.state.guests || report.state.guests.total === 0))
        };

        // Return sanitized report by default for privacy
        return this.sanitizeReport(report);
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
                    name: this.sanitizeUrl(pve.name),
                    // Remove potentially sensitive fields, keep only structure info
                    tokenConfigured: pve.tokenConfigured,
                    selfSignedCerts: pve.selfSignedCerts
                }));
            }
            
            if (sanitized.configuration.pbs) {
                sanitized.configuration.pbs = sanitized.configuration.pbs.map((pbs, index) => ({
                    ...pbs,
                    host: this.sanitizeUrl(pbs.host),
                    name: this.sanitizeUrl(pbs.name),
                    // Sanitize node_name
                    node_name: (pbs.node_name === 'NOT SET' || pbs.node_name === 'auto-discovered') ? pbs.node_name : `pbs-node-${index + 1}`,
                    // Keep namespace if configured
                    namespace: pbs.namespace || null,
                    // Remove potentially sensitive fields, keep only structure info
                    tokenConfigured: pbs.tokenConfigured,
                    selfSignedCerts: pbs.selfSignedCerts
                }));
            }
        }
        
        // Sanitize permissions section
        if (sanitized.permissions) {
            if (sanitized.permissions.proxmox) {
                sanitized.permissions.proxmox = sanitized.permissions.proxmox.map((perm, permIndex) => ({
                    ...perm,
                    host: this.sanitizeUrl(perm.host),
                    name: this.sanitizeUrl(perm.name),
                    // Sanitize storage details if present
                    storageBackupAccess: perm.storageBackupAccess ? {
                        ...perm.storageBackupAccess,
                        storageDetails: perm.storageBackupAccess.storageDetails ? 
                            perm.storageBackupAccess.storageDetails.map((storage, idx) => ({
                                node: `node-${idx + 1}`,
                                storage: `storage-${permIndex + 1}-${idx + 1}`,
                                type: storage.type,
                                accessible: storage.accessible,
                                backupCount: storage.backupCount
                            })) : []
                    } : perm.storageBackupAccess,
                    // Keep diagnostic info but sanitize error messages
                    errors: perm.errors ? perm.errors.map(err => this.sanitizeErrorMessage(err)) : []
                }));
            }
            
            if (sanitized.permissions.pbs) {
                sanitized.permissions.pbs = sanitized.permissions.pbs.map((perm, index) => ({
                    ...perm,
                    host: this.sanitizeUrl(perm.host),
                    name: this.sanitizeUrl(perm.name),
                    // Sanitize node_name
                    node_name: (perm.node_name === 'NOT SET' || perm.node_name === 'auto-discovered') ? perm.node_name : `pbs-node-${index + 1}`,
                    // Keep namespace if configured
                    namespace: perm.namespace || null,
                    // Keep namespace test results
                    canListNamespaces: perm.canListNamespaces,
                    discoveredNamespaces: perm.discoveredNamespaces ? perm.discoveredNamespaces.length : 0,
                    // Sanitize namespace names but keep structure
                    namespaceAccess: perm.namespaceAccess ? Object.keys(perm.namespaceAccess).reduce((acc, ns, nsIdx) => {
                        const sanitizedNs = ns === 'root' ? 'root' : `namespace-${nsIdx}`;
                        acc[sanitizedNs] = {
                            ...perm.namespaceAccess[ns],
                            namespace: sanitizedNs
                        };
                        return acc;
                    }, {}) : {},
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
            
            // Sanitize storage debug information
            if (sanitized.state.storageDebug && sanitized.state.storageDebug.storageByNode) {
                sanitized.state.storageDebug.storageByNode = sanitized.state.storageDebug.storageByNode.map((nodeInfo, nodeIndex) => ({
                    node: `node-${nodeIndex + 1}`,
                    endpointId: nodeInfo.endpointId === 'primary' ? 'primary' : 'secondary',
                    storageCount: nodeInfo.storageCount,
                    storages: nodeInfo.storages.map((storage, storageIndex) => ({
                        name: `storage-${nodeIndex + 1}-${storageIndex + 1}`,
                        type: storage.type,
                        content: storage.content,
                        shared: storage.shared,
                        enabled: storage.enabled,
                        hasBackupContent: storage.hasBackupContent
                    }))
                }));
            }
        }
        
        // Sanitize PBS namespace info if present
        if (sanitized.state && sanitized.state.pbs && sanitized.state.pbs.namespaceInfo) {
            const sanitizedNamespaceInfo = {};
            Object.keys(sanitized.state.pbs.namespaceInfo).forEach((ns, idx) => {
                const sanitizedNs = ns === 'root' ? 'root' : `namespace-${idx}`;
                sanitizedNamespaceInfo[sanitizedNs] = {
                    ...sanitized.state.pbs.namespaceInfo[ns],
                    instances: sanitized.state.pbs.namespaceInfo[ns].instances || []
                };
            });
            sanitized.state.pbs.namespaceInfo = sanitizedNamespaceInfo;
        }
        
        // Sanitize namespace filtering debug info if present
        if (sanitized.state && sanitized.state.namespaceFilteringDebug) {
            if (sanitized.state.namespaceFilteringDebug.sharedNamespaces) {
                sanitized.state.namespaceFilteringDebug.sharedNamespaces = 
                    sanitized.state.namespaceFilteringDebug.sharedNamespaces.map((ns, idx) => ({
                        namespace: ns.namespace === 'root' ? 'root' : `namespace-${idx}`,
                        instances: ns.instances || [],
                        totalBackups: ns.totalBackups,
                        perInstanceCounts: ns.perInstanceCounts || {}
                    }));
            }
            if (sanitized.state.namespaceFilteringDebug.currentFilters) {
                const filters = sanitized.state.namespaceFilteringDebug.currentFilters;
                if (filters.namespace && filters.namespace !== 'all' && filters.namespace !== 'root') {
                    filters.namespace = 'namespace-filtered';
                }
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
            .replace(/"[^"]*\.local[^"]*"/g, '"[HOSTNAME]"')
            .replace(/namespaces?: ([a-zA-Z0-9-_]+(?:, [a-zA-Z0-9-_]+)*)/g, (match, namespaces) => {
                const nsList = namespaces.split(', ');
                const sanitizedList = nsList.map(ns => ns === 'root' ? 'root' : '[namespace]');
                return match.replace(namespaces, sanitizedList.join(', '));
            });
            
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
                    canListStorage: false,
                    canAccessStorageBackups: false,
                    storageBackupAccess: {
                        totalStoragesTested: 0,
                        accessibleStorages: 0,
                        storageDetails: []
                    },
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

                    if (permCheck.canListNodes && permCheck.nodeCount > 0) {
                        try {
                            const nodesData = await clientObj.client.get('/nodes');
                            
                            // Test storage listing on each node
                            let storageTestSuccessful = false;
                            let totalStoragesTested = 0;
                            let accessibleStorages = 0;
                            const storageDetails = [];
                            
                            for (const node of nodesData.data.data) {
                                if (node && node.node) {
                                    try {
                                        // Test storage listing endpoint
                                        const storageData = await clientObj.client.get(`/nodes/${node.node}/storage`);
                                        if (storageData && storageData.data && Array.isArray(storageData.data.data)) {
                                            storageTestSuccessful = true;
                                            
                                            // Test backup content access on each storage that supports backups
                                            for (const storage of storageData.data.data) {
                                                if (storage && storage.storage && storage.content && 
                                                    storage.content.includes('backup') && storage.type !== 'pbs') {
                                                    totalStoragesTested++;
                                                    
                                                    try {
                                                        // This is the critical test - accessing backup content requires PVEDatastoreAdmin
                                                        const backupData = await clientObj.client.get(
                                                            `/nodes/${node.node}/storage/${storage.storage}/content`,
                                                            { params: { content: 'backup' } }
                                                        );
                                                        
                                                        if (backupData && backupData.data) {
                                                            accessibleStorages++;
                                                            storageDetails.push({
                                                                node: node.node,
                                                                storage: storage.storage,
                                                                type: storage.type,
                                                                accessible: true,
                                                                backupCount: backupData.data.data ? backupData.data.data.length : 0
                                                            });
                                                        }
                                                    } catch (storageError) {
                                                        // 403 errors are common here - this is what we want to detect
                                                        const is403 = storageError.response?.status === 403;
                                                        storageDetails.push({
                                                            node: node.node,
                                                            storage: storage.storage,
                                                            type: storage.type,
                                                            accessible: false,
                                                            error: is403 ? 'Permission denied (403) - needs PVEDatastoreAdmin role' : storageError.message
                                                        });
                                                        
                                                        if (is403) {
                                                            permCheck.errors.push(`Storage ${storage.storage} on ${node.node}: Permission denied accessing backup content. Token needs 'PVEDatastoreAdmin' role on '/storage'.`);
                                                        } else {
                                                            permCheck.errors.push(`Storage ${storage.storage} on ${node.node}: ${storageError.message}`);
                                                        }
                                                    }
                                                }
                                            }
                                        }
                                    } catch (nodeStorageError) {
                                        permCheck.errors.push(`Cannot list storage on node ${node.node}: ${nodeStorageError.message}`);
                                    }
                                }
                            }
                            
                            if (storageTestSuccessful) {
                                permCheck.canListStorage = true;
                            }
                            
                            permCheck.storageBackupAccess = {
                                totalStoragesTested,
                                accessibleStorages,
                                storageDetails: storageDetails.slice(0, 10) // Limit details for report size
                            };
                            
                            // Set overall storage backup access status
                            permCheck.canAccessStorageBackups = totalStoragesTested > 0 && accessibleStorages > 0;
                            
                        } catch (error) {
                            permCheck.errors.push(`Cannot test storage permissions: ${error.message}`);
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
                    node_name: clientObj.config?.nodeName || clientObj.config?.node_name || 'auto-discovered',
                    namespace: clientObj.config?.namespace || null,
                    canConnect: false,
                    canListDatastores: false,
                    canListBackups: false,
                    canListNamespaces: false,
                    namespaceAccess: {},
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
                            
                            // Test backup listing and namespace access on first datastore
                            const firstDatastore = datastoreData.data.data[0];
                            if (firstDatastore && firstDatastore.store) {
                                // Test namespace listing capability
                                try {
                                    const namespaceResponse = await clientObj.client.get(`/admin/datastore/${firstDatastore.store}/namespace`);
                                    if (namespaceResponse && namespaceResponse.data) {
                                        permCheck.canListNamespaces = true;
                                        const namespaces = namespaceResponse.data.data || [];
                                        permCheck.discoveredNamespaces = namespaces.map(ns => ns.ns || ns.path || ns.name).filter(ns => ns !== undefined);
                                    }
                                } catch (nsError) {
                                    if (nsError.response?.status !== 404) {
                                        permCheck.errors.push(`Cannot list namespaces in datastore ${firstDatastore.store}: ${nsError.message}`);
                                    }
                                    // 404 is expected on older PBS versions without namespace support
                                }
                                
                                // Test backup listing in configured namespace or root
                                try {
                                    const groupsParams = {};
                                    const namespacesToTest = [];
                                    
                                    // Always test root namespace
                                    namespacesToTest.push({ ns: '', label: 'root' });
                                    
                                    // Test configured namespace if present
                                    if (clientObj.config.namespace) {
                                        namespacesToTest.push({ ns: clientObj.config.namespace, label: 'configured' });
                                    }
                                    
                                    for (const nsTest of namespacesToTest) {
                                        try {
                                            const testParams = { ...groupsParams };
                                            if (nsTest.ns) {
                                                testParams.ns = nsTest.ns;
                                            }
                                            
                                            const backupData = await clientObj.client.get(`/admin/datastore/${firstDatastore.store}/groups`, {
                                                params: testParams
                                            });
                                            
                                            if (backupData && backupData.data) {
                                                permCheck.canListBackups = true;
                                                permCheck.namespaceAccess[nsTest.label] = {
                                                    namespace: nsTest.ns || 'root',
                                                    accessible: true,
                                                    backupCount: backupData.data.data ? backupData.data.data.length : 0
                                                };
                                            }
                                        } catch (nsBackupError) {
                                            permCheck.namespaceAccess[nsTest.label] = {
                                                namespace: nsTest.ns || 'root',
                                                accessible: false,
                                                error: nsBackupError.message
                                            };
                                        }
                                    }
                                    
                                    // Calculate total backup count from accessible namespaces
                                    permCheck.backupCount = Object.values(permCheck.namespaceAccess)
                                        .filter(ns => ns.accessible)
                                        .reduce((sum, ns) => sum + (ns.backupCount || 0), 0);
                                        
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
                        node_name: nodeName || 'auto-discovered',
                        namespace: clientObj.config.namespace || null,
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
                    sampleBackupIds: [],
                    instanceDetails: [], // Add array to store individual PBS instance details
                    namespaceInfo: {} // Track namespace usage
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

            // Add storage diagnostics
            if (state.nodes && Array.isArray(state.nodes)) {
                info.storageDebug = {
                    nodeCount: state.nodes.length,
                    storageByNode: []
                };
                
                state.nodes.forEach(node => {
                    const nodeStorage = {
                        node: node.node,
                        endpointId: node.endpointId,
                        storageCount: node.storage?.length || 0,
                        storages: []
                    };
                    
                    if (node.storage && Array.isArray(node.storage)) {
                        nodeStorage.storages = node.storage.map(s => ({
                            name: s.storage,
                            type: s.type,
                            content: s.content,
                            shared: s.shared,
                            enabled: s.enabled,
                            hasBackupContent: s.content?.includes('backup') || false
                        }));
                    }
                    
                    info.storageDebug.storageByNode.push(nodeStorage);
                });
            }
            
            // Add namespace filtering diagnostics
            if (state.pbs && Array.isArray(state.pbs) && state.pbs.length > 1) {
                info.namespaceFilteringDebug = {
                    multiplePbsInstances: true,
                    pbsInstanceCount: state.pbs.length,
                    sharedNamespaces: [],
                    currentFilters: {
                        namespace: state.backupsFilterNamespace || 'all',
                        pbsInstance: state.backupsFilterPbsInstance || 'all'
                    }
                };
                
                // Find namespaces that exist on multiple PBS instances
                Object.entries(info.pbs.namespaceInfo || {}).forEach(([namespace, nsInfo]) => {
                    if (nsInfo.instances && nsInfo.instances.length > 1) {
                        info.namespaceFilteringDebug.sharedNamespaces.push({
                            namespace: namespace,
                            instances: nsInfo.instances,
                            totalBackups: nsInfo.backupCount,
                            perInstanceCounts: nsInfo.instanceBackupCounts || {}
                        });
                    }
                });
            }
            
            // Count PBS backups and get samples
            if (state.pbs && Array.isArray(state.pbs)) {
                state.pbs.forEach((pbsInstance, idx) => {
                    // Store instance details for matching in recommendations
                    const instanceDetail = {
                        name: pbsInstance.pbsInstanceName || `pbs-${idx}`,
                        index: idx,
                        datastores: 0,
                        snapshots: 0,
                        namespaces: new Set(),
                        namespaceBackupCounts: {} // Track backup count per namespace
                    };
                    
                    if (pbsInstance.datastores) {
                        info.pbs.datastores += pbsInstance.datastores.length;
                        instanceDetail.datastores = pbsInstance.datastores.length;
                        
                        pbsInstance.datastores.forEach(ds => {
                            if (ds.snapshots) {
                                info.pbs.totalBackups += ds.snapshots.length;
                                instanceDetail.snapshots += ds.snapshots.length;
                                // Get unique backup IDs and track namespaces
                                ds.snapshots.forEach(snap => {
                                    const backupId = snap['backup-id'];
                                    if (backupId && !info.pbs.sampleBackupIds.includes(backupId)) {
                                        info.pbs.sampleBackupIds.push(backupId);
                                    }
                                    
                                    // Track namespace usage
                                    if (snap.ns !== undefined) {
                                        const namespace = snap.ns || 'root';
                                        instanceDetail.namespaces.add(namespace);
                                        
                                        // Track backup count per namespace for this instance
                                        if (!instanceDetail.namespaceBackupCounts[namespace]) {
                                            instanceDetail.namespaceBackupCounts[namespace] = 0;
                                        }
                                        instanceDetail.namespaceBackupCounts[namespace]++;
                                        
                                        // Track global namespace info
                                        if (!info.pbs.namespaceInfo[namespace]) {
                                            info.pbs.namespaceInfo[namespace] = {
                                                backupCount: 0,
                                                instances: new Set(),
                                                instanceBackupCounts: {} // Track per-instance counts
                                            };
                                        }
                                        info.pbs.namespaceInfo[namespace].backupCount++;
                                        info.pbs.namespaceInfo[namespace].instances.add(instanceDetail.name);
                                        info.pbs.namespaceInfo[namespace].instanceBackupCounts[instanceDetail.name] = 
                                            (info.pbs.namespaceInfo[namespace].instanceBackupCounts[instanceDetail.name] || 0) + 1;
                                    }
                                });
                            }
                        });
                    }
                    
                    // Convert Set to Array for JSON serialization
                    instanceDetail.namespaces = Array.from(instanceDetail.namespaces);
                    info.pbs.instanceDetails.push(instanceDetail);
                });
                
                // Convert namespace info Sets to Arrays for JSON serialization
                Object.keys(info.pbs.namespaceInfo).forEach(ns => {
                    info.pbs.namespaceInfo[ns].instances = Array.from(info.pbs.namespaceInfo[ns].instances);
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
                        if (!perm.canListStorage) {
                            report.recommendations.push({
                                severity: 'warning',
                                category: 'Proxmox Permissions',
                                message: `Proxmox "${perm.name}": Token cannot list storage. This may affect backup discovery. Ensure your API token has the 'Sys.Audit' permission on '/'.`
                            });
                        }
                        if (perm.canListStorage && !perm.canAccessStorageBackups) {
                            const storageAccess = perm.storageBackupAccess;
                            if (storageAccess && storageAccess.totalStoragesTested > 0) {
                                const inaccessibleStorages = storageAccess.totalStoragesTested - storageAccess.accessibleStorages;
                                if (inaccessibleStorages > 0) {
                                    report.recommendations.push({
                                        severity: 'critical',
                                        category: 'Proxmox Storage Permissions',
                                        message: `Proxmox "${perm.name}": Token cannot access backup content in ${inaccessibleStorages} of ${storageAccess.totalStoragesTested} backup-enabled storages. This prevents PVE backup discovery. Grant 'PVEDatastoreAdmin' role on '/storage' using: pveum acl modify /storage --tokens ${perm.id} --roles PVEDatastoreAdmin`
                                    });
                                }
                            } else {
                                report.recommendations.push({
                                    severity: 'info',
                                    category: 'Proxmox Storage',
                                    message: `Proxmox "${perm.name}": No backup-enabled storage found to test. If you have backup storage configured, ensure it has 'backup' in its content types.`
                                });
                            }
                        }
                        if (perm.canAccessStorageBackups && perm.storageBackupAccess) {
                            const storageAccess = perm.storageBackupAccess;
                            if (storageAccess.accessibleStorages > 0) {
                                const backupCount = storageAccess.storageDetails
                                    .filter(s => s.accessible)
                                    .reduce((sum, s) => sum + (s.backupCount || 0), 0);
                                
                                report.recommendations.push({
                                    severity: 'info',
                                    category: 'Backup Status',
                                    message: `Proxmox "${perm.name}": Successfully accessing ${storageAccess.accessibleStorages} backup storage(s) with ${backupCount} backup files (PBS storage excluded). Storage permissions are correctly configured.`
                                });
                            }
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
                    
                    // Node name is now auto-discovered, no need to check for it
                    
                    // Check namespace configuration and access
                    if (perm.namespace && perm.namespaceAccess) {
                        const configuredNsAccess = perm.namespaceAccess.configured;
                        if (configuredNsAccess && !configuredNsAccess.accessible) {
                            report.recommendations.push({
                                severity: 'critical',
                                category: 'PBS Namespace Access',
                                message: `PBS "${perm.name}": Cannot access configured namespace '${perm.namespace}'. Error: ${configuredNsAccess.error}. Verify the namespace exists and the token has permission.`
                            });
                        }
                    }
                    
                    // Check if namespaces are discovered but not configured
                    if (perm.canListNamespaces && perm.discoveredNamespaces && perm.discoveredNamespaces.length > 0 && !perm.namespace) {
                        report.recommendations.push({
                            severity: 'info',
                            category: 'PBS Namespace Configuration',
                            message: `PBS "${perm.name}": Found ${perm.discoveredNamespaces.length} namespace(s) but none configured. Available namespaces: ${perm.discoveredNamespaces.slice(0, 5).join(', ')}${perm.discoveredNamespaces.length > 5 ? '...' : ''}. Consider adding PBS_NAMESPACE to target a specific namespace.`
                        });
                    }
                    
                    // Add success message for PBS with backup counts and namespace info
                    if (perm.canConnect && perm.canListDatastores && report.state && report.state.pbs && report.state.pbs.instanceDetails) {
                        // Find the corresponding PBS instance in state data by matching name
                        const pbsStateData = report.state.pbs.instanceDetails.find(instance => 
                            instance.name === perm.name
                        );
                        
                        if (pbsStateData && pbsStateData.datastores > 0) {
                            let successMsg = `PBS "${perm.name}": Successfully accessing ${pbsStateData.datastores} datastore(s) with ${pbsStateData.snapshots} backup snapshots.`;
                            
                            // Add namespace info if available
                            if (perm.namespaceAccess && Object.keys(perm.namespaceAccess).length > 0) {
                                const accessibleNamespaces = Object.values(perm.namespaceAccess).filter(ns => ns.accessible);
                                if (accessibleNamespaces.length > 0) {
                                    const nsInfo = accessibleNamespaces.map(ns => 
                                        `${ns.namespace || 'root'} (${ns.backupCount} backups)`
                                    ).join(', ');
                                    successMsg += ` Namespace access: ${nsInfo}.`;
                                }
                            }
                            
                            successMsg += ' PBS permissions are correctly configured.';
                            
                            report.recommendations.push({
                                severity: 'info',
                                category: 'Backup Status',
                                message: successMsg
                            });
                        } else if (perm.canListDatastores && perm.datastoreCount > 0) {
                            // Fallback to permission data if state data not available
                            let successMsg = `PBS "${perm.name}": Successfully connected with ${perm.datastoreCount} datastore(s) accessible.`;
                            
                            // Add namespace info if available
                            if (perm.namespaceAccess && Object.keys(perm.namespaceAccess).length > 0) {
                                const accessibleNamespaces = Object.values(perm.namespaceAccess).filter(ns => ns.accessible);
                                if (accessibleNamespaces.length > 0) {
                                    successMsg += ` Can access ${accessibleNamespaces.length} namespace(s).`;
                                }
                            }
                            
                            successMsg += ' PBS permissions are correctly configured.';
                            
                            report.recommendations.push({
                                severity: 'info',
                                category: 'Backup Status',
                                message: successMsg
                            });
                        }
                    }
                });
            }
        }

        if (report.configuration && report.configuration.pbs && Array.isArray(report.configuration.pbs)) {
            // Node name is now auto-discovered, no need to check for it
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
            
            // Check for storage discovery issues
            if (report.state.pveBackups.backupTasks > 0 && report.state.pveBackups.storageBackups === 0) {
                // We have backup tasks but no storage backups found
                let storageIssue = false;
                let hasBackupStorage = false;
                
                if (report.state.storageDebug && report.state.storageDebug.storageByNode) {
                    report.state.storageDebug.storageByNode.forEach(nodeInfo => {
                        const backupStorages = nodeInfo.storages.filter(s => s.hasBackupContent && s.type !== 'pbs');
                        if (backupStorages.length > 0) {
                            hasBackupStorage = true;
                        }
                    });
                }
                
                if (hasBackupStorage) {
                    report.recommendations.push({
                        severity: 'warning',
                        category: 'Storage Access',
                        message: `Found ${report.state.pveBackups.backupTasks} backup tasks but 0 storage backups. This suggests backup files exist but cannot be read. Check that the Pulse API user has 'Datastore.Audit' or 'Datastore.AllocateSpace' permissions on your backup storage.`
                    });
                } else {
                    report.recommendations.push({
                        severity: 'info',
                        category: 'Storage Configuration',
                        message: `Found ${report.state.pveBackups.backupTasks} backup tasks but no non-PBS storage configured for backups. If you're using PBS exclusively, this is normal. Otherwise, check your storage configuration.`
                    });
                }
            }
            
            // If no PBS configured but PVE backups exist, that's fine
            if ((!report.state.pbs || report.state.pbs.instances === 0) && totalPveBackups > 0) {
                report.recommendations.push({
                    severity: 'info',
                    category: 'Backup Status',
                    message: `Found ${totalPveBackups} PVE backups and ${totalPveSnapshots} VM/CT snapshots. Note: PBS is not configured, showing only local PVE backups.`
                });
            }
        }

        // Check namespace filtering for multiple PBS instances
        if (report.state && report.state.namespaceFilteringDebug) {
            const debug = report.state.namespaceFilteringDebug;
            if (debug.multiplePbsInstances && debug.sharedNamespaces.length > 0) {
                report.recommendations.push({
                    severity: 'info',
                    category: 'PBS Namespace Filtering',
                    message: `Multiple PBS instances detected (${debug.pbsInstanceCount}) with shared namespaces. Shared namespaces: ${debug.sharedNamespaces.map(ns => `${ns.namespace} (${ns.instances.join(', ')})`).join(', ')}. When filtering by namespace, backups from ALL PBS instances with that namespace will be shown.`
                });
                
                // Add detailed namespace backup distribution info
                if (debug.currentFilters.namespace !== 'all' && debug.currentFilters.namespace !== null) {
                    const namespaceData = debug.sharedNamespaces.find(ns => ns.namespace === debug.currentFilters.namespace);
                    if (namespaceData && namespaceData.instances.length > 1) {
                        const breakdown = Object.entries(namespaceData.perInstanceCounts || {})
                            .map(([instance, count]) => `${instance}: ${count}`)
                            .join(', ');
                        report.recommendations.push({
                            severity: 'info',
                            category: 'PBS Namespace Filter Active',
                            message: `Currently filtering by namespace "${debug.currentFilters.namespace}" which exists on ${namespaceData.instances.length} PBS instances. Backup distribution: ${breakdown}. Total backups in this namespace: ${namespaceData.totalBackups}.`
                        });
                    }
                }
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
        
        // Handle URLs that may not have protocol
        let urlToParse = url;
        if (!url.includes('://')) {
            urlToParse = 'https://' + url;
        }
        
        try {
            const parsed = new URL(urlToParse);
            // Anonymize hostname/IP but keep protocol and port structure
            const port = parsed.port || (parsed.protocol === 'https:' ? '443' : '80');
            
            // Check if hostname is an IP address
            const isIP = /^(?:\d{1,3}\.){3}\d{1,3}$/.test(parsed.hostname);
            const anonymizedHost = isIP ? 'REDACTED-IP' : 'REDACTED-HOST';
            
            // Only include port if it's non-standard
            if ((parsed.protocol === 'https:' && port === '443') || 
                (parsed.protocol === 'http:' && port === '80')) {
                return `${parsed.protocol}//${anonymizedHost}`;
            }
            return `${parsed.protocol}//${anonymizedHost}:${port}`;
        } catch {
            // Fallback for malformed URLs - sanitize more aggressively
            return url
                .replace(/\/\/[^:]+:[^@]+@/, '//REDACTED:REDACTED@')
                .replace(/\b(?:\d{1,3}\.){3}\d{1,3}\b/g, 'REDACTED-IP')
                .replace(/:[0-9]{2,5}/g, ':PORT')
                .replace(/[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*/g, 'REDACTED-HOST');
        }
    }
}

module.exports = DiagnosticTool;