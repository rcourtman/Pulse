const customThresholdManager = require('./customThresholds');

function setupThresholdRoutes(app) {
    
    // Get all custom thresholds
    app.get('/api/thresholds', async (req, res) => {
        try {
            
            // Initialize the threshold manager if needed
            if (!customThresholdManager.initialized) {
                await customThresholdManager.init();
            }
            
            const thresholds = customThresholdManager.getAllThresholds();
            res.json({ success: true, data: thresholds });
        } catch (error) {
            console.error('[API /api/thresholds] Error:', error);
            res.status(500).json({ 
                success: false, 
                error: error.message || 'Failed to get thresholds' 
            });
        }
    });

    // Get thresholds for specific VM/LXC
    app.get('/api/thresholds/:endpointId/:nodeId/:vmid', async (req, res) => {
        try {
            const { endpointId, nodeId, vmid } = req.params;
            
            if (!customThresholdManager.initialized) {
                await customThresholdManager.init();
            }
            
            const thresholds = customThresholdManager.getThresholds(endpointId, nodeId, vmid);
            
            if (thresholds) {
                res.json({ success: true, data: thresholds });
            } else {
                res.status(404).json({ 
                    success: false, 
                    error: 'No custom thresholds found for this VM/LXC' 
                });
            }
        } catch (error) {
            console.error('[API /api/thresholds/:endpointId/:nodeId/:vmid] Error:', error);
            res.status(500).json({ 
                success: false, 
                error: error.message || 'Failed to get thresholds' 
            });
        }
    });

    // Set custom thresholds for VM/LXC
    app.post('/api/thresholds/:endpointId/:nodeId/:vmid', async (req, res) => {
        try {
            let { endpointId, nodeId, vmid } = req.params;
            const { thresholds } = req.body;
            
            if (!thresholds) {
                return res.status(400).json({ 
                    success: false, 
                    error: 'Thresholds configuration is required' 
                });
            }
            
            // Handle auto-detect node
            if (nodeId === 'auto-detect') {
                const state = require('./state');
                // Find the VM/LXC in the current state
                const allGuests = [...(state.vms || []), ...(state.containers || [])];
                const guest = allGuests.find(g => 
                    g.endpointId === endpointId && g.id === vmid
                );
                
                if (guest && guest.node) {
                    nodeId = guest.node;
                } else {
                    // If we can't find the node, use a wildcard that will match any node
                    nodeId = '*';
                }
            }
            
            if (!customThresholdManager.initialized) {
                await customThresholdManager.init();
            }
            
            await customThresholdManager.setThresholds(endpointId, nodeId, vmid, thresholds);
            res.json({ success: true, message: 'Custom thresholds saved successfully' });
        } catch (error) {
            console.error('[API /api/thresholds/:endpointId/:nodeId/:vmid] Error:', error);
            res.status(400).json({ 
                success: false, 
                error: error.message || 'Failed to save thresholds' 
            });
        }
    });

    // Update existing custom thresholds
    app.put('/api/thresholds/:endpointId/:nodeId/:vmid', async (req, res) => {
        try {
            let { endpointId, nodeId, vmid } = req.params;
            const { thresholds } = req.body;
            
            if (!thresholds) {
                return res.status(400).json({ 
                    success: false, 
                    error: 'Thresholds configuration is required' 
                });
            }
            
            // Handle auto-detect node
            if (nodeId === 'auto-detect') {
                const state = require('./state');
                // Find the VM/LXC in the current state
                const allGuests = [...(state.vms || []), ...(state.containers || [])];
                const guest = allGuests.find(g => 
                    g.endpointId === endpointId && g.id === vmid
                );
                
                if (guest && guest.node) {
                    nodeId = guest.node;
                } else {
                    // If we can't find the node, use a wildcard that will match any node
                    nodeId = '*';
                }
            }
            
            if (!customThresholdManager.initialized) {
                await customThresholdManager.init();
            }
            
            // Check if thresholds exist
            const existing = customThresholdManager.getThresholds(endpointId, nodeId, vmid);
            if (!existing) {
                return res.status(404).json({ 
                    success: false, 
                    error: 'No custom thresholds found for this VM/LXC' 
                });
            }
            
            await customThresholdManager.setThresholds(endpointId, nodeId, vmid, thresholds);
            res.json({ success: true, message: 'Custom thresholds updated successfully' });
        } catch (error) {
            console.error('[API /api/thresholds/:endpointId/:nodeId/:vmid] Error:', error);
            res.status(400).json({ 
                success: false, 
                error: error.message || 'Failed to update thresholds' 
            });
        }
    });

    // Delete custom thresholds for VM/LXC
    app.delete('/api/thresholds/:endpointId/:nodeId/:vmid', async (req, res) => {
        try {
            const { endpointId, nodeId, vmid } = req.params;
            
            if (!customThresholdManager.initialized) {
                await customThresholdManager.init();
            }
            
            const removed = await customThresholdManager.removeThresholds(endpointId, nodeId, vmid);
            
            if (removed) {
                res.json({ success: true, message: 'Custom thresholds removed successfully' });
            } else {
                res.status(404).json({ 
                    success: false, 
                    error: 'No custom thresholds found for this VM/LXC' 
                });
            }
        } catch (error) {
            console.error('[API /api/thresholds/:endpointId/:nodeId/:vmid] Error:', error);
            res.status(500).json({ 
                success: false, 
                error: error.message || 'Failed to remove thresholds' 
            });
        }
    });

    // Toggle custom thresholds enabled/disabled
    app.patch('/api/thresholds/:endpointId/:nodeId/:vmid/toggle', async (req, res) => {
        try {
            const { endpointId, nodeId, vmid } = req.params;
            const { enabled } = req.body;
            
            if (typeof enabled !== 'boolean') {
                return res.status(400).json({ 
                    success: false, 
                    error: 'Enabled flag must be a boolean' 
                });
            }
            
            if (!customThresholdManager.initialized) {
                await customThresholdManager.init();
            }
            
            await customThresholdManager.toggleThresholds(endpointId, nodeId, vmid, enabled);
            res.json({ 
                success: true, 
                message: `Custom thresholds ${enabled ? 'enabled' : 'disabled'} successfully` 
            });
        } catch (error) {
            console.error('[API /api/thresholds/:endpointId/:nodeId/:vmid/toggle] Error:', error);
            res.status(400).json({ 
                success: false, 
                error: error.message || 'Failed to toggle thresholds' 
            });
        }
    });

}

module.exports = { setupThresholdRoutes };