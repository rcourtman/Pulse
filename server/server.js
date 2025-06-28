
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
    app.use(cors());
    app.use(express.json());

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

    const debugRoutes = require('./routes/debug');
    app.use('/api/debug', debugRoutes);

    const alertRoutes = require('./routes/alerts');
    app.use('/api/alerts', alertRoutes);

    const snapshotsRoutes = require('./routes/snapshots');
    app.use('/api', snapshotsRoutes);

    const backupsRoutes = require('./routes/backups');
    app.use('/api', backupsRoutes);

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
