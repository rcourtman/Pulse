/**
 * Authentication middleware and utilities
 */

const crypto = require('crypto');
const bcrypt = require('bcryptjs');
const { 
    getSecurityConfig, 
    requiresAuth, 
    hasPermission,
    USER_ROLES,
    SESSION_CONFIG,
    SECURITY_MODES
} = require('./config');

// In-memory storage (will be replaced with proper storage later)
const sessions = new Map();
const loginAttempts = new Map();

// Initialize with default admin user if needed
async function initializeAuth() {
    const config = getSecurityConfig();
    
    // Generate session secret if not provided
    if (!config.sessionSecret && config.mode !== SECURITY_MODES.OPEN) {
        console.warn('[Security] No SESSION_SECRET provided, generating random secret. This will invalidate sessions on restart!');
        process.env.SESSION_SECRET = crypto.randomBytes(32).toString('hex');
    }
    
    // Create default admin user if no password is set
    if (!config.adminPassword && config.mode !== SECURITY_MODES.OPEN) {
        const defaultPassword = crypto.randomBytes(16).toString('hex');
        process.env.ADMIN_PASSWORD = await bcrypt.hash(defaultPassword, config.bcryptRounds);
        
        console.log('╔════════════════════════════════════════════════════════════════╗');
        console.log('║                    SECURITY NOTICE                             ║');
        console.log('╠════════════════════════════════════════════════════════════════╣');
        console.log('║ No admin password configured. Generated temporary password:    ║');
        console.log(`║ Username: admin                                                ║`);
        console.log(`║ Password: ${defaultPassword}                            ║`);
        console.log('║                                                                ║');
        console.log('║ Please set ADMIN_PASSWORD in your .env file!                  ║');
        console.log('╚════════════════════════════════════════════════════════════════╝');
    }
    
}

// Session management
function createSession(user) {
    const sessionId = crypto.randomBytes(32).toString('hex');
    const session = {
        id: sessionId,
        user: user,
        createdAt: Date.now(),
        lastAccess: Date.now()
    };
    
    sessions.set(sessionId, session);
    
    // Clean up expired sessions
    cleanupSessions();
    
    return sessionId;
}

function getSession(sessionId) {
    const session = sessions.get(sessionId);
    if (!session) return null;
    
    const config = getSecurityConfig();
    const now = Date.now();
    
    // Get dynamic session timeout
    const sessionTimeoutHours = parseInt(process.env.SESSION_TIMEOUT_HOURS || '24', 10);
    const sessionTimeout = sessionTimeoutHours * 60 * 60 * 1000;
    
    // Check if session expired
    if (now - session.lastAccess > sessionTimeout) {
        sessions.delete(sessionId);
        return null;
    }
    
    // Update last access
    session.lastAccess = now;
    return session;
}

function destroySession(sessionId) {
    sessions.delete(sessionId);
}

function cleanupSessions() {
    const now = Date.now();
    
    // Get dynamic session timeout
    const sessionTimeoutHours = parseInt(process.env.SESSION_TIMEOUT_HOURS || '24', 10);
    const sessionTimeout = sessionTimeoutHours * 60 * 60 * 1000;
    
    for (const [id, session] of sessions.entries()) {
        if (now - session.lastAccess > sessionTimeout) {
            sessions.delete(id);
        }
    }
}


// Login attempt tracking
function recordLoginAttempt(identifier, success) {
    const attempts = loginAttempts.get(identifier) || {
        count: 0,
        lastAttempt: 0,
        lockedUntil: 0
    };
    
    const now = Date.now();
    const config = getSecurityConfig();
    
    if (success) {
        loginAttempts.delete(identifier);
    } else {
        attempts.count++;
        attempts.lastAttempt = now;
        
        if (attempts.count >= config.maxLoginAttempts) {
            attempts.lockedUntil = now + config.lockoutDuration;
        }
        
        loginAttempts.set(identifier, attempts);
    }
}

function isLockedOut(identifier) {
    const attempts = loginAttempts.get(identifier);
    if (!attempts) return false;
    
    const now = Date.now();
    if (attempts.lockedUntil && attempts.lockedUntil > now) {
        return true;
    }
    
    // Clean up if lockout expired
    if (attempts.lockedUntil && attempts.lockedUntil <= now) {
        loginAttempts.delete(identifier);
    }
    
    return false;
}

// Authentication strategies
async function authenticateBasic(username, password) {
    const config = getSecurityConfig();
    
    // Check lockout
    if (isLockedOut(username)) {
        return { success: false, error: 'Account locked due to too many failed attempts' };
    }
    
    // For now, only support admin user
    if (username !== 'admin') {
        recordLoginAttempt(username, false);
        return { success: false, error: 'Invalid credentials' };
    }
    
    const validPassword = await bcrypt.compare(password, config.adminPassword || '');
    
    recordLoginAttempt(username, validPassword);
    
    if (validPassword) {
        return {
            success: true,
            user: {
                username: 'admin',
                role: USER_ROLES.ADMIN
            }
        };
    }
    
    return { success: false, error: 'Invalid credentials' };
}


