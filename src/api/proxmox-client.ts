// This file will be refactored into smaller modules
// Import from the new modules instead

import { ProxmoxClient } from './proxmox';
import './proxmox/cluster-resources'; // Ensure the cluster-resources module is loaded

// Re-export the ProxmoxClient class
export { ProxmoxClient }; 