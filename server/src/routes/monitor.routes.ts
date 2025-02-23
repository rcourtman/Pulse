import express from 'express';
import { getMonitor } from '../services/monitorService';

const router = express.Router();

router.get('/stats', (req, res) => {
  const monitor = getMonitor();
  res.json(monitor.getStats());
});

export default router; 