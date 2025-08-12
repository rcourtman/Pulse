// Simple logger - just console.log with timestamps
const isDev = import.meta.env.DEV;

export const logger = {
  debug: (message: string, data?: unknown) => {
    if (isDev) console.log(`[DEBUG] ${message}`, data || '');
  },
  
  info: (message: string, data?: unknown) => {
    console.log(`[INFO] ${message}`, data || '');
  },
  
  warn: (message: string, data?: unknown) => {
    console.warn(`[WARN] ${message}`, data || '');
  },
  
  error: (message: string, error?: unknown) => {
    console.error(`[ERROR] ${message}`, error || '');
  }
};

export const logError = logger.error;