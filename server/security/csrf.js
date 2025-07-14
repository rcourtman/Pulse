/**
 * CSRF Protection Module
 * Implements double-submit cookie pattern for CSRF protection
 */

const crypto = require('crypto');

// CSRF token storage (in-memory for now, like sessions)
const csrfTokens = new Map();

/**
 * Generate a new CSRF token
 */
function generateToken() {
    return crypto.randomBytes(32).toString('hex');
}

/**
 * Create and store CSRF token for a session
 */
function createCsrfToken(sessionId) {
    const token = generateToken();
    csrfTokens.set(sessionId, token);
    return token;
}

/**
 * Get CSRF token for a session
 */
function getCsrfToken(sessionId) {
    return csrfTokens.get(sessionId);
}

/**
 * Validate CSRF token
 */
function validateCsrfToken(sessionId, providedToken) {
    if (!sessionId || !providedToken) {
        return false;
    }
    
    const storedToken = csrfTokens.get(sessionId);
    if (!storedToken) {
        return false;
    }
    
    // Timing-safe comparison
    return crypto.timingSafeEqual(
        Buffer.from(storedToken),
        Buffer.from(providedToken)
    );
}

/**
 * Clean up CSRF token when session is destroyed
 */
function destroyCsrfToken(sessionId) {
    csrfTokens.delete(sessionId);
}

/**
 * CSRF Protection Middleware
 * Implements double-submit cookie pattern
 */
function csrfProtection(options = {}) {
    const {
        excludePaths = ['/api/auth/login', '/api/health', '/api/config/test'],
        headerName = 'x-csrf-token',
        skipMethods = ['GET', 'HEAD', 'OPTIONS'],
        enabled = true
    } = options;
    
    return async (req, res, next) => {
        // Skip if CSRF protection is disabled
        if (!enabled) {
            return next();
        }
        
        // Skip for excluded paths
        if (excludePaths.some(path => req.path === path)) {
            return next();
        }
        
        // Skip for safe methods
        if (skipMethods.includes(req.method)) {
            return next();
        }
        
        // Skip for API key authentication
        if (req.auth?.type === 'apikey') {
            return next();
        }
        
        // Get session ID from cookie
        const sessionId = req.cookies?.pulse_session;
        if (!sessionId) {
            // No session, no CSRF check needed (auth will handle this)
            return next();
        }
        
        // Get CSRF token from header
        const providedToken = req.headers[headerName] || req.body?._csrf;
        
        // Validate token
        if (!validateCsrfToken(sessionId, providedToken)) {
            return res.status(403).json({
                error: 'Invalid CSRF token',
                message: 'Request validation failed. Please refresh the page and try again.'
            });
        }
        
        next();
    };
}

/**
 * Middleware to inject CSRF token into response
 */
function injectCsrfToken() {
    return (req, res, next) => {
        // Check if user has a session
        const sessionId = req.cookies?.pulse_session;
        if (sessionId) {
            // Get or create CSRF token
            let token = getCsrfToken(sessionId);
            if (!token) {
                token = createCsrfToken(sessionId);
            }
            
            // Add token to response locals for templates
            res.locals.csrfToken = token;
            
            // Add token as response header for API responses
            res.setHeader('X-CSRF-Token', token);
        }
        
        next();
    };
}

module.exports = {
    generateToken,
    createCsrfToken,
    getCsrfToken,
    validateCsrfToken,
    destroyCsrfToken,
    csrfProtection,
    injectCsrfToken
};