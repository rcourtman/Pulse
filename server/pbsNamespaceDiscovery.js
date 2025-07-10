/**
 * PBS Namespace Discovery Module
 * Discovers namespaces using the PBS API namespace list endpoint
 */

// Cache for discovered namespaces per PBS instance with size limit
const namespaceCache = new Map();
const CACHE_TTL = 300000; // 5 minutes cache TTL
const CACHE_MAX_SIZE = 100; // Maximum number of cached namespace entries

// Clean up old cache entries periodically
setInterval(() => {
    const now = Date.now();
    for (const [key, entry] of namespaceCache) {
        if (entry.timestamp < now - CACHE_TTL) {
            namespaceCache.delete(key);
        }
    }
}, CACHE_TTL);

/**
 * Lists namespaces for a given PBS datastore
 * @param {Object} client - PBS API client
 * @param {string} datastoreName - Name of the datastore
 * @param {string} parentNamespace - Parent namespace to list from (empty string for root)
 * @param {number} maxDepth - Maximum recursion depth (null for unlimited)
 * @returns {Promise<Array<string>>} Array of namespace paths
 */
async function listNamespaces(client, datastoreName, parentNamespace = '', maxDepth = null) {
    try {
        const params = {};
        
        if (parentNamespace) {
            params.ns = parentNamespace;
        }
        
        if (maxDepth !== null) {
            params['max-depth'] = maxDepth;
        }
        
        // Try to use the namespace list endpoint
        const response = await client.get(`/admin/datastore/${datastoreName}/namespace`, { params });
        const namespaces = response.data?.data || [];
        
        // Extract namespace paths
        return namespaces.map(ns => ns.ns || ns.path || ns.name).filter(ns => ns !== undefined);
    } catch (error) {
        if (error.response?.status === 404) {
            // Namespace endpoint might not exist in older PBS versions
            console.log(`[PBS Namespace Discovery] Namespace list endpoint not available for datastore ${datastoreName}`);
            return [];
        }
        throw error;
    }
}

/**
 * Discovers all namespaces for a given PBS datastore
 * @param {Object} client - PBS API client
 * @param {string} datastoreName - Name of the datastore
 * @param {Object} config - PBS configuration
 * @returns {Promise<Array<string>>} Array of discovered namespace names
 */
async function discoverNamespaces(client, datastoreName, config) {
    const cacheKey = `${config.id}-${datastoreName}`;
    const cached = namespaceCache.get(cacheKey);
    
    // Return cached results if still valid
    if (cached && cached.timestamp > Date.now() - CACHE_TTL) {
        console.log(`[PBS Namespace Discovery] Using cached namespaces for ${datastoreName}: ${cached.namespaces.join(', ') || '(root only)'}`);
        return cached.namespaces;
    }
    
    console.log(`[PBS Namespace Discovery] Starting discovery for datastore: ${datastoreName}`);
    
    try {
        // Try to list namespaces using the API
        const namespaces = await listNamespaces(client, datastoreName, '', null);
        
        // Always include root namespace
        const allNamespaces = ['', ...namespaces];
        const uniqueNamespaces = [...new Set(allNamespaces)].sort();
        
        // Cache the results with size limit enforcement
        if (namespaceCache.size >= CACHE_MAX_SIZE) {
            // Remove oldest entry
            const oldestKey = namespaceCache.keys().next().value;
            namespaceCache.delete(oldestKey);
        }
        namespaceCache.set(cacheKey, {
            namespaces: uniqueNamespaces,
            timestamp: Date.now()
        });
        
        console.log(`[PBS Namespace Discovery] Discovered namespaces for ${datastoreName}: ${uniqueNamespaces.join(', ') || '(root only)'}`);
        return uniqueNamespaces;
    } catch (error) {
        console.warn(`WARN: [PBS Namespace Discovery] Failed to discover namespaces for ${datastoreName}: ${error.message}`);
        
        // Fall back to checking if specific namespaces exist by probing groups
        return fallbackNamespaceDiscovery(client, datastoreName, config);
    }
}

/**
 * Fallback namespace discovery by probing the groups endpoint
 * @param {Object} client - PBS API client
 * @param {string} datastoreName - Name of the datastore
 * @param {Object} config - PBS configuration
 * @returns {Promise<Array<string>>} Array of discovered namespace names
 */
async function fallbackNamespaceDiscovery(client, datastoreName, config) {
    console.log(`[PBS Namespace Discovery] Using fallback discovery for ${datastoreName}`);
    
    const discoveredNamespaces = new Set(['']); // Always include root
    
    try {
        // Get groups from root namespace
        const rootGroups = await client.get(`/admin/datastore/${datastoreName}/groups`);
        const groups = rootGroups.data?.data || [];
        
        // Look for namespace indicators in the backup data
        for (const group of groups) {
            // Check if group has namespace information
            if (group.ns && group.ns !== '') {
                discoveredNamespaces.add(group.ns);
            }
            
            if (group.owner && group.owner.includes('/')) {
                const parts = group.owner.split('/');
                if (parts.length > 1) {
                    // Might contain namespace info
                    const possibleNs = parts[0];
                    if (possibleNs && !possibleNs.includes('@')) {
                        discoveredNamespaces.add(possibleNs);
                    }
                }
            }
        }
        
    } catch (error) {
        console.error(`ERROR: [PBS Namespace Discovery] Fallback discovery failed: ${error.message}`);
    }
    
    const namespaceArray = Array.from(discoveredNamespaces).sort();
    
    // Cache the results
    const cacheKey = `${config.id}-${datastoreName}`;
    namespaceCache.set(cacheKey, {
        namespaces: namespaceArray,
        timestamp: Date.now()
    });
    
    console.log(`[PBS Namespace Discovery] Fallback discovered namespaces for ${datastoreName}: ${namespaceArray.join(', ') || '(root only)'}`);
    return namespaceArray;
}

/**
 * Gets the list of namespaces to query based on configuration
 * @param {Object} client - PBS API client
 * @param {string} datastoreName - Name of the datastore
 * @param {Object} config - PBS configuration
 * @returns {Promise<Array<string>>} Array of namespace names to query
 */
async function getNamespacesToQuery(client, datastoreName, config) {
    // Always discover all namespaces automatically
    return discoverNamespaces(client, datastoreName, config);
}

/**
 * Clears the namespace cache for a specific PBS instance
 * @param {string} pbsId - PBS instance ID
 */
function clearNamespaceCache(pbsId) {
    for (const [key, value] of namespaceCache.entries()) {
        if (key.startsWith(pbsId + '-')) {
            namespaceCache.delete(key);
        }
    }
}

/**
 * Clears all namespace caches
 */
function clearAllNamespaceCaches() {
    namespaceCache.clear();
}

module.exports = {
    discoverNamespaces,
    getNamespacesToQuery,
    clearNamespaceCache,
    clearAllNamespaceCaches
};