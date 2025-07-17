/**
 * Configuration constants for the Pulse monitoring application
 * This file centralizes all magic numbers and configuration values
 */

// Server Configuration
const SERVER_DEFAULTS = {
    PORT: 7655,
    HOST: '0.0.0.0',
};

// Timeout Configuration (in milliseconds)
const TIMEOUTS = {
    API_REQUEST: 8000,           // Default API request timeout
    API_REQUEST_SHORT: 3000,     // Short API requests (health checks)
    API_REQUEST_LONG: 30000,     // Long API requests (backups, large data)
    GRACEFUL_SHUTDOWN: 5000,     // Time to wait for graceful shutdown
    SOCKET_HANDSHAKE: 10000,     // Socket.io handshake timeout
    ENV_FILE_RELOAD_DEBOUNCE: 1000, // Debounce for .env file changes
    MIN_RELOAD_INTERVAL: 2000,   // Minimum time between config reloads
};

// Update Intervals (in milliseconds)
const UPDATE_INTERVALS = {
    METRICS: process.env.PULSE_METRIC_INTERVAL_MS ? parseInt(process.env.PULSE_METRIC_INTERVAL_MS, 10) : 2000,
    DISCOVERY: process.env.PULSE_DISCOVERY_INTERVAL_MS ? parseInt(process.env.PULSE_DISCOVERY_INTERVAL_MS, 10) : 30000,
    ALERT_EVALUATION: process.env.PULSE_ALERT_EVALUATION_INTERVAL_MS ? parseInt(process.env.PULSE_ALERT_EVALUATION_INTERVAL_MS, 10) : 15000,     // Alert evaluation interval
    METRICS_PERSISTENCE: {
        INITIAL: 30000,          // First 2 minutes: every 30s
        MEDIUM: 60000,           // 2-5 minutes: every 60s
        LONG: 120000,            // After 5 minutes: every 2 minutes
    },
    WEBHOOK_BATCH: 1000,         // Webhook batching interval
    DNS_REFRESH: 5 * 60 * 1000,  // DNS cache refresh (5 minutes)
};

// Retry Configuration
const RETRY_CONFIG = {
    MAX_RETRIES: 3,
    MAX_RETRIES_EXTENDED: 5,
    EXPONENTIAL_BASE: 2,
    MAX_BACKOFF: 300000,         // Max 5 minutes backoff
};

// Request Limits
const REQUEST_LIMITS = {
    JSON_SIZE: '10mb',           // Express JSON body limit
    RATE_LIMIT_WINDOW: 60000,    // Rate limit window (1 minute)
    RATE_LIMIT_MAX: 300,         // Max requests per window (increased for better UX)
    MAX_CONCURRENT_REQUESTS: 10, // Max concurrent API requests
};

// Data Retention
const DATA_RETENTION = {
    METRICS_HISTORY_DAYS: 7,     // Keep metrics for 7 days
    ALERT_HISTORY_DAYS: 30,      // Keep alert history for 30 days
    WEBHOOK_COOLDOWN_MINUTES: 15, // Webhook cooldown period
    NOTIFICATION_HISTORY_MAX: 1000, // Max notification history entries
};

// Proxmox Configuration
const PROXMOX_CONFIG = {
    PVE_DEFAULT_PORT: 8006,
    PBS_DEFAULT_PORT: 8007,
    SPECIAL_STATUS_CODE: 596,    // Special PVE error status code
    MAX_VMID: 999999999,         // Maximum valid VMID
};

// Cache Configuration
const CACHE_CONFIG = {
    NODE_CONNECTION_TTL: 5 * 60 * 1000, // 5 minutes
    WEB_FETCH_TTL: 15 * 60 * 1000,      // 15 minutes
    DNS_CACHE_TTL: 5 * 60 * 1000,       // 5 minutes
};

// Alert Configuration
const ALERT_CONFIG = {
    DEFAULT_THRESHOLDS: {
        CPU: 80,
        MEMORY: 85,
        DISK: 90,
        IOWAIT: 50,
    },
    SEVERITY_LEVELS: {
        CRITICAL: 'critical',
        WARNING: 'warning',
        INFO: 'info',
    },
    NOTIFICATION_CHANNELS: {
        DASHBOARD: 'dashboard',
        EMAIL: 'email',
        WEBHOOK: 'webhook',
    },
};

// Performance Thresholds
const PERFORMANCE_THRESHOLDS = {
    MAX_DISCOVERY_TIME: 120000,  // 2 minutes max for discovery
    MAX_METRICS_TIME: 30000,     // 30 seconds max for metrics
    SLOW_RESPONSE_MS: 1000,      // Log slow responses over 1 second
};

// File System Configuration
const FILE_CONFIG = {
    LOCK_STALE_TIME: 30000,      // Consider lock stale after 30 seconds
    LOCK_RETRY_DELAY: 100,       // Retry lock acquisition every 100ms
    LOCK_MAX_RETRIES: 50,        // Max retries for lock acquisition
};

// UI Configuration
const UI_CONFIG = {
    TOAST_DURATION: 5000,        // Toast notification duration
    DEBOUNCE_DELAY: 300,         // General debounce delay
    CHART_DATA_POINTS: 720,      // Max chart data points (12 hours at 1 min intervals)
    SKELETON_ANIMATION_DELAY: 200, // Loading skeleton animation delay
};

module.exports = {
    SERVER_DEFAULTS,
    TIMEOUTS,
    UPDATE_INTERVALS,
    RETRY_CONFIG,
    REQUEST_LIMITS,
    DATA_RETENTION,
    PROXMOX_CONFIG,
    CACHE_CONFIG,
    ALERT_CONFIG,
    PERFORMANCE_THRESHOLDS,
    FILE_CONFIG,
    UI_CONFIG,
};