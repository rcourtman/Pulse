
const express = require('express');
const UpdateManager = require('../updateManager');

const router = express.Router();
const updateManager = new UpdateManager();

// Get the socket.io instance
const getIO = () => {
    const { getIO: getSocketIO } = require('../socket');
    return getSocketIO();
};

// Check for updates endpoint
router.get('/check', async (req, res) => {
    try {
        // Check if we're in test mode
        if (process.env.UPDATE_TEST_MODE === 'true') {
            const testVersion = process.env.UPDATE_TEST_VERSION || '99.99.99';
            const currentVersion = updateManager.currentVersion;
            
            // Create mock update data
            const mockUpdateInfo = {
                currentVersion: currentVersion,
                latestVersion: testVersion,
                updateAvailable: true,
                isDocker: updateManager.isDockerEnvironment(),
                releaseNotes: 'Test release for update mechanism testing\n\n- Testing download functionality\n- Testing backup process\n- Testing installation process',
                releaseUrl: 'https://github.com/rcourtman/Pulse/releases/test',
                publishedAt: new Date().toISOString(),
                assets: [{
                    name: 'pulse-v' + testVersion + '.tar.gz',
                    size: 1024000,
                    downloadUrl: 'http://localhost:3000/api/test/mock-update.tar.gz'
                }]
            };
            
            console.log('[UpdateManager] Test mode enabled, returning mock update info');
            return res.json(mockUpdateInfo);
        }
        
        // Allow override of update channel via query parameter for preview
        const channelOverride = req.query.channel;
        const updateInfo = await updateManager.checkForUpdates(channelOverride);
        res.json(updateInfo);
    } catch (error) {
        console.error('Error checking for updates:', error);
        res.status(500).json({ error: error.message });
    }
});

// Download and apply update endpoint
router.post('/apply', async (req, res) => {
    try {
        const { downloadUrl } = req.body;
        
        if (!downloadUrl) {
            return res.status(400).json({ error: 'Download URL is required' });
        }

        // Send immediate response
        res.json({ 
            message: 'Update started. The application will restart automatically when complete.',
            status: 'in_progress'
        });

        // Apply update in background
        setTimeout(async () => {
            try {
                // Download update
                const updateFile = await updateManager.downloadUpdate(downloadUrl, (progress) => {
                    const io = getIO();
                    if (io) io.emit('updateProgress', progress);
                });

                await updateManager.applyUpdate(updateFile, (progress) => {
                    const io = getIO();
                    if (io) io.emit('updateProgress', progress);
                }, downloadUrl);

                const io = getIO();
                if (io) io.emit('updateComplete', { success: true });
            } catch (error) {
                console.error('Error applying update:', error);
                const io = getIO();
                if (io) io.emit('updateError', { error: error.message });
            }
        }, 100);

    } catch (error) {
        console.error('Error initiating update:', error);
        res.status(500).json({ error: error.message });
    }
});

// Update status endpoint
router.get('/status', (req, res) => {
    try {
        const status = updateManager.getUpdateStatus();
        res.json(status);
    } catch (error) {
        console.error('Error getting update status:', error);
        res.status(500).json({ error: error.message });
    }
});

module.exports = router;
