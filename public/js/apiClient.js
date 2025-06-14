/**
 * API Client - Centralized API communication with consistent error handling
 * Reduces code duplication across API calls
 */
(function() {
    'use strict';

    const apiClient = {
        /**
         * Default options for all requests
         */
        defaultOptions: {
            headers: {
                'Content-Type': 'application/json'
            }
        },

        /**
         * Make an API request with consistent error handling
         * @param {string} url - The API endpoint
         * @param {Object} options - Fetch options
         * @returns {Promise<Object>} The response data
         */
        request: async function(url, options = {}) {
            const finalOptions = {
                ...this.defaultOptions,
                ...options,
                headers: {
                    ...this.defaultOptions.headers,
                    ...(options.headers || {})
                }
            };

            try {
                const response = await fetch(url, finalOptions);
                const data = await response.json();

                if (!response.ok) {
                    throw new ApiError(data.error || `Request failed: ${response.statusText}`, response.status, data);
                }

                return data;
            } catch (error) {
                if (error instanceof ApiError) {
                    throw error;
                }
                
                // Network or parsing error
                throw new ApiError(
                    error.message || 'Network error occurred',
                    0,
                    null
                );
            }
        },

        /**
         * GET request
         * @param {string} url - The API endpoint
         * @param {Object} options - Additional options
         * @returns {Promise<Object>} The response data
         */
        get: function(url, options = {}) {
            return this.request(url, {
                ...options,
                method: 'GET'
            });
        },

        /**
         * POST request
         * @param {string} url - The API endpoint
         * @param {Object} data - The data to send
         * @param {Object} options - Additional options
         * @returns {Promise<Object>} The response data
         */
        post: function(url, data = null, options = {}) {
            const body = data ? JSON.stringify(data) : undefined;
            return this.request(url, {
                ...options,
                method: 'POST',
                body
            });
        },

        /**
         * PUT request
         * @param {string} url - The API endpoint
         * @param {Object} data - The data to send
         * @param {Object} options - Additional options
         * @returns {Promise<Object>} The response data
         */
        put: function(url, data = null, options = {}) {
            const body = data ? JSON.stringify(data) : undefined;
            return this.request(url, {
                ...options,
                method: 'PUT',
                body
            });
        },

        /**
         * DELETE request
         * @param {string} url - The API endpoint
         * @param {Object} options - Additional options
         * @returns {Promise<Object>} The response data
         */
        delete: function(url, options = {}) {
            return this.request(url, {
                ...options,
                method: 'DELETE'
            });
        },

        /**
         * Handle API errors consistently
         * @param {Error} error - The error to handle
         * @param {string} context - Context for error message
         * @param {Function} showMessage - Function to display error messages
         */
        handleError: function(error, context = 'Operation', showMessage = null) {
            const errorMessage = error instanceof ApiError ? error.message : error.message || 'Unknown error occurred';
            const displayMessage = `${context} failed: ${errorMessage}`;
            
            console.error(`[API] ${context} error:`, error);
            
            if (showMessage) {
                showMessage(displayMessage, 'error');
            } else if (window.PulseApp && window.PulseApp.ui && window.PulseApp.ui.toast) {
                window.PulseApp.ui.toast.error(displayMessage);
            }
            
            return displayMessage;
        },

        /**
         * Wrapper for common API patterns with loading states
         * @param {Object} options - Configuration object
         * @returns {Promise<any>} Result of the API call
         */
        withLoading: async function(options) {
            const {
                request,
                button = null,
                loadingText = 'Loading...',
                onSuccess = null,
                onError = null,
                showMessage = null,
                context = 'Operation'
            } = options;

            let originalText = '';
            let originalDisabled = false;

            // Set loading state
            if (button) {
                originalText = button.textContent;
                originalDisabled = button.disabled;
                button.textContent = loadingText;
                button.disabled = true;
            }

            try {
                const result = await request();
                
                if (onSuccess) {
                    await onSuccess(result);
                }
                
                return result;
            } catch (error) {
                this.handleError(error, context, showMessage);
                
                if (onError) {
                    await onError(error);
                }
                
                throw error;
            } finally {
                // Restore button state
                if (button) {
                    button.textContent = originalText;
                    button.disabled = originalDisabled;
                }
            }
        }
    };

    /**
     * Custom error class for API errors
     */
    class ApiError extends Error {
        constructor(message, status, data) {
            super(message);
            this.name = 'ApiError';
            this.status = status;
            this.data = data;
        }
    }

    // Export to global scope
    window.PulseApp = window.PulseApp || {};
    window.PulseApp.apiClient = apiClient;
    window.PulseApp.ApiError = ApiError;
})();