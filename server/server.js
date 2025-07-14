
const express = require('express');
const http = require('http');
const cors = require('cors');
const compression = require('compression');
const path = require('path');
const { createLogger } = require('./utils/logger');
const { createRateLimiter } = require('./middleware/rateLimiter');
const { applySecurity } = require('./security');

function createServer() {
    const app = express();
    const server = http.createServer(app);
    const logger = createLogger('Server');
    
    // Configure proxy trust settings
    const trustProxy = process.env.TRUST_PROXY || false;
    if (trustProxy) {
        // Trust proxy settings - can be boolean, number, or string
        // true = trust all proxies
        // false = trust no proxies (default)
        // Number = trust n proxies from the front
        // String = trust specific IPs/subnets (comma-separated)
        if (trustProxy === 'true') {
            app.set('trust proxy', true);
            logger.info('[Server] Trusting all proxies');
        } else if (!isNaN(trustProxy)) {
            app.set('trust proxy', parseInt(trustProxy));
            logger.info(`[Server] Trusting ${trustProxy} proxies from front`);
        } else {
            const trustedProxies = trustProxy.split(',').map(ip => ip.trim());
            app.set('trust proxy', trustedProxies);
            logger.info(`[Server] Trusting specific proxies: ${trustedProxies.join(', ')}`);
        }
    } else {
        logger.info('[Server] Not trusting any proxies (direct connection mode)');
    }
    
    // Create rate limiters for different endpoint types
    const generalLimiter = createRateLimiter('default');
    const apiLimiter = createRateLimiter('api');
    const strictLimiter = createRateLimiter('strict');

    // Middleware
    app.use(compression({
        filter: (req, res) => {
            if (req.headers['x-no-compression']) {
                return false;
            }
            return compression.filter(req, res);
        },
        threshold: 1024,
        level: 6
    }));
    // Configure CORS with specific origins
    const corsOptions = {
        origin: function (origin, callback) {
            // Allow requests with no origin (like mobile apps or curl requests)
            if (!origin) return callback(null, true);
            
            // In production, you should specify exact origins
            // For now, allow same-origin and local development
            const allowedOrigins = [
                'http://localhost:7655',
                'http://127.0.0.1:7655',
                `http://localhost:${process.env.PORT || 7655}`,
                `http://127.0.0.1:${process.env.PORT || 7655}`
            ];
            
            // Allow same-origin requests by checking if origin matches the port
            // This handles any IP address or hostname accessing the same port
            try {
                const originUrl = new URL(origin);
                const port = process.env.PORT || 7655;
                if (originUrl.port === String(port)) {
                    return callback(null, true);
                }
            } catch (e) {
                // Invalid URL, continue to check allowed origins
            }
            
            // Also allow the actual server URL if known
            if (process.env.PULSE_PUBLIC_URL) {
                allowedOrigins.push(process.env.PULSE_PUBLIC_URL);
            }
            
            // Check if origin matches allowed list
            if (allowedOrigins.indexOf(origin) !== -1) {
                callback(null, true);
            } else {
                // Log rejected origins for debugging
                logger.warn(`CORS: Rejected origin ${origin}`);
                callback(new Error('Not allowed by CORS. If using a reverse proxy, set PULSE_PUBLIC_URL environment variable to your proxy URL.'));
            }
        },
        credentials: true,
        optionsSuccessStatus: 200
    };
    
    app.use(cors(corsOptions));
    
    // Body parsing middleware - MUST be before routes that need it
    app.use(express.json({ limit: '10mb' })); // Reduced from 50mb to prevent DoS attacks
    
    // Static files - serve BEFORE authentication to allow CSS/JS loading
    const publicDir = path.join(__dirname, '../src/public');
    app.use(express.static(publicDir, { index: false }));
    
    // Apply security middleware (auth, audit, etc.) AFTER static files
    applySecurity(app);
    
    // Security headers middleware (iframe-specific)
    app.use((req, res, next) => {
        // Configurable frame options for embedding support
        const allowEmbedding = process.env.ALLOW_EMBEDDING === 'true';
        const allowedOrigins = process.env.ALLOWED_EMBED_ORIGINS || '';
        
        if (allowEmbedding) {
            if (allowedOrigins) {
                // Parse comma-separated origins and validate format
                const origins = allowedOrigins.split(',')
                    .map(origin => origin.trim())
                    .filter(origin => origin.length > 0)
                    .map(origin => {
                        // Basic URL validation
                        try {
                            const url = new URL(origin);
                            
                            // Allow HTTP for local/private networks, require HTTPS for public
                            const hostname = url.hostname;
                            const isLocalNetwork = 
                                hostname === 'localhost' ||
                                hostname.endsWith('.local') ||
                                hostname.endsWith('.lan') ||
                                /^192\.168\.\d+\.\d+$/.test(hostname) ||
                                /^10\.\d+\.\d+\.\d+$/.test(hostname) ||
                                /^172\.(1[6-9]|2\d|3[01])\.\d+\.\d+$/.test(hostname) ||
                                /^127\.\d+\.\d+\.\d+$/.test(hostname);
                            
                            if (url.protocol === 'http:' && !isLocalNetwork) {
                                console.warn(`[Security] HTTP origin blocked for non-local address: ${origin}. Use HTTPS for public addresses.`);
                                return null;
                            }
                            
                            return origin;
                        } catch (e) {
                            console.warn(`[Security] Invalid origin format blocked: ${origin}`);
                            return null;
                        }
                    })
                    .filter(origin => origin !== null);
                
                if (origins.length > 0) {
                    // Set both headers for maximum compatibility
                    // Remove X-Frame-Options when using CSP to avoid conflicts
                    // res.setHeader('X-Frame-Options', 'SAMEORIGIN');
                    
                    // CSP frame-ancestors as primary control (takes precedence in modern browsers)
                    const frameAncestors = "'self' " + origins.join(' ');
                    res.setHeader('Content-Security-Policy', 
                        "default-src 'self'; " +
                        "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; " +
                        "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
                        "font-src 'self' data: https://cdn.jsdelivr.net; " +
                        "img-src 'self' data: blob:; " +
                        "connect-src 'self' ws: wss:; " +
                        `frame-ancestors ${frameAncestors};`
                    );
                } else {
                    // No valid origins specified, fall back to SAMEORIGIN
                    res.setHeader('X-Frame-Options', 'SAMEORIGIN');
                    res.setHeader('Content-Security-Policy', 
                        "default-src 'self'; " +
                        "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; " +
                        "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
                        "font-src 'self' data: https://cdn.jsdelivr.net; " +
                        "img-src 'self' data: blob:; " +
                        "connect-src 'self' ws: wss:; " +
                        "frame-ancestors 'self';"
                    );
                }
            } else {
                // Allow embedding but only from same origin
                res.setHeader('X-Frame-Options', 'SAMEORIGIN');
                res.setHeader('Content-Security-Policy', 
                    "default-src 'self'; " +
                    "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; " +
                    "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
                    "font-src 'self' data: https://cdn.jsdelivr.net; " +
                    "img-src 'self' data: blob:; " +
                    "connect-src 'self' ws: wss:; " +
                    "frame-ancestors 'self';"
                );
            }
        } else {
            // Default: Prevent all embedding (secure by default)
            res.setHeader('X-Frame-Options', 'DENY');
            res.setHeader('Content-Security-Policy', 
                "default-src 'self'; " +
                "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; " +
                "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
                "font-src 'self' data: https://cdn.jsdelivr.net; " +
                "img-src 'self' data: blob:; " +
                "connect-src 'self' ws: wss:; " +
                "frame-ancestors 'none';"
            );
        }
        
        // Prevent MIME type sniffing
        res.setHeader('X-Content-Type-Options', 'nosniff');
        
        // Enable browser XSS protection
        res.setHeader('X-XSS-Protection', '1; mode=block');
        
        // Referrer policy for privacy
        res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin');
        
        // Remove X-Powered-By header
        res.removeHeader('X-Powered-By');
        
        next();
    });
    
    // Apply general rate limiting to all routes
    app.use(generalLimiter.middleware());

    // --- API Routes ---
    const configApi = new (require('./configApi'))();
    configApi.setupRoutes(app);

    const { setupThresholdRoutes } = require('./thresholdRoutes');
    setupThresholdRoutes(app);


    const updateRoutes = require('./routes/updates');
    app.use('/api/updates', apiLimiter.middleware(), updateRoutes);

    const healthRoutes = require('./routes/health');
    app.use('/api/health', healthRoutes); // No rate limit on health checks



    const alertRoutes = require('./routes/alerts');
    app.use('/api/alerts', apiLimiter.middleware(), alertRoutes);

    const serviceRoutes = require('./routes/service');
    app.use('/api/service', apiLimiter.middleware(), serviceRoutes);

    const snapshotsRoutes = require('./routes/snapshots');
    app.use('/api', apiLimiter.middleware(), snapshotsRoutes);

    const backupsRoutes = require('./routes/backups');
    app.use('/api', apiLimiter.middleware(), backupsRoutes);

    const pushRoutes = require('./routes/push');
    app.use('/api/push', strictLimiter.middleware(), pushRoutes); // Stricter limit for push endpoints

    // --- HTML Routes ---
    app.get('/', (req, res) => {
        const indexPath = path.join(publicDir, 'index.html');
        res.sendFile(indexPath, (err) => {
            if (err) {
                logger.error(`Error sending index.html: ${err.message}`);
                res.status(err.status || 500).send('Internal Server Error loading page.');
            }
        });
    });


    return { app, server };
}

module.exports = { createServer };
