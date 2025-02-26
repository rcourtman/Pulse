const express = require('express');
const router = express.Router();
const proxmoxController = require('../controllers/proxmoxController');

// Health check route
router.get('/health', (req, res) => {
  res.status(200).json({ status: 'ok' });
});

// Proxmox routes
router.post('/proxmox/validate', proxmoxController.validateConnection);
router.get('/proxmox/resources', proxmoxController.getResources);
router.get('/proxmox/nodes', proxmoxController.getConfiguredNodes);

// New API routes (v2)
router.get('/nodes', proxmoxController.getConfiguredNodes);

module.exports = router; 