// Main authentication middleware
function authMiddleware() {
    return async (req, res, next) => {
        const config = getSecurityConfig();
        
        
        // Skip authentication for static assets
        const staticExtensions = ['.js', '.css', '.svg', '.png', '.jpg', '.jpeg', '.gif', '.ico', '.woff', '.woff2', '.ttf', '.eot'];
        const isStaticAsset = staticExtensions.some(ext => req.path.endsWith(ext));
        
        if (isStaticAsset) {
            return next();
        }
        
        // Skip authentication for specific public pages
        const publicPages = ['/login.html', '/setup.html'];
        if (publicPages.includes(req.path)) {
            return next();
        }
        
        // Check if endpoint requires authentication
        if (!requiresAuth(req.method, req.path, config.mode)) {
            return next();
        }
        
        
        // Try session authentication
        const sessionId = req.cookies?.pulse_session;
        if (sessionId) {
            const session = getSession(sessionId);
            if (session) {
                req.auth = {
                    type: 'session',
                    user: session.user
                };
                
                // Check permissions
                if (hasPermission(session.user.role, req.method, req.path)) {
                    return next();
                } else {
                    return res.status(403).json({
                        error: 'Insufficient permissions',
                        required: ENDPOINT_SECURITY[`${req.method} ${req.path}`] || 'WRITE'
                    });
                }
            }
        }
        
        // Try basic authentication (always enabled when auth is required)
        const authHeader = req.headers.authorization;
        if (authHeader && authHeader.startsWith('Basic ')) {
            const credentials = Buffer.from(authHeader.slice(6), 'base64').toString();
            const [username, password] = credentials.split(':');
            
            const result = await authenticateBasic(username, password);
            if (result.success) {
                req.auth = {
                    type: 'basic',
                    user: result.user
                };
                
                // Check permissions
                if (hasPermission(result.user.role, req.method, req.path)) {
                    return next();
                } else {
                    return res.status(403).json({
                        error: 'Insufficient permissions',
                        required: ENDPOINT_SECURITY[`${req.method} ${req.path}`] || 'WRITE'
                    });
                }
            }
        }
        
        // No valid authentication found
        if (config.mode === SECURITY_MODES.PUBLIC) {
            // In public mode, allow all access
            return next();
        }
        
        // Check if this is a browser request (not API)
        const acceptHeader = req.headers.accept || '';
        const isApiRequest = req.path.startsWith('/api/') || 
                           acceptHeader.includes('application/json') ||
                           !acceptHeader.includes('text/html');
        
        if (!isApiRequest) {
            // Redirect to login page for browser requests
            const currentUrl = req.originalUrl || req.url;
            res.redirect(`/login.html?redirect=${encodeURIComponent(currentUrl)}`);
        } else {
            // Return 401 Unauthorized for API requests
            res.status(401).json({
                error: 'Authentication required',
                message: 'Please provide valid credentials',
                authMethods: {
                    session: true,
                    basic: true
                }
            });
        }
    };
}

// Login endpoint handler
async function handleLogin(req, res) {
    try {
        const { username, password } = req.body;
        
        if (!username || !password) {
            return res.status(400).json({ error: 'Username and password required' });
        }
        
        const result = await authenticateBasic(username, password);
        
        if (result.success) {
        const sessionId = createSession(result.user);
        
        // Get current configuration for dynamic cookie settings
        const allowEmbedding = process.env.ALLOW_EMBEDDING === 'true';
        const isProduction = process.env.NODE_ENV === 'production';
        const sessionTimeoutHours = parseInt(process.env.SESSION_TIMEOUT_HOURS || '24', 10);
        
        // Calculate session timeout in milliseconds
        const sessionTimeout = sessionTimeoutHours * 60 * 60 * 1000;
        
        const cookieOptions = {
            httpOnly: true,
            secure: isProduction,
            maxAge: sessionTimeout
        };
        
        // Set SameSite policy automatically based on embedding config
        if (allowEmbedding) {
            if (isProduction) {
                cookieOptions.sameSite = 'none';
                cookieOptions.secure = true;
            } else {
                cookieOptions.sameSite = 'lax';
            }
        } else {
            cookieOptions.sameSite = 'strict';
        }
        
        res.cookie('pulse_session', sessionId, cookieOptions);
        
        // Create CSRF token for this session
        const csrf = require('./csrf');
        const csrfToken = csrf.createCsrfToken(sessionId);
        
        res.json({
            success: true,
            user: {
                username: result.user.username,
                role: result.user.role
            },
            csrfToken: csrfToken
        });
    } else {
        res.status(401).json({ error: result.error });
    }
    } catch (error) {
        console.error('[Auth] Login error:', error);
        res.status(500).json({ error: 'Internal server error during login' });
    }
}

// Logout endpoint handler
async function handleLogout(req, res) {
    const sessionId = req.cookies?.pulse_session;
    if (sessionId) {
        destroySession(sessionId);
        
        // Clean up CSRF token
        const csrf = require('./csrf');
        csrf.destroyCsrfToken(sessionId);
    }
    
    res.clearCookie('pulse_session');
    res.json({ success: true });
}

module.exports = {
    initializeAuth,
    authMiddleware,
    handleLogin,
    handleLogout,
    createSession,
    getSession,
    destroySession
};