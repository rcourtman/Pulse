// API Client - Centralized API communication
PulseApp.apiClient = (() => {
    // Get CSRF token from sessionStorage or response header
    function getCsrfToken() {
        return sessionStorage.getItem('csrfToken') || '';
    }
    
    // Update CSRF token from response header
    function updateCsrfToken(response) {
        const newToken = response.headers.get('X-CSRF-Token');
        if (newToken) {
            sessionStorage.setItem('csrfToken', newToken);
        }
    }
    
    async function get(url, options = {}) {
        try {
            const response = await fetch(url, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                    ...options.headers
                },
                ...options
            });
            
            // Update CSRF token if provided
            updateCsrfToken(response);
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            return await response.json();
        } catch (error) {
            console.error(`API GET error for ${url}:`, error);
            throw error;
        }
    }

    async function post(url, data, options = {}) {
        try {
            // Get CSRF token for POST requests
            const csrfToken = getCsrfToken();
            
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken,
                    ...options.headers
                },
                body: JSON.stringify(data),
                ...options
            });
            
            // Update CSRF token if provided
            updateCsrfToken(response);
            
            if (!response.ok) {
                // Try to get error details from response body
                let errorMessage = `HTTP error! status: ${response.status}`;
                try {
                    const errorBody = await response.json();
                    if (errorBody.error) {
                        errorMessage = errorBody.error;
                    } else if (errorBody.details) {
                        errorMessage = errorBody.details;
                    }
                } catch (e) {
                    // Response body wasn't JSON, use default error message
                }
                throw new Error(errorMessage);
            }
            
            return await response.json();
        } catch (error) {
            console.error(`API POST error for ${url}:`, error);
            throw error;
        }
    }

    return {
        get,
        post
    };
})();