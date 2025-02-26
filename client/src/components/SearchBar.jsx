import React from 'react';
import { FaSearch } from 'react-icons/fa';

const SearchBar = ({ searchTerm, onSearchChange }) => {
  return (
    <div className="search-bar">
      <div className="search-input-container">
        <FaSearch className="search-icon" />
        <input
          type="text"
          placeholder="Search resources..."
          value={searchTerm}
          onChange={(e) => onSearchChange(e.target.value)}
          className="search-input dark:bg-gray-800 dark:border-gray-600 dark:text-white"
        />
        {searchTerm && (
          <button 
            className="clear-search" 
            onClick={() => onSearchChange('')}
            aria-label="Clear search"
          >
            ×
          </button>
        )}
      </div>
    </div>
  );
};

export default SearchBar; 