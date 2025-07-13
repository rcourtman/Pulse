
const express = require('express');
const http = require('http');
const cors = require('cors');
const compression = require('compression');
const path = require('path');
const { createLogger } = require('./utils/logger');
const { createRateLimiter } = require('./middleware/rateLimiter');

function createServer() {
    const app = express();
    const server = http.createServer(app);
    const logger = createLogger('Server');
    
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
                callback(new Error('Not allowed by CORS'));
            }
        },
        credentials: true,
        optionsSuccessStatus: 200
    };
    
    app.use(cors(corsOptions));
    
    // Security headers middleware
    app.use((req, res, next) => {
        // Configurable frame options for embedding support
        const allowEmbedding = process.env.ALLOW_EMBEDDING === 'true';
        const allowedOrigins = process.env.ALLOWED_EMBED_ORIGINS || '';
        
        if (allowEmbedding) {
            if (allowedOrigins) {
                // Parse comma-separated origins and format for CSP
                const origins = allowedOrigins.split(',')
                    .map(origin => origin.trim())
                    .filter(origin => origin.length > 0)
                    .map(origin => {
                        // Ensure origin has protocol
                        if (!origin.startsWith('http://') && !origin.startsWith('https://')) {
                            return `https://${origin}`;
                        }
                        return origin;
                    });
                
                if (origins.length > 0) {
                    // X-Frame-Options doesn't support multiple origins, so we'll rely on CSP
                    // Don't set X-Frame-Options header when using custom origins
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
                    // Fall back to SAMEORIGIN if no origins specified
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
            // Default: Prevent all embedding
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
    
    app.use(express.json({ limit: '10mb' })); // Reduced from 50mb to prevent DoS attacks

    // Static files
    const publicDir = path.join(__dirname, '../src/public');
    app.use(express.static(publicDir, { index: false }));

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

    app.get('/setup.html', (req, res) => {
        const setupPath = path.join(publicDir, 'setup.html');
        res.sendFile(setupPath, (err) => {
            if (err) {
                logger.error(`Error sending setup.html: ${err.message}`);
                res.status(err.status || 500).send('Internal Server Error loading setup page.');
            }
        });
    });

    return { app, server };
}

module.exports = { createServer };
