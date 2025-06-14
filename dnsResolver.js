const dns = require('dns').promises;
const { promisify } = require('util');
const lookup = promisify(require('dns').lookup);

// Cache for DNS resolutions
const dnsCache = new Map();
const DNS_CACHE_TTL = 60000; // 1 minute cache

/**
 * Custom DNS resolver with caching and round-robin support
 */
class DnsResolver {
  constructor() {
    this.failedHosts = new Map(); // Track failed hosts with timestamps
    this.FAILED_HOST_RETRY_DELAY = 30000; // 30 seconds before retrying a failed host
  }

  /**
   * Clear DNS cache
   */
  clearCache() {
    dnsCache.clear();
    this.failedHosts.clear();
  }

  /**
   * Mark a host/IP as failed
   */
  markHostFailed(hostOrIp) {
    console.log(`[DnsResolver] Marking host as failed: ${hostOrIp}`);
    this.failedHosts.set(hostOrIp, Date.now());
  }

  /**
   * Check if a host is marked as failed and still in the retry delay period
   */
  isHostFailed(hostOrIp) {
    const failedTime = this.failedHosts.get(hostOrIp);
    if (!failedTime) return false;
    
    const elapsed = Date.now() - failedTime;
    if (elapsed > this.FAILED_HOST_RETRY_DELAY) {
      // Remove from failed list after delay period
      this.failedHosts.delete(hostOrIp);
      return false;
    }
    
    return true;
  }

  /**
   * Resolve hostname to IP addresses with caching
   * @param {string} hostname - The hostname to resolve
   * @returns {Promise<Array>} - Array of IP addresses
   */
  async resolveHostname(hostname) {
    // Check cache first
    const cached = dnsCache.get(hostname);
    if (cached && (Date.now() - cached.timestamp < DNS_CACHE_TTL)) {
      console.log(`[DnsResolver] Using cached DNS resolution for ${hostname}: ${cached.addresses.length} addresses`);
      return cached.addresses;
    }

    try {
      // Try DNS resolution with both IPv4 and IPv6
      const addresses = await dns.resolve4(hostname).catch(() => []);
      const addresses6 = await dns.resolve6(hostname).catch(() => []);
      
      const allAddresses = [...addresses, ...addresses6];
      
      if (allAddresses.length === 0) {
        // Fallback to lookup (uses system resolver)
        console.log(`[DnsResolver] DNS resolve failed for ${hostname}, trying system lookup`);
        const result = await lookup(hostname, { all: true });
        const lookupAddresses = result.map(r => r.address);
        
        if (lookupAddresses.length > 0) {
          dnsCache.set(hostname, { addresses: lookupAddresses, timestamp: Date.now() });
          console.log(`[DnsResolver] System lookup resolved ${hostname} to: ${lookupAddresses.join(', ')}`);
          return lookupAddresses;
        }
        
        throw new Error(`No IP addresses found for ${hostname}`);
      }

      // Cache the results
      dnsCache.set(hostname, { addresses: allAddresses, timestamp: Date.now() });
      console.log(`[DnsResolver] Resolved ${hostname} to: ${allAddresses.join(', ')}`);
      
      // Filter out failed IPs
      const workingAddresses = allAddresses.filter(ip => !this.isHostFailed(ip));
      
      if (workingAddresses.length === 0) {
        console.warn(`[DnsResolver] All ${allAddresses.length} IPs for ${hostname} are marked as failed, using all anyway`);
        return allAddresses;
      }
      
      return workingAddresses;
      
    } catch (error) {
      console.error(`[DnsResolver] Failed to resolve ${hostname}: ${error.message}`);
      
      // Check if we have a stale cache entry we can use
      const staleCache = dnsCache.get(hostname);
      if (staleCache) {
        console.warn(`[DnsResolver] Using stale DNS cache for ${hostname} due to resolution failure`);
        return staleCache.addresses;
      }
      
      throw error;
    }
  }

  /**
   * Extract hostname from a URL or host:port string
   */
  extractHostname(hostString) {
    try {
      if (hostString.includes('://')) {
        const url = new URL(hostString);
        return url.hostname;
      } else {
        // Handle host:port format
        const parts = hostString.split(':');
        return parts[0];
      }
    } catch (error) {
      return hostString; // Return as-is if parsing fails
    }
  }

  /**
   * Test if a hostname can be resolved
   */
  async canResolve(hostname) {
    try {
      const addresses = await this.resolveHostname(hostname);
      return addresses.length > 0;
    } catch (error) {
      return false;
    }
  }
}

// Export singleton instance
module.exports = new DnsResolver();