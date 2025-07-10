/**
 * Input validation middleware for API endpoints
 * Provides centralized validation and sanitization for all incoming requests
 */

const { createLogger } = require('../utils/logger');
const logger = createLogger('ValidationMiddleware');

/**
 * Validates and sanitizes input based on schema
 */
class ValidationMiddleware {
    /**
     * Validate request body against schema
     */
    static validateBody(schema) {
        return (req, res, next) => {
            try {
                const validated = this.validateSchema(req.body, schema);
                // Replace body with validated data
                req.body = validated;
                next();
            } catch (error) {
                logger.warn(`Invalid request body: ${error.message}`, {
                    path: req.path,
                    body: req.body
                });
                return res.status(400).json({
                    success: false,
                    error: 'Invalid request body',
                    details: error.message
                });
            }
        };
    }

    /**
     * Validate request params against schema
     */
    static validateParams(schema) {
        return (req, res, next) => {
            try {
                const validated = this.validateSchema(req.params, schema);
                req.params = validated;
                next();
            } catch (error) {
                logger.warn(`Invalid request params: ${error.message}`, {
                    path: req.path,
                    params: req.params
                });
                return res.status(400).json({
                    success: false,
                    error: 'Invalid request parameters',
                    details: error.message
                });
            }
        };
    }

    /**
     * Validate request query against schema
     */
    static validateQuery(schema) {
        return (req, res, next) => {
            try {
                const validated = this.validateSchema(req.query || {}, schema);
                // Store validated query params in a new property to avoid modifying read-only req.query
                req.validatedQuery = validated;
                // For backwards compatibility, try to update req.query if possible
                try {
                    Object.keys(validated).forEach(key => {
                        req.query[key] = validated[key];
                    });
                } catch (e) {
                    // If req.query is read-only, just use validatedQuery
                }
                next();
            } catch (error) {
                logger.warn(`Invalid query params: ${error.message}`, {
                    path: req.path,
                    query: req.query
                });
                return res.status(400).json({
                    success: false,
                    error: 'Invalid query parameters',
                    details: error.message
                });
            }
        };
    }

    /**
     * Validate data against schema
     */
    static validateSchema(data, schema) {
        const validated = {};
        const errors = [];

        // Check required fields
        if (schema.required) {
            for (const field of schema.required) {
                if (data[field] === undefined || data[field] === null) {
                    errors.push(`Required field '${field}' is missing`);
                }
            }
        }

        // Validate each field
        for (const [field, rules] of Object.entries(schema.fields || {})) {
            if (data[field] !== undefined) {
                try {
                    validated[field] = this.validateField(data[field], rules, field);
                } catch (error) {
                    errors.push(error.message);
                }
            } else if (rules.default !== undefined) {
                validated[field] = rules.default;
            }
        }

        // Check for unknown fields
        if (schema.strict) {
            const allowedFields = Object.keys(schema.fields || {});
            const unknownFields = Object.keys(data).filter(field => !allowedFields.includes(field));
            if (unknownFields.length > 0) {
                errors.push(`Unknown fields: ${unknownFields.join(', ')}`);
            }
        }

        if (errors.length > 0) {
            throw new Error(errors.join('; '));
        }

        return validated;
    }

    /**
     * Validate individual field
     */
    static validateField(value, rules, fieldName) {
        let validated = value;

        // Type validation
        if (rules.type) {
            switch (rules.type) {
                case 'string':
                    if (typeof value !== 'string') {
                        throw new Error(`Field '${fieldName}' must be a string`);
                    }
                    validated = value.trim();
                    break;
                
                case 'number':
                    if (typeof value === 'string' && value.trim() !== '') {
                        validated = Number(value);
                    }
                    if (typeof validated !== 'number' || isNaN(validated)) {
                        throw new Error(`Field '${fieldName}' must be a number`);
                    }
                    break;
                
                case 'boolean':
                    if (typeof value === 'string') {
                        validated = value === 'true';
                    } else if (typeof value !== 'boolean') {
                        throw new Error(`Field '${fieldName}' must be a boolean`);
                    }
                    break;
                
                case 'array':
                    if (!Array.isArray(value)) {
                        throw new Error(`Field '${fieldName}' must be an array`);
                    }
                    break;
                
                case 'object':
                    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
                        throw new Error(`Field '${fieldName}' must be an object`);
                    }
                    break;
            }
        }

