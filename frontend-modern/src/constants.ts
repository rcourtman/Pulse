// Constants used throughout the application

// Polling and update intervals (in milliseconds)
export const POLLING_INTERVALS = {
  DEFAULT: 5000, // 5 seconds - default polling interval
  RECONNECT_BASE: 1000, // 1 second - base reconnect delay
  RECONNECT_MAX: 30000, // 30 seconds - max reconnect delay
  DATA_FLASH: 1000, // 1 second - data update indicator flash duration
  TOAST_DURATION: 5000, // 5 seconds - default toast notification duration
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
