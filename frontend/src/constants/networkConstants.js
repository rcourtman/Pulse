import { keyframes } from '@mui/material';

// Define pulse animation for background
export const pulseAnimation = keyframes`
  0% {
    transform: scale(1);
    opacity: 0.9;
  }
  50% {
    transform: scale(1.02);
    opacity: 1;
  }
  100% {
    transform: scale(1);
    opacity: 0.9;
  }
`;

// Define logo pulse animation - more subtle and elegant for the logo
export const logoPulseAnimation = keyframes`
  0% {
    opacity: 0.8;
    transform: scale(0.98);
    box-shadow: 0 0 0 0 rgba(255, 255, 255, 0.4);
  }
  50% {
    opacity: 1;
    transform: scale(1.02);
    box-shadow: 0 0 6px 1px rgba(255, 255, 255, 0.3);
  }
  100% {
    opacity: 0.8;
    transform: scale(0.98);
    box-shadow: 0 0 0 0 rgba(255, 255, 255, 0.4);
  }
`;

// Define center dot glow animation
export const centerGlowAnimation = keyframes`
  0% {
    box-shadow: 0 0 4px 0px rgba(255, 255, 255, 0.5);
  }
  50% {
    box-shadow: 0 0 8px 2px rgba(255, 255, 255, 0.7);
  }
  100% {
    box-shadow: 0 0 4px 0px rgba(255, 255, 255, 0.5);
  }
`;

// Add fade-in animation
export const fadeIn = keyframes`
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
`;

// Add localStorage keys as constants
export const STORAGE_KEY_FILTERS = 'network_display_filters';
export const STORAGE_KEY_SORT = 'network_display_sort';
export const STORAGE_KEY_SHOW_STOPPED = 'network_display_show_stopped';
export const STORAGE_KEY_SHOW_FILTERS = 'network_display_show_filters';
export const STORAGE_KEY_SEARCH_TERMS = 'network_display_search_terms';
export const STORAGE_KEY_COLUMN_VISIBILITY = 'network_display_column_visibility';
export const STORAGE_KEY_COLUMN_ORDER = 'network_display_column_order';
export const STORAGE_KEY_COLUMN_DRAG_ENABLED = 'network_display_column_drag_enabled';
export const STORAGE_KEY_GUEST_TYPE_FILTER = 'network_display_guest_type_filter';

// Define default column configuration
export const DEFAULT_COLUMN_CONFIG = {
  name: { id: 'name', label: 'Name', visible: true },
  status: { id: 'status', label: 'Status', visible: true },
  node: { id: 'node', label: 'Node', visible: true },
  role: { id: 'role', label: 'HA Status', visible: true },
  type: { id: 'type', label: 'Type', visible: true },
  id: { id: 'id', label: 'ID', visible: true },
  cpu: { id: 'cpu', label: 'CPU', visible: true },
  memory: { id: 'memory', label: 'Memory', visible: true },
  disk: { id: 'disk', label: 'Disk', visible: true },
  download: { id: 'download', label: 'Download', visible: true },
  upload: { id: 'upload', label: 'Upload', visible: true },
  uptime: { id: 'uptime', label: 'Uptime', visible: true }
}; 