        // String validations
        if (rules.type === 'string') {
            if (rules.minLength && validated.length < rules.minLength) {
                throw new Error(`Field '${fieldName}' must be at least ${rules.minLength} characters`);
            }
            if (rules.maxLength && validated.length > rules.maxLength) {
                throw new Error(`Field '${fieldName}' must be at most ${rules.maxLength} characters`);
            }
            if (rules.pattern && !new RegExp(rules.pattern).test(validated)) {
                throw new Error(`Field '${fieldName}' does not match required pattern`);
            }
            if (rules.enum && !rules.enum.includes(validated)) {
                throw new Error(`Field '${fieldName}' must be one of: ${rules.enum.join(', ')}`);
            }
        }

        // Number validations
        if (rules.type === 'number') {
            if (rules.min !== undefined && validated < rules.min) {
                throw new Error(`Field '${fieldName}' must be at least ${rules.min}`);
            }
            if (rules.max !== undefined && validated > rules.max) {
                throw new Error(`Field '${fieldName}' must be at most ${rules.max}`);
            }
            if (rules.integer && !Number.isInteger(validated)) {
                throw new Error(`Field '${fieldName}' must be an integer`);
            }
        }

        // Array validations
        if (rules.type === 'array') {
            if (rules.minItems && validated.length < rules.minItems) {
                throw new Error(`Field '${fieldName}' must have at least ${rules.minItems} items`);
            }
            if (rules.maxItems && validated.length > rules.maxItems) {
                throw new Error(`Field '${fieldName}' must have at most ${rules.maxItems} items`);
            }
        }

        // Custom validation function
        if (rules.validate) {
            const customError = rules.validate(validated);
            if (customError) {
                throw new Error(customError);
            }
        }

        // Sanitization
        if (rules.sanitize) {
            validated = rules.sanitize(validated);
        }

        return validated;
    }

    /**
     * Common validation schemas
     */
    static schemas = {
        id: {
            type: 'string',
            pattern: '^[a-zA-Z0-9_-]+$',
            minLength: 1,
            maxLength: 100
        },
        
        name: {
            type: 'string',
            minLength: 1,
            maxLength: 255,
            sanitize: (value) => value.trim()
        },
        
        email: {
            type: 'string',
            pattern: '^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$',
            maxLength: 255
        },
        
        url: {
            type: 'string',
            pattern: '^https?://',
            maxLength: 2048
        },
        
        port: {
            type: 'number',
            integer: true,
            min: 1,
            max: 65535
        },
        
        percentage: {
            type: 'number',
            min: 0,
            max: 100
        },
        
        positiveInteger: {
            type: 'number',
            integer: true,
            min: 1
        },
        
        vmid: {
            type: 'number',
            integer: true,
            min: 100,
            max: 999999999
        },
        
        nodeName: {
            type: 'string',
            pattern: '^[a-zA-Z0-9][a-zA-Z0-9-_.]*$',
            maxLength: 255
        }
    };

    /**
     * Sanitize HTML to prevent XSS
     */
    static sanitizeHtml(str) {
        if (typeof str !== 'string') return str;
        return str
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#x27;')
            .replace(/\//g, '&#x2F;');
    }

    /**
     * Validate VMID parameter
     */
    static validateVmid() {
        return this.validateParams({
            fields: {
                vmid: this.schemas.vmid
            },
            required: ['vmid']
        });
    }

    /**
     * Validate node parameter
     */
    static validateNode() {
        return this.validateParams({
            fields: {
                node: this.schemas.nodeName
            },
            required: ['node']
        });
    }

    /**
     * Validate pagination query params
     */
    static validatePagination() {
        return this.validateQuery({
            fields: {
                page: {
                    type: 'number',
                    integer: true,
                    min: 1,
                    default: 1
                },
                limit: {
                    type: 'number',
                    integer: true,
                    min: 1,
                    max: 100,
                    default: 20
                }
            }
        });
    }
}

module.exports = ValidationMiddleware;