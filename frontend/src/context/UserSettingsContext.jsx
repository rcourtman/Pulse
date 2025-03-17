import React, { createContext, useState, useContext, useEffect, useMemo } from 'react';
import { useThemeContext } from './ThemeContext';

// Storage keys
const STORAGE_KEY_COMPACT_MODE = 'app_compact_mode';
const STORAGE_KEY_SHOW_ONLY_RUNNING = 'app_show_only_running';

// Create the context
export const UserSettingsContext = createContext({
  compactMode: true,
  toggleCompactMode: () => {},
  getTableCellPadding: () => {},
  showOnlyRunning: false,
  toggleShowOnlyRunning: () => {},
});

// Custom hook to use the user settings context
export const useUserSettings = () => useContext(UserSettingsContext);

// User Settings provider component
export const UserSettingsProvider = ({ children }) => {
  const { darkMode, toggleDarkMode } = useThemeContext();
  
  // Initialize compact mode - always true (no longer checking localStorage)
  const [compactMode, setCompactMode] = useState(true);

  // Initialize showOnlyRunning state from localStorage
  const [showOnlyRunning, setShowOnlyRunning] = useState(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY_SHOW_ONLY_RUNNING);
      return saved === 'true';
    } catch (e) {
      console.error('Error loading show only running preference:', e);
      return false;
    }
  });

  // Save compact mode preference when it changes - keeping for backward compatibility
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_COMPACT_MODE, 'true');
  }, []);

  // Save showOnlyRunning preference when it changes
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_SHOW_ONLY_RUNNING, showOnlyRunning);
    
    // Dispatch a custom event that NetworkDisplay can listen for
    if (window) {
      const event = new CustomEvent('showOnlyRunningChange', {
        detail: { showOnlyRunning }
      });
      window.dispatchEvent(event);
    }
  }, [showOnlyRunning]);

  // Toggle compact mode function - keeping for backward compatibility
  const toggleCompactMode = () => {
    // No-op since we always want compact mode
    console.debug('Compact mode toggle attempted but compact mode is always enabled');
  };
  
  // Toggle showOnlyRunning function
  const toggleShowOnlyRunning = () => {
    setShowOnlyRunning(prev => !prev);
  };
  
  // Helper function to get the table cell padding based on the compact mode
  const getTableCellPadding = (isNarrowColumn = false) => {
    if (isNarrowColumn) {
      return '0px 8px'; // Fixed narrow columns always have minimal padding
    }
    
    // Always use compact mode padding
    return '4px 8px';
  };

  // Context value
  const settingsContextValue = useMemo(() => ({
    compactMode: true,  // Always true
    toggleCompactMode,
    getTableCellPadding,
    darkMode,
    toggleDarkMode,
    showOnlyRunning,
    toggleShowOnlyRunning,
  }), [darkMode, toggleDarkMode, showOnlyRunning]);

  return (
    <UserSettingsContext.Provider value={settingsContextValue}>
      {children}
    </UserSettingsContext.Provider>
  );
}; 