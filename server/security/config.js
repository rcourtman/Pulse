/**
 * Security configuration and constants
 */

// Security modes
const SECURITY_MODES = {
    PUBLIC: 'public',   // No authentication required (for trusted networks)
    PRIVATE: 'private'  // Authentication required for all access
};

// Endpoint security levels
const SECURITY_LEVELS = {
    PUBLIC: 'public',       // No auth required (health checks, static assets)
    READ: 'read',          // Read-only access to monitoring data
    WRITE: 'write',        // Can modify configuration
    CRITICAL: 'critical'   // Restart, updates, credential operations
};

// User roles
const USER_ROLES = {
    VIEWER: 'viewer',      // Read-only access
    OPERATOR: 'operator',  // Can change settings
    ADMIN: 'admin'        // Full access
};

// Role permissions mapping
const ROLE_PERMISSIONS = {
    [USER_ROLES.VIEWER]: [SECURITY_LEVELS.PUBLIC, SECURITY_LEVELS.READ],
    [USER_ROLES.OPERATOR]: [SECURITY_LEVELS.PUBLIC, SECURITY_LEVELS.READ, SECURITY_LEVELS.WRITE],
    [USER_ROLES.ADMIN]: [SECURITY_LEVELS.PUBLIC, SECURITY_LEVELS.READ, SECURITY_LEVELS.WRITE, SECURITY_LEVELS.CRITICAL]
};

// Endpoint security mapping
const ENDPOINT_SECURITY = {
    // Public endpoints
    'GET /': SECURITY_LEVELS.READ,  // Changed from PUBLIC to READ to require auth in secure/strict modes
    'GET /api/health': SECURITY_LEVELS.PUBLIC,
    'HEAD /api/health': SECURITY_LEVELS.PUBLIC,
    'GET /diagnostics.html': SECURITY_LEVELS.PUBLIC,
    
    // Read-only endpoints
    'GET /api/status': SECURITY_LEVELS.READ,
    'GET /api/charts': SECURITY_LEVELS.READ,
    'GET /api/storage-charts': SECURITY_LEVELS.READ,
    'GET /api/snapshots': SECURITY_LEVELS.READ,
    'GET /api/updates/check': SECURITY_LEVELS.READ,
    'GET /api/updates/status': SECURITY_LEVELS.READ,
    'GET /api/alerts': SECURITY_LEVELS.READ,
    'GET /api/alerts/history': SECURITY_LEVELS.READ,
    'GET /api/thresholds': SECURITY_LEVELS.READ,
    
    // Write endpoints
    'POST /api/config': SECURITY_LEVELS.WRITE,
    'PUT /api/config': SECURITY_LEVELS.WRITE,
    'POST /api/alerts': SECURITY_LEVELS.WRITE,
    'PUT /api/alerts': SECURITY_LEVELS.WRITE,
    'DELETE /api/alerts': SECURITY_LEVELS.WRITE,
    'POST /api/alerts/test': SECURITY_LEVELS.WRITE,
    'POST /api/thresholds': SECURITY_LEVELS.WRITE,
    'PUT /api/thresholds': SECURITY_LEVELS.WRITE,
    'DELETE /api/thresholds': SECURITY_LEVELS.WRITE,
    
    // Critical endpoints
    'POST /api/service/restart': SECURITY_LEVELS.CRITICAL,
    'POST /api/updates/apply': SECURITY_LEVELS.CRITICAL,
    'POST /api/config/test': SECURITY_LEVELS.PUBLIC, // Allow during setup
    'GET /api/config': SECURITY_LEVELS.CRITICAL, // Contains sensitive data
    'GET /api/config/debug': SECURITY_LEVELS.CRITICAL
};

// Session configuration
const SESSION_CONFIG = {
    SECRET_LENGTH: 64,
    COOKIE_NAME: 'pulse_session',
    MAX_AGE: 24 * 60 * 60 * 1000, // 24 hours
    SECURE_COOKIE: process.env.NODE_ENV === 'production',
    HTTP_ONLY: true,
    SAME_SITE: 'strict'
};


// Security headers
const SECURITY_HEADERS = {
    'X-Content-Type-Options': 'nosniff',
    'X-XSS-Protection': '1; mode=block',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy': 'accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()'
};

// Get security configuration from environment
function getSecurityConfig() {
    return {
        mode: process.env.SECURITY_MODE || SECURITY_MODES.PRIVATE,
        sessionSecret: process.env.SESSION_SECRET,
        adminPassword: process.env.ADMIN_PASSWORD,
        bcryptRounds: parseInt(process.env.BCRYPT_ROUNDS || '10', 10),
        maxLoginAttempts: parseInt(process.env.MAX_LOGIN_ATTEMPTS || '5', 10),
        lockoutDuration: parseInt(process.env.LOCKOUT_DURATION || '900000', 10), // 15 minutes
        auditLog: process.env.AUDIT_LOG === 'true'
    };
}

// Check if endpoint requires authentication based on security mode
function requiresAuth(method, path, securityMode) {
    const endpoint = `${method} ${path}`;
    const level = ENDPOINT_SECURITY[endpoint];
    
    // If endpoint not mapped, default to WRITE level
    if (!level) {
        console.warn(`[Security] Unmapped endpoint: ${endpoint}, defaulting to auth required`);
        return securityMode === SECURITY_MODES.PRIVATE;
    }
    
    switch (securityMode) {
        case SECURITY_MODES.PUBLIC:
            // No authentication required for anything
            return false;
            
        case SECURITY_MODES.PRIVATE:
            // Everything except public endpoints (health, static assets) requires auth
            return level !== SECURITY_LEVELS.PUBLIC;
            
        default:
            // Unknown mode, default to private
            return true;
    }
}

// Check if user has permission for endpoint
function hasPermission(userRole, method, path) {
    const endpoint = `${method} ${path}`;
    const level = ENDPOINT_SECURITY[endpoint] || SECURITY_LEVELS.WRITE;
    const allowedLevels = ROLE_PERMISSIONS[userRole] || [];
    
    return allowedLevels.includes(level);
}

module.exports = {
    SECURITY_MODES,
    SECURITY_LEVELS,
    USER_ROLES,
    ROLE_PERMISSIONS,
    ENDPOINT_SECURITY,
    SESSION_CONFIG,
    SECURITY_HEADERS,
    getSecurityConfig,
    requiresAuth,
    hasPermission
};