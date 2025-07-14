/**
 * Security audit logging
 */

const fs = require('fs').promises;
const path = require('path');
const { getSecurityConfig } = require('./config');

// Audit event types
const AUDIT_EVENTS = {
    // Authentication events
    LOGIN_SUCCESS: 'LOGIN_SUCCESS',
    LOGIN_FAILED: 'LOGIN_FAILED',
    LOGOUT: 'LOGOUT',
    SESSION_EXPIRED: 'SESSION_EXPIRED',
    API_KEY_USED: 'API_KEY_USED',
    
    // Authorization events
    ACCESS_DENIED: 'ACCESS_DENIED',
    PERMISSION_DENIED: 'PERMISSION_DENIED',
    
    // Configuration events
    CONFIG_CHANGED: 'CONFIG_CHANGED',
    CONFIG_READ: 'CONFIG_READ',
    CREDENTIALS_TESTED: 'CREDENTIALS_TESTED',
    
    // Service events
    SERVICE_RESTARTED: 'SERVICE_RESTARTED',
    UPDATE_INITIATED: 'UPDATE_INITIATED',
    
    // Security events
    RATE_LIMIT_EXCEEDED: 'RATE_LIMIT_EXCEEDED',
    SUSPICIOUS_ACTIVITY: 'SUSPICIOUS_ACTIVITY',
    LOCKOUT_TRIGGERED: 'LOCKOUT_TRIGGERED'
};

// Audit log storage
let auditLogPath = process.env.AUDIT_LOG_PATH || '/opt/pulse/data/audit.log';
let auditBuffer = [];
let flushTimer = null;

// Initialize audit logging
async function initializeAudit() {
    const config = getSecurityConfig();
    
    if (!config.auditLog) {
        console.log('[Audit] Audit logging disabled');
        return;
    }
    
    // Ensure audit log directory exists
    try {
        await fs.mkdir(path.dirname(auditLogPath), { recursive: true });
    } catch (error) {
        console.error('[Audit] Failed to create audit log directory:', error);
    }
    
    // Start periodic flush
    flushTimer = setInterval(flushAuditLog, 5000); // Flush every 5 seconds
}

// Log audit event
async function logAuditEvent(event, details = {}) {
    const config = getSecurityConfig();
    
    if (!config.auditLog) {
        return;
    }
    
    const entry = {
        timestamp: new Date().toISOString(),
        event: event,
        ...details,
        // Add request context if available
        ...(details.req ? {
            ip: details.req.ip || details.req.connection?.remoteAddress,
            userAgent: details.req.headers?.['user-agent'],
            method: details.req.method,
            path: details.req.path
        } : {})
    };
    
    // Remove sensitive data
    if (entry.password) delete entry.password;
    if (entry.token) entry.token = entry.token.substring(0, 8) + '...';
    if (entry.req) delete entry.req;
    
    // Add to buffer
    auditBuffer.push(JSON.stringify(entry));
    
    // Flush if buffer is large
    if (auditBuffer.length >= 100) {
        await flushAuditLog();
    }
}

// Flush audit log buffer to disk
async function flushAuditLog() {
    if (auditBuffer.length === 0) {
        return;
    }
    
    const entries = auditBuffer.join('\n') + '\n';
    auditBuffer = [];
    
    try {
        await fs.appendFile(auditLogPath, entries);
    } catch (error) {
        console.error('[Audit] Failed to write audit log:', error);
        // Put entries back in buffer to retry
        auditBuffer = entries.split('\n').filter(e => e);
    }
}

// Middleware to audit requests
function auditMiddleware() {
    return async (req, res, next) => {
        // Store original end function
        const originalEnd = res.end;
        const startTime = Date.now();
        
        // Override end function to capture response
        res.end = function(...args) {
            const duration = Date.now() - startTime;
            
            // Log security-relevant events
            if (res.statusCode === 401) {
                logAuditEvent(AUDIT_EVENTS.ACCESS_DENIED, {
                    req,
                    statusCode: res.statusCode,
                    duration,
                    auth: req.auth
                });
            } else if (res.statusCode === 403) {
                logAuditEvent(AUDIT_EVENTS.PERMISSION_DENIED, {
                    req,
                    statusCode: res.statusCode,
                    duration,
                    auth: req.auth
                });
            } else if (res.statusCode === 429) {
                logAuditEvent(AUDIT_EVENTS.RATE_LIMIT_EXCEEDED, {
                    req,
                    statusCode: res.statusCode,
                    duration
                });
            }
            
            // Call original end
            originalEnd.apply(res, args);
        };
        
        next();
    };
}

// Audit specific events
const audit = {
    loginSuccess: (username, req) => {
        logAuditEvent(AUDIT_EVENTS.LOGIN_SUCCESS, {
            username,
            req
        });
    },
    
    loginFailed: (username, reason, req) => {
        logAuditEvent(AUDIT_EVENTS.LOGIN_FAILED, {
            username,
            reason,
            req
        });
    },
    
    logout: (username, req) => {
        logAuditEvent(AUDIT_EVENTS.LOGOUT, {
            username,
            req
        });
    },
    
    apiKeyUsed: (keyName, req) => {
        logAuditEvent(AUDIT_EVENTS.API_KEY_USED, {
            keyName,
            req
        });
    },
    
    configChanged: (changes, user, req) => {
        logAuditEvent(AUDIT_EVENTS.CONFIG_CHANGED, {
            changes: Object.keys(changes),
            user: user?.username || user?.name,
            req
        });
    },
    
    configRead: (user, req) => {
        logAuditEvent(AUDIT_EVENTS.CONFIG_READ, {
            user: user?.username || user?.name,
            req
        });
    },
    
    credentialsTested: (service, success, user, req) => {
        logAuditEvent(AUDIT_EVENTS.CREDENTIALS_TESTED, {
            service,
            success,
            user: user?.username || user?.name,
            req
        });
    },
    
    serviceRestarted: (reason, user, req) => {
        logAuditEvent(AUDIT_EVENTS.SERVICE_RESTARTED, {
            reason,
            user: user?.username || user?.name,
            req
        });
    },
    
    updateInitiated: (version, user, req) => {
        logAuditEvent(AUDIT_EVENTS.UPDATE_INITIATED, {
            version,
            user: user?.username || user?.name,
            req
        });
    },
    
    suspiciousActivity: (description, req) => {
        logAuditEvent(AUDIT_EVENTS.SUSPICIOUS_ACTIVITY, {
            description,
            req
        });
    },
    
    lockoutTriggered: (identifier, req) => {
        logAuditEvent(AUDIT_EVENTS.LOCKOUT_TRIGGERED, {
            identifier,
            req
        });
    }
};

// Cleanup on shutdown
async function shutdownAudit() {
    if (flushTimer) {
        clearInterval(flushTimer);
        flushTimer = null;
    }
    
    await flushAuditLog();
}

module.exports = {
    AUDIT_EVENTS,
    initializeAudit,
    auditMiddleware,
    audit,
    shutdownAudit
};