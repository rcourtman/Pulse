/**
 * Centralized logging utility to replace console.log statements
 * and provide consistent log formatting
 */

const LOG_LEVELS = {
    ERROR: 0,
    WARN: 1,
    INFO: 2,
    DEBUG: 3
};

class Logger {
    constructor(context = '') {
        this.context = context;
        this.level = process.env.LOG_LEVEL ? LOG_LEVELS[process.env.LOG_LEVEL.toUpperCase()] : LOG_LEVELS.INFO;
    }
    
    /**
     * Format log message with timestamp and context
     */
    formatMessage(level, message, ...args) {
        const timestamp = new Date().toISOString();
        const prefix = this.context ? `[${this.context}]` : '';
        return [`[${timestamp}] ${prefix} ${message}`, ...args];
    }
    
    error(message, ...args) {
        if (this.level >= LOG_LEVELS.ERROR) {
            console.error(...this.formatMessage('ERROR', message, ...args));
        }
    }
    
    warn(message, ...args) {
        if (this.level >= LOG_LEVELS.WARN) {
            console.warn(...this.formatMessage('WARN', message, ...args));
        }
    }
    
    info(message, ...args) {
        if (this.level >= LOG_LEVELS.INFO) {
            console.log(...this.formatMessage('INFO', message, ...args));
        }
    }
    
    debug(message, ...args) {
        if (this.level >= LOG_LEVELS.DEBUG) {
            console.log(...this.formatMessage('DEBUG', message, ...args));
        }
    }
    
    /**
     * Log with custom context temporarily
     */
    withContext(tempContext) {
        return {
            error: (msg, ...args) => this.error(`[${tempContext}] ${msg}`, ...args),
            warn: (msg, ...args) => this.warn(`[${tempContext}] ${msg}`, ...args),
            info: (msg, ...args) => this.info(`[${tempContext}] ${msg}`, ...args),
            debug: (msg, ...args) => this.debug(`[${tempContext}] ${msg}`, ...args)
        };
    }
}

/**
 * Create a logger instance for a specific module/context
 */
function createLogger(context) {
    return new Logger(context);
}

/**
 * Global error handler for consistent error logging
 */
function handleError(error, context = '', additionalInfo = {}) {
    const logger = new Logger(context);
    
    if (error.response) {
        // Axios error
        logger.error('API Error:', {
            status: error.response.status,
            statusText: error.response.statusText,
            data: error.response.data,
            ...additionalInfo
        });
    } else if (error.request) {
        // Network error
        logger.error('Network Error:', {
            message: error.message,
            ...additionalInfo
        });
    } else {
        // General error
        logger.error('Error:', {
            message: error.message,
            stack: error.stack,
            ...additionalInfo
        });
    }
    
    return {
        message: error.message || 'An error occurred',
        details: error.response?.data || error.message
    };
}

module.exports = {
    Logger,
    createLogger,
    handleError,
    LOG_LEVELS
};