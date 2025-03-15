import { useState, useEffect, useCallback, useRef } from 'react';
import { useSearchContext } from '../../../context/SearchContext';

const useKeyboardShortcuts = ({
  openFilters,
  openColumnMenu,
  resetFilters,
  closeAllPopovers,
  showNotification
}) => {
  const [escRecentlyPressed, setEscRecentlyPressed] = useState(false);
  
  // Get search functions from context directly
  const { setSearchTerm, setIsSearching, clearSearchTerms, isSearching } = useSearchContext();
  
  // Track last key pressed timestamp to prevent double handling of events
  const lastKeyPressTime = useRef(0);
  
  // Set up keyboard shortcuts
  useEffect(() => {
    // Global keyboard handler
    const handleGlobalKeyDown = (e) => {
      // Critical protection against double event handling
      // This prevents the same key event from being processed twice
      const now = Date.now();
      if (now - lastKeyPressTime.current < 50) {
        return; // Ignore events that are too close together
      }
      lastKeyPressTime.current = now;
      
      // STOP HANDLING KEYBOARD SHORTCUTS IF USER IS TYPING IN A FORM ELEMENT
      // This is the most important check to prevent shortcuts from interfering with typing
      const isEditableElement = 
        e.target.tagName === 'INPUT' || 
        e.target.tagName === 'TEXTAREA' || 
        e.target.isContentEditable ||
        e.target.getAttribute('role') === 'textbox';
      
      // If user is typing in ANY input, don't hijack their keystrokes
      if (isEditableElement) {
        return;
      }
      
      // Escape key to close filters or clear search
      if (e.key === 'Escape') {
        // Close any open popovers
        closeAllPopovers();
        
        // Reset all filters
        resetFilters();
        
        // Clear all search terms
        clearSearchTerms();
        
        // Set flag to prevent other shortcuts from triggering
        setEscRecentlyPressed(true);
        setTimeout(() => setEscRecentlyPressed(false), 300);
        
        // Show notification
        showNotification('All filters have been cleared', 'info');
        return;
      }
      
      // Ctrl+F or Cmd+F to focus search without setting a term
      if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
        e.preventDefault();
        setIsSearching(true);
        return;
      }
      
      // / to focus search without setting a term
      if (e.key === '/' && !escRecentlyPressed) {
        e.preventDefault();
        setIsSearching(true);
        return;
      }
      
      // Enter to focus search without setting a term
      if (e.key === 'Enter' && !escRecentlyPressed) {
        e.preventDefault();
        setIsSearching(true);
        return;
      }
      
      // Respect the search state
      if (isSearching) {
        // If the search field is focused, don't handle keyboard shortcuts
        return;
      }
      
      // Capture single printable characters to focus search AND start typing
      const isPrintableChar = 
        e.key.length === 1 && 
        !e.ctrlKey && 
        !e.metaKey && 
        !e.altKey && 
        !e.key.match(/^F\d+$/); // Exclude function keys
      
      if (isPrintableChar) {
        e.preventDefault();
        
        // Set the first character
        setSearchTerm(e.key);
        
        // Focus the search field 
        setIsSearching(true);
      }
    };
    
    // Use the capture phase to get events before other handlers
    window.addEventListener('keydown', handleGlobalKeyDown);
    
    return () => {
      window.removeEventListener('keydown', handleGlobalKeyDown);
    };
  }, [
    openFilters, 
    openColumnMenu, 
    escRecentlyPressed,
    resetFilters,
    closeAllPopovers,
    setSearchTerm,
    setIsSearching,
    clearSearchTerms,
    showNotification,
    isSearching
  ]);

  return {
    escRecentlyPressed
  };
};

export default useKeyboardShortcuts; 