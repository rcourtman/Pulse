/**
 * Rate limiting middleware to protect API endpoints
 */

const { createLogger } = require('../utils/logger');
const { REQUEST_LIMITS } = require('../config/constants');

const logger = createLogger('RateLimiter');

/**
 * Token bucket algorithm for rate limiting
 */
class TokenBucket {
    constructor(capacity, refillRate, refillInterval = 1000) {
        this.capacity = capacity;
        this.tokens = capacity;
        this.refillRate = refillRate;
        this.refillInterval = refillInterval;
        this.lastRefill = Date.now();
        
        // Start refill timer
        this.refillTimer = setInterval(() => this.refill(), this.refillInterval);
    }

    /**
     * Try to consume tokens
     */
    consume(tokens = 1) {
        this.refill();
        
        if (this.tokens >= tokens) {
            this.tokens -= tokens;
            return true;
        }
        
        return false;
    }

    /**
     * Refill tokens based on elapsed time
     */
    refill() {
        const now = Date.now();
        const timePassed = now - this.lastRefill;
        const tokensToAdd = (timePassed / this.refillInterval) * this.refillRate;
        
        this.tokens = Math.min(this.capacity, this.tokens + tokensToAdd);
        this.lastRefill = now;
    }

    /**
     * Get current token count
     */
    getTokens() {
        this.refill();
        return Math.floor(this.tokens);
    }

    /**
     * Clean up timer
     */
    destroy() {
        if (this.refillTimer) {
            clearInterval(this.refillTimer);
            this.refillTimer = null;
        }
    }
}

/**
 * Rate limiter with multiple strategies
 */
class RateLimiter {
    constructor(options = {}) {
        this.windowMs = options.windowMs || REQUEST_LIMITS.RATE_LIMIT_WINDOW;
        this.maxRequests = options.maxRequests || REQUEST_LIMITS.RATE_LIMIT_MAX;
        this.keyGenerator = options.keyGenerator || this.defaultKeyGenerator;
        this.skipSuccessfulRequests = options.skipSuccessfulRequests || false;
        this.skipFailedRequests = options.skipFailedRequests || false;
        
        // Storage for different strategies
        this.slidingWindow = new Map(); // For sliding window
        this.tokenBuckets = new Map(); // For token bucket
        this.strategy = options.strategy || 'sliding-window';
        
        // Statistics
        this.stats = {
            totalRequests: 0,
            allowedRequests: 0,
            blockedRequests: 0,
            uniqueClients: new Set()
        };
        
        // Cleanup old entries periodically
        this.cleanupInterval = setInterval(() => this.cleanup(), 60000); // Every minute
    }

    /**
     * Default key generator (by IP)
     */
    defaultKeyGenerator(req) {
        return req.ip || req.connection.remoteAddress || 'unknown';
    }

    /**
     * Rate limiting middleware
     */
    middleware() {
        return async (req, res, next) => {
            const key = this.keyGenerator(req);
            this.stats.totalRequests++;
            this.stats.uniqueClients.add(key);
            
            try {
                const allowed = await this.checkLimit(key);
                
                if (!allowed) {
                    this.stats.blockedRequests++;
                    logger.warn(`Rate limit exceeded for ${key}`);
                    
                    res.setHeader('X-RateLimit-Limit', this.maxRequests);
                    res.setHeader('X-RateLimit-Remaining', '0');
                    res.setHeader('X-RateLimit-Reset', new Date(Date.now() + this.windowMs).toISOString());
                    res.setHeader('Retry-After', Math.ceil(this.windowMs / 1000));
                    
                    return res.status(429).json({
                        error: 'Too Many Requests',
                        message: `Rate limit exceeded. Please retry after ${Math.ceil(this.windowMs / 1000)} seconds.`,
                        retryAfter: Math.ceil(this.windowMs / 1000)
                    });
                }
                
                this.stats.allowedRequests++;
                
                // Add rate limit headers
                const remaining = this.getRemainingRequests(key);
                res.setHeader('X-RateLimit-Limit', this.maxRequests);
                res.setHeader('X-RateLimit-Remaining', remaining);
                res.setHeader('X-RateLimit-Reset', new Date(Date.now() + this.windowMs).toISOString());
                
                // Track response for conditional limiting
                if (this.skipSuccessfulRequests || this.skipFailedRequests) {
                    const originalEnd = res.end;
                    res.end = (...args) => {
                        const shouldSkip = (
                            (this.skipSuccessfulRequests && res.statusCode < 400) ||
                            (this.skipFailedRequests && res.statusCode >= 400)
                        );
                        
                        if (shouldSkip) {
                            this.decrementCount(key);
                        }
                        
                        originalEnd.apply(res, args);
                    };
                }
                
                next();
            } catch (error) {
                logger.error('Rate limiter error:', error);
                next(); // Don't block on errors
            }
        };
    }

    /**
     * Check if request is within rate limit
     */
    async checkLimit(key) {
        switch (this.strategy) {
            case 'token-bucket':
                return this.checkTokenBucket(key);
            case 'sliding-window':
            default:
                return this.checkSlidingWindow(key);
        }
    }

