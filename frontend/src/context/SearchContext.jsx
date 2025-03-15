import React, { createContext, useState, useContext, useEffect, useCallback } from 'react';
import { STORAGE_KEY_SEARCH_TERMS } from '../constants/networkConstants';

// Create the context
export const SearchContext = createContext({
  searchTerm: '',
  setSearchTerm: () => {},
  activeSearchTerms: [],
  addSearchTerm: () => {},
  removeSearchTerm: () => {},
  clearSearchTerms: () => {},
  isSearching: false,
  setIsSearching: () => {},
  handleSpecialSearchTerm: () => {},
});

// Custom hook to use the search context
export const useSearchContext = () => useContext(SearchContext);

// Search provider component
export const SearchProvider = ({ children }) => {
  // Search state
  const [searchTerm, setSearchTermInternal] = useState('');
  const [activeSearchTerms, setActiveSearchTerms] = useState(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY_SEARCH_TERMS);
      return saved ? JSON.parse(saved) : [];
    } catch (e) {
      console.error('Error loading active search terms:', e);
      return [];
    }
  });
  const [isSearching, setIsSearching] = useState(false);

  // Wrap setSearchTerm to add logging for debugging
  const setSearchTerm = useCallback((value) => {
    console.log('SearchContext: setSearchTerm called with value:', value);
    // Make sure we always have a string, even if undefined is passed
    const safeValue = value === undefined ? '' : value;
    setSearchTermInternal(safeValue);
  }, []);

  // Save active search terms whenever they change
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_SEARCH_TERMS, JSON.stringify(activeSearchTerms));
  }, [activeSearchTerms]);

  // Function to handle special search terms (status:, type:, etc.)
  const handleSpecialSearchTerm = useCallback((term, action = 'add') => {
    const termLower = term.toLowerCase().trim();
    const isAdding = action === 'add';
    
    // Create a custom event to dispatch
    const event = new CustomEvent('searchTermAction', {
      detail: {
        term: termLower,
        action: isAdding ? 'add' : 'remove'
      }
    });
    
    // Dispatch the event for other components to listen to
    window.dispatchEvent(event);
  }, []);

  // Function to add a search term
  const addSearchTerm = useCallback((term) => {
    if (!activeSearchTerms.includes(term) && term.trim()) {
      setActiveSearchTerms(prev => [...prev, term]);
      
      // Handle special search terms
      handleSpecialSearchTerm(term, 'add');
    }
  }, [activeSearchTerms, handleSpecialSearchTerm]);

  // Function to remove a search term
  const removeSearchTerm = useCallback((term) => {
    setActiveSearchTerms(prev => prev.filter(t => t !== term));
    
    // Handle special search terms
    handleSpecialSearchTerm(term, 'remove');
  }, [handleSpecialSearchTerm]);

  // Function to clear all search terms
  const clearSearchTerms = useCallback(() => {
    // Handle special search terms for each active term
    activeSearchTerms.forEach(term => {
      handleSpecialSearchTerm(term, 'remove');
    });
    
    setActiveSearchTerms([]);
    // Use the wrapped setSearchTerm
    setSearchTerm('');
  }, [activeSearchTerms, handleSpecialSearchTerm, setSearchTerm]);

  // Context value
  const searchContextValue = {
    searchTerm,
    setSearchTerm,
    activeSearchTerms,
    addSearchTerm,
    removeSearchTerm,
    clearSearchTerms,
    isSearching,
    setIsSearching,
    handleSpecialSearchTerm,
  };

  return (
    <SearchContext.Provider value={searchContextValue}>
      {children}
    </SearchContext.Provider>
  );
}; 