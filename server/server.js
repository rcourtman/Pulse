
const express = require('express');
const http = require('http');
const cors = require('cors');
const compression = require('compression');
const path = require('path');

function createServer() {
    const app = express();
    const server = http.createServer(app);

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
            
            // Also allow the actual server URL if known
            if (process.env.PULSE_PUBLIC_URL) {
                allowedOrigins.push(process.env.PULSE_PUBLIC_URL);
            }
            
            // Check if origin matches allowed list
            if (allowedOrigins.indexOf(origin) !== -1) {
                callback(null, true);
            } else {
                // Log rejected origins for debugging
                console.warn(`CORS: Rejected origin ${origin}`);
                callback(new Error('Not allowed by CORS'));
            }
        },
        credentials: true,
        optionsSuccessStatus: 200
    };
    
    app.use(cors(corsOptions));
    
    // Security headers middleware
    app.use((req, res, next) => {
        // Prevent clickjacking
        res.setHeader('X-Frame-Options', 'DENY');
        
        // Prevent MIME type sniffing
        res.setHeader('X-Content-Type-Options', 'nosniff');
        
        // Enable browser XSS protection
        res.setHeader('X-XSS-Protection', '1; mode=block');
        
        // Referrer policy for privacy
        res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin');
        
        // Basic CSP - adjust based on your needs
        res.setHeader('Content-Security-Policy', 
            "default-src 'self'; " +
            "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; " +
            "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
            "font-src 'self' data: https://cdn.jsdelivr.net; " +
            "img-src 'self' data: blob:; " +
            "connect-src 'self' ws: wss:; " +
            "frame-ancestors 'none';"
        );
        
        // Remove X-Powered-By header
        res.removeHeader('X-Powered-By');
        
        next();
    });
    
    app.use(express.json({ limit: '10mb' })); // Reduced from 50mb to prevent DoS attacks

    // Static files
    const publicDir = path.join(__dirname, '../src/public');
    app.use(express.static(publicDir, { index: false }));

    // --- API Routes ---
    const configApi = new (require('./configApi'))();
    configApi.setupRoutes(app);

    const { setupThresholdRoutes } = require('./thresholdRoutes');
    setupThresholdRoutes(app);


    const updateRoutes = require('./routes/updates');
    app.use('/api/updates', updateRoutes);

    const healthRoutes = require('./routes/health');
    app.use('/api/health', healthRoutes);


    const alertRoutes = require('./routes/alerts');
    app.use('/api/alerts', alertRoutes);

    const snapshotsRoutes = require('./routes/snapshots');
    app.use('/api', snapshotsRoutes);

    const backupsRoutes = require('./routes/backups');
    app.use('/api', backupsRoutes);

    const pushRoutes = require('./routes/push');
    app.use('/api/push', pushRoutes);

    // --- HTML Routes ---
    app.get('/', (req, res) => {
        const indexPath = path.join(publicDir, 'index.html');
        res.sendFile(indexPath, (err) => {
            if (err) {
                console.error(`Error sending index.html: ${err.message}`);
                res.status(err.status || 500).send('Internal Server Error loading page.');
            }
        });
    });

    app.get('/setup.html', (req, res) => {
        const setupPath = path.join(publicDir, 'setup.html');
        res.sendFile(setupPath, (err) => {
            if (err) {
                console.error(`Error sending setup.html: ${err.message}`);
                res.status(err.status || 500).send('Internal Server Error loading setup page.');
            }
        });
    });

    return { app, server };
}

module.exports = { createServer };