    /**
     * Sliding window rate limiting
     */
    checkSlidingWindow(key) {
        const now = Date.now();
        const windowStart = now - this.windowMs;
        
        if (!this.slidingWindow.has(key)) {
            this.slidingWindow.set(key, []);
        }
        
        const requests = this.slidingWindow.get(key);
        
        // Remove old requests outside the window
        const validRequests = requests.filter(timestamp => timestamp > windowStart);
        
        if (validRequests.length >= this.maxRequests) {
            return false;
        }
        
        validRequests.push(now);
        this.slidingWindow.set(key, validRequests);
        
        return true;
    }

    /**
     * Token bucket rate limiting
     */
    checkTokenBucket(key) {
        if (!this.tokenBuckets.has(key)) {
            const bucket = new TokenBucket(
                this.maxRequests,
                this.maxRequests / (this.windowMs / 1000), // Tokens per second
                1000 // Refill every second
            );
            this.tokenBuckets.set(key, bucket);
        }
        
        const bucket = this.tokenBuckets.get(key);
        return bucket.consume(1);
    }

    /**
     * Get remaining requests for a key
     */
    getRemainingRequests(key) {
        switch (this.strategy) {
            case 'token-bucket':
                const bucket = this.tokenBuckets.get(key);
                return bucket ? Math.floor(bucket.getTokens()) : this.maxRequests;
            
            case 'sliding-window':
            default:
                const requests = this.slidingWindow.get(key) || [];
                const windowStart = Date.now() - this.windowMs;
                const validRequests = requests.filter(timestamp => timestamp > windowStart);
                return Math.max(0, this.maxRequests - validRequests.length);
        }
    }

    /**
     * Decrement count (for conditional limiting)
     */
    decrementCount(key) {
        switch (this.strategy) {
            case 'token-bucket':
                const bucket = this.tokenBuckets.get(key);
                if (bucket) {
                    bucket.tokens = Math.min(bucket.capacity, bucket.tokens + 1);
                }
                break;
                
            case 'sliding-window':
                const requests = this.slidingWindow.get(key);
                if (requests && requests.length > 0) {
                    requests.pop();
                }
                break;
        }
    }

    /**
     * Clean up old entries
     */
    cleanup() {
        const now = Date.now();
        const windowStart = now - this.windowMs;
        
        // Clean sliding window
        for (const [key, requests] of this.slidingWindow) {
            const validRequests = requests.filter(timestamp => timestamp > windowStart);
            if (validRequests.length === 0) {
                this.slidingWindow.delete(key);
            } else {
                this.slidingWindow.set(key, validRequests);
            }
        }
        
        // Clean token buckets (remove unused ones)
        for (const [key, bucket] of this.tokenBuckets) {
            if (bucket.getTokens() >= bucket.capacity) {
                bucket.destroy();
                this.tokenBuckets.delete(key);
            }
        }
        
        logger.debug(`Rate limiter cleanup: ${this.slidingWindow.size} active clients`);
    }

    /**
     * Get rate limiter statistics
     */
    getStats() {
        return {
            ...this.stats,
            uniqueClients: this.stats.uniqueClients.size,
            activeClients: this.strategy === 'sliding-window' 
                ? this.slidingWindow.size 
                : this.tokenBuckets.size,
            blockedPercentage: this.stats.totalRequests > 0
                ? ((this.stats.blockedRequests / this.stats.totalRequests) * 100).toFixed(2) + '%'
                : '0%'
        };
    }

    /**
     * Reset rate limiter
     */
    reset() {
        this.slidingWindow.clear();
        
        for (const bucket of this.tokenBuckets.values()) {
            bucket.destroy();
        }
        this.tokenBuckets.clear();
        
        this.stats = {
            totalRequests: 0,
            allowedRequests: 0,
            blockedRequests: 0,
            uniqueClients: new Set()
        };
        
        logger.info('Rate limiter reset');
    }

    /**
     * Destroy rate limiter
     */
    destroy() {
        if (this.cleanupInterval) {
            clearInterval(this.cleanupInterval);
            this.cleanupInterval = null;
        }
        
        for (const bucket of this.tokenBuckets.values()) {
            bucket.destroy();
        }
        
        this.reset();
    }
}

/**
 * Factory function for creating rate limiters with presets
 */
function createRateLimiter(preset = 'default', customOptions = {}) {
    const presets = {
        default: {
            windowMs: REQUEST_LIMITS.RATE_LIMIT_WINDOW,
            maxRequests: REQUEST_LIMITS.RATE_LIMIT_MAX
        },
        strict: {
            windowMs: 60000, // 1 minute
            maxRequests: 20
        },
        relaxed: {
            windowMs: 60000, // 1 minute
            maxRequests: 500  // Increased for rapid UI interactions
        },
        api: {
            windowMs: 60000, // 1 minute
            maxRequests: 300, // Increased for better frontend experience
            skipSuccessfulRequests: false,
            skipFailedRequests: true
        },
        auth: {
            windowMs: 900000, // 15 minutes
            maxRequests: 5,
            skipSuccessfulRequests: true
        }
    };

    const options = {
        ...presets[preset] || presets.default,
        ...customOptions
    };

    return new RateLimiter(options);
}

module.exports = {
    RateLimiter,
    createRateLimiter,
    TokenBucket
};