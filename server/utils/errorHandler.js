/**
 * Centralized error handling utilities to reduce duplication
 */

const { createLogger } = require('./logger');

/**
 * Standard API error response handler
 */
function sendErrorResponse(res, error, context = '') {
    const logger = createLogger(context);
    
    // Log the error
    logger.error('Request failed:', error);
    
    // Determine status code
    let statusCode = 500;
    let errorMessage = 'Internal server error';
    let errorDetails = {};
    
    if (error.response) {
        // External API error
        statusCode = error.response.status || 500;
        errorMessage = error.response.statusText || errorMessage;
        errorDetails = error.response.data || {};
    } else if (error.statusCode) {
        // Custom error with status code
        statusCode = error.statusCode;
        errorMessage = error.message || errorMessage;
    } else if (error.name === 'ValidationError') {
        statusCode = 400;
        errorMessage = 'Validation error';
        errorDetails = { errors: error.errors };
    }
    
    // Send response
    res.status(statusCode).json({
        error: errorMessage,
        message: error.message,
        details: errorDetails,
        timestamp: new Date().toISOString()
    });
}

/**
 * Async route wrapper to handle errors consistently
 */
function asyncHandler(fn) {
    return (req, res, next) => {
        Promise.resolve(fn(req, res, next)).catch(next);
    };
}

/**
 * Try-catch wrapper with logging
 */
async function tryCatchWithLogging(fn, context = '', fallbackValue = null) {
    const logger = createLogger(context);
    
    try {
        return await fn();
    } catch (error) {
        logger.error('Operation failed:', error);
        return fallbackValue;
    }
}

/**
 * Standard error types
 */
class ValidationError extends Error {
    constructor(message, errors = {}) {
        super(message);
        this.name = 'ValidationError';
        this.statusCode = 400;
        this.errors = errors;
    }
}

class NotFoundError extends Error {
    constructor(message = 'Resource not found') {
        super(message);
        this.name = 'NotFoundError';
        this.statusCode = 404;
    }
}

class UnauthorizedError extends Error {
    constructor(message = 'Unauthorized') {
        super(message);
        this.name = 'UnauthorizedError';
        this.statusCode = 401;
    }
}

module.exports = {
    sendErrorResponse,
    asyncHandler,
    tryCatchWithLogging,
    ValidationError,
    NotFoundError,
    UnauthorizedError
};