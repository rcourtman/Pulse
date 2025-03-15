/**
 * Utility functions for managing localStorage data
 * These functions help ensure proper cleanup when switching between environments
 */

// List of all localStorage keys used by the application
const APP_STORAGE_KEYS = [
  // Network display filters
  'network_display_filters',
  'network_display_show_stopped',
  'network_display_show_filters',
  'network_display_search_terms',
  'network_display_guest_type_filter',
  'network_display_column_visibility',
  'network_display_column_order',
  'network_display_sort',
  
  // Theme settings
  'app_dark_mode',
  'app_filter_state',
  
  // Mock data settings
  'use_mock_data',
  'MOCK_DATA_ENABLED',
  'mock_enabled',
  'MOCK_SERVER_URL',
  
  // Session data
  'last_environment',
  'guest_data_cache'
];

/**
 * Clear all application data from localStorage
 * This should be called when switching between environments
 */
export const clearAppData = () => {
  console.log('Clearing all application data from localStorage');
  
  try {
    // Clear each key individually
    APP_STORAGE_KEYS.forEach(key => {
      localStorage.removeItem(key);
    });
    
    // Also clear any other items that might have been added
    // This ensures we don't miss anything
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.startsWith('pulse_') || key.includes('guest') || key.includes('node') || key.includes('mock')) {
        localStorage.removeItem(key);
      }
    }
    
    // Set the current environment
    localStorage.setItem('last_environment', process.env.NODE_ENV || 'development');
    
    console.log('Successfully cleared application data');
    return true;
  } catch (error) {
    console.error('Error clearing application data:', error);
    return false;
  }
};

/**
 * Force clear all data regardless of environment
 * This can be called manually when needed
 */
export const forceClearAllData = () => {
  console.log('Force clearing all application data');
  
  try {
    // Clear all localStorage items
    localStorage.clear();
    
    // Set the current environment
    localStorage.setItem('last_environment', process.env.NODE_ENV || 'development');
    
    console.log('Successfully force cleared all application data');
    return true;
  } catch (error) {
    console.error('Error force clearing application data:', error);
    return false;
  }
};

/**
 * Check if we need to clear data (when switching between environments)
 * Returns true if data was cleared
 */
export const checkAndClearDataIfNeeded = () => {
  try {
    const lastEnvironment = localStorage.getItem('last_environment');
    const currentEnvironment = process.env.NODE_ENV || 'development';
    
    // If the environment has changed, clear the data
    if (lastEnvironment && lastEnvironment !== currentEnvironment) {
      console.log(`Environment changed from ${lastEnvironment} to ${currentEnvironment}, clearing data`);
      return clearAppData();
    }
    
    // If this is the first run (no last environment), set it
    if (!lastEnvironment) {
      localStorage.setItem('last_environment', currentEnvironment);
    }
    
    return false;
  } catch (error) {
    console.error('Error checking environment change:', error);
    return false;
  }
};

/**
 * Clear all mock data settings from localStorage
 * This should be called at application startup to ensure
 * we start with a clean state and respect environment variables
 */
export const clearMockDataSettings = () => {
  console.log('Clearing all mock data settings from localStorage');
  
  try {
    // Remove all mock data related keys
    localStorage.removeItem('use_mock_data');
    localStorage.removeItem('MOCK_DATA_ENABLED');
    localStorage.removeItem('mock_enabled');
    localStorage.removeItem('MOCK_SERVER_URL');
    
    // Also clear any other mock-related items that might have been added
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.includes('mock')) {
        localStorage.removeItem(key);
      }
    }
    
    console.log('Successfully cleared mock data settings');
    return true;
  } catch (error) {
    console.error('Error clearing mock data settings:', error);
    return false;
  }
};

/**
 * Initialize the storage system
 * This should be called when the application starts
 */
export const initializeStorage = () => {
  // First clear any mock data settings to ensure we start fresh
  clearMockDataSettings();
  
  // Then check if we need to clear other data
  const wasCleared = checkAndClearDataIfNeeded();
  
  // Return whether data was cleared
  return wasCleared;
}; 