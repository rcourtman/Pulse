// Constants used throughout the application

// Polling and update intervals (in milliseconds)
export const POLLING_INTERVALS = {
  DEFAULT: 5000, // 5 seconds - default polling interval
  RECONNECT_BASE: 1000, // 1 second - base reconnect delay
  RECONNECT_MAX: 30000, // 30 seconds - max reconnect delay
  DATA_FLASH: 1000, // 1 second - data update indicator flash duration
  TOAST_DURATION: 5000, // 5 seconds - default toast notification duration
} as const;

// Display thresholds (percentages)
export const THRESHOLDS = {
  WARNING: 60, // Yellow warning threshold
  CRITICAL: 80, // Orange critical threshold
  DANGER: 90, // Red danger threshold
} as const;

// Network and I/O metrics thresholds (MB/s)
export const IO_THRESHOLDS = {
  LOW: 1,
  MEDIUM: 10,
  HIGH: 50,
  VERY_HIGH: 100,
} as const;

// Animation durations (in milliseconds)
export const ANIMATIONS = {
  TOAST_SLIDE: 300, // Toast slide in/out animation
} as const;

// UI configuration
export const UI = {
  DEBOUNCE_DELAY: 300, // 300ms - input debounce delay
} as const;

// WebSocket configuration
export const WEBSOCKET = {
  PING_INTERVAL: 25000, // 25 seconds - WebSocket ping interval
  MESSAGE_TYPES: {
    INITIAL_STATE: 'initialState',
    RAW_DATA: 'rawData',
    ERROR: 'error',
  } as const,
} as const;

// Storage keys for localStorage
export const STORAGE_KEYS = {
  DARK_MODE: 'darkMode',
  VIEW_MODE: 'viewMode',
  DISPLAY_MODE: 'displayMode',
  SORT_KEY: 'sortKey',
  SORT_DIRECTION: 'sortDirection',
  ALERT_THRESHOLDS: 'alertThresholds',
  ACTIVE_TAB: 'activeTab',
} as const;

// File size units
export const FILE_SIZE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const;

// Log levels for the logger
export const LOG_LEVELS = {
  DEBUG: 0,
  INFO: 1,
  WARN: 2,
  ERROR: 3,
} as const;

export type LogLevel = keyof typeof LOG_LEVELS;
