const express = require('express');
const { execAsync } = require('child_process').promises || { 
    execAsync: require('util').promisify(require('child_process').exec) 
};
const { audit } = require('../security');

const router = express.Router();

// Restart service endpoint (for config changes that require restart)
router.post('/restart', async (req, res) => {
    try {
        // Audit log
        const reason = req.body?.reason || 'Configuration change';
        if (req.auth?.user) {
            audit.serviceRestarted(reason, req.auth.user, req);
        }
        
        // Send immediate response
        res.json({ 
            message: 'Service restart initiated. Pulse will be back online shortly.',
            status: 'restarting'
        });

        // Schedule restart after response is sent
        setTimeout(async () => {
            console.log('[Service] Initiating restart for configuration changes...');
            
            try {
                // Try multiple restart strategies in order
                
                // Strategy 1: pkexec (requires polkit rule)
                try {
                    await execAsync('pkexec systemctl restart pulse.service', { timeout: 10000 });
                    console.log('[Service] Restart initiated via pkexec');
                    return;
                } catch (e) {
                    console.log('[Service] pkexec restart failed:', e.message);
                }

                // Strategy 2: Direct systemctl
                try {
                    await execAsync('systemctl restart pulse.service', { timeout: 10000 });
                    console.log('[Service] Restart initiated via systemctl');
                    return;
                } catch (e) {
                    console.log('[Service] Direct systemctl restart failed:', e.message);
                }

                // Strategy 3: Graceful shutdown (rely on systemd auto-restart)
                console.log('[Service] Attempting graceful shutdown for systemd auto-restart...');
                
                // Just exit and let systemd restart us
                // The service file has Restart=always
                process.exit(0);
                
            } catch (error) {
                console.error('[Service] Error during restart:', error);
                // Last resort - just exit and let systemd restart us
                process.exit(1);
            }
        }, 1000); // 1 second delay to ensure response is sent

    } catch (error) {
        console.error('[Service] Error initiating restart:', error);
        res.status(500).json({ error: error.message });
    }
});

module.exports = router;