/**
 * PBS Namespace Discovery Module
 * Handles automatic discovery of namespaces in Proxmox Backup Server
 */

// Cache for discovered namespaces per PBS instance
const namespaceCache = new Map();
const CACHE_TTL = 300000; // 5 minutes cache TTL

/**
 * Discovers all namespaces for a given PBS datastore
 * @param {Object} client - PBS API client
 * @param {string} datastoreName - Name of the datastore
 * @param {Object} config - PBS configuration
 * @returns {Array<string>} Array of discovered namespace names
 */
async function discoverNamespaces(client, datastoreName, config) {
    const cacheKey = `${config.id}-${datastoreName}`;
    const cached = namespaceCache.get(cacheKey);
    
    // Return cached results if still valid
    if (cached && cached.timestamp > Date.now() - CACHE_TTL) {
        return cached.namespaces;
    }
    
    const discoveredNamespaces = new Set(['']); // Always include root namespace
    const namespacesToCheck = ['']; // Start with root
    const checkedNamespaces = new Set();
    
    // Breadth-first search for namespaces (up to 8 levels deep as per PBS limits)
    while (namespacesToCheck.length > 0 && discoveredNamespaces.size < 1000) { // Safety limit
        const currentNamespace = namespacesToCheck.shift();
        
        if (checkedNamespaces.has(currentNamespace)) {
            continue;
        }
        checkedNamespaces.add(currentNamespace);
        
        try {
            const params = {};
            if (currentNamespace) {
                params.ns = currentNamespace;
            }
            
            const groupsResponse = await client.get(`/admin/datastore/${datastoreName}/groups`, { params });
            const groups = groupsResponse.data?.data || [];
            
            // Extract unique namespaces from backup groups
            // PBS stores backups as namespace/type/id, so we need to parse the namespace from the group
            for (const group of groups) {
                // Check if this group indicates a sub-namespace
                // This is a heuristic approach since PBS doesn't have a direct namespace listing API
                const groupNs = group.ns || currentNamespace;
                if (groupNs && !discoveredNamespaces.has(groupNs)) {
                    discoveredNamespaces.add(groupNs);
                    namespacesToCheck.push(groupNs);
                }
            }
            
            // Also try to probe common sub-namespace patterns if we're in root
            if (currentNamespace === '') {
                const commonPatterns = ['archive', 'backup', 'daily', 'weekly', 'monthly', 'prod', 'dev', 'test'];
                for (const pattern of commonPatterns) {
                    if (!checkedNamespaces.has(pattern)) {
                        namespacesToCheck.push(pattern);
                    }
                }
            }
            
        } catch (error) {
            // Namespace doesn't exist or no access - continue with others
            if (error.response?.status !== 404 && error.response?.status !== 403) {
                console.warn(`WARN: [PBS Namespace Discovery] Error checking namespace '${currentNamespace}': ${error.message}`);
            }
        }
    }
    
    // Convert Set to Array and cache the results
    const namespaceArray = Array.from(discoveredNamespaces).sort();
    namespaceCache.set(cacheKey, {
        namespaces: namespaceArray,
        timestamp: Date.now()
    });
    
    return namespaceArray;
}

/**
 * Filters namespaces based on include/exclude patterns
 * @param {Array<string>} namespaces - Array of namespace names
 * @param {string} includePatterns - Comma-separated include patterns (supports wildcards)
 * @param {string} excludePatterns - Comma-separated exclude patterns (supports wildcards)
 * @returns {Array<string>} Filtered array of namespaces
 */
function filterNamespaces(namespaces, includePatterns, excludePatterns) {
    // Parse patterns
    const includes = includePatterns ? includePatterns.split(',').map(p => p.trim()).filter(p => p) : [];
    const excludes = excludePatterns ? excludePatterns.split(',').map(p => p.trim()).filter(p => p) : [];
    
    // If no patterns specified, return all namespaces
    if (includes.length === 0 && excludes.length === 0) {
        return namespaces;
    }
    
    // Helper function to check if namespace matches pattern
    const matchesPattern = (namespace, pattern) => {
        // Convert wildcard pattern to regex
        const regexPattern = pattern
            .replace(/[.+?^${}()|[\]\\]/g, '\\$&') // Escape special regex chars
            .replace(/\*/g, '.*'); // Convert * to .*
        const regex = new RegExp(`^${regexPattern}$`);
        return regex.test(namespace);
    };
    
    return namespaces.filter(namespace => {
        // Check excludes first (excludes take precedence)
        if (excludes.some(pattern => matchesPattern(namespace, pattern))) {
            return false;
        }
        
        // If includes are specified, namespace must match at least one
        if (includes.length > 0) {
            return includes.some(pattern => matchesPattern(namespace, pattern));
        }
        
        // If no includes specified, include by default
        return true;
    });
}

/**
 * Gets the list of namespaces to query based on configuration
 * @param {Object} client - PBS API client
 * @param {string} datastoreName - Name of the datastore
 * @param {Object} config - PBS configuration
 * @returns {Array<string>} Array of namespace names to query
 */
async function getNamespacesToQuery(client, datastoreName, config) {
    // If auto-discovery is disabled and a specific namespace is configured, use only that
    if (!config.namespaceAuto && config.namespace) {
        return [config.namespace];
    }
    
    // If auto-discovery is disabled and no namespace specified, use root only
    if (!config.namespaceAuto) {
        return [''];
    }
    
    // Auto-discover namespaces
    const discoveredNamespaces = await discoverNamespaces(client, datastoreName, config);
    
    // Apply include/exclude filters
    return filterNamespaces(discoveredNamespaces, config.namespaceInclude, config.namespaceExclude);
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
    filterNamespaces,
    getNamespacesToQuery,
    clearNamespaceCache,
    clearAllNamespaceCaches
};