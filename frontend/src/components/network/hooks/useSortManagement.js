import { useState, useCallback, useEffect } from 'react';
import { STORAGE_KEY_SORT } from '../../../constants/networkConstants';

const useSortManagement = () => {
  // Sort state
  const [sortConfig, setSortConfig] = useState(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY_SORT);
      return saved ? JSON.parse(saved) : { key: "node", direction: 'asc' };
    } catch (e) {
      console.error('Error loading sort preferences:', e);
      return { key: "node", direction: 'asc' };
    }
  });

  // Request sort by key
  const requestSort = useCallback((key, forcedDirection) => {
    console.log(`Sorting by ${key}${forcedDirection ? ' with forced direction: ' + forcedDirection : ''}`);
    
    setSortConfig(prev => {
      // If a forced direction is provided, use that
      if (forcedDirection) {
        const newConfig = {
          key,
          direction: forcedDirection
        };
        console.log(`Setting sort: ${key} → ${forcedDirection} (forced)`);
        return newConfig;
      }
      
      // Otherwise toggle direction if same key, or default to ascending for new key
      const newDirection = prev.key === key && prev.direction === 'asc' ? 'desc' : 'asc';
      
      const newConfig = {
        key,
        direction: newDirection
      };
      
      console.log(`Setting sort: ${key} → ${newDirection} (toggled from ${prev.key === key ? prev.direction : 'new column'})`);
      return newConfig;
    });
  }, []);

  // Log sort config changes for debugging
  useEffect(() => {
    console.log('Current sort config:', sortConfig);
  }, [sortConfig]);

  // Save sort preferences whenever they change
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_SORT, JSON.stringify(sortConfig));
  }, [sortConfig]);

  return {
    sortConfig,
    setSortConfig,
    requestSort
  };
};

export default useSortManagement; 