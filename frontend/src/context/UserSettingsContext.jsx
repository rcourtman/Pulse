import React, { createContext, useState, useContext, useEffect, useMemo } from 'react';
import { useThemeContext } from './ThemeContext';

// Storage keys
const STORAGE_KEY_COMPACT_MODE = 'app_compact_mode';

// Create the context
export const UserSettingsContext = createContext({
  compactMode: false,
  toggleCompactMode: () => {},
  getTableCellPadding: () => {},
});

// Custom hook to use the user settings context
export const useUserSettings = () => useContext(UserSettingsContext);

// User Settings provider component
export const UserSettingsProvider = ({ children }) => {
  const { darkMode, toggleDarkMode } = useThemeContext();
  
  // Initialize compact mode from localStorage
  const [compactMode, setCompactMode] = useState(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY_COMPACT_MODE);
      return saved === 'true';
    } catch (e) {
      console.error('Error loading compact mode preference:', e);
      return false;
    }
  });

  // Save compact mode preference when it changes
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_COMPACT_MODE, compactMode);
  }, [compactMode]);

  // Toggle compact mode function
  const toggleCompactMode = () => {
    setCompactMode(prev => !prev);
  };
  
  // Helper function to get the table cell padding based on the compact mode
  const getTableCellPadding = (isNarrowColumn = false) => {
    if (isNarrowColumn) {
      return '0px 8px'; // Fixed narrow columns always have minimal padding
    }
    
    if (compactMode) {
      return '4px 8px'; // Very compact
    }
    
    return '8px 8px'; // Default for non-compact mode
  };

  // Context value
  const settingsContextValue = useMemo(() => ({
    compactMode,
    toggleCompactMode,
    getTableCellPadding,
    darkMode,
    toggleDarkMode,
  }), [compactMode, darkMode, toggleDarkMode]);

  return (
    <UserSettingsContext.Provider value={settingsContextValue}>
      {children}
    </UserSettingsContext.Provider>
  );
}; 