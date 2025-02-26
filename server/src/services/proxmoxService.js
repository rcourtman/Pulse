const axios = require('axios');
const https = require('https');
const logger = require('../utils/logger');

// In-memory cache for resource data with TTL
class ResourceCache {
  constructor(ttlSeconds = 5) {
    this.cache = new Map();
    this.ttl = ttlSeconds * 1000;
  }

  get(key) {
    const item = this.cache.get(key);
    if (!item) return null;
    
    const now = Date.now();
    if (now > item.expiry) {
      this.cache.delete(key);
      return null;
    }
    
    return item.value;
  }

  set(key, value) {
    const expiry = Date.now() + this.ttl;
    this.cache.set(key, { value, expiry });
  }

  clear() {
    this.cache.clear();
  }
}

// Create global resource cache with 5-second TTL
const resourceCache = new ResourceCache(5);

class ProxmoxService {
  constructor(url, tokenId, token) {
    this.url = url;
    this.tokenId = tokenId;
    this.token = token;
    this.lastFetchTime = 0;
    this.minFetchInterval = 1000; // Minimum 1 second between API calls to the same node
    
    logger.debug(`Creating ProxmoxService instance for ${url}`);
    
    // Create axios instance with default config
    this.client = axios.create({
      baseURL: this.url,
      headers: {
        'Authorization': `PVEAPIToken=${tokenId}=${token}`,
      },
      // Allow self-signed certificates
      httpsAgent: new https.Agent({
        rejectUnauthorized: false,
      }),
      timeout: 10000, // 10 second timeout - shorter for better responsiveness
    });
    
    // Add response time logging and retry logic
    this.client.interceptors.request.use(config => {
      config.metadata = { startTime: Date.now() };
      return config;
    });
    
    this.client.interceptors.response.use(
      response => {
        const duration = Date.now() - response.config.metadata.startTime;
        logger.debug(`Proxmox API call to ${response.config.url} completed in ${duration}ms`);
        return response;
      },
      async error => {
        // If it's a timeout, log it specifically
        if (error.code === 'ECONNABORTED') {
          logger.error(`Proxmox API call to ${error.config.url} timed out after ${this.client.defaults.timeout}ms`);
        }
        return Promise.reject(error);
      }
    );
  }
  
  /**
   * Validate connection to Proxmox
   */
  async validateConnection() {
    try {
      logger.debug(`Validating connection to Proxmox server at ${this.url}`);
      
      // Use a simple endpoint that doesn't require much permission
      const response = await this.client.get('/api2/json/version', {
        timeout: 8000 // 8 second timeout for validation
      });
      
      logger.debug(`Successfully validated connection to Proxmox server at ${this.url}`);
      
      return { valid: true, version: response.data };
    } catch (error) {
      // Provide detailed error information
      if (error.response) {
        logger.error(`Response error: ${error.response.status} - ${JSON.stringify(error.response.data)}`);
      } else if (error.request) {
        logger.error(`Request error: ${error.code || 'Unknown error code'}`);
      } else {
        logger.error(`Setup error: ${error.message}`);
      }
      
      // Throw a more specific error message
      if (error.code === 'ECONNREFUSED') {
        throw new Error(`Connection refused to Proxmox server at ${this.url}`);
      } else if (error.code === 'ETIMEDOUT' || error.code === 'ECONNABORTED') {
        throw new Error(`Connection timed out to Proxmox server at ${this.url}`);
      } else if (error.code === 'CERT_HAS_EXPIRED') {
        throw new Error(`SSL certificate has expired for Proxmox server at ${this.url}`);
      } else if (error.code === 'ENOTFOUND') {
        throw new Error(`DNS lookup failed for Proxmox server at ${this.url}`);
      } else if (error.response && error.response.status === 401) {
        throw new Error(`Authentication failed for Proxmox server at ${this.url}: Invalid API token`);
      } else if (error.response && error.response.status === 403) {
        throw new Error(`Permission denied for Proxmox server at ${this.url}: Insufficient privileges`);
      } else {
        throw new Error(`Failed to connect to Proxmox server at ${this.url}: ${error.message}`);
      }
    }
  }
  
  /**
   * Get all resources (VMs and containers) with rate limiting and caching
   */
  async getResources() {
    try {
      const now = Date.now();
      const cacheKey = `resources_${this.url}_${this.tokenId}`;
      
      // Check if we need to respect rate limiting
      const timeSinceLastFetch = now - this.lastFetchTime;
      if (timeSinceLastFetch < this.minFetchInterval) {
        logger.debug(`Rate limiting in effect, ${this.minFetchInterval - timeSinceLastFetch}ms remaining. Using cached data.`);
        
        // Check cache first during rate limit period
        const cachedData = resourceCache.get(cacheKey);
        if (cachedData) {
          return cachedData;
        }
        
        // If no cache but we're rate limited, wait a bit
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      
      // Check cache before making a request
      const cachedData = resourceCache.get(cacheKey);
      if (cachedData) {
        logger.debug(`Using cached resources for ${this.url}`);
        return cachedData;
      }
      
      // Update the last fetch time
      this.lastFetchTime = now;
      
      // Fetch cluster resources
      logger.info(`Fetching resources from Proxmox server at ${this.url}`);
      const resourcesResponse = await this.client.get('/api2/json/cluster/resources', {
        timeout: 8000 // 8 second timeout for data fetching
      });
      
      // Get the resources data
      const resources = resourcesResponse.data.data || [];
      logger.debug(`Fetched ${resources.length} resources from Proxmox at ${this.url}`);
      
      // Process network data for each resource
      const processedResources = resources.map(resource => {
        // For each resource with network data, preprocess into a consistent format
        if (resource.type === 'qemu' || resource.type === 'lxc') {
          // Ensure network in/out are always present
          if (resource.netin === undefined) resource.netin = 0;
          if (resource.netout === undefined) resource.netout = 0;
          
          // Convert to numbers if they're strings
          if (typeof resource.netin === 'string') resource.netin = parseInt(resource.netin, 10) || 0;
          if (typeof resource.netout === 'string') resource.netout = parseInt(resource.netout, 10) || 0;
        }
        
        return resource;
      });
      
      // Cache the processed results
      resourceCache.set(cacheKey, processedResources);
      
      return processedResources;
    } catch (error) {
      // Log detailed error information
      logger.error(`Failed to fetch Proxmox resources from ${this.url}: ${error.message}`);
      
      // Use the specific error handling from validateConnection
      if (error.code === 'ECONNREFUSED') {
        throw new Error(`Connection refused while fetching resources from ${this.url}`);
      } else if (error.code === 'ETIMEDOUT' || error.code === 'ECONNABORTED') {
        throw new Error(`Connection timed out while fetching resources from ${this.url}`);
      } else if (error.response && error.response.status === 401) {
        throw new Error(`Authentication failed while fetching resources from ${this.url}`);
      } else {
        throw new Error(`Failed to fetch resources from Proxmox server: ${error.message}`);
      }
    }
  }
  
  /**
   * Get resource by ID
   */
  async getResourceById(id) {
    try {
      const resources = await this.getResources();
      return resources.find(resource => resource.id === id) || null;
    } catch (error) {
      logger.error(`Failed to fetch resource ${id}: ${error.message}`);
      throw new Error(`Failed to fetch resource ${id}: ${error.message}`);
    }
  }
}

module.exports = ProxmoxService; 