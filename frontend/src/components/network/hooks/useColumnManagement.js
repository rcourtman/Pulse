import { useState, useCallback, useEffect } from 'react';
import {
  STORAGE_KEY_COLUMN_VISIBILITY,
  STORAGE_KEY_COLUMN_ORDER,
  DEFAULT_COLUMN_CONFIG
} from '../../../constants/networkConstants';

const useColumnManagement = (showNotification) => {
  // Initialize column visibility state from localStorage or defaults
  const [columnVisibility, setColumnVisibility] = useState(() => {
    try {
      // Always enable cluster detection for column visibility
      localStorage.setItem('CLUSTER_DETECTED', 'true');
      console.log('Forcing HA Status column to be visible by default');
      
      // Get column visibility from localStorage
      const storedColumnVisibility = localStorage.getItem(STORAGE_KEY_COLUMN_VISIBILITY);
      
      let config;
      
      // If localStorage has data, use it
      if (storedColumnVisibility) {
        config = JSON.parse(storedColumnVisibility);
      } else {
        // Use the defaults as defined in DEFAULT_COLUMN_CONFIG
        config = JSON.parse(JSON.stringify(DEFAULT_COLUMN_CONFIG));
        
        // Ensure the 'role' column (HA Status) is always visible by default
        if (config.role) {
          config.role.visible = true;
        }
      }
      
      return config;
    } catch (error) {
      console.error('Error initializing column visibility:', error);
      return DEFAULT_COLUMN_CONFIG;
    }
  });
  
  // Initialize column order from localStorage or defaults
  const [columnOrder, setColumnOrder] = useState(() => {
    try {
      const storedOrder = localStorage.getItem(STORAGE_KEY_COLUMN_ORDER);
      if (storedOrder) {
        return JSON.parse(storedOrder);
      } else {
        return Object.keys(DEFAULT_COLUMN_CONFIG);
      }
    } catch (error) {
      console.error('Error initializing column order:', error);
      return Object.keys(DEFAULT_COLUMN_CONFIG);
    }
  });
  
  // Column menu state for the dropdown
  const [columnMenuAnchorEl, setColumnMenuAnchorEl] = useState(null);
  const openColumnMenu = Boolean(columnMenuAnchorEl);
  
  // Force update counter for triggering re-renders
  const [forceUpdateCounter, setForceUpdateCounter] = useState(0);
  
  // Handle column menu open
  const handleColumnMenuOpen = useCallback((event) => {
    setColumnMenuAnchorEl(event.currentTarget);
  }, []);
  
  // Handle column menu close
  const handleColumnMenuClose = useCallback(() => {
    setColumnMenuAnchorEl(null);
  }, []);
  
  // Force update function
  const forceUpdate = useCallback(() => {
    setForceUpdateCounter(c => c + 1);
  }, []);
  
  // Toggle column visibility
  const toggleColumnVisibility = useCallback((columnId) => {
    setColumnVisibility(prev => {
      const newState = JSON.parse(JSON.stringify(prev));
      
      // Verify column exists
      if (!newState[columnId]) {
        console.warn(`Attempted to toggle non-existent column: ${columnId}`);
        return prev;
      }
      
      // Don't allow hiding the last visible column
      if (newState[columnId].visible && Object.values(newState).filter(col => col.visible).length <= 1) {
        if (showNotification) {
          showNotification('At least one column must remain visible');
        }
        return prev;
      }
      
      // Toggle visibility
      newState[columnId].visible = !newState[columnId].visible;
      
      // If this is the role column, log it specially
      if (columnId === 'role') {
        console.log(`User manually set HA Status column to ${newState[columnId].visible ? 'visible' : 'hidden'}`);
      }
      
      // Save to localStorage
      localStorage.setItem(STORAGE_KEY_COLUMN_VISIBILITY, JSON.stringify(newState));
      
      return newState;
    });
  }, [showNotification]);
  
  // Reset column visibility to defaults
  const resetColumnVisibility = useCallback(() => {
    try {
      // Make a copy of the default config
      const defaultConfig = JSON.parse(JSON.stringify(DEFAULT_COLUMN_CONFIG));
      
      // Ensure HA Status (role) column is visible
      if (defaultConfig.role) {
        defaultConfig.role.visible = true;
      }
      
      // Reset column order to the default order as well
      const defaultOrder = Object.keys(DEFAULT_COLUMN_CONFIG);
      
      // Update state and save to localStorage
      setColumnVisibility(defaultConfig);
      setColumnOrder(defaultOrder);
      
      // Always enable cluster detection for HA Status column
      localStorage.setItem('CLUSTER_DETECTED', 'true');
      
      // Save both to localStorage
      localStorage.setItem(STORAGE_KEY_COLUMN_VISIBILITY, JSON.stringify(defaultConfig));
      localStorage.setItem(STORAGE_KEY_COLUMN_ORDER, JSON.stringify(defaultOrder));
      
      // Force UI update
      setForceUpdateCounter(c => c + 1);
      
      // Close the menu if it's open
      setColumnMenuAnchorEl(null);
      
      // Show a notification
      if (showNotification) {
        showNotification('Column visibility reset to defaults');
      }
      
      console.log('Column visibility and order reset to defaults');
    } catch (error) {
      console.error('Error resetting column visibility:', error);
      
      if (showNotification) {
        showNotification('Error resetting column visibility');
      }
    }
  }, [showNotification, setColumnMenuAnchorEl]);
  
  // Update role column visibility
  const updateRoleColumnVisibility = useCallback((shouldShow) => {
    // Log what we're doing
    console.log('updateRoleColumnVisibility called with:', {
      shouldShow
    });
    
    // Save the cluster detection state to localStorage for persistence
    localStorage.setItem('CLUSTER_DETECTED', shouldShow ? 'true' : 'false');
    
    // Only update column visibility for new users or on explicit calls
    // This allows the system to set default visibility but lets users override
    const hasExistingVisibilitySettings = localStorage.getItem(STORAGE_KEY_COLUMN_VISIBILITY);
    
    // Only force column visibility if there are no existing user settings
    if (!hasExistingVisibilitySettings) {
      setColumnVisibility(prev => {
        if (!prev.role) return prev;
        
        const newState = {
          ...prev,
          role: { ...prev.role, visible: shouldShow }
        };
        
        // Save to localStorage
        localStorage.setItem(STORAGE_KEY_COLUMN_VISIBILITY, JSON.stringify(newState));
        console.log(`Setting initial HA Status column visibility to ${shouldShow}`);
        
        return newState;
      });
      
      // Force UI update
      setForceUpdateCounter(c => c + 1);
    }
  }, []);
  
  // Save column visibility preferences whenever they change
  useEffect(() => {
    try {
      // Check if columnVisibility is valid
      if (!columnVisibility || Object.keys(columnVisibility).length === 0) {
        console.error('Invalid column visibility state, resetting to defaults');
        setColumnVisibility(JSON.parse(JSON.stringify(DEFAULT_COLUMN_CONFIG)));
        return;
      }
      
      // Ensure at least one column is visible
      const hasVisibleColumn = Object.values(columnVisibility).some(col => col.visible);
      if (!hasVisibleColumn) {
        console.error('No visible columns in state, not saving to localStorage');
        return;
      }
      
      // Save to localStorage
      localStorage.setItem(STORAGE_KEY_COLUMN_VISIBILITY, JSON.stringify(columnVisibility));
    } catch (error) {
      console.error('Error saving column visibility:', error);
    }
  }, [columnVisibility]);
  
  // Save column order preferences whenever they change
  useEffect(() => {
    try {
      // Check if columnOrder is valid
      if (!columnOrder || !Array.isArray(columnOrder) || columnOrder.length === 0) {
        console.error('Invalid column order state, resetting to defaults');
        setColumnOrder(Object.keys(DEFAULT_COLUMN_CONFIG));
        return;
      }
      
      // Save to localStorage
      localStorage.setItem(STORAGE_KEY_COLUMN_ORDER, JSON.stringify(columnOrder));
    } catch (error) {
      console.error('Error saving column order:', error);
    }
  }, [columnOrder]);

  return {
    columnVisibility,
    setColumnVisibility,
    columnOrder,
    setColumnOrder,
    columnMenuAnchorEl,
    setColumnMenuAnchorEl,
    openColumnMenu,
    handleColumnMenuOpen,
    handleColumnMenuClose,
    forceUpdateCounter,
    forceUpdate,
    toggleColumnVisibility,
    resetColumnVisibility,
    updateRoleColumnVisibility
  };
};

export default useColumnManagement; 