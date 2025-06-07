const axios = require('axios');
const https = require('https');
const dnsResolver = require('./dnsResolver');

/**
 * Creates a resilient API client that handles DNS failures gracefully
 */
class ResilientApiClient {
  constructor(baseConfig, authInterceptor) {
    this.baseConfig = baseConfig;
    this.authInterceptor = authInterceptor;
    this.hostname = this.extractHostname(baseConfig.baseURL);
    this.clients = new Map(); // Map of IP -> axios instance
    this.lastWorkingIp = null;
  }

  extractHostname(url) {
    try {
      const urlObj = new URL(url);
      return urlObj.hostname;
    } catch (error) {
      // Handle cases where baseURL might not be a full URL
      const match = url.match(/^https?:\/\/([^:\/]+)/);
      return match ? match[1] : null;
    }
  }

  /**
   * Create an axios instance for a specific IP
   */
  createClientForIp(ip) {
    // Replace hostname with IP in the base URL
    const baseURL = this.baseConfig.baseURL.replace(this.hostname, ip);
    
    const client = axios.create({
      ...this.baseConfig,
      baseURL: baseURL,
      httpsAgent: new https.Agent({
        rejectUnauthorized: this.baseConfig.httpsAgent?.options?.rejectUnauthorized ?? true,
        // Set a shorter keepAlive timeout for failed connections
        keepAlive: true,
        keepAliveMsecs: 1000,
        timeout: this.baseConfig.timeout || 30000
      }),
      headers: {
        ...this.baseConfig.headers,
        // Add Host header to ensure proper SSL certificate validation
        'Host': this.hostname
      }
    });

    // Apply auth interceptor if provided
    if (this.authInterceptor) {
      client.interceptors.request.use(this.authInterceptor);
    }

    // Add response interceptor to track working IPs
    client.interceptors.response.use(
      (response) => {
        // Mark this IP as working
        this.lastWorkingIp = ip;
        return response;
      },
      (error) => {
        // Mark IP as failed on certain errors
        if (error.code === 'ECONNREFUSED' || error.code === 'ETIMEDOUT' || error.code === 'EHOSTUNREACH') {
          dnsResolver.markHostFailed(ip);
        }
        return Promise.reject(error);
      }
    );

    return client;
  }

  /**
   * Get or create a client for a specific IP
   */
  getClientForIp(ip) {
    if (!this.clients.has(ip)) {
      this.clients.set(ip, this.createClientForIp(ip));
    }
    return this.clients.get(ip);
  }

  /**
   * Try to make a request with automatic failover
   */
  async request(config) {
    // First, try to resolve the hostname
    let ips;
    try {
      ips = await dnsResolver.resolveHostname(this.hostname);
    } catch (error) {
      console.error(`[ResilientApiClient] DNS resolution failed for ${this.hostname}: ${error.message}`);
      throw new Error(`DNS resolution failed for ${this.hostname}: ${error.message}`);
    }

    if (ips.length === 0) {
      throw new Error(`No IP addresses available for ${this.hostname}`);
    }

    // If we have a last working IP, try it first
    if (this.lastWorkingIp && ips.includes(this.lastWorkingIp)) {
      ips = [this.lastWorkingIp, ...ips.filter(ip => ip !== this.lastWorkingIp)];
    }

    let lastError;
    
    // Try each IP in sequence
    for (const ip of ips) {
      if (dnsResolver.isHostFailed(ip)) {
        console.log(`[ResilientApiClient] Skipping failed IP ${ip} for ${this.hostname}`);
        continue;
      }

      try {
        const client = this.getClientForIp(ip);
        console.log(`[ResilientApiClient] Attempting request to ${this.hostname} via IP ${ip}`);
        
        const response = await client.request(config);
        return response;
        
      } catch (error) {
        lastError = error;
        console.warn(`[ResilientApiClient] Request failed for ${this.hostname} via IP ${ip}: ${error.message}`);
        
        // If this is a connection error, mark the IP as failed
        if (error.code === 'ECONNREFUSED' || error.code === 'ETIMEDOUT' || error.code === 'EHOSTUNREACH') {
          dnsResolver.markHostFailed(ip);
        }
        
        // Continue to next IP
      }
    }

    // All IPs failed
    console.error(`[ResilientApiClient] All ${ips.length} IPs failed for ${this.hostname}`);
    throw lastError || new Error(`All IPs failed for ${this.hostname}`);
  }

  // Proxy common axios methods
  async get(url, config) {
    return this.request({ ...config, method: 'GET', url });
  }

  async post(url, data, config) {
    return this.request({ ...config, method: 'POST', url, data });
  }

  async put(url, data, config) {
    return this.request({ ...config, method: 'PUT', url, data });
  }

  async delete(url, config) {
    return this.request({ ...config, method: 'DELETE', url });
  }

  async patch(url, data, config) {
    return this.request({ ...config, method: 'PATCH', url, data });
  }
}

module.exports = ResilientApiClient;