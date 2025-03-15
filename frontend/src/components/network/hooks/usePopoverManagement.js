import { useState, useRef, useCallback } from 'react';

const usePopoverManagement = () => {
  // State for filter menu
  const [filterAnchorEl, setFilterAnchorEl] = useState(null);
  const openFilters = Boolean(filterAnchorEl);
  
  // Add a ref for the filter button
  const filterButtonRef = useRef(null);
  
  // Filter popover handlers
  const handleFilterButtonClick = useCallback((event) => {
    setFilterAnchorEl(event.currentTarget);
  }, []);
  
  const handleCloseFilterPopover = useCallback(() => {
    setFilterAnchorEl(null);
  }, []);

  // Function to close all popovers
  const closeAllPopovers = useCallback(() => {
    setFilterAnchorEl(null);
  }, []);

  return {
    filterAnchorEl,
    openFilters,
    filterButtonRef,
    handleFilterButtonClick,
    handleCloseFilterPopover,
    closeAllPopovers
  };
};

export default usePopoverManagement